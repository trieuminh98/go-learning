# Operational Metrics with `expvar` Architecture & Specification

This document details the architectural design, mental models, implementation patterns, and security boundaries for operational metrics using Go's standard-library `expvar` in the backend service.

---

## 1. Core Mental Models

> **Mental core của Observability & Metrics**:
>
> *Observability = biến trạng thái bên trong application thành signal có thể quan sát từ bên ngoài.*
>
> *(Biến runtime state vô hình của tiến trình thành các tín hiệu số liệu cụ thể để người vận hành hoặc hệ thống giám sát có thể đo lường và theo dõi).*

### 1.1. Luồng xử lý Observability

```text
Runtime state
   ↓
Instrumentation
   ↓
Metrics endpoint
   ↓
Operator / Monitoring
```

Với `expvar`, luồng dữ liệu là:

```text
Go runtime
├─ memory (heap, stack)
├─ GC (pause, frequency)
├─ goroutines (concurrency count)
├─ DB pool stats (max, total, idle, acquired)
└─ custom metrics (version)
        ↓
   /debug/vars
```

### 1.2. Mental thứ hai: Operational Interface vs Business API

Metrics endpoint là giao diện vận hành, không phải API nghiệp vụ:

```text
Business API (Public / End-Users)
→ users / posts / feed

Operations API (Internal / Operators)
→ health / metrics / debug
```

Các endpoint vận hành phải luôn được bảo vệ nghiêm ngặt (ví dụ: HTTP Basic Authentication) để không làm rò rỉ thông tin hạ tầng nội bộ ra ngoài Internet.

### 1.3. Mental cuối cùng: Instrumentation vs Monitoring

Cần phân định rạch ròi giữa việc tạo tín hiệu và hệ thống giám sát:

> *Instrumentation chỉ tạo signal; monitoring mới collect, visualize và alert.*

```text
Application
  ↓ instrument
expose metrics (/debug/vars)
  ↓ collect (Prometheus / Poller / Agent)
dashboard (Grafana) / alert (Alertmanager / PagerDuty)
```

`expvar` đóng vai trò là tầng **instrumentation & export bề mặt thô (raw metrics surface)**. Bản thân ứng dụng không gánh vác trách nhiệm lưu trữ time-series, dựng đồ thị, hay gửi thông báo cảnh báo.

### 1.4. Rút gọn trong một câu (Core Takeaway)

> *Make the running system observable by exposing useful internal signals through a controlled operational interface.*

---

## 2. Architecture & Layer Boundaries

Metrics thuộc về tầng **Runtime / Operations Infrastructure**, không thuộc về Domain Logic:

```text
cmd/api (Composition Root)
   ↓ Khởi tạo dbPool, load config
internal/gateway/metrics.go
   ↓ RegisterMetrics(version, pool) [sync.Once]
internal/api/router.go
   ↓ BasicAuthMiddleware ──→ expvar.Handler()
/debug/vars & /v1/debug/vars
```

**Nguyên tắc ranh giới (Boundary Rules)**:
- **Tuyệt đối không đưa `expvar` vào**: `internal/core`, `internal/logic`, hay `internal/repository`. Các tầng nghiệp vụ hoàn toàn không biết đến sự tồn tại của metrics exporter.
- **Đăng ký tường minh (Explicit Registration)**: Tuyệt đối không dùng blank import `_ "expvar"`. Blank import sẽ âm thầm gắn handler vào `http.DefaultServeMux` mà không có lớp bảo vệ xác thực, tạo lỗ hổng bảo mật nghiêm trọng.

---

## 3. Metrics Taxonomy & Dynamic Evaluation

Hệ thống phân tách rõ rệt giữa chỉ số tĩnh (static) và chỉ số tính toán động (dynamic):

| Chỉ số | Kiểu đánh giá | Nguồn dữ liệu | Mục đích vận hành |
| :--- | :--- | :--- | :--- |
| `cmdline` | Built-in | `os.Args` | Xác định cờ khởi động của tiến trình |
| `memstats` | Built-in | `runtime.MemStats` | Giám sát Heap, Stack, tần suất và thời gian GC |
| `version` | Static-ish | `Config.Version` (`APP_VERSION`) | Nhận diện phiên bản bản build đang chạy |
| `goroutines` | Dynamic | `runtime.NumGoroutine()` | Theo dõi biến thiên concurrency, phát hiện rò rỉ goroutine |
| `database` | Dynamic | `pool.Stat()` (`*pgxpool.Pool`) | Giám sát độ bão hòa connection pool theo thời gian thực |

