# Database Seeding Architecture

This document defines the architectural guidelines, mental models, and implementation patterns for seeding local and development database environments in Go backend services.

---

## 1. Core Mental Model

> **Core Mental**:
> - **Seed = Intentionally build a useful DB state.**
> - **Dependency graph = Determines insertion order.**

```text
Seed
= intentionally build a useful DB state

Dependency graph
= determines insertion order
```

Seeding is not about blindly dumping arbitrary records into tables. It is an intentional process of constructing a valid, relational snapshot of the domain so developers can immediately test query performance, pagination, feeds, and business logic locally.

---

## 2. The Dependency Graph & Insertion Order

In relational databases (such as PostgreSQL), tables are strictly bound by **foreign key constraints** (`REFERENCES parent(id)`):

```text
Users (Root parent)
  │
  ├───────────────────────────────────┐
  ▼ (author of post)                  ▼ (author of comment)
Posts (Child of Users)                │
  │                                   │
  ▼ (target post)                     │
Comments (Child of Posts and Users) ◄─┘
```

### Golden Rule: Parents Before Children

1. **Insert Parents First**:
   - Insert `Users` first.
   - Retrieve and collect the actual generated IDs returned by PostgreSQL (`RETURNING id` / `user.ID`).
2. **Reuse Real IDs for Children**:
   - Insert `Posts` by assigning `user_id` from the collected pool of real user IDs.
   - Retrieve and collect the actual generated `post.ID` values.
3. **Link Multi-Parent Dependents**:
   - Insert `Comments` using valid `post_id` values from the post pool and valid `user_id` values from the user pool.

> [!CAUTION]
> **Never invent random or hardcoded foreign keys.**
> Synthesizing random IDs (e.g. `user_id = rand.Intn(1000)`) violates foreign key constraints (`23503 foreign_key_violation`) whenever the referenced ID does not exist in the database.

---

## 3. Dedicated CLI Lifecycle (Separation of Concerns)

Seeding logic lives in its own dedicated executable:

```text
cmd/
├── api/      (HTTP server runtime)
├── migrate/  (Schema migrations CLI)
└── seed/     (Dev-only database seeding CLI)
```

```text
cmd/seed
  ↓
Load DB Config (env / flag)
  ↓
Initialize Connection Pool (db.NewPool)
  ↓
Wire Repositories (repository.NewPostgresStorage)
  ↓
Execute Seed Operations (Users -> Posts -> Comments)
  ↓
Close Pool & Exit
```

### Why Seeding Must Never Run on API Startup:

1. **Production Safety**: Running seeds during API startup risks accidental data mutation or corruption in production or staging environments.
2. **Horizontal Scaling**: If multiple container replicas boot simultaneously, running seeds concurrently causes race conditions, duplicate key violations, and deadlocks.
3. **Startup Latency**: Seeding hundreds or thousands of records delays HTTP server readiness probes.
4. **Architectural Purity**: Seeding is an administrative/developer task, not a service runtime concern.

---

## 4. Direct Repository Access (Bypassing HTTP)

Seed tooling talks directly to the repository layer:

```text
Seed Tooling ──► Repository ──► PostgreSQL
```

- **No HTTP Overhead**: Seeding does not need to serialize/deserialize HTTP requests, run router middlewares, or authenticate tokens.
- **Invariants Preserved**: Calling repositories preserves schema typing, SQL generation (`sqlc`), and database triggers. If business validation is essential, domain logic helpers can be reused directly.

---

## 5. Deterministic vs. Random Fake Data

For local development and testing, **deterministic fake data** is preferred over completely random data:

- Uses fixed arrays of realistic templates (names, topics, tags, feedback).
- Uses modular arithmetic (`i % len(templates)`) and structured indices (`#1`, `#2`) to guarantee reproducible datasets.
- Eliminates heavy third-party dependencies (like large faker libraries) when simple Go slices are sufficient.
- Guarantees that every developer who runs `make seed` gets an identical, predictable dataset for debugging.

---

## 6. Developer Workflow

A single `make` target encapsulates the seed execution:

```bash
make seed
```

This executes:

```bash
go run ./cmd/seed -db-url=$(DB_URL)
```

Optional flags allow adjusting volume:

```bash
go run ./cmd/seed -users=50 -posts=100 -comments=150
```
