# Optimistic Concurrency Control (OCC) Architecture

This document defines the architectural guidelines, mental models, and implementation patterns for Optimistic Concurrency Control (OCC) in Go backend services using PostgreSQL.

---

## 1. Core Mental Model

> **Core mental của OCC**:
> *Đừng update dựa trên state cũ nếu data đã thay đổi kể từ lúc mình đọc nó.*
>
> Nói ngắn hơn: **"Update only what I actually read."**

```text
Read version N
      ↓
modify
      ↓
UPDATE ... WHERE version = N
      ↓
version still N?
├─ yes → commit + version++
└─ no  → stale → conflict
```

Nếu request đọc v2 mà lúc write DB đã là v3:

```text
assumption is stale
→ stop
→ 409 Conflict
→ refresh / merge / retry intentionally
```

### Key Distinction: Mutex vs. OCC

```text
Mutex
→ protect shared memory in one process

OCC
→ protect database state across concurrent requests/processes
```

> **Takeaway**:
> *Đó là mental chính; `version` column chỉ là implementation detail để enforce nó.*


---

## 2. The Problem: Lost Updates in Read-Modify-Write

In RESTful APIs, partial updates (PATCH) and aggregate edits typically follow the **Read-Modify-Write** cycle:

1. **Client A** reads Post (version 1).
2. **Client B** reads Post (version 1).
3. **Client A** submits PATCH `{"title": "Updated by A"}` $\rightarrow$ committed to DB (version 2).
4. **Client B** submits PATCH `{"content": "Updated by B"}`.

Without concurrency control, Client B's update silently overwrites Client A's changes, leading to the **Lost Update** anomaly.

---

## 3. OCC vs. In-Memory Mutex (`sync.Mutex`)

A common beginner mistake is attempting to guard database writes using a Go `sync.Mutex`.

| Dimension | `sync.Mutex` | Optimistic Concurrency Control (OCC) |
| :--- | :--- | :--- |
| **Scope** | Single OS process memory space | Entire distributed system across all replicas |
| **Multi-replica Safety** | ❌ **Useless**: Multiple server instances do not share memory | ✅ **Safe**: Database is the single source of truth |
| **Lock Contention** | Blocks execution threads in-memory | Lock-free: no database row-locks held across HTTP requests |
| **Failure Recovery** | Process crash loses lock state | Transactions and atomic checks are durable in DB |

> [!IMPORTANT]
> **Never use a process-level mutex to solve database concurrency.**
> Web servers are stateless and scaled horizontally across multiple containers or instances. Concurrency guarantees must be enforced by the storage boundary.

---

## 4. Why Optimistic Concurrency Control?

There are two primary paradigms for database concurrency:

1. **Pessimistic Locking (`SELECT ... FOR UPDATE`)**:
   - Holds an open database lock while the client thinks, decodes, and decides.
   - Requires holding a DB connection/transaction open across network round-trips.
   - Causes connection starvation, deadlocks, and severe latency bottlenecks in web services.
2. **Optimistic Locking (OCC via `version`)**:
   - Assumes conflicts are rare.
   - Read is standard (`SELECT`). No locks held.
   - The write is guarded by an atomic condition: `WHERE id = $1 AND version = $2`.
   - If a collision occurs, the database safely rejects the stale write.

---

## 5. Architectural Implementation Across Layers

### 5.1. Database & Schema Migration

Each protected table contains a dedicated `version` integer column:

```sql
ALTER TABLE posts ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1;
```

### 5.2. Atomic Update Query (`sqlc`)

The update query atomically checks the read version, increments it, and updates the timestamp:

```sql
-- name: UpdatePost :one
UPDATE posts
SET
    title = $2,
    content = $3,
    tags = $4,
    version = version + 1,
    updated_at = NOW()
WHERE id = $1
  AND version = $5
RETURNING
    id,
    user_id,
    title,
    content,
    tags,
    created_at,
    updated_at,
    version;
```

### 5.3. Repository Layer: Conflict Detection (409 vs 404)

When `UpdatePost` returns zero affected rows (which `pgx` reports as `pgx.ErrNoRows`), the repository must distinguish two cases:
1. Did the post exist, but someone else modified it (version changed)? $\rightarrow$ `core.ErrConflict`
2. Does the post not exist at all? $\rightarrow$ `core.ErrPostNotFound`

```go
func (r *PostRepository) Update(ctx context.Context, p core.Post) (core.Post, error) {
    updated, err := r.queries.UpdatePost(ctx, db.UpdatePostParams{
        ID:      p.ID,
        Title:   p.Title,
        Content: p.Content,
        Tags:    p.Tags,
        Version: p.Version,
    })
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            // Check if the post actually exists to distinguish 404 from 409
            if _, checkErr := r.queries.GetPostByID(ctx, p.ID); checkErr == nil {
                return core.Post{}, core.ErrConflict
            }
            return core.Post{}, core.ErrPostNotFound
        }
        return core.Post{}, err
    }
    return toCorePost(updated), nil
}
```

> [!WARNING]
> **Do not map stale updates to `404 Not Found`.**
> `404` communicates that the resource does not exist. Mapping concurrency collisions to `404` misleads API clients into thinking the resource was deleted, when in fact it simply has newer data.

### 5.4. Domain / Logic Layer

The domain layer remains pure:
- `core.ErrConflict` represents concurrent mutation.
- `logic.PostService` propagates domain errors without knowing about HTTP status codes or SQL queries.

### 5.5. API Layer & HTTP 409 Conflict

The API layer translates `core.ErrConflict` to `409 Conflict`:

```go
// In MapDomainErrorToHTTP:
case errors.Is(err, core.ErrUserAlreadyExists),
     errors.Is(err, core.ErrDuplicateTruckID),
     errors.Is(err, core.ErrConflict):
    conflict(w, r, err.Error())
```

Responses include the `version` field in `PostResponse`:

```json
{
  "data": {
    "id": 1,
    "user_id": 1,
    "title": "Clean Architecture",
    "content": "...",
    "tags": ["go"],
    "created_at": "2026-09-22T21:30:00Z",
    "updated_at": "2026-09-22T21:35:00Z",
    "version": 2
  }
}
```

---

## 6. Stale Write Behavior & Client Reconciliation

When a client receives `409 Conflict`:
- **No silent retry**: The server must not silently overwrite or guess how to merge conflicting fields.
- **Client reconciliation**:
  1. Client catches `409 Conflict`.
  2. Client queries `GET /v1/posts/{id}` to fetch the current version and contents.
  3. Client either prompts the user with the conflict or intentionally merges the changes.
  4. Client re-submits PATCH with the updated version.
