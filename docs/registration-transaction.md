# Registration Flow: Transaction & Compensation Architecture

This document details the mental models, architectural boundaries, and implementation patterns for multi-step user registration with transactional persistence and post-commit cross-system compensation.

---

## 1. Core Mental Models

> **Mental core của Registration Transaction & Compensation**:
>
> *Atomicity chỉ tồn tại trong cùng một transactional boundary. Ra ngoài boundary đó thì phải orchestration + compensation.*

```text
One business workflow
        ↓
┌─────────────────────┐
│ Same DB boundary    │
│ user + invitation   │
│ → SQL transaction   │
└─────────────────────┘
        ↓
External system
email provider
        ↓
cannot rollback
        ↓
retry / compensate / saga
```

Source architecture áp dụng đúng sự phân định (distinction) này:
- `create user` + `create invitation` diễn ra trong cùng một cơ sở dữ liệu nên có thể rollback cùng nhau bằng SQL transaction.
- `send activation email` gọi ra external system (SMTP provider) nên không thể rollback bằng transaction semantics của SQL. Do đó cần workflow orchestration và compensating action.

### 1.1. Hai câu cốt lõi để nhớ:

```text
Transaction
= all-or-nothing inside one system

Compensation
= repair the workflow across systems
```

### 1.2. Bản chất của Registration:

```text
Register user
≠ one INSERT

Register user
= multi-step state transition
```

### 1.3. Câu ngắn nhất:

> **"Local consistency uses transactions; cross-boundary consistency uses workflows."**

---

## 2. Multi-Step Registration Workflow

Registration is not a single CRUD insert; it is an orchestrated multi-step workflow:

```text
Client Request: POST /v1/authentication/user
      │
      ▼
1. Validate Input (names, email format, minimum password length)
      │
      ▼
2. Hash Password (bcrypt cost=12 encapsulated in domain Password entity)
      │
      ▼
3. Generate Invitation Token (unguessable 32-byte crypto/rand token + SHA-256 hash)
      │
      ▼
4. Atomic Local DB Transaction (WithinTransaction)
   ┌──────────────────────────────────────────────┐
   │ BEGIN                                        │
   │  ├─ Insert Inactive User (is_active = false) │
   │  └─ Insert Invitation (token_hash, expiry)   │
   │ COMMIT                                       │
   └──────────────────────────────────────────────┘
      │
      ▼ (Post-commit)
5. Send Activation Email (Mailer — external I/O outside DB transaction)
      ├─ [Success] ──► Return 201 Created to Client
      │
      └─ [Failure] ──► 6. Execute Compensation in Logic
                            ├─ Delete Invitation record
                            ├─ Delete Inactive User record
                            └─ Log error & return 500 Internal Error
```

---

## 3. Architecture & Boundary Rules

### 3.1. Layered Separation

```text
API Layer (internal/api)
  │  1. Unmarshal JSON & validate payload rules
  │  2. Invoke Logic.RegisterUser
  │  3. Map domain errors to safe HTTP responses (400, 409, 500)
  ▼
Logic Layer (internal/logic)
  │  1. Owns domain business rules & validation
  │  2. Encapsulates bcrypt password hashing
  │  3. Generates secure random tokens
  │  4. Runs WithinTransaction callback
  │  5. Dispatches email via Mailer OUTSIDE the SQL transaction
  │  6. Coordinates compensating cleanup if mailer dispatch fails
  ▼
Storage & Repositories (internal/repository)
  │  1. WithinTransaction manages tx lifecycle (Begin, Commit, Rollback)
  │  2. Exposes transaction-scoped repositories (Users, Invitations)
  │  3. Normalizes pgx/database errors to domain errors
  ▼
Database (PostgreSQL)
     ACID guarantees for local user and invitation rows
```

### 3.2. Why Email is Kept Outside the SQL Transaction

External network I/O must never occur inside a database transaction:
- **No 2PC support**: External SMTP servers and email providers do not participate in database two-phase commit or XA protocols.
- **Connection pool exhaustion**: A slow SMTP connection (e.g. 3–5 seconds) holds a database connection open and locks transaction resources, rapidly exhausting pool capacity.
- **Irreversible external action**: An email, once sent across the network, cannot be "rolled back" by issuing an SQL `ROLLBACK`.

### 3.3. Compensation Semantics

When the local database transaction commits successfully but the external mailer fails:

```text
DB commit ✅
email ❌
```

SQL rollback is no longer available. The workflow coordinates an explicit compensating action:

```text
email failed
   ↓
compensate in Logic
   ↓
delete invitation
delete inactive user
```

- **Explicit in Logic**: The compensation is an orchestration concern owned by `UserService`, never hidden inside repository methods.
- **Idempotency & Re-registration**: Cleaning up the unactivated user ensures the email address is immediately freed so the user can re-attempt registration without receiving an unexpected 409 Conflict.

---

## 4. Security & Cryptographic Invariants

1. **Password Hashing**:
   - Plaintext passwords are never persisted to PostgreSQL, never logged, and never included in API responses or public DTOs.
   - Hashing is handled by `core.Password.Set(plaintext)` using `golang.org/x/crypto/bcrypt` at cost 12.
   - `core.Password.Matches(plaintext)` encapsulates timing-safe verification for future authentication workflows.

2. **Invitation Tokens**:
   - Generated with `crypto/rand` producing 32 unguessable random bytes (64 hex characters).
   - Only the SHA-256 cryptographic hash (`token_hash BYTEA`) is stored in PostgreSQL.
   - The plaintext token is delivered exclusively to the client through the activation email link, preventing token leakage from database snapshots or read replicas.

3. **Inactive User State**:
   - Users are created with `is_active = false`.
   - Accounts are not permitted to log in or perform authenticated actions until invitation verification confirms ownership of the email.

---

## 5. Failure Semantics & HTTP Status Codes

| Stage / Condition | Root Cause | HTTP Status | Response Payload |
|---|---|---|---|
| Input validation | Missing name, invalid email, password < 8 chars | `400 Bad Request` | `{"error": "invalid email address"}` |
| Duplicate email | Unique constraint violation (`users_email_key`) | `409 Conflict` | `{"error": "user already exists"}` |
| DB transaction failure | PostgreSQL query error, connection drop, timeout | `500 Internal Server Error` | `{"error": "the server encountered a problem"}` |
| Mailer dispatch failure | SMTP timeout / network error (triggers compensation) | `500 Internal Server Error` | `{"error": "the server encountered a problem"}` |
