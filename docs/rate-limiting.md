# API Rate Limiting Architecture & Specification

This document details the architectural design, mental models, implementation patterns, and production boundaries for API Rate Limiting in the Go backend.

---

## 1. Core Mental Models

> **Mental core của Rate Limiting**:
>
> *Rate limiting = decide who owns a quota, how much they can consume, and reject excess traffic before it reaches expensive backend work.*
>
> *(Phân bổ hạn ngạch cho từng đối tượng client, xác định họ được tiêu thụ bao nhiêu, và chặn đứng lưu lượng vượt quá trước khi nó chạm tới các tác vụ backend tốn kém).*

### 1.1. Luồng xử lý Rate Limiting

```text
Request
  ↓
Identify consumer
  ↓
Check quota
  ├─ available → allow
  └─ exhausted → 429 + Retry-After
```

Mô hình Fixed Window của source (mặc định 20 requests / 5s):

```text
20 requests / 5s

request 1..20
→ allow

request 21+
→ reject

after 5s
→ reset
```

### 1.2. Mental thứ hai: Key định danh đối tượng tiêu thụ

> *The rate-limit key defines whose usage you're counting.*

Hạn ngạch luôn gắn liền với một danh tính cụ thể:

```text
Whose quota?
 ├─ IP           (client network identity — learning / public routes)
 ├─ User ID      (authenticated users)
 ├─ API key      (developer / third-party integration)
 └─ Tenant ID    (multi-tenant B2B systems)
```

Source hiện tại sử dụng client IP làm key định danh.

### 1.3. Mental thứ ba khi scale: Vị trí của Rate-Limit State

> *Rate-limit state must live wherever all instances can see it.*

```text
1 server (Single Instance)
→ local memory works (in-memory map + mutex)

many servers (Distributed Cluster)
→ local counters diverge
→ need shared state, e.g. Redis
```

Source cũng chỉ rõ limitation này: bộ nhớ cục bộ chỉ hoạt động đúng khi chạy một instance duy nhất. Khi triển khai cụm phân tán, state đếm quota bắt buộc phải chia sẻ tập trung (như Redis).

### 1.4. Rút gọn trong một câu (Core Takeaway)

> *Protect shared capacity by giving each consumer a bounded quota, and keep that quota state shared when the system is distributed.*

---

### 1.5. Vị trí trong kiến trúc (Cross-Cutting Gateway Infrastructure)

Rate limiting là mối quan tâm xuyên suốt (cross-cutting HTTP concern) thuộc về tầng **Gateway Middleware**:

```text
Request
  ↓
Real IP / RemoteAddr
  ↓
Rate Limit Middleware (internal/gateway)
  ↓
API Route Handlers (internal/api)
  ↓
Domain Logic (internal/logic)
  ↓
Repository / Database (internal/repository)
```

**Boundary Rules**:
- **Không đặt logic rate limiting vào**: `internal/core`, `internal/logic`, hay `internal/repository`.
- Middleware chỉ tương tác thông qua abstraction interface `gateway.RateLimiter`.
- Server composition root (`cmd/api/main.go`) chịu trách nhiệm khởi tạo implementation và inject vào router.

---

## 2. RateLimiter Interface & Contracts

Để bảo đảm tính linh hoạt và khả năng thay thế (e.g., chuyển từ in-memory sang Redis trong tương lai), interface `RateLimiter` được định nghĩa trong `internal/gateway/limiter.go`:

```go
type RateLimiter interface {
	// Allow checks whether a request associated with key is permitted within the rate limit.
	// If permitted, it returns true and 0.
	// If quota is exhausted, it returns false and the duration until the quota resets.
	Allow(ctx context.Context, key string) (bool, time.Duration)
}
```

### 2.1. Thứ tự Middleware Stack

Trong `internal/api/router.go`, middleware rate limit được đặt sau `RequestIDMiddleware` và trước các tầng tốn kém tài nguyên:

```text
1. RecoveryMiddleware      (bắt panic ở tầng ngoài cùng)
   ↓
2. RequestIDMiddleware     (gán X-Request-ID cho mọi request)
   ↓
3. RateLimitMiddleware     (chặn sớm nếu vượt quota, trả 429 kèm X-Request-ID)
   ↓
4. LoggingMiddleware       (ghi log thời gian xử lý cho request hợp lệ)
   ↓
5. TimeoutMiddleware       (áp đặt deadline 10s cho request hợp lệ)
```

**Lợi ích của thứ tự này**:
- Khi request bị rate limit (429), client vẫn nhận được header `X-Request-ID` để đối soát.
- Hệ thống từ chối ngay lập tức, không tốn tài nguyên khởi tạo timer timeout, xử lý routing chi tiết, phân quyền hay truy vấn cơ sở dữ liệu.

---

## 3. Fixed Window V1 Implementation

### 3.1. Thuật toán Fixed Window

Mỗi client key (IP address) được lưu trữ trạng thái hạn ngạch trong bộ nhớ:

```go
type clientQuota struct {
	count     int
	windowEnd time.Time
}
```

Khi có request mới:
1. **Chưa có record hoặc cửa sổ thời gian đã hết hạn (`now.After(quota.windowEnd)`):**
   - Khởi tạo cửa sổ mới bắt đầu từ `now` đến `now + window`.
   - Thiết lập `count = 1`.
   - Cho phép request (`allowed = true, retryAfter = 0`).
