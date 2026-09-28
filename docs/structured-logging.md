# Structured Logging Architecture: Mental Model & Boundary Enforcement

This document outlines the architectural principles, core mental models, and boundary rules for structured logging using Uber Zap in the Go backend application.

---

## 1. Core Mental Models

> **Mental core của Structured Logging**:
>
> *Logs are structured events, and each event should be logged once at the boundary that has enough context to explain it.*

```text
Repository
    ↓ (return / wrap raw DB error)
Logic
    ↓ (propagate / translate to domain error)
API boundary
    ↓ (enrich with HTTP request context: request_id, method, path, user_id)
Log Once (Uber Zap structured JSON event)
    ↓
Return Safe Public Response to Client ({"error": "the server encountered a problem"})
```

### 1.1. Log tốt là gì?

Log tốt không chỉ là một câu text rời rạc:

```text
// ❌ Ad-hoc, unstructured text
"failed to get user"
"[ERROR] get user failed: connection timeout"
```

mà là một **Structured Event** bao gồm 4 thành phần thiết yếu:

```text
Structured Event = Event + Severity + Request Context + Error Context
```

Ví dụ một bản ghi log chuẩn:

```json
{
  "level": "error",
  "ts": "2026-09-28T07:38:00.123Z",
  "caller": "api/errors.go:48",
  "msg": "internal server error",
  "environment": "production",
  "method": "GET",
  "path": "/v1/users/42",
  "request_id": "req-1727483880123456000",
  "user_id": 42,
  "error": "pq: connection timeout"
}
```

### 1.2. Câu ngắn nhất để nhớ

> **"Return errors inward, add context outward, log once."**
>
> *(Tầng bên dưới chỉ trả lỗi về; tầng biên ngoài cùng bổ sung ngữ cảnh và chỉ ghi log duy nhất một lần).*

---

## 2. Architectural Boundaries & Placement

Toàn bộ mối quan tâm về logging được định vị chính xác tại ranh giới ngoài cùng của hệ thống (Composition Root, Gateway và API Error Boundary):

```text
┌────────────────────────────────────────────────────────┐
│ cmd/api/main.go (Composition Root)                     │
│ ─► Creates *zap.Logger (Production vs Development)     │
│ ─► Injects constant field: .With("environment", env)   │
│ ─► Owns shutdown flush: defer zapLogger.Sync()         │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────┐
│ internal/gateway (Middlewares)                         │
│ ─► RequestIDMiddleware: Injects X-Request-ID to context│
│ ─► LoggingMiddleware: Logs request completed (INFO)    │
│ ─► RecoveryMiddleware: Catches panics & stack (ERROR)  │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────┐
│ internal/api/errors.go (Outer API Error Boundary)      │
│ ─► Gathers context: request_id, method, path, user_id  │
│ ─► Logs once with severity (ERROR, WARN, INFO)         │
│ ─► Returns safe error envelope {"error": "..."}        │
└──────────────────────────┬─────────────────────────────┘
                           │ (HTTP Transport Boundary)
                           ▼
   ═══════════════════════════════════════════════════
   VÙNG CẤM LOGGING (Zero Logger Dependencies)
   ───────────────────────────────────────────────────
   internal/logic      ─► Propagate / wrap domain errors
   internal/repository ─► Wrap SQL errors with func name
   internal/core       ─► Pure domain types & sentinels
   ═══════════════════════════════════════════════════
```

### 2.1. Layer Responsibilities

