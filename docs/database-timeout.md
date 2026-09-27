# Database Query Timeout Architecture

This document defines the architectural guidelines, mental models, and implementation patterns for applying database query timeouts to repository operations in Go backend services using PostgreSQL.

---

## 1. Core Mental Model

> **Core mental của Database Timeout**:
> *Mỗi operation chỉ nên được phép chạy trong khoảng thời gian mà caller còn thấy nó có giá trị.*

```text
Caller budget
    ↓
derive shorter context
    ↓
DB operation
    ↓
finish in time?
├─ yes → return
└─ no  → cancel
```

### Hai nguyên tắc vàng đi kèm:

1. **Timeout follows the use case**:
   ```text
   GetByID (~500ms) < Write (~1s) < Feed / Complex query (~2s)
   ```
2. **Context deadline only gets tighter downward**:
   ```text
   Never replace parent context with Background()
   ```

> **Đúc kết ngắn nhất**:
> - **Context propagates lifetime.**
> - **Timeout defines the budget.**
>
> *(500ms, 1s, 2s chỉ là implementation choice; mental cốt lõi thực sự là **bounded work** + **deadline propagation**).*

---

## 2. Context Derivation & Inheritance

### 2.1. Always Derive from Incoming Context

Always create timeouts by deriving from the context passed down from the caller:

```go
ctx, cancel := context.WithTimeout(ctx, timeout)
defer cancel()
```

### 2.2. Never Sever the Chain with `context.Background()`

> [!CAUTION]
> **Never use `context.Background()` or `context.TODO()` inside Repository methods.**
> Using `context.Background()` breaks the request lifecycle:
> 1. If the client closes their browser, the HTTP cancellation signal is lost.
> 2. The database query keeps executing as an orphaned process, causing connection pool exhaustion and resource starvation.

### 2.3. Parent Deadlines Are Never Extended

Go's `context.WithTimeout` automatically respects whichever deadline expires first:

```text
Parent Request Context (e.g. Gateway sets 300ms)
       │
       ▼
Repository.Create(ctx) ── derives 1s timeout
       │
       ▼
Effective Deadline: 300ms (earlier deadline wins!)
```

A derived context can only **shorten** or maintain a deadline; it can never extend a parent deadline.

### 2.4. Always `defer cancel()`

When an operation completes before the timeout, calling `cancel()` immediately releases the internal timer resources allocated by the runtime. Omitting `defer cancel()` leads to memory leaks in high-throughput services.

---

## 3. Timeout Budget by Use Case

Timeout defaults are declared close to the repository methods they govern:

```go
const (
    getPostTimeout    = 500 * time.Millisecond // Single entity read by ID
    createPostTimeout = 1 * time.Second        // Single entity insertion
    updatePostTimeout = 1 * time.Second        // Single entity modification (OCC)
    deletePostTimeout = 1 * time.Second        // Single entity removal
    listPostsTimeout  = 2 * time.Second        // Multi-row query
)
```

| Operation Category | Default Budget | Example Methods | Rationale |
| :--- | :--- | :--- | :--- |
| **Point Read** | `~500ms` | `GetByID` | Indexed single-row lookup; fails fast on connection hiccups. |
| **Mutation** | `~1s` | `Create`, `Update`, `Delete` | Single-row write; includes transaction overhead, foreign keys, and indexes. |
| **Collection / Join** | `~2s` | `List`, `GetCommentsByPostID` | Multi-row scanning, aggregate joins, and sorting. |

> [!NOTE]
> These are sensible defaults for initial system stages, not immutable constants. They can be tuned later using telemetry and p99 latency metrics.

---

## 4. Repository Flow

```text
HTTP Request (API)
      │
      ▼ (ctx)
PostService (Logic)
      │
      ▼ (ctx)
PostRepository (postgres)
      │
      ├─► ctx, cancel := context.WithTimeout(ctx, getPostTimeout)
      │   defer cancel()
      │
      ▼ (ctx with timeout)
sqlc / pgx QueryRow
      │
      ├─► Success (< 500ms) ──────────► Return core.Post
      └─► Timed out / Cancelled ──────► context.DeadlineExceeded / Canceled
```

---

## 5. Error Handling & Boundaries

> [!WARNING]
> **Do not convert timeout errors into `404 Not Found` or validation errors.**

- A timeout is an **infrastructure / latency failure**, not a missing resource or a bad client payload.
- Returning `404` on timeout tricks clients into believing data was deleted, causing phantom recreate attempts.
- In our clean architecture boundary:
  1. The Repository returns `err` directly (e.g. `context.DeadlineExceeded` or PostgreSQL connection error).
  2. The Logic layer propagates the error up.
  3. The API layer's `MapDomainErrorToHTTP` falls through to `internalServerError(w, r, ...)`, producing an HTTP `500 Internal Server Error` and logging the failure with request correlation.
  4. Observability and monitoring can easily isolate `context.DeadlineExceeded` as database latency degradation rather than business logic bugs.
