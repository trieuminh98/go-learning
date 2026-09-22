# PATCH, Resource Middleware, and Context Architecture

This document defines the architectural guidelines and mental models for partial resource updates (PATCH), shared resource pre-fetching via HTTP middleware, and proper hygiene for `context.Context` in Go backend services.

---

## 1. Core Mental Models

The architecture of this layer is summarized by three core principles:

1. **PATCH → Presence matters**: Distinguish between an omitted field and a field explicitly sent with a zero value.
2. **Middleware → Resolve shared request data once**: Extract parameters, load the target entity via Logic, and attach it to context for downstream handlers that need it.
3. **Context → Carry request-scoped data, not dependencies**: Context represents the request lifecycle and its transient data; it is never a dependency injection container.

> **One-line takeaway**:
> *Partial updates require knowing field presence; shared request state can be resolved once by middleware and propagated via context.*

---

## 2. Principle 1: PATCH and Field Presence

### 2.1. The Zero-Value Ambiguity Problem in Go

In Go, default primitive types always deserialize to their zero values when omitted from JSON:
- If `Title string` is used in a DTO, both `{"content": "new"}` (title omitted) and `{"title": "", "content": "new"}` (title explicitly set to empty) result in `Title == ""`.
- A naive update handler cannot tell whether the client wanted to erase the title or simply leave it untouched.

### 2.2. Solution: Pointer-Based DTOs

Using pointer fields (`*string`, `*[]string`) allows `encoding/json` to distinguish three distinct client intents:

```go
type UpdatePostRequest struct {
    Title   *string   `json:"title" validate:"omitempty,max=100"`
    Content *string   `json:"content" validate:"omitempty,max=1000"`
    Tags    *[]string `json:"tags"`
}
```

```text
Client Payload          Pointer Value       Application Interpretation
----------------------------------------------------------------------------------------
(field omitted)    ──>  nil            ──>  Field not sent: preserve existing value.
{"title": ""}      ──>  &""            ──>  Field sent as empty: apply empty string
                                            (fails Logic validation if required).
{"title": "Go"}    ──>  &"Go"          ──>  Field sent with value: update to "Go".
```

### 2.3. Boundary Separation: Decode ≠ Validate

- **API Layer**: Unmarshals into pointers and validates structural constraints via `validator` (e.g. `omitempty,max=100`).
- **Selective Mutation**: The handler copies only non-nil fields into the existing entity:
  ```go
  if req.Title != nil {
      post.Title = *req.Title
  }
  if req.Content != nil {
      post.Content = *req.Content
  }
  ```
- **Logic Layer**: Enforces business truth on the updated entity (e.g. `trimmedTitle == "" -> ErrInvalidTitle`).

---

## 3. Principle 2: Resource Middleware

### 3.1. Purpose

When multiple handlers or route operations need the same resolved entity (such as `GET /posts/{id}` or `PATCH /posts/{id}`), an HTTP middleware can resolve the entity once, handle failure states early, and store the result in `r.Context()`.

```text
PATCH /v1/posts/{postID}
          │
          ▼
PostContextMiddleware
  ├── Parse postID route parameter (400 if invalid)
  ├── Load entity: PostService.GetPost(ctx, postID)
  ├── Handle failure: 404 Not Found or 500 Internal Error
  ├── Inject into context: ContextWithPost(ctx, post)
  └── Call next.ServeHTTP(w, r.WithContext(ctx))
          │
          ▼
UpdatePost Handler
  ├── Read post from r.Context() (already verified)
  ├── Unmarshal UpdatePostRequest & apply non-nil fields
  └── PostService.UpdatePost(ctx, post)
```

### 3.2. Architecture Boundary

- **Always call Logic, never Repository**: The middleware must call `PostService`, never the database or repository directly. This guarantees cache policies, soft-delete checks, or business validation remain intact.
- **Do not force middleware onto DELETE**:
  - `DELETE /v1/posts/{postID}` does not need a pre-fetched post.
  - Doing a `SELECT` before `DELETE FROM posts WHERE id = $1` adds an unnecessary database round-trip.
  - A direct SQL `DELETE` returning `RowsAffected() == 0 -> ErrPostNotFound` is atomic, faster, and idiomatic.

---

## 4. Principle 3: Context Hygiene

### 4.1. The Role of `context.Context`

```text
context.Context = Request Lifecycle + Request-Scoped Data + Cancellation / Deadlines
```

| Allowed in Context (Request-Scoped) | Forbidden in Context (Dependencies) |
| :--- | :--- |
| Resolved resource (e.g. `core.Post`) | Database pool (`*pgxpool.Pool`) |
| Authenticated User ID (`int64`) | Repositories (`PostRepository`) |
| Request correlation ID (`request_id`) | Business Services (`PostService`) |
| Timeout deadline / cancellation signal | Application configuration (`Config`) |

Dependencies must always be injected explicitly into struct fields via constructors. Using `context.Context` as a dependency container (service locator anti-pattern) conceals dependencies, hinders testing, and defeats Go's static typing.

### 4.2. Safe Key Typing

Never use raw strings as context keys. Package-scoped private types prevent collisions across different packages:

```go
type postContextKey string

const contextPostKey postContextKey = "post"

func ContextWithPost(ctx context.Context, post core.Post) context.Context {
    return context.WithValue(ctx, contextPostKey, post)
}

func PostFromContext(ctx context.Context) (core.Post, bool) {
    post, ok := ctx.Value(contextPostKey).(core.Post)
    return post, ok
}
```

---

## 5. Summary Matrix

| Operation | Route | Middleware | Context Injected | DB Execution |
| :--- | :--- | :--- | :--- | :--- |
| **Get Detail** | `GET /v1/posts/{id}` | Standard routing | `user_id` (auth) | Composed `GetByID` + `GetCommentsByPostID` |
| **Patch Post** | `PATCH /v1/posts/{id}` | `PostContextMiddleware` | `user_id`, resolved `core.Post` | `UpdatePost` (`RETURNING`) |
| **Delete Post** | `DELETE /v1/posts/{id}` | Direct | `user_id` (auth) | Direct `DELETE` (`RowsAffected()`) |
