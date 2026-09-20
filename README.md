# Go Learning — Best Practice 4-Layer Architecture

Dự án mẫu (reference template) thể hiện kiến trúc phân lớp chuẩn trong Go, tuân thủ **Standard Go Project Layout** và ranh giới trách nhiệm rõ ràng với chiều phụ thuộc một chiều:

```text
Gateway → API → Logic → Core
```

## Cấu trúc thư mục (Standard Go Project Layout)

Phân định rạch ròi giữa các điểm khởi chạy (`cmd/`), mã nguồn ứng dụng đóng gói (`internal/`), và các thư mục phụ trợ:

```text
go-learning/
├── cmd/
│   ├── api/
│   │   └── main.go               # HTTP API entrypoint: bootstrap, wiring & start server
│   └── migrate/
│       └── main.go               # Database migration CLI entrypoint
├── internal/                     # Private application packages (Go compiler boundary)
│   ├── gateway/
│   │   └── middleware.go         # Gateway: Recovery, Request ID, Logging, Timeout, Auth
│   ├── api/
│   │   ├── common.go             # API: Common response helpers (writeJSON, errorResponse, welcome)
│   │   ├── health.go             # API: Health & protected example handlers
│   │   ├── truck.go              # API: Truck HTTP handlers & logic error mapping
│   │   ├── user.go               # API: User HTTP handlers & JSON DTOs (io.Writer composition)
│   │   └── router.go             # API: Chi Router (/v1 public & private groups)
│   ├── logic/
│   │   ├── truck.go              # Logic: Truck use-case orchestration (sync.RWMutex)
│   │   └── user.go               # Logic: User use-case orchestration (sync.RWMutex)
│   └── core/
│       ├── truck.go              # Core: Truck entity & invariants
│       ├── user.go               # Core: User entity (pure Go, no JSON tags)
│       └── errors.go             # Core: Pure domain errors
├── migrations/                   # SQL schema migrations (up/down)
├── docs/                         # OpenAPI / Swagger specs
├── scripts/                      # Build & deployment helper scripts
├── bin/                          # Compiled binaries (git-ignored)
├── .air.toml                     # Air hot reload configuration
├── .gitignore
├── go.mod
└── README.md
```

### Ranh giới quan trọng: Repository Structure ≠ Application Architecture

- **Repository Structure (Tổ chức vật lý trên đĩa):**
  - `cmd/`: Chỉ chứa các file `main.go` mỏng làm composition root.
  - `internal/`: Ranh giới đóng gói của Go compiler (ngăn package ngoài import). Không phải là một tầng kiến trúc.
  - `migrations/`, `docs/`, `scripts/`, `bin/`: Tài nguyên và công cụ hỗ trợ vòng đời ứng dụng.
- **Application Architecture (Ranh giới luồng xử lý bên trong `internal/`):**
  - `Gateway` → `API` → `Logic` → `Core` (phụ thuộc 1 chiều, thuần Go, zero dependency ở Core).

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

### 1. Khởi chạy thông thường:
```bash
go run ./cmd/api
```
Mặc định server sẽ lắng nghe tại cổng `:8080`.

### 2. Khởi chạy với Hot Reload (Air):
Cài đặt Air (nếu chưa có):
```bash
go install github.com/air-verse/air@latest
```

Khởi chạy tiến trình tự động phát hiện thay đổi và rebuild:
```bash
air
```
Air sẽ tự động biên dịch lại vào `./bin/api` và restart server mỗi khi lưu file `.go` trong `cmd/` hoặc `internal/`.

## Kiểm thử qua cURL

### 1. Public Route: Kiểm tra trạng thái dịch vụ (`GET /v1/health`)
*Không cần truyền Authorization header*:
```bash
curl -i http://localhost:8080/v1/health
```

### 2. Private Route: Route bảo vệ mẫu (`GET /v1/private/profile`)
```bash
# Không có token -> 401 Unauthorized
curl -i http://localhost:8080/v1/private/profile

# Có Bearer token -> 200 OK
curl -i -H "Authorization: Bearer secret-token" http://localhost:8080/v1/private/profile
```

### 3. Private Route: Quản lý xe tải (`/v1/trucks`)
```bash
# Danh sách xe
curl -i -H "Authorization: Bearer secret-token" http://localhost:8080/v1/trucks

# Tạo xe mới
curl -i -X POST http://localhost:8080/v1/trucks \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"id":"3","capacity":8000,"current_load":0}'
```

### 4. Private Route: Quản lý người dùng (`/v1/users`)
```bash
# Danh sách người dùng
curl -i -H "Authorization: Bearer secret-token" http://localhost:8080/v1/users

# Tạo người dùng mới
curl -i -X POST http://localhost:8080/v1/users \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"first_name":"John","last_name":"Doe"}'
```

### 5. Kiểm tra Panic Recovery Middleware (`GET /v1/debug/panic`)
```bash
# Kích hoạt panic có chủ đích -> Server không crash, trả về 500 JSON
curl -i http://localhost:8080/v1/debug/panic
```

## Kiểm tra chất lượng mã nguồn

```bash
go vet ./...
go build ./...
```
