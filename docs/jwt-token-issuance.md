# JWT Token Issuance: Stateless Authentication & Security Guardrails

This document details the mental models, architectural boundaries, security guardrails, and implementation patterns for issuing signed JSON Web Tokens (JWT) during user authentication (`POST /v1/authentication/token`).

---

## 1. Core Mental Models

> **Mental core của JWT Token Issuance**:
>
> *Login = đổi credentials thành một signed proof có thời hạn.*

```text
email + password
      ↓
verify identity
      ↓
issue JWT
      ↓
reuse JWT on later requests
```

### 1.1. JWT không phải là User

> [!IMPORTANT]
> **Điểm cần nhớ nhất**:
>
> **JWT không phải là user; nó là một signed statement về user.**
>
> Server không lưu trữ token trong database (stateless authentication). Sau khi đăng nhập thành công, client mang token này trong header `Authorization: Bearer <token>` ở các request tiếp theo để chứng minh danh tính.
>
> *Lưu ý*: Source code hiện tại đang tập trung đúng vào bước **credentials → token issuance**; bước validation middleware bảo vệ các endpoints sẽ được thực hiện ở lesson kế tiếp.

```text
JWT
├─ who?         → sub
├─ valid until? → exp
├─ issued when? → iat
├─ issued by?   → iss
└─ for whom?    → aud
```

### 1.2. Signed vs. Encrypted

```text
JWT
= signed
≠ encrypted
```

> [!CAUTION]
> **Payload của JWT chỉ được mã hóa Base64URL, không hề được mã hóa bảo mật (not encrypted)**. Bất kỳ ai có được token đều có thể decode và đọc được các claims bên trong.
>
> Do đó:
> - **Tuyệt đối không** đưa mật khẩu, secret key, dữ liệu cá nhân nhạy cảm vào claims.
> - Secret key dùng để ký (signing secret) phải được giữ bí mật tuyệt đối trên server.
> - Luôn truyền tải JWT qua kênh mã hóa HTTPS/TLS.

### 1.3. Rút gọn một câu để nhớ:

```text
Authenticate once, then carry a short-lived signed proof.
```

---

## 2. Authentication Flow & Boundary Sequence

```text
HTTP Client
    │  POST /v1/authentication/token {"email": "...", "password": "..."}
    ▼
[internal/api] UserHandler.CreateToken
    │  1. Decode & validate request payload
    ▼
[internal/logic] UserService.AuthenticateUser
    │  2. Lookup user by email (storage.Users.GetByEmail)
    │  3. Anti-timing mitigation (run dummy bcrypt if not found or inactive)
    │  4. Verify password hash using bcrypt.CompareHashAndPassword
    │  5. Ensure user is active (is_active == true)
    │  6. Construct TokenClaims (sub, iss, aud, exp, iat, nbf)
    ▼
[internal/logic] Authenticator interface
    │  7. GenerateToken(claims)
    ▼
[internal/authenticator] JWTAuthenticator
    │  8. Map to jwt.RegisteredClaims & sign with HMAC-SHA256 (HS256)
    ▼
HTTP Client
    ◄─ 200 OK {"data": {"token": "<signed-jwt>"}}
```

---

## 3. Security Guardrails

### 3.1. Unified Credential Failure Semantics (Anti-Enumeration)

> [!IMPORTANT]
> **Rule bảo mật quan trọng**:
>
> ```text
> unknown email
> wrong password
> inactive user
> → same generic 401
> ```
>
> Để triệt tiêu hoàn toàn nguy cơ rò rỉ sự tồn tại của tài khoản trong hệ thống (**user enumeration attack**).

Khi người dùng cung cấp thông tin đăng nhập không hợp lệ, hệ thống tuyệt đối không tiết lộ chi tiết nguyên nhân:

- Email không tồn tại trong hệ thống.
- Tài khoản chưa được kích hoạt (`is_active = false`).
- Mật khẩu cung cấp không chính xác.

