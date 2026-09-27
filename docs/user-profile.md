# User Profile Architecture: Internal Entity vs Public Representation

This document outlines the architectural principles, mental models, and implementation patterns for exposing user profile information in Go backend services while maintaining strict separation between internal persistence models and public API contracts.

---

## 1. Core Mental Model

> **Core mental của User Profile**:
>
> ```text
> Internal User ≠ Public User Profile
> ```
>
> *The public API exposes only what the client is intentionally allowed to know. Information hiding is not an afterthought; it is enforced by structural separation.*

```text
Database Record (PostgreSQL)
├─ id
├─ first_name
├─ last_name
├─ email (private)
├─ password_hash (secret)
├─ failed_logins (internal)
└─ ...
        │
        ▼ (sqlc scan to domain)
Core User (Domain Entity)
├─ ID
├─ FirstName
└─ LastName
        │
        ▼ (explicit mapping function)
UserResponse (Public DTO)
├─ id
├─ first_name
└─ last_name
        │
        ▼ (standardized envelope)
HTTP Response 200 OK: {"data": {...}}
```

### Hai nguyên tắc vàng về an toàn dữ liệu:

1. **Explicit Whitelist over Blacklist**:
   - Never return persistence models directly to the HTTP client.
   - Do not rely solely on `json:"-"` struct tags on database/domain models to protect secrets.
   - Define a dedicated public DTO (`UserResponse`) and map fields explicitly. Any new database column added in the future remains private by default until intentionally exposed.

2. **Layered Separation of Concerns**:
   - Untrusted transport parameters (`userID`) are validated at the API boundary before hitting business logic.
   - The API layer never calls the database repository directly; it orchestrates through `logic.UserService`.
   - Infrastructure-specific errors (`pgx.ErrNoRows`) are normalized into domain errors (`core.ErrUserNotFound`) at the repository boundary so upper layers remain decoupled from database technology.

---

## 2. End-to-End Request Flow

The `GET /v1/users/{userID}` endpoint follows Clean Architecture:

```text
HTTP Request: GET /v1/users/{userID}
  │
  ▼
[API Layer] userHandler.GetUser
  │  1. Validate untrusted ID (readIDParam) ──[invalid]──► 400 Bad Request
  ▼
[Logic Layer] userService.GetUser
  │  2. Coordinate use-case & domain validations
  ▼
[Repository Layer] userRepository.GetByID
  │  3. Bound execution with point-read timeout (500ms)
  │  4. Normalize pgx.ErrNoRows ──[not found]──► core.ErrUserNotFound ──► 404 Not Found
  ▼
[Database] PostgreSQL (sqlc query)
  │
  ▼ Return internal core.User
[API Layer] Explicit DTO Mapping: toUserResponse(core.User) -> UserResponse
  │  (Applies information hiding — exposes only public fields)
  ▼
HTTP 200 OK: {"data": {"id": "1", "first_name": "Alice", "last_name": "Smith"}}
```

---

## 3. Implementation Details

### 3.1. Persistence Layer (`sqlc`)

The point-read query is defined in `internal/db/query/users.sql`:

```sql
-- name: GetUserByID :one
SELECT id, first_name, last_name, created_at
FROM users
WHERE id = $1;
```

`sqlc` compiles this into type-safe Go code with zero manual boilerplate:

```go
func (q *Queries) GetUserByID(ctx context.Context, id int64) (User, error)
```

### 3.2. Repository Implementation & Timeout Propagation

In `internal/repository/postgres/user.go`:

- Query execution is bounded by `getUserTimeout = 500 * time.Millisecond` derived from the parent context.
- Database errors (`pgx.ErrNoRows`) are caught and translated into the domain error `core.ErrUserNotFound`.

```go
const getUserTimeout = 500 * time.Millisecond

func (r *UserRepository) GetByID(ctx context.Context, id int64) (core.User, error) {
	ctx, cancel := context.WithTimeout(ctx, getUserTimeout)
	defer cancel()

	user, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.User{}, core.ErrUserNotFound
		}
		return core.User{}, err
	}

	return toCoreUser(user), nil
}
```

### 3.3. Logic Layer Orchestration

The `UserRepository` interface in `internal/logic/repository.go` defines the domain contract without leaking PostgreSQL details:

```go
type UserRepository interface {
	List(ctx context.Context) ([]core.User, error)
	GetByID(ctx context.Context, id int64) (core.User, error)
	Create(ctx context.Context, user core.User) (core.User, error)
}
```

`logic.UserService` mediates between the HTTP handler and repository:

```go
func (s *UserService) GetUser(ctx context.Context, id int64) (core.User, error) {
	return s.repo.GetByID(ctx, id)
}
```

### 3.4. API Layer & DTO Mapping

The HTTP handler in `internal/api/user.go`:
1. Treats path parameters as untrusted input using `readIDParam`.
2. Invokes domain logic.
3. Translates domain errors through `MapDomainErrorToHTTP`.
4. Maps `core.User` to `UserResponse` and wraps the result in the standard `{"data": ...}` envelope.

```go
type UserResponse struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func toUserResponse(u core.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID, err := readIDParam(r, "userID")
	if err != nil {
		badRequest(w, r, err)
		return
	}

	user, err := h.service.GetUser(r.Context(), userID)
	if err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	_ = jsonResponse(w, http.StatusOK, toUserResponse(user))
}
```

---

## 4. HTTP Status Code Conventions

| Condition | Status Code | Error Message / Envelope |
|---|---|---|
| Invalid or non-positive ID (`/v1/users/abc`, `/v1/users/-1`) | `400 Bad Request` | `{"error": "invalid id parameter: must be a positive integer"}` |
| User does not exist | `404 Not Found` | `{"error": "user not found"}` |
| Unhandled database failure / timeout cancellation | `500 Internal Server Error` | `{"error": "the server encountered a problem"}` |
| Successful retrieval | `200 OK` | `{"data": {"id": "1", "first_name": "...", "last_name": "..."}}` |

---

## 5. Architectural Guardrails

> [!IMPORTANT]
> **Key Guardrails to Maintain**:
> - **Never bypass the Logic layer**: The API handler must never call `userRepo.GetByID` directly.
> - **No database types in the presentation layer**: Never import `db.User` in `internal/api`.
> - **Private by default**: When adding fields such as `email`, `phone_number`, or `role` to users, treat their exposure as an explicit privacy decision rather than adding them to `UserResponse` indiscriminately.
> - **Bounded timeouts**: Always wrap repository queries in use-case bounded contexts (`500ms` for point queries) so connection pools are protected against cascading stalls.
