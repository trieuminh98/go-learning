# Standard API Response Envelope Architecture & Core Mental Model

This document outlines the architecture, design principles, and core mental model for standardizing HTTP response payloads in the Go backend application.

---

## 1. Core Mental Model: Business Outcome ≠ Public Response Format

The guiding principle of this architecture is:

> **"Handlers must not independently decide the wire response shape; handlers produce domain outcomes (status + data), while centralized API response helpers define and enforce a stable public contract for clients."**

```text
Handler (Application outcome)
    │ "Operation succeeded, here is the DTO"
    ▼
jsonResponse(w, status, data) (Public contract layer)
    │ Wraps payload: {"data": ...}
    ▼
writeJSON(w, status, envelope) (Transport serialization)
    │ Sets Content-Type: application/json, writes HTTP status
    ▼
Client (Network boundary)
    │ Receives consistent, predictable response contract
```

### Why Decentralized Response Formatting Fails

Without a centralized helper, each handler decides how to format its response:
- Handler A returns a raw object: `{ "id": 1, "title": "..." }`
- Handler B returns a wrapped object: `{ "result": { ... } }`
- Handler C returns a data envelope: `{ "data": { ... } }`
- Handler D returns an inconsistent array: `[ ... ]`

This forces API consumers (web, mobile, third-party integrations) to write customized parsing and unmarshaling logic for every single endpoint.

By centralizing response formatting behind `jsonResponse()`:
- **Handler responsibility**: *What happened* (status code + domain DTO).
- **API helper responsibility**: *How that outcome is exposed to clients* (standard wire contract).
- `{"data": ...}` is the convention; **stable, predictable response contracts across all endpoints** is the core mental model.

---

## 2. The Standard Response Contracts

The application establishes two standardized JSON envelopes at the API layer:

### 2.1. Success Envelope (`{"data": ...}`)

All successful operations returning a response body (HTTP 200 OK, HTTP 201 Created) are encapsulated in a `data` field:

```json
// Single resource (e.g. GET /v1/posts/1)
{
  "data": {
    "id": 1,
    "user_id": 1,
    "title": "Clean Architecture in Go",
    "content": "Separation of concerns...",
    "tags": ["go", "backend"],
    "created_at": "2026-09-22T20:00:00Z",
    "updated_at": "2026-09-22T20:00:00Z"
  }
}

// Resource collection (e.g. GET /v1/posts)
{
  "data": [
    {
      "id": 1,
      "title": "..."
    }
  ]
}
```

### 2.2. Error Envelope (`{"error": ...}`)

All client errors (HTTP 4xx) and server errors (HTTP 5xx) retain the standardized error envelope:

```json
{
  "error": "post not found"
}
```

Error responses are managed by `writeJSONError()` and `MapDomainErrorToHTTP()`, completely isolated from success responses.

### 2.3. Bodyless Success (`204 No Content`)

Certain HTTP verbs and operations inherently carry no response body by definition (RFC 9110):
- `DELETE /v1/posts/{postID}` returns `204 No Content`.
- **Rule**: Never force an envelope or response body onto a 204 response. Call `w.WriteHeader(http.StatusNoContent)` directly.

---

## 3. Architecture & Boundary Separation

```text
┌────────────────────────────────────────────────────────────────────────┐
│ HTTP Transport Layer (api/json.go)                                     │
│                                                                        │
│   jsonResponse(w, status, data) ─── wraps ──> envelope{Data: data}    │
│            │                                                           │
│            ▼                                                           │
│   writeJSON(w, status, envelope) ── encodes JSON & writes headers      │
└────────────────────────────────────────────────────────────────────────┘
                                 ▲
                                 │ calls jsonResponse()
┌────────────────────────────────────────────────────────────────────────┐
│ API Handlers (api/post.go, api/user.go, api/truck.go, api/health.go)   │
│   - Coordinates request lifecycle                                      │
│   - Converts Core entities into API DTOs (e.g. PostResponse)           │
│   - Passes DTOs to jsonResponse()                                      │
└────────────────────────────────────────────────────────────────────────┘
                                 ▲
                                 │ returns domain entity
┌────────────────────────────────────────────────────────────────────────┐
│ Logic / Core Domain Layer (logic/post.go, core/post.go)                │
│   - Enforces business rules and invariants                             │
│   - Completely decoupled from HTTP, JSON, and envelope wrappers        │
└────────────────────────────────────────────────────────────────────────┘
```

### 3.1. Layering Guardrails
1. **Never put envelopes in Core/Logic**: Core entities (`core.Post`) and Logic service outputs must remain pure domain types. Envelopes belong exclusively to the HTTP presentation layer.
2. **Never expose `sqlc` models**: Always transform database models into domain entities and then into API DTOs before passing to `jsonResponse()`.
3. **Keep `writeJSON` primitive**: `writeJSON()` handles `Content-Type: application/json`, status codes, and low-level `json.Encoder`. Higher-level helpers (`jsonResponse`, `writeJSONError`) encapsulate contract conventions.

---

## 4. Contract Invariants & Edge Cases

### 4.1. The Nil Slice Trap (`{"data": []}` vs `{"data": null}`)

In Go, an uninitialized slice (`var items []PostResponse`) marshals to JSON `null`. When wrapped inside `envelope`, it produces:

```json
// INVALID / DANGEROUS:
{
  "data": null
}
```

Frontend clients expecting an array will encounter runtime `TypeError: Cannot read properties of null (reading 'map')`.

**Invariant**: All collection mappers must explicitly initialize empty slices:
```go
resp := make([]PostResponse, 0, len(posts))
```
This guarantees that empty results serialize predictably to:
```json
// CORRECT:
{
  "data": []
}
```

### 4.2. Future-Proofing for Metadata (`meta`)

The envelope is designed to easily evolve to support pagination, cursors, and filtering metadata without breaking existing clients:

```json
{
  "data": [ ... ],
  "meta": {
    "next_cursor": "cursor_token_xyz",
    "total_count": 42
  }
}
```

When metadata is introduced, an extended helper (e.g. `jsonResponseWithMeta(w, status, data, meta)`) can build on top of the same foundation without touching domain business logic.