### 3.1. Dynamic Evaluation của Database Connection Pool

Chỉ số database phản ánh áp lực tải tức thời lên PostgreSQL pool:

```go
expvar.Publish("database", expvar.Func(func() any {
    if pool == nil {
        return map[string]any{"status": "uninitialized"}
    }
    stat := pool.Stat()
    return map[string]any{
        "max_conns":      stat.MaxConns(),
        "total_conns":    stat.TotalConns(),
        "idle_conns":     stat.IdleConns(),
        "acquired_conns": stat.AcquiredConns(),
    }
}))
```

- **`max_conns`**: Giới hạn kết nối tối đa cấu hình cho pool.
- **`total_conns`**: Tổng số kết nối TCP hiện đang được mở tới PostgreSQL.
- **`idle_conns`**: Số kết nối rảnh rỗi sẵn sàng phục vụ query mới.
- **`acquired_conns`**: Số kết nối đang được các transaction/query nghiệp vụ chiếm dụng.

### 3.2. Chống Panic với `sync.Once`

Hàm `expvar.Publish(name, v)` trong Go standard library sẽ gọi thẳng `log.Panic` nếu key đã được đăng ký trước đó. Do đó, hàm `gateway.RegisterMetrics` được bảo vệ bằng `sync.Once`:

```go
var registerMetricsOnce sync.Once

func RegisterMetrics(version string, pool *pgxpool.Pool) {
    registerMetricsOnce.Do(func() {
        // đăng ký các metrics...
    })
}
```

Điều này triệt tiêu hoàn toàn rủi ro crash server khi chạy test hoặc tái khởi tạo dependency nhiều lần.

---

## 4. Security Boundary & Production Rules

1. **Bảo vệ bằng Basic Auth**:
   - `/debug/vars` để lộ cấu trúc bộ nhớ, thời gian GC và số lượng goroutines. Bắt buộc phải bọc qua `BasicAuthMiddleware` (sử dụng `AUTH_BASIC_USER` và `AUTH_BASIC_PASS`).
   - Endpoint trả về `401 Unauthorized` kèm header `WWW-Authenticate: Basic` nếu client chưa xác thực.

2. **Tuyệt đối không xuất dữ liệu nhạy cảm**:
   - Không xuất chuỗi kết nối cơ sở dữ liệu (`DB_URL`).
   - Không xuất JWT secret, Redis password, hay API keys.
   - Không xuất thông tin định danh người dùng (PII) hay request body.

3. **Chống suy giảm hiệu năng do Polling dồn dập**:
   - `/debug/vars` được đặt sau `RateLimitMiddleware`, ngăn chặn các công cụ quét tự động gửi hàng nghìn request/giây gây nghẽn CPU khi đọc `memstats`.
   - Hàm dynamic của database chỉ đọc snapshot bộ nhớ cục bộ từ `pool.Stat()` (atomic counter), không thực hiện bất kỳ network round-trip nào tới PostgreSQL.

---

## 5. Verification & Acceptance Guide

### 5.1. Kiểm tra xác thực (Rejection without Credentials)

```bash
curl -i http://localhost:8080/debug/vars
```

**Kết quả kỳ vọng**:
```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Basic realm="Restricted", charset="UTF-8"
Content-Type: application/json

{"error":"unauthorized"}
```

### 5.2. Đọc Metrics với Basic Auth hợp lệ

```bash
curl -i -u admin:admin http://localhost:8080/debug/vars
# hoặc qua route versioned:
curl -i -u admin:admin http://localhost:8080/v1/debug/vars
```

**Kết quả kỳ vọng**: `200 OK` kèm JSON:
```json
{
  "cmdline": ["./bin/api"],
  "database": {
    "acquired_conns": 0,
    "idle_conns": 2,
    "max_conns": 10,
    "total_conns": 2
  },
  "goroutines": 8,
  "memstats": {
    "Alloc": 2541360,
    "TotalAlloc": 5120480,
    "Sys": 15482928,
    "NumGC": 3
  },
  "version": "1.0.0"
}
```

### 5.3. Quan sát biến thiên động khi có tải

Khi kích hoạt request chậm mô phỏng hoặc gửi traffic dồn dập:
- `goroutines` tăng lên tương ứng với số lượng request đồng thời.
- `database.acquired_conns` tăng lên phản ánh số connection đang xử lý query.
- Sau khi request hoàn tất, `acquired_conns` trở về 0 và `idle_conns` sẵn sàng tái sử dụng.