Tất cả các trường hợp trên đều trả về cùng một mã trạng thái HTTP và thông báo lỗi tổng quát:

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "error": "invalid credentials"
}
```

### 3.2. Dummy Bcrypt Hash (Anti-Timing Attacks)

Bcrypt là một thuật toán băm chậm có chủ đích (adaptive hash function với work factor `Cost = 12`), tiêu tốn khoảng ~50-100ms CPU để thực hiện phép so sánh.

Nếu hệ thống trả về ngay lập tức khi email không tồn tại nhưng lại mất 70ms khi email tồn tại và mật khẩu sai, kẻ tấn công có thể đo thời gian phản hồi (timing analysis) để suy ra danh sách email người dùng có trong cơ sở dữ liệu (User Enumeration via Timing).

Để triệt tiêu lỗ hổng này, `UserService.AuthenticateUser` luôn thực hiện một phép tính bcrypt giả lập trên một hash mẫu khi user không tồn tại hoặc tài khoản chưa kích hoạt:

```go
// Precomputed valid bcrypt hash at cost 12 to equalize execution timing
var dummyBcryptHash = []byte("$2a$12$e80x6kY15lVUG1Z/fQ5LueG7o3YJm6UvZlDqUfZQd5hL6X5cR0ZWy")

func (s *UserService) AuthenticateUser(ctx context.Context, email, password string) (string, error) {
    user, err := s.repo.GetByEmail(ctx, email)
    if err != nil {
        if errors.Is(err, core.ErrUserNotFound) {
            // Anti-timing mitigation: execute dummy bcrypt check to prevent email enumeration
            _ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(password))
            return "", core.ErrInvalidCredentials
        }
        return "", err
    }

    if !user.IsActive {
        _ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(password))
        return "", core.ErrInvalidCredentials
    }

    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
        if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
            return "", core.ErrInvalidCredentials
        }
        return "", err
    }
    ...
}
```

### 3.3. Key Length Enforcement & Production Guardrails

- `JWTAuthenticator` kiểm tra độ dài secret key ngay khi khởi tạo: tối thiểu **32 bytes** (256-bit entropy) để đảm bảo độ an toàn toán học cho thuật toán HS256.
- Tại `cmd/api/main.go`, môi trường `production` bắt buộc phải cấu hình `JWT_SECRET` riêng biệt; ứng dụng sẽ từ chối khởi động (`logger.Fatalw`) nếu phát hiện sử dụng secret mặc định của môi trường phát triển.

---

## 4. Claims Structure

Hệ thống tuân thủ nghiêm ngặt tiêu chuẩn **RFC 7519** và sử dụng kiểu dữ liệu `jwt.RegisteredClaims`:

| Claim | Tên tiêu chuẩn | Giá trị thiết lập | Mục đích |
| :--- | :--- | :--- | :--- |
| `sub` | Subject | `user.ID` (e.g. `123`) | **who?** — Định danh người dùng sở hữu token |
| `exp` | Expiration Time | `now + JWT_TTL` (mặc định: 60 phút) | **valid until?** — Thời điểm hết hạn của token |
| `iat` | Issued At | `now` | **issued when?** — Thời điểm token được tạo ra |
| `iss` | Issuer | Config `JWT_ISSUER` (mặc định: `golearning`) | **issued by?** — Định danh đơn vị phát hành token |
| `aud` | Audience | Config `JWT_AUDIENCE` (mặc định: `golearning-users`) | **for whom?** — Đối tượng người nhận hợp lệ |
| `nbf` | Not Before | `now` | Thời điểm sớm nhất token có hiệu lực |

Token không chứa các thông tin thay đổi thường xuyên (roles, permissions, profile email/username) nhằm giảm thiểu kích thước payload và tránh tình trạng stale state khi dữ liệu người dùng thay đổi.

---

## 5. Architectural Port & Adapter

Để giữ cho tầng Business Logic (`internal/logic`) độc lập hoàn toàn với thư viện bên thứ ba (`golang-jwt/jwt/v5`), ta áp dụng mô hình Clean Architecture (Hexagonal):

```text
[internal/logic] (Port)
    └─ type Authenticator interface {
           GenerateToken(claims TokenClaims) (string, error)
       }

[internal/authenticator] (Adapter)
    └─ type JWTAuthenticator struct { ... }
           Func (a *JWTAuthenticator) GenerateToken(...) (string, error)
```

`cmd/api/main.go` đóng vai trò Composition Root, thực hiện khởi tạo `JWTAuthenticator` với các tham số từ biến môi trường và inject vào `UserService`.
