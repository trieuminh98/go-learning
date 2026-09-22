# Error Handling Architecture & Core Mental Model

This document captures the design philosophy and error handling rules across all architectural layers in the Go backend application. These standards are mandatory to ensure system reliability, information security, and maintainability.

---

## 1. Core Mental Model: The Layered Error Flow

```text
Database (PostgreSQL / pgx)
      │  (pgx.ErrNoRows, unique_violation, driver errors)
      ▼
Repository Layer
      │  Translates database driver errors into Domain Sentinel Errors
      │  Rule: NEVER leak pgx/sql driver errors to higher layers
      ▼
Logic Layer (Service / Use Case)
      │  Executes business rules and workflows
      │  Rule: Only wrap & return errors; DO NOT log, DO NOT know about HTTP
      ▼
API Layer (Handler & Transport)
      │  Final Boundary
      ├── 400 Bad Request: Client input error (actionable message, DO NOT log error)
      ├── 404 Not Found: Resource not found (DO NOT log error)
      └── 500 Internal Server Error: Unexpected system failure (LOG ERROR + hide internal details)
```

---

## 2. Core Principles (Hard Rules)

### 2.1. "Log Once at the Boundary" (No Duplicate Logs)
- **Lower Layers (`Repository`, `Logic`)**: Only wrap error context (`fmt.Errorf("...: %w", err)`) and `return err`. Never call `log.Printf` before returning.
- **Topmost Layer (`API Layer`)**: The single place responsible for deciding whether and how to log when mapping an error to an HTTP response.
- **Benefit**: Prevents log clutter where a single failure generates 3-4 redundant log entries scattered across Repo, Service, and Handler.

### 2.2. Categorize Logging: Avoid Log Noise
Not every error represents a system failure:
- **500 Internal Server Error**: Mandatory `[ERROR]` level log containing full request context (`HTTP method`, `path`, `request_id`, `actual error`).
- **400 Bad Request**: Caused by invalid client payload/params. Do NOT log at ERROR level.
- **404 Not Found**: Standard flow when querying non-existent resources. Do NOT log at ERROR level.

### 2.3. Hide Internal Errors in 500 Responses (Security & Data Leak Prevention)
- **NEVER** expose the raw `err.Error()` string of internal system errors to clients on 500 responses.
  - *Reason*: `err.Error()` may expose DB table names, SQL syntax, internal schemas, credentials, or stack traces.
- **Standard**: Always return a stable, generic message:
  ```json
  {
    "error": "the server encountered a problem"
  }
  ```
  The actual root error is safely recorded on the server log for investigation via `request_id`.

### 2.4. Error Envelope Consistency
All API error responses (whether 400, 401, 403, 404, 409, or 500) must adhere strictly to a single uniform JSON envelope:
```json
{
  "error": "concise error description"
}
```
Individual handlers must never invent custom JSON error shapes.

---

## 3. Error Mapping Summary

| Error Category | Source | HTTP Status | Server Log | Client Message |
| :--- | :--- | :---: | :---: | :--- |
| **Invalid Input** | Malformed JSON, unknown field, invalid path param | `400 Bad Request` | No | Clear, actionable message for the client |
| **Business Rule** | Business invariant violation (`ErrInvalidTitle`, `ErrOverloaded`) | `400 Bad Request` | No | Specific domain error description |
| **Not Found** | `pgx.ErrNoRows` -> `core.ErrNotFound` | `404 Not Found` | No | `"resource not found"` or `"post not found"` |
| **Conflict** | `23505 unique_violation` -> `core.ErrDuplicate*` | `409 Conflict` | No | Duplicate data message |
| **Unexpected System Error** | Database down, disk full, unhandled error | `500 Internal Server Error` | **YES [ERROR]** | `"the server encountered a problem"` |
