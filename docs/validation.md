# Request Validation Architecture & Core Mental Model

This document defines the design philosophy and layering rules for request and data validation across the Go backend application. The guiding principle is: **"Validate at the boundary that owns the rule."**

---

## 1. Core Mental Model: Layered Validation Boundaries

Each architectural layer has an independent role and perspective on what makes data "valid":

```text
HTTP Request
     │
     ▼
API Boundary (Transport & DTO)
     │  "Is this input structurally valid?"
     │  Shape, required fields, data type, min/max string length, formatting
     │  Failure ──> 400 Bad Request (Stops immediately, does not call Logic)
     ▼
Logic / Core Boundary (Business Domain)
     │  "Is this action and business state valid?"
     │  Business truth, permissions, state transitions, domain invariants
     │  Failure ──> Domain Sentinel Error (API maps to corresponding HTTP status)
     ▼
Database Boundary (Persistence & Storage)
     │  "Is persisted data still consistent?"
     │  NOT NULL, UNIQUE, FOREIGN KEY, CHECK constraints
     │  Failure ──> Driver Error (Repo maps to core.ErrDuplicate*, core.ErrNotFound)
```

---

## 2. Core Principles (Hard Rules)

### 2.1. "Decode ≠ Validate"
- **Decode (e.g. `json.NewDecoder(r.Body).Decode(&dto)`)**: Serves one singular purpose: converting raw incoming bytes into Go struct fields.
- Successful decoding **does not guarantee** the data is valid (strings can be empty, numbers negative, slices exceeding length bounds).
- **Rule**: Always execute an explicit validation step immediately following decoding, before executing any business logic.

### 2.2. "Validate at the Boundary That Owns the Rule"
Do not centralize all validation in one place, nor push one layer's responsibility into another:

1. **API Layer**:
   - **Owns**: Structural rules (`shape`, `format`, `required`, `length`).
   - **Tooling**: Struct tags with `go-playground/validator` on Request DTOs (`validate:"required,max=100"`).
   - **Action**: On violation, immediately return a safe `400 Bad Request`. Never invoke the Logic layer.

2. **Logic / Core Layer**:
   - **Owns**: Business truth (`business truth`, `domain invariants`, `permissions`).
   - **Tooling**: Pure Go functions, business rules, domain service checks.
   - **Action**: Return sentinel domain errors (`core.Err*`). Independent of struct tags or HTTP validator libraries.

3. **Database Layer**:
   - **Owns**: Physical data integrity (`last line of data integrity`).
   - **Tooling**: PostgreSQL schema constraints (`NOT NULL`, `UNIQUE`, `FOREIGN KEY`, `CHECK`).
   - **Action**: Guards against race conditions and defects missed by upper layers (Defense in Depth).

### 2.3. Do Not Leak Validator Internals
- Do not expose raw reflection errors from `validator.ValidationErrors` in the public API contract.
- Standardize 400 error messages using the uniform JSON envelope defined in [docs/error.md](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/docs/error.md):
  ```json
  {
    "error": "invalid request payload"
  }
  ```

---

## 3. Case Study: `Create Post` Flow

Examining how responsibilities are divided when creating a post:

| Validation Scenario | Owning Layer | Rationale | Resulting Action |
| :--- | :---: | :--- | :--- |
| `title` is empty or exceeds 100 characters | **API** | Dictated by incoming payload shape and length constraints | `400 Bad Request` (stopped at Handler) |
| `content` is empty or exceeds 1000 characters | **API** | Dictated by input payload structure | `400 Bad Request` (stopped at Handler) |
| User is banned from posting | **Logic** | Requires checking the real-world business state of the account | `core.ErrUserBanned` → `403 Forbidden` |
| Daily post limit reached | **Logic** | Coordination logic dependent on time and historical records | `core.ErrDailyPostLimitExceeded` → `400 / 429` |
| `user_id` does not exist | **Database** | Relational integrity constraint via `FOREIGN KEY (user_id)` | DB FK violation → `core.ErrUserNotFound` → `404 Not Found` |
| Duplicate post title or slug | **Database** | `UNIQUE constraint` protecting against concurrent race conditions | DB unique violation → `core.ErrDuplicate*` → `409 Conflict` |

---

## 4. Architectural Anti-Patterns Checklist

- [ ] **Anti-Pattern 1: Querying the DB to check raw input structure**
  *Incorrect*: Querying the database to check if the user sent an empty `title`.
  *Correct*: The API struct validation tag catches it immediately during request ingestion.
- [ ] **Anti-Pattern 2: Smuggling business logic into struct validation tags**
  *Incorrect*: Writing a custom validator tag `validate:"check_user_not_banned"`.
  *Correct*: Struct tags only evaluate syntax/length. Permissions and business state belong in Logic.
- [ ] **Anti-Pattern 3: Skipping DB constraints because API validated the input**
  *Incorrect*: Omitting `NOT NULL` or `FOREIGN KEY` at the database level assuming "the API already checked it".
  *Correct*: Concurrent requests can bypass application-level checks; database constraints remain the ultimate line of defense.