2. **Trong cửa sổ hiện tại và `count < limit`:**
   - Tăng `count++`.
   - Cho phép request (`allowed = true, retryAfter = 0`).
3. **Trong cửa sổ hiện tại và `count >= limit`:**
   - Từ chối request (`allowed = false`).
   - Tính toán thời gian cần chờ: `retryAfter = quota.windowEnd.Sub(now)`.

### 3.2. Cơ chế chống rò rỉ bộ nhớ (Memory Leak Protection)

**Vấn đề**: Trong ứng dụng HTTP thực tế, số lượng IP client truy cập là vô hạn. Nếu chỉ lưu vào `map[string]*clientQuota` mà không dọn dẹp, map sẽ phình to liên tục theo thời gian dẫn đến OOM (Out Of Memory).

**Giải pháp**:
- `FixedWindowLimiter` khởi chạy một background goroutine ticker định kỳ (mặc định mỗi 1 phút hoặc theo độ dài window).
- Goroutine quét map dưới sự bảo vệ của `mu.Lock()` và xóa sạch các bản ghi có `now.After(quota.windowEnd)`.
- Khi server tắt, hàm `Stop()` sử dụng `sync.Once` để đóng channel dừng cleaner một cách an toàn.

```go
func (l *FixedWindowLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for key, quota := range l.clients {
		if now.After(quota.windowEnd) {
			delete(l.clients, key)
		}
	}
}
```

---

## 4. HTTP Rejection & Observability Contract

Khi client vượt quá hạn mức, hệ thống tuân thủ nghiêm ngặt các tiêu chuẩn HTTP:

### 4.1. Response Headers & Body

- **HTTP Status Code**: `429 Too Many Requests`
- **Header**: `Retry-After: <seconds>` (số nguyên giây làm tròn lên, tối thiểu 1s theo RFC 6585 / RFC 7231)
- **JSON Error Body**:
  ```json
  {
    "error": "too many requests"
  }
  ```

### 4.2. Structured Logging

Mọi vi phạm rate limit đều được ghi log dạng structured warning kèm mã `request_id` và IP vi phạm:

```json
{
  "level": "warn",
  "msg": "rate limit exceeded",
  "request_id": "req-1727572800000000000",
  "ip": "192.168.1.100",
  "method": "POST",
  "path": "/v1/authentication/token",
  "retry_after_seconds": 3
}
```

---

## 5. Configuration & Environment Variables

Rate limiting được điều khiển hoàn toàn thông qua cấu hình môi trường, không hardcode:

| Biến môi trường | Kiểu dữ liệu | Mặc định | Mô tả |
| :--- | :--- | :--- | :--- |
| `RATE_LIMIT_ENABLED` | bool | `false` | Bật (`true`) hoặc tắt (`false`) middleware rate limit |
| `RATE_LIMIT_REQUESTS`| int | `20` | Số lượng request tối đa được phép trong một cửa sổ |
| `RATE_LIMIT_WINDOW`  | duration / int | `5s` | Thời gian của một cửa sổ (hỗ trợ `5s`, `1m` hoặc số giây nguyên `5`) |

Khi `RATE_LIMIT_ENABLED=false`, `rateLimiter` được gán `nil`, middleware trở thành pass-through không làm suy giảm hiệu năng.

---

## 6. Distributed Production Boundary & Future Roadmap

### 6.1. Giới hạn của In-Memory Limiter (Single-Instance vs Multi-Instance)

```text
Client
  ↓
Load Balancer
 ├─ Pod A (in-memory map) → local count = 20 (bị chặn)
 ├─ Pod B (in-memory map) → local count = 3  (vẫn cho qua)
 └─ Pod C (in-memory map) → local count = 0  (vẫn cho qua)
```

- Phiên bản in-memory V1 hoàn toàn chính xác cho một single application instance hoặc môi trường test / dev.
- **Ranh giới sản xuất phân tán**: Khi hệ thống scale ngang nhiều pod đằng sau load balancer, bộ nhớ in-memory của các pod bị cô lập, dẫn đến việc client có thể gửi tổng số request gấp $N$ lần hạn mức cho phép.
- **Giải pháp cho Multi-Instance**:
  ```text
  API Pod 1 \
  API Pod 2 ──→ Shared Redis Cluster (Atomic INCR / Sliding Window / Lua Script)
  API Pod 3 /
  ```
  Nhờ thiết kế `RateLimiter` interface, ta có thể bổ sung `RedisSlidingWindowLimiter` mà không phải thay đổi một dòng code nào trong `RateLimitMiddleware` hay các API handlers.

### 6.2. Định danh người dùng (Consumer Identity)

- **V1 (Hiện tại)**: Sử dụng IP từ `net.SplitHostPort(r.RemoteAddr)` để ngăn chặn tấn công giả mạo header khi chưa có trusted proxy.
- **V2 (Tương lai)**: Mở rộng keying theo danh tính:
  - **Authenticated Routes**: Sử dụng `user_id` từ JWT context.
  - **B2B / API Partner**: Sử dụng `API-Key` hoặc `tenant_id`.
  - **Tier-based Limiting**: Hạn mức khác nhau cho Free tier vs Pro/Enterprise tier.
