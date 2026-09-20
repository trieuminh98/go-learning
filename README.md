# Go Learning — Best Practice 4-Layer Architecture

Dự án mẫu (reference template) thể hiện kiến trúc phân lớp chuẩn trong Go, tuân thủ **Standard Go Project Layout** và ranh giới trách nhiệm rõ ràng với chiều phụ thuộc một chiều:

```text
Gateway → API → Logic → Core
```

## Cấu trúc thư mục (Standard Go Project Layout)

Toàn bộ repository chỉ có duy nhất một điểm khởi chạy `main.go` tại `cmd/api/`:

```text
go-learning/
├── cmd/
│   └── api/
│       └── main.go               # Entrypoint duy nhất: bootstrap & wiring dependencies
├── internal/
│   ├── gateway/
│   │   └── middleware.go         # Gateway: Inbound Auth, Request ID, Logging
│   ├── api/
│   │   ├── handler.go            # API: HTTP Handlers, parse params, map response/status
│   │   └── router.go             # API: Route registration (http.ServeMux)
│   ├── logic/
│   │   └── truck.go              # Logic: Use-case orchestration (thuần Go, không dính HTTP)
│   └── core/
│       ├── truck.go              # Core: Domain entity & business invariants
│       └── errors.go             # Core: Pure domain errors
├── go.mod
└── README.md
```

## Ranh giới trách nhiệm (Layer Boundaries)

1. **Gateway (`internal/gateway`)**:
   - Chịu trách nhiệm các mối quan tâm cross-cutting (Authentication, Request ID, Structured Logging).
   - Quyết định request có được phép vào hệ thống hay không (`Can this request enter?`).
   - Đóng vai trò middleware bọc ngoài `http.Handler`.

2. **API (`internal/api`)**:
   - Ranh giới giao tiếp HTTP: định tuyến URL, đọc `http.Request`, validate dữ liệu đầu vào.
   - Gọi tầng Logic và ánh xạ kết quả hoặc lỗi sang mã HTTP status code & JSON response (`Translate HTTP ↔ application calls`).
   - Tuyệt đối không chứa business rules.

3. **Logic (`internal/logic`)**:
   - Tầng điều phối luồng nghiệp vụ (`What steps are required to complete this use case?`).
   - Lấy dữ liệu và áp dụng các quy tắc của tầng Core.
   - Không chứa bất kỳ khái niệm HTTP nào (không status code, không headers, không `net/http`).

4. **Core (`internal/core`)**:
   - Trái tim của domain nghiệp vụ (`What is true in the business domain?`).
   - Chứa structs, invariants validation và domain errors thuần túy.
   - Không phụ thuộc vào bất kỳ tầng nào khác (Zero external dependency).

## Khởi chạy ứng dụng

```bash
go run ./cmd/api
```

Mặc định server sẽ lắng nghe tại cổng `:8080`.

## Kiểm thử qua cURL

### 1. Root / Welcome (`GET /`)
```bash
curl -i -H "Authorization: Bearer secret-token" http://localhost:8080/
```

### 2. Danh sách xe tải (`GET /trucks`)
```bash
curl -i -H "Authorization: Bearer secret-token" http://localhost:8080/trucks
```

### 3. Lấy thông tin một xe tải (`GET /trucks/{id}`)
```bash
curl -i -H "Authorization: Bearer secret-token" http://localhost:8080/trucks/1
```

### 4. Tạo xe tải mới (`POST /trucks`)
```bash
curl -i -X POST http://localhost:8080/trucks \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"id":"3","capacity":8000,"current_load":0}'
```

### 5. Thiếu Auth header (401 Unauthorized do Gateway chặn)
```bash
curl -i http://localhost:8080/trucks
```

### 6. Không tìm thấy tài nguyên (404 Not Found do API map từ domain error)
```bash
curl -i -H "Authorization: Bearer secret-token" http://localhost:8080/trucks/999
```

## Kiểm tra chất lượng mã nguồn

```bash
go vet ./...
go build ./...
```