| Tầng / Thành phần | Vị trí tập tin | Trách nhiệm logging |
| :--- | :--- | :--- |
| **Composition Root** | [`cmd/api/main.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/cmd/api/main.go) | Khởi tạo logger instance duy nhất, cấu hình encoder (JSON cho production, console cho local dev), gắn field `"environment"`, và đăng ký `defer zapLogger.Sync()` để xả buffer khi shutdown. |
| **Gateway Tracing** | [`internal/gateway/middleware.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/gateway/middleware.go) | `LoggingMiddleware`: ghi nhận vòng đời request (method, path, request_id, duration_ms). `RecoveryMiddleware`: bắt panic, ghi nhận stack trace ở level ERROR. Tách biệt hoàn toàn với log nghiệp vụ. |
| **API Error Boundary** | [`internal/api/errors.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/errors.go) | Nơi duy nhất ghi nhận lỗi nghiệp vụ và runtime. Bổ sung `request_id`, `method`, `path`, `user_id`, `error`. Giữ an toàn cho client: trả về `{"error": "the server encountered a problem"}` cho 500 mà không rò rỉ stack trace. |
| **Business Logic** | [`internal/logic/*.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/logic) | **Zero Logging**: Không nhận logger, không import `zap`. Chỉ kiểm tra nghiệp vụ và trả về domain errors (`ErrPostNotFound`, `ErrCannotFollowSelf`). |
| **Data Storage** | [`internal/repository/*.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/repository) | **Zero Logging**: Không nhận logger. Chỉ wrap database errors với ngữ cảnh thao tác (`fmt.Errorf("repo.GetPost: %w", err)`). |

---

## 3. Severity Level Conventions

Không ánh xạ trực tiếp mã HTTP status sang log level. Ánh xạ dựa trên **tác động vận hành (operational impact)**:

```text
ERROR ──► Sự cố bất ngờ, cần kỹ sư can thiệp (Unplanned operational failure)
          Ví dụ: 500 Internal Server Error, Database down, Panic recovered.

WARN  ──► Bất thường nhưng nằm trong kịch bản dự kiến / client làm sai
          Ví dụ: 400 Bad Request, 409 Conflict, 401 Unauthorized.

INFO  ──► Sự kiện bình thường trong vòng đời ứng dụng
          Ví dụ: Server listening, DB pool connected, Request completed, 404 Resource Not Found.
```

### Bảng ánh xạ Error Helpers:

| Helper Function | HTTP Status | Log Level | Log Message | Mục đích vận hành |
| :--- | :--- | :--- | :--- | :--- |
| `internalServerError` | 500 | `ERROR` | `"internal server error"` | Kích hoạt cảnh báo hệ thống; cần điều tra nguyên nhân gốc rễ. |
| `badRequest` | 400 | `WARN` | `"bad request"` | Client gửi payload sai; không kích hoạt alert của backend. |
| `conflict` | 409 | `WARN` | `"conflict"` | Xung đột phiên bản optimistic locking hoặc trùng lặp bản ghi. |
| `unauthorized` | 401 | `WARN` | `"unauthorized"` | Truy cập thiếu hoặc sai bearer token. |
| `notFound` | 404 | `INFO` | `"resource not found"` | Truy vấn tài nguyên không tồn tại; hành vi truy vấn bình thường. |

---

## 4. Stable Fields & Security Allowlist

Để log có thể tìm kiếm, lọc và phân tích tự động trên Elasticsearch, Kibana, Datadog hay CloudWatch, danh sách trường được chuẩn hóa cố định:

```text
Stable Field Allowlist:
- request_id  : Mã định danh phân tán truy vết request (X-Request-ID)
- method      : HTTP Verb (GET, POST, PATCH, PUT, DELETE)
- path        : URL Path đã chuẩn hóa (/v1/posts/1)
- user_id     : ID người dùng đã xác thực (chỉ lưu số nguyên, không lưu token/credential)
- error       : Thông báo lỗi kỹ thuật chi tiết phục vụ debug
- duration_ms : Thời gian xử lý request tính bằng mili-giây
- environment : Môi trường chạy (development, staging, production)
```

### 4.1. Quy tắc an toàn bảo mật (Security Guardrails)

1. **Tuyệt đối không log credentials**: Không bao giờ đưa header `Authorization`, JWT token, mật khẩu hoặc thẻ thanh toán vào log.
2. **Không log raw request body**: Body có thể chứa thông tin định danh cá nhân (PII) hoặc dữ liệu nhạy cảm.
3. **Phân tách Public Response và Private Log**:
   - Client nhận: `{"error": "the server encountered a problem"}`.
   - Log server lưu: `error="pq: connection refused"`, `db_query="SELECT ... FROM posts"`.
4. **Không nhân bản log (Eliminate Log Amplification)**: Một lỗi chỉ được ghi log một lần duy nhất tại API boundary, ngăn chặn hiện tượng 1 bug database sinh ra 3 dòng log rác ở Repo, Logic và API.
