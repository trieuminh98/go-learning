# Graceful Shutdown Architecture & Server Lifecycle

This document describes the design, mental models, runtime architecture, and verification procedures for graceful shutdown in the API server.

---

## 1. Core Mental Models

> **Mental core của Graceful Shutdown**:
>
> *Treat shutdown as a controlled lifecycle phase, not an abrupt crash.*
>
> *(Server không nên “chết ngay”; nó nên ngừng nhận việc mới, cho việc đang chạy một khoảng thời gian để hoàn tất, rồi mới exit. Termination là một phần bình thường của server lifecycle — đặc biệt trong môi trường Kubernetes hay autoscaling).*

### 1.1. Luồng xử lý Graceful Shutdown

```text
SIGTERM / SIGINT
      ↓
stop accepting new requests
      ↓
drain in-flight requests
      ↓
shutdown deadline
      ↓
exit
```

> Hệ thống hiện thực hóa bằng sự kết hợp chặt chẽ giữa **OS signal handling** (`signal.Notify`) và **`http.Server.Shutdown(ctx)`** kèm context deadline hữu hạn.

### 1.2. Termination là một phần bình thường của Server Lifecycle

```text
Start
  ↓
Serve
  ↓
Termination signal (SIGINT / SIGTERM)
  ↓
Drain
  ↓
Exit (code 0)
```

- Trong các hệ thống container hiện đại (Kubernetes rolling deployment, HPA autoscaling, pod eviction), tiến trình bị restart hoặc replace là chuyện diễn ra hàng ngày.
- Một server đạt chuẩn production phải coi việc terminate là một giai đoạn có kiểm soát trong vòng đời ứng dụng, không phải là một sự cố sập bất thường.

### 1.3. Ranh giới cốt lõi: Graceful Shutdown vs. Background Worker

```text
Graceful shutdown
→ chỉ bảo vệ in-flight work trong một grace period (5s)
≠ không thay thế queue / worker cho long-running work
```

- Graceful shutdown chỉ bảo đảm các HTTP requests đang chạy dở (vài millisecond đến vài giây) có đủ thời gian hoàn tất nghiệp vụ và trả về kết quả cho client.
- Nếu tác vụ kéo dài (gửi hàng loạt email, encode video, export file nặng), bắt buộc phải tách sang message queue (RabbitMQ, Kafka) hoặc background worker, không được phụ thuộc vào HTTP request path.

### 1.4. So sánh hành vi: Đột ngột vs. Dừng an toàn

```text
BEFORE (Abrupt Exit / Mặc định):
Request in-flight
       ↓
SIGTERM / SIGINT
       ↓
Process killed ngay lập tức
       ↓
Client bị ngắt kết nối đột ngột (broken pipe / connection reset / 502 Bad Gateway)

AFTER (Graceful Shutdown):
Request in-flight
       ↓
SIGTERM / SIGINT
       ↓
Bắt đầu tiến trình shutdown (ngừng nhận request mới)
       ↓
In-flight request tiếp tục được xử lý và hoàn tất
       ↓
Trả về phản hồi 200 OK cho client
       ↓
Đóng an toàn các kết nối DB & Redis
       ↓
Tiến trình dừng sạch sẽ (exit 0)
```

---

## 2. Architecture & Layer Boundaries

Shutdown handling thuộc về **tầng runtime composition root (`cmd/api`)**, gắn liền với vòng đời của `http.Server`.

```text
cmd/api/main.go (Composition Root)
├─ 1. Khởi tạo DB pool & Redis client (defer Close())
├─ 2. Khởi tạo Logic Services & Chi Router
├─ 3. Application.run()
│     ├─ Background goroutine: srv.ListenAndServe()
│     ├─ Buffered signal channel: SIGINT & SIGTERM
│     ├─ Select: Lỗi khởi động vs Tín hiệu dừng
│     └─ srv.Shutdown(ctx) với timeout hữu hạn
└─ 4. Khi app.run() trả về: các lệnh defer Close() mới được giải phóng
```

> [!IMPORTANT]
> **Tuyệt đối không đưa signal handling hoặc shutdown logic vào các layer nghiệp vụ**:
>
> - `API handlers` (chỉ xử lý HTTP request/response)
> - `Logic layer` (chỉ chứa use-cases và business rules)
> - `Core layer` (chỉ chứa pure domain entities)
> - `Repository layer` (chỉ chứa persistence queries)

---

## 3. Thứ tự thu hồi tài nguyên (Resource Destruction Sequence)

Một trong những rủi ro nghiêm trọng nhất trong graceful shutdown là **đóng kết nối database trước khi in-flight requests xử lý xong**, khiến các request đang chạy dở bị lỗi truy vấn cơ sở dữ liệu.

Thứ tự dọn dẹp được đảm bảo tuyệt đối:

