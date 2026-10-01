# Cross-Origin Resource Sharing (CORS) Architecture & Specification

This document details the architectural design, mental models, implementation patterns, middleware ordering rationale, and security boundaries for Cross-Origin Resource Sharing (CORS) in the Go backend.

---

## 1. Core Mental Models

> **Mental core của CORS**:
>
> *CORS = browser kiểm tra origin của frontend có được API cho phép hay không.*
>
> *(Cơ chế bảo mật trên trình duyệt nhằm kiểm soát việc ứng dụng web chạy ở một origin khác có được phép truy cập và đọc dữ liệu trả về từ API hay không).*

### 1.1. Luồng xử lý CORS trên Browser

```text
Frontend
   ↓
Browser
   ↓
Origin = scheme + host + port
   ↓
CORS allowlist
├─ allowed → browser cho request/response đi tiếp
└─ denied  → browser block
```

### 1.2. Định nghĩa một "Origin"

Một Origin trong chuẩn web cấu thành từ 3 yếu tố:

```text
Origin = scheme + host + port
```

Source nhấn mạnh: chỉ cần khác scheme, host/subdomain, hoặc port là đã thành different origin:

```text
http://localhost:3000   ≠   http://localhost:8080      (khác port)
http://example.com      ≠   https://example.com       (khác scheme: http vs https)
https://app.example.com ≠   https://api.example.com   (khác subdomain)
```

### 1.3. Mental thứ hai: CORS không phải Auth

Giữ các khái niệm hoàn toàn tách bạch:

```text
CORS
→ browser origin nào được gọi?

Authentication
→ caller là ai?

Authorization
→ caller được làm gì?

Rate Limiting
→ caller được tiêu thụ bao nhiêu tài nguyên?
```

- CORS là chính sách do **trình duyệt (browser) thực thi**.
- CORS **không bảo vệ** API trước `curl`, Postman, backend-to-backend services, hay tấn công từ bot.

### 1.4. Production Mental: Allow Explicit Origins, đừng mặc định `*`

Trong môi trường production, luôn cấu hình explicit allowlist qua biến môi trường thay vì dùng wildcard `*`:

```text
local (development)
→ http://localhost:3000

prod (production)
→ https://app.example.com
```

Source chuyển đổi từ broad allow sang quản lý danh sách origins cụ thể qua biến môi trường `CORS_ALLOWED_ORIGINS` để ngăn chặn các website độc hại đọc trộm tài nguyên API.

### 1.5. Rút gọn trong một câu (Core Takeaway)

> *CORS is a browser-enforced boundary to protect web clients; configure explicit allowlists at the HTTP boundary and never confuse it with authentication.*

---

## 2. Architecture & Layer Boundaries

CORS là một mối quan tâm xuyên suốt (cross-cutting HTTP concern) thuần túy của tầng **Gateway / HTTP Middleware**:

```text
Browser Request
  ↓
Gateway Middleware Stack (internal/api)
  ↓
Route Dispatcher (Chi Router)
  ↓
Domain Logic (internal/logic)
  ↓
Repository / Database (internal/repository)
```

**Nguyên tắc ranh giới (Boundary Rules)**:
- **Tuyệt đối không đưa CORS vào**: `internal/core`, `internal/logic`, hay `internal/repository`. Các tầng nghiệp vụ bên dưới hoàn toàn không quan tâm request đến từ browser hay terminal hay mobile app.
- Sử dụng middleware tiêu chuẩn được cộng đồng duy trì (`github.com/go-chi/cors`) thay vì tự hiện thực hóa bằng code thủ công.

---

## 3. Thứ tự Middleware Stack & Rationale

Thứ tự sắp xếp middleware trong `internal/api/router.go` mang ý nghĩa sống còn đối với trải nghiệm người dùng và tính chính xác của hệ thống:

```text
1. RecoveryMiddleware     (bắt panic ở tầng ngoài cùng, bảo vệ tiến trình)
   ↓
2. RequestIDMiddleware    (gán mã X-Request-ID cho mọi request)
   ↓
3. CORS Middleware        (go-chi/cors: xử lý preflight & gắn CORS response headers)
   ↓
4. RateLimitMiddleware    (kiểm tra quota client IP; chặn 429 nếu vượt mức)
   ↓
5. LoggingMiddleware      (ghi log duration cho request hợp lệ)
   ↓
6. TimeoutMiddleware      (áp đặt deadline xử lý 10s)
   ↓
7. Authentication & Route Handlers
```

### 3.1. Tại sao CORS phải nằm TRƯỚC Rate Limiter?

