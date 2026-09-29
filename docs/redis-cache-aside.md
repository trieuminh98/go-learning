# Redis Cache-Aside Architecture

This document describes the design, configuration, failure tolerance, and cache invalidation policies for the Redis cache-aside layer accelerating user reads.

---

## 1. Core Mental Models

> **Mental core của Cache-Aside**:
>
> *Cache is a fast, disposable copy of source-of-truth data — use it on reads, fall back on miss, and never let it become required for correctness.*
>
> *(Cache là một lớp tăng tốc nằm trước source of truth; hit thì trả nhanh, miss thì fallback DB rồi fill cache. Cache miss không phải là lỗi, và tính đúng đắn của ứng dụng không bao giờ được phụ thuộc vào Redis).*

```text
Cache
→ acceleration (fast, disposable)

PostgreSQL
→ source of truth (authoritative, persistent)
```

### 1.1. Luồng đọc dữ liệu Cache-Aside (HIT vs MISS)

```text
READ
  ↓
Cache
├─ HIT  → return
└─ MISS → DB → SET cache → return
```

Chi tiết luồng thực thi trong `UserService.GetUser`:

```text
GetUser(id)
   ↓
Redis GET user:{id}
   ↓
┌───────────┐
│ hit?      │
├─────┬─────┤
│ yes │ no  │
↓     ↓
return PostgreSQL
        ↓
     fetch user
        ↓
     SET Redis user:{id} + TTL
        ↓
       return
```

### 1.2. Phân định quan trọng: Cache Miss ≠ Error

```text
redis.Nil
→ "chưa cache"
→ tiếp tục xuống DB

chứ không phải:
redis.Nil
→ request fail
```

- **Cache Hit**: Dữ liệu tồn tại trong Redis, giải nén JSON và trả về `core.User` trực tiếp mà không cần truy vấn DB.
- **Cache Miss**: Redis trả về `redis.Nil`, đây là trạng thái bình thường ("chưa cache"); hệ thống tiếp tục truy vấn PostgreSQL và nạp lại cache.
- **Cache Error**: Lỗi mạng, timeout, hoặc Redis sập; hệ thống ghi log cảnh báo (warning) và tự động fallback về PostgreSQL, tuyệt đối không trả về `500` hoặc làm gián đoạn request.

### 1.3. Nguyên lý cốt lõi: Cache phải Optional (App Correctness độc lập với Redis)

Ứng dụng phải hoạt động đúng đắn ngay cả khi không có Redis:

```text
Redis ON
→ cache-aside

Redis OFF
→ DB trực tiếp
→ app vẫn chạy bình thường
```

- Hệ thống hỗ trợ cờ cấu hình `REDIS_ENABLED`: khi `false`, toàn bộ luồng đọc truy vấn trực tiếp PostgreSQL.
- Khi `REDIS_ENABLED=true` nhưng Redis server offline hoặc bị crash trong runtime, ứng dụng tự động chuyển tiếp sang PostgreSQL mà không ngắt quãng kết nối của client.

### 1.4. Bảng phân định vai trò: Cache vs Source of Truth

| Thuộc tính | Redis (Cache) | PostgreSQL (Database) |
| :--- | :--- | :--- |
| **Vai trò** | Tầng gia tốc tạm thời (*acceleration*) | Nguồn dữ liệu chuẩn duy nhất (*source of truth*) |
| **Tính bền vững** | Bản sao dùng một lần, dễ xóa bỏ (*disposable copy*) | Bền vững và đảm bảo toàn vẹn (*persistent ACID*) |
| **Khi gặp sự cố** | Ghi nhận cảnh báo và fallback xuống DB | Báo lỗi hệ thống (không có tầng thay thế) |
| **Tính đúng đắn (*Correctness*)** | Không quyết định tính đúng đắn của dữ liệu | Đảm bảo tính đúng đắn và bất biến của dữ liệu |

---

## 2. Architecture & Layer Boundaries

```text
Application
├─ UserRepository
│   └─ PostgreSQL / sqlc (Source of truth)
│
└─ UserCache
    └─ Redis / go-redis (Optional acceleration)
```

### 2.1. Tách bạch Core Entity và Serialization DTO

Domain entity `core.User` giữ nguyên sự tinh gọn, không bị phụ thuộc vào Redis types hay JSON struct tags:

```go
// internal/logic/repository.go
type UserCache interface {
    Get(ctx context.Context, userID int64) (*core.User, error)
    Set(ctx context.Context, user *core.User, ttl time.Duration) error
    Delete(ctx context.Context, userID int64) error
}

type UserGetter interface {
    GetByID(ctx context.Context, id int64) (core.User, error)
}
```

Tầng adapter `internal/repository/redis` đóng gói DTO nội bộ `cachedUserDTO` để quản lý serialization JSON và bảo toàn hash mật khẩu an toàn:

