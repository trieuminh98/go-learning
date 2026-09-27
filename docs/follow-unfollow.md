# Follow and Unfollow Architecture: Relationship State & Actor Boundaries

This document details the architectural principles, mental models, and implementation patterns for managing directional user follow relationships in Go backend applications adhering to Clean Architecture.

---

## 1. Core Mental Models

### 1.1. Actor vs Target Separation

> **Core mental**:
>
> ```text
> Actor ≠ Target
> ```
>
> *The entity performing the action (`actorID` / `follower_id`) must never be confused with the entity being acted upon (`targetID` / `followed_user_id`).*

- **Actor (Follower)**: The authenticated subject initiating the request.
- **Target (Followed User)**: The resource target specified in the URL path (`/v1/users/{userID}/follow`).
- **Domain Invariant**: A user cannot follow themselves (`actorID == targetID`). This invariant belongs strictly in the **Logic layer** (`core.ErrCannotFollowSelf`), never in SQL constraints or repository queries.

### 1.2. Relationship State & Responsibilities

```text
Follow   = Create relationship
Unfollow = Remove relationship

Relationship rules       ──► Logic Layer
Relationship persistence ──► Repository Layer
```

### 1.3. Composite Primary Key & Integrity

The persistence table uses a composite primary key:

```sql
PRIMARY KEY (follower_id, followed_user_id)
```

- **Guarantees Uniqueness**: Duplicate relationships are physically impossible at the database level.
- **Error Normalization**:
  - `23505 (unique_violation)` maps to domain error `core.ErrConflict` (`409 Conflict`).
  - `23503 (foreign_key_violation)` maps to domain error `core.ErrUserNotFound` (`404 Not Found`).

---

## 2. End-to-End Request Flow

```text
Client: PUT /v1/users/{targetID}/follow (optional body: {"user_id": actorID})
  │
  ▼
[API Layer] userHandler.Follow / Unfollow
  │  1. Parse targetID from URL (readIDParam) ──[invalid]──► 400 Bad Request
  │  2. Resolve actorID via transport helper (body with context fallback)
  ▼
[Logic Layer] userService.FollowUser / UnfollowUser
  │  3. Invariant check: actorID == targetID ──[match]──► 400 Bad Request (Cannot follow self)
  ▼
[Repository Layer] followerRepository.Follow / Unfollow
  │  4. Bounded write timeout (1s)
  │  5. SQL: INSERT / DELETE on followers table
  │  6. Error translation:
  │     - 23505 (unique_violation) ────────► core.ErrConflict ──────► 409 Conflict
  │     - 23503 (foreign_key_violation) ────► core.ErrUserNotFound ──► 404 Not Found
  ▼
[Database] PostgreSQL: followers (follower_id, followed_user_id, created_at)
  │
  ▼
HTTP 204 No Content (Success)
```

---

## 3. Pre-Auth Workaround Isolation

In early development stages before full JWT/token authentication middleware is configured:

1. **The Workaround**: The client may temporarily send the actor's ID in a JSON payload (`{"user_id": 1}`).
2. **The Risk**: Allowing request body schemas to bleed into domain services or repositories couples business logic to temporary transport hacks.
3. **The Solution**: The extraction is isolated in a transport helper (`resolveActorID`) inside `internal/api/user.go`:

```go
func resolveActorID(r *http.Request) (int64, error) {
	if r.Body != nil && r.ContentLength > 0 {
		var input struct {
			UserID int64 `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err == nil && input.UserID > 0 {
			return input.UserID, nil
		}
	}
	return UserIDFromContext(r.Context())
}
```

When full authentication is activated, `resolveActorID` is removed and handlers simply read `UserIDFromContext(r.Context())` with zero modifications required in `logic.UserService` or `FollowerRepository`.

---

## 4. HTTP Status Code Conventions

| Condition | Status Code | Response Body |
|---|---|---|
| Success (follow or unfollow) | `204 No Content` | *(empty)* |
| Self-follow attempt (`actorID == targetID`) | `400 Bad Request` | `{"error": "cannot follow self"}` |
| Invalid target ID format (`/v1/users/abc/follow`) | `400 Bad Request` | `{"error": "invalid id parameter: must be a positive integer"}` |
| Target user does not exist | `404 Not Found` | `{"error": "user not found"}` |
| Duplicate follow relationship | `409 Conflict` | `{"error": "conflict: resource was modified concurrently"}` |
| Unhandled database error | `500 Internal Server Error` | `{"error": "the server encountered a problem"}` |

---

## 5. Architectural Guardrails

> [!IMPORTANT]
> **Key Guardrails to Maintain**:
> - **Logic owns business invariants**: Do not let repositories or database triggers handle self-follow validation.
> - **Repositories own database error normalization**: Never leak PostgreSQL error codes (`23505`, `23503`) or `pgconn.PgError` to Logic or API layers.
> - **Bodyless success**: Use `204 No Content` on successful `PUT` operations since no response body is needed.
> - **Bounded timeouts**: Always wrap write operations in context timeouts (`1s`) to protect database connection pools.