1. **Bảo toàn Header lỗi (Error Masking Prevention)**:
   - Nếu Rate Limiter đặt trước CORS, khi client gửi dồn dập và nhận mã `429 Too Many Requests`, response trả về sẽ **thiếu** header `Access-Control-Allow-Origin`.
   - Trình duyệt phát hiện thiếu header CORS sẽ lập tức chặn response và ném ra lỗi mạng chung chung (`CORS Network Error`), che giấu hoàn toàn lỗi 429 thực sự và khiến frontend không thể đọc được header `Retry-After`.
   - Khi CORS nằm trước, response 429 vẫn được bọc trong các header CORS hợp lệ, cho phép frontend JavaScript đọc đúng mã lỗi và thời gian chờ để thông báo cho người dùng.

2. **Preflight OPTIONS không tiêu hao Quota**:
   - Trình duyệt tự động gửi preflight request `OPTIONS` trước mọi request non-simple (như `POST` có header `Authorization` hoặc `Content-Type: application/json`).
   - Khi CORS nằm trước Rate Limiter, `go-chi/cors` sẽ chặn request `OPTIONS`, phản hồi ngay lập tức với `200 OK` kèm các headers cấp phép, và **dừng luồng (không gọi `next.ServeHTTP`)**.
   - Nhờ đó, preflight requests **không tiêu tốn bất kỳ token/quota nào** của Rate Limiter.

---

## 4. Preflight & Methods/Headers Configuration

Hệ thống chỉ mở các methods và headers mà ứng dụng thực tế sử dụng:

### 4.1. Allowed Methods
- `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`

### 4.2. Allowed Headers
- `Accept`
- `Authorization` (bắt buộc cho JWT Bearer token)
- `Content-Type` (bắt buộc cho JSON payload)
- `X-CSRF-Token`
- `X-Request-ID` (cho phép frontend gửi custom trace ID)

### 4.3. Exposed Headers
Mặc định trình duyệt không cho phép JavaScript đọc các header tùy chỉnh trong response trừ khi được khai báo trong `Access-Control-Expose-Headers`:
- `Link` (phục vụ phân trang Pagination)
- `Retry-After` (phục vụ Rate Limiting 429)
- `X-Request-ID` (phục vụ truy vết đối soát lỗi giữa frontend và backend)

### 4.4. Preflight Cache Duration (`MaxAge`)
- Đặt `MaxAge: 300` (5 phút). Trình duyệt sẽ cache kết quả preflight cho cùng một URL trong 5 phút, giảm thiểu số lượng HTTP round-trips không cần thiết.

---

## 5. Security Boundary & Production Rules

### 5.1. CORS KHÔNG PHẢI là Authentication hay Authorization

```text
CORS           → Trình duyệt nào được phép đọc dữ liệu trả về?
Authentication → Caller này là ai? (JWT / Basic Auth)
Authorization  → Caller này có quyền làm gì? (RBAC / Ownership)
Rate Limiting  → Caller này được dùng bao nhiêu tài nguyên? (Quota)
```

- CORS là chính sách được thực thi bởi **Trình duyệt (Browser-enforced)**.
- CORS **không ngăn chặn** các cuộc gọi từ:
  - `curl`
  - Postman
  - Backend-to-backend services
  - Scraping bots / automated attack tools
- Vì vậy, không bao giờ dựa vào CORS để bảo vệ các private endpoint khỏi bị truy cập trái phép.

### 5.2. Cấm Wildcard `*` và Dynamic Reflection trong Production

- **Wildcard `*`**: Cho phép mọi trang web trên Internet gọi API và đọc kết quả. Tuyệt đối tránh trong production.
- **Reflected Origin**: Tránh việc đọc header `Origin` từ request và gán thẳng vào `Access-Control-Allow-Origin` mà không kiểm tra qua allowlist.
- **Credentials (`AllowCredentials`)**: Mặc định đặt `false`. API sử dụng Authorization header (Bearer JWT) nên không yêu cầu `withCredentials` (cookie/session). Việc bật `AllowCredentials: true` kèm allowlist lỏng lẻo sẽ tạo ra nguy cơ rò rỉ dữ liệu cực kỳ nguy hiểm.

---

## 6. Configuration & Environment Variables

| Biến môi trường | Kiểu dữ liệu | Mặc định | Mô tả |
| :--- | :--- | :--- | :--- |
| `CORS_ALLOWED_ORIGINS` | string (comma-separated) | `http://localhost:3000` | Danh sách các frontend origins được phép gọi API |

### Ví dụ cấu hình theo môi trường:

```bash
# Local development
CORS_ALLOWED_ORIGINS="http://localhost:3000,http://localhost:5173"

# Staging
CORS_ALLOWED_ORIGINS="https://staging.myapp.com"

# Production
CORS_ALLOWED_ORIGINS="https://myapp.com,https://admin.myapp.com"
```