```go
type cachedUserDTO struct {
    ID           string `json:"id"`
    FirstName    string `json:"first_name"`
    LastName     string `json:"last_name"`
    Email        string `json:"email"`
    PasswordHash []byte `json:"password_hash,omitempty"`
    IsActive     bool   `json:"is_active"`
    RoleID       int64  `json:"role_id"`
    RoleName     string `json:"role_name"`
    RoleLevel    int    `json:"role_level"`
    RoleDesc     string `json:"role_desc"`
}
```

### 2.2. Điểm đọc tập trung (Centralized Read Path)

Tất cả các luồng đọc user theo ID đều đi qua `UserService.GetUser`:
1. `GET /v1/users/{id}` (Handler): Gọi `UserService.GetUser`.
2. `JWTAuthMiddleware` (Gateway): Nhận `logic.UserGetter` (được thỏa mãn bởi `*logic.UserService`), tự động tái sử dụng tầng cache.

---

## 3. Configuration & Runtime Flags

| Biến môi trường | Kiểu dữ liệu | Mặc định | Mô tả |
| :--- | :--- | :--- | :--- |
| `REDIS_ENABLED` | boolean | `false` | Bật/tắt tầng cache Redis. Khi `false`, hoàn toàn bỏ qua Redis. |
| `REDIS_ADDR` | string | `localhost:6379` | Địa chỉ kết nối Redis server. |
| `REDIS_PASSWORD`| string | `""` | Mật khẩu xác thực Redis (nếu có). |
| `REDIS_DB` | int | `0` | Chỉ số database Redis. |
| `CACHE_USER_TTL`| duration | `15m` | Thời gian sống (TTL) của cache user (hỗ trợ `"15m"`, `"1h"`, hoặc số giây). |

### Hành vi bật/tắt (Feature Toggle):

```text
REDIS_ENABLED=false
→ bỏ qua Redis
→ ứng dụng chạy bình thường với PostgreSQL trực tiếp

REDIS_ENABLED=true
→ kết nối Redis, kiểm tra ping
→ nếu Redis offline khi khởi động: log warning, vô hiệu hóa cache, tiếp tục chạy bình thường
→ nếu Redis online: kích hoạt cache-aside
```

---

## 4. Invalidation Strategy (Chiến lược làm sạch Cache)

Để ngăn chặn hiện tượng dữ liệu cũ (stale data), mọi thao tác làm biến đổi trạng thái tài nguyên user đều thực hiện xóa cache tương ứng:

```text
User Mutation
      ↓
DB Transaction Commit (PostgreSQL)
      ↓
Cache Invalidation (DEL user:{id})
```

- **Kích hoạt tài khoản (`ActivateUser`)**: Sau khi commit cập nhật `is_active = true` thành công trong database, `UserService` gọi `cache.Delete(ctx, userID)` để xóa bản ghi cũ trước khi kích hoạt.
- **Bồi hoàn đăng ký (`compensateRegistration`)**: Xóa user khỏi cache nếu bước gửi email bị lỗi.
- **Thực hiện ngoài transaction**: Thao tác xóa cache luôn được thực thi sau khi transaction cơ sở dữ liệu đã commit thành công, loại bỏ rủi ro cache bị xóa oan khi transaction rollback.

---

## 5. Resilience & Graceful Degradation (Khả năng chịu lỗi)

- **Network Timeout Bounded**: Client Redis cấu hình `ReadTimeout: 150ms` và `WriteTimeout: 150ms` để ngăn chặn các request HTTP bị nghẽn (hang) nếu Redis phản hồi chậm.
- **Fail-open Policy**: Sự cố tại tầng cache (connection dropped, timeout, OOM) được ghi nhận qua logger mức cảnh báo (`Warnw`) và tự động fallback về truy vấn trực tiếp PostgreSQL. Người dùng cuối không bao giờ nhận mã lỗi `500 Internal Server Error` do lỗi Redis.

---

## 6. Acceptance & Verification

1. **Khi tắt cache (`REDIS_ENABLED=false`)**:
   - Ứng dụng khởi động bình thường, thông báo `redis cache is disabled (REDIS_ENABLED=false)`.
   - Các API đăng ký, kích hoạt, đọc thông tin và xác thực JWT hoạt động ổn định.
2. **Khi bật cache (`REDIS_ENABLED=true`)**:
   - Lượt đọc đầu tiên: Truy vấn cơ sở dữ liệu và ghi vào key `user:{id}` với TTL tương ứng.
   - Các lượt đọc tiếp theo: Đọc trực tiếp từ key `user:{id}` mà không phát sinh truy vấn PostgreSQL.
   - Khi kích hoạt tài khoản: Key `user:{id}` bị xóa để đảm bảo request tiếp theo tải trạng thái active mới nhất.
3. **Khi Redis gặp sự cố**:
   - Request người dùng vẫn nhận phản hồi thành công nhờ cơ chế tự động fallback về PostgreSQL.
