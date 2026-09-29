# JWT Authentication Middleware: Bearer Token Validation & Context Propagation

This document details the mental models, architectural boundaries, security guardrails, and implementation patterns for validating incoming Bearer JWT tokens and propagating the authenticated actor to protected HTTP handlers.

---

## 1. Core Mental Models

> **Mental core của JWT Authentication Middleware**:
>
> *JWT middleware biến một token chưa đáng tin thành một identity đáng tin cho request.*

```text
Bearer token
   ↓
validate signature + claims
   ↓
extract sub = user_id
   ↓
fetch current user
   ↓
attach user to context
   ↓
handler uses authenticated actor
```

### 1.1. Decode vs. Validate

> [!IMPORTANT]
> **Điểm quan trọng nhất**:
>
> **Decode token chưa đủ. Chỉ sau validation thì mới được tin claims bên trong.**

```text
decode
→ token đang nói gì? (chưa đáng tin)

validate
→ có nên tin điều nó nói không? (xác thực mật mã)
```

Việc giải mã (decode) payload Base64URL chỉ cho biết token *tự xưng* là ai. Chỉ có quá trình xác thực mật mã (**cryptographic validation**) với server secret key mới chứng minh được tính toàn vẹn và đáng tin cậy của token.

### 1.2. JWT vs. Context: Handler không cần biết JWT

Sau khi qua cửa ngõ middleware, các tầng nghiệp vụ bên trong (handlers, logic) không cần quan tâm đến định dạng hay thư viện JWT nữa:

```text
JWT
→ proves who (bằng chứng số chứng minh danh tính)

Context
→ carries who (vật chứa mang danh tính tin cậy vào trong)
```

Source code dùng đúng pattern này: middleware validate token, trích xuất `sub`, nạp active user từ repository rồi inject thực thể `core.User` vào request context để downstream sử dụng trực tiếp.

### 1.3. Authentication Is Not Authorization

```text
Authentication = who are you?
Authorization  = are you allowed to do this?
```

> [!CAUTION]
> **Một JWT hợp lệ chỉ chứng minh user là ai, chưa chứng minh họ được sửa resource của người khác.**
>
> - **Authentication** (Middleware đảm nhiệm): Xác định actor đang gọi request là `User A`.
> - **Authorization** (Tầng nghiệp vụ / Policy đảm nhiệm): Kiểm tra xem `User A` có quyền cập nhật bài viết của `User B` hay không.

### 1.4. Rút gọn một câu để nhớ:

```text
Validate once at the boundary, then pass trusted identity inward.
```

---

## 2. Architecture & Data Flow

```text
HTTP Request (Authorization: Bearer <jwt>)
       │
       ▼
[internal/gateway] JWTAuthMiddleware
       │
       ├─ 1. Parse header (require "Bearer <token>")
       ├─ 2. Authenticator.ValidateToken(tokenString)
       │         └─ Validate HS256 algorithm, expiry, iss, aud
       ├─ 3. Parse Subject claim ("sub") → int64 userID
       ├─ 4. UserRepository.GetByID(ctx, userID)
       │         └─ Check user.IsActive == true
       ├─ 5. Inject core.User into typed request context
       │
       ▼
[internal/api] Protected Route Group (/v1/posts, /v1/feed, /v1/users/{id}/*)
       │
       ├─ Handlers retrieve actor via api.UserIDFromContext(r.Context())
       │   (or gateway.UserFromContext(r.Context()))
       │
       ▼
[internal/logic] Services (PostService, UserService, FeedService)
       │  Receive verified actor ID
```

---

## 3. Trust Boundary & Removing Temporary Actor IDs

Trước khi có Authentication Middleware, các endpoint phát triển ban đầu (`/posts`, `/feed`, `/users/{id}/follow`, `/users/{id}/unfollow`) sử dụng cơ chế tạm thời:
- Đọc `user_id` qua query parameters (`?user_id=123`).
- Đọc `user_id` qua JSON body payload (`{"user_id": 123}`).
- Fallback về ID mặc định (`1`) cho môi trường dev.

> [!CAUTION]
> **Quy tắc ranh giới tin cậy (Trust Boundary)**:
>
> ```text
> ❌ Client chooses actor identity (Client tự gửi ID của người thực hiện)
> ✅ Authentication middleware establishes actor identity (Hệ thống xác lập danh tính)
> ```

Trong phiên bản này:
1. Hàm `resolveActorID` đã bị **xóa bỏ hoàn toàn**.
2. Toàn bộ fallback về user ID `1` đã bị **loại bỏ**.
3. Các handler `CreatePost`, `Follow`, `Unfollow`, và `GetFeed` bắt buộc phải trích xuất actor từ request context thông qua [`UserIDFromContext`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/post.go#L31-L42). Nếu không có actor trong context, request lập tức bị từ chối với HTTP `401 Unauthorized`.

---

## 4. Security Guardrails

### 4.1. Algorithm Confusion Prevention

`JWTAuthenticator.ValidateToken` cấu hình parser với danh sách thuật toán hợp lệ duy nhất:

```go
opts := []jwt.ParserOption{
    jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
    jwt.WithExpirationRequired(),
    jwt.WithIssuer(a.issuer),
    jwt.WithAudience(a.audience),
}
```

Điều này ngăn chặn kẻ tấn công:
- Đổi thuật toán thành `"none"` (`alg: none`) để vượt qua bước kiểm tra chữ ký.
- Tấn công nhầm lẫn thuật toán đối xứng / bất đối xứng (HMAC vs RSA/ECDSA).

### 4.2. Inactive / Revoked User Rejection

Một token JWT dù có chữ ký hợp lệ và chưa hết hạn vẫn sẽ bị từ chối nếu:
- User tương ứng đã bị xóa khỏi cơ sở dữ liệu (`ErrUserNotFound`).
- Tài khoản đã bị vô hiệu hóa hoặc khóa (`user.IsActive == false`).

### 4.3. Unified Error Semantics

Tất cả các nguyên nhân thất bại:
- Thiếu header `Authorization`
- Header không đúng định dạng `Bearer <token>`
- Token hết hạn, sai chữ ký, sai issuer hoặc audience
- User không tồn tại hoặc `is_active = false`

đều trả về cùng một mã trạng thái HTTP chuẩn kèm challenge header:

```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer
Content-Type: application/json

{
  "error": "unauthorized"
}
```

Không tiết lộ chi tiết lỗi cụ thể ra ngoài client để ngăn ngừa dò quét hệ thống.

---

## 5. Request Context Encapsulation

Để tránh xung đột khóa context giữa các package và thư viện bên thứ ba:
1. Package `gateway` định nghĩa kiểu dữ liệu unexported:
   ```go
   type contextKey string

   const userContextKey contextKey = "authenticated_user"
   ```
2. Cung cấp hai helper tường minh:
   - [`ContextWithUser(ctx, user)`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/gateway/middleware.go#L29-L32)
   - [`UserFromContext(ctx)`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/gateway/middleware.go#L34-L37)
3. Package `api` cung cấp helper domain [`UserIDFromContext(ctx)`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/post.go#L31-L42) để các handler lấy trực tiếp `actorID int64` một cách an toàn.