```text
1. Nhận tín hiệu SIGINT/SIGTERM
2. srv.Shutdown(ctx)
   ├── Ngừng tiếp nhận kết nối TCP mới
   ├── Đóng các kết nối idle
   └── Chờ các request in-flight hoàn thành (dùng chung DB & Redis bình thường)
3. srv.Shutdown(ctx) hoàn thành và return
4. Application.run() return
5. Hàm main() kết thúc, kích hoạt tuần tự:
   ├── defer rdb.Close()   (Giải phóng kết nối Redis)
   └── defer pool.Close()  (Giải phóng kết nối PostgreSQL)
```

---

## 4. Runtime Configuration

Thời gian chờ tối đa để drain request được quản lý qua biến môi trường:

| Biến môi trường | Kiểu dữ liệu | Mặc định | Mô tả |
| :--- | :--- | :--- | :--- |
| `SHUTDOWN_TIMEOUT_SECONDS` | integer | `5` | Thời gian tối đa (giây) server chờ các in-flight requests hoàn thành trước khi buộc đóng kết nối. |
| `SERVER_ADDR` | string | `:8080` | Địa chỉ mạng và port server lắng nghe. |
| `SERVER_READ_TIMEOUT_SECONDS` | integer | `5` | Thời gian tối đa để đọc request header & body. |
| `SERVER_WRITE_TIMEOUT_SECONDS` | integer | `10` | Thời gian tối đa để ghi response cho client. |
| `SERVER_IDLE_TIMEOUT_SECONDS` | integer | `120` | Thời gian giữ kết nối keep-alive rảnh rỗi. |

---

## 5. Xử lý lỗi & Logging

- **`http.ErrServerClosed`**: Khi `srv.Shutdown()` được gọi, `srv.ListenAndServe()` sẽ lập tức trả về lỗi `http.ErrServerClosed`. Đây là kết quả **bình thường và hợp lệ** của quá trình shutdown có chủ đích, không được coi là server crash hay in lỗi mức `Fatal`/`Error`.
- **Lỗi khởi động (Startup failure)**: Bọc `srv.ListenAndServe()` trong goroutine và gửi lỗi qua channel `serverErrors := make(chan error, 1)`. Nếu port đã bị chiếm dụng hoặc địa chỉ không hợp lệ, server thoát ngay lập tức mà không bị deadlock đứng chờ signal.
- **Vượt quá deadline (Timeout exceeded)**: Nếu in-flight request chạy quá thời gian cấu hình (5s), `srv.Shutdown(ctx)` trả về lỗi deadline exceeded; server ghi log cảnh báo và thực hiện `srv.Close()` cưỡng chế để đảm bảo tiến trình không bị treo vĩnh viễn (no indefinite hang).

---

## 6. Ranh giới với công việc chạy nền lâu dài (Long-Running Work)

> [!CAUTION]
> **Graceful shutdown không phải là giải pháp cho các tác vụ đồng bộ chạy quá lâu (vài phút đến hàng giờ).**
>
> Các tác vụ nặng như: gửi email hàng loạt, xử lý video, tạo báo cáo phân tích, đồng bộ dữ liệu lớn... **phải được tách rời khỏi chu trình request-response đồng bộ của HTTP**, chuyển sang background worker, message queue (RabbitMQ, Kafka) hoặc cron runner.

---

## 7. Hướng dẫn kiểm thử thủ công (Verification Procedures)

Endpoint thử nghiệm độ trễ được cung cấp tại `/v1/debug/slow` (thuộc nhóm tooling routes, bảo vệ bởi Basic Auth `admin:admin`, giả lập delay 3 giây).

### Kịch bản 1: Request hoàn thành thành công trong khi shutdown

1. Khởi động server API:
   ```bash
   go run ./cmd/api
   ```
2. Mở terminal thứ hai, gửi request giả lập độ trễ:
   ```bash
   curl -i -u admin:admin http://localhost:8080/v1/debug/slow
   ```
3. Ngay lập tức gửi tín hiệu `SIGTERM` đến tiến trình API (trong vòng 1-2 giây đầu):
   ```bash
   kill -SIGTERM $(pgrep -f "cmd/api")
   ```
4. **Kết quả mong đợi**:
   - Terminal 1 (Server log): Ghi nhận `termination signal received, initiating graceful shutdown`.
   - Terminal 2 (Curl): Nhận đầy đủ phản hồi `HTTP/1.1 200 OK` với body `{"message":"request finished after artificial delay","status":"completed"}`.
   - Terminal 1: Ghi nhận `graceful shutdown completed successfully` và tiến trình thoát sạch sẽ với mã `0`.

### Kịch bản 2: Server dừng sạch sẽ khi không có request

1. Khởi động server.
2. Nhấn `Ctrl+C` (tín hiệu `SIGINT`).
3. **Kết quả mong đợi**: Server ghi log nhận tín hiệu và thoát ngay lập tức mà không có lỗi.
