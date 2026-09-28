# Basic Authentication Middleware: Tooling & Gateway Protection

This document details the mental models, architectural boundaries, and implementation patterns for the HTTP Basic Authentication middleware guarding internal tooling endpoints.

---

## 1. Core Mental Models

> **Mental core của Basic Auth Middleware**:
>
> *Authentication là một gate ở HTTP boundary: verify trước, business logic chỉ chạy sau khi identity proof hợp lệ.*

```text
Request
  ↓
Auth middleware
  ↓
credentials valid?
├─ no  → 401
└─ yes → handler
```

### 1.1. Cơ chế hoạt động của Basic Auth

```text
username:password
      ↓
Base64 encode
      ↓
Authorization header
      ↓
middleware decode + compare
```

> [!IMPORTANT]
> **Điểm cần nhớ**: Base64 chỉ là **encoding, không phải encryption**, do đó Basic Auth chỉ thực sự an toàn khi chạy qua HTTPS/TLS.
>
> Source implement đúng pattern `middleware → reject / continue` này cho các internal/tooling endpoints như `/health`, `/swagger/*`, `/debug/*` mà không dùng cho user authentication thông thường.

### 1.2. Tooling Boundary vs. Application Users

Basic Auth được định vị **chỉ dành riêng cho internal / tooling endpoints**:

```text
/v1/health
/v1/swagger/*
/v1/debug/*
```

> [!CAUTION]
> **Không sử dụng Basic Auth làm cơ chế xác thực chính cho người dùng ứng dụng**. Luồng người dùng sử dụng Bearer tokens (JWT / Session), trong khi Basic Auth chỉ đóng vai trò chốt chặn cho các công cụ giám sát và tài liệu kỹ thuật.

### 1.3. Rút gọn một câu để nhớ:

```text
Auth guards the boundary; handlers should receive only already-authenticated requests.
```

---

## 2. Sequence & Parsing Flow

```text
Client Request (Authorization: Basic <base64>)
      ↓
Inspect Authorization Header
      ├─ [missing or scheme != "Basic"] ──> 401 Unauthorized
      ↓
Base64 Decode Payload
      ├─ [invalid base64 encoding] ───────> 401 Unauthorized
      ↓
Extract "username:password"
      ├─ [delimiter ':' not found] ───────> 401 Unauthorized
      ↓
SHA-256 Digest & Constant-Time Compare
      ├─ [credentials mismatch] ──────────> 401 Unauthorized
      ↓
Target Tooling Handler (200 OK)
```

---

## 3. Implementation Patterns

### 3.1. Constant-Time Verification (`internal/gateway/middleware.go`)

Để triệt tiêu nguy cơ **Timing Attacks (tấn công phân tích thời gian phản hồi CPU)**, credentials được băm qua SHA-256 trước khi so sánh bằng `crypto/subtle.ConstantTimeCompare`:

```go
func BasicAuthMiddleware(expectedUser, expectedPass string) func(http.Handler) http.Handler {
	expectedUserHash := sha256.Sum256([]byte(expectedUser))
	expectedPassHash := sha256.Sum256([]byte(expectedPass))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expectedUser == "" || expectedPass == "" {
				unauthorizedBasicAuth(w)
				return
			}
			...
			givenUserHash := sha256.Sum256([]byte(username))
			givenPassHash := sha256.Sum256([]byte(password))

			userMatch := subtle.ConstantTimeCompare(givenUserHash[:], expectedUserHash[:]) == 1
			passMatch := subtle.ConstantTimeCompare(givenPassHash[:], expectedPassHash[:]) == 1

			if !userMatch || !passMatch {
				unauthorizedBasicAuth(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

### 3.2. Response Challenge & Error Envelope

Khi từ chối yêu cầu, middleware tuân thủ chuẩn RFC HTTP Basic Auth và envelope JSON hiện có của hệ thống:

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json
WWW-Authenticate: Basic realm="Restricted", charset="UTF-8"

{"error":"unauthorized"}
```

### 3.3. Route Grouping Isolation (`internal/api/router.go`)

Middleware không bọc toàn bộ ứng dụng mà được gán vào group riêng:

```go
// Tooling route group: protected by Gateway BasicAuthMiddleware
v1.Group(func(tooling chi.Router) {
	tooling.Use(basicAuthMiddleware)

	tooling.Get("/health", HealthHandler)
	tooling.Get("/swagger/*", httpSwagger.WrapHandler)
	tooling.Get("/debug/panic", ...)
})

// Public route group: registration & activation (no auth required)
v1.Group(func(public chi.Router) {
	public.Route("/authentication", ...)
})

// Private route group: user endpoints (Bearer auth required)
v1.Group(func(private chi.Router) {
	private.Use(gateway.AuthMiddleware)
	...
})
```

---

## 4. Security Guardrails

1. **Transport Layer Security**: Basic Auth truyền credential dạng Base64 trên wire; **bắt buộc chạy qua HTTPS / TLS termination** trong môi trường production.
2. **Fail-Closed on Unset Config**: Nếu `AUTH_BASIC_USER` hoặc `AUTH_BASIC_PASS` không được cấu hình (rỗng), middleware từ chối mọi yêu cầu thay vì cho phép lọt qua.
3. **Audit Log Privacy**: Tuyệt đối không ghi log plain password hoặc toàn bộ header `Authorization` vào hệ thống structured logging.
4. **Generic Error Message**: Thông điệp lỗi luôn là `unauthorized`, không tiết lộ username sai hay password sai.
