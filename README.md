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
│   │   └── middleware.go         # Gateway: Inbound Auth, Request ID, Logging
│   ├── api/
│   │   ├── common.go             # API: Common response helpers (writeJSON, errorResponse, welcome)
│   │   ├── truck.go              # API: Truck HTTP handlers & logic error mapping
│   │   ├── user.go               # API: User HTTP handlers & JSON DTOs (io.Writer composition)
│   │   └── router.go             # API: Route registration (http.ServeMux)
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

### 7. User JSON API (`GET /users` & `POST /users`)

#### Danh sách người dùng:
```bash
curl -i -H "Authorization: Bearer secret-token" http://localhost:8080/users
```

#### Tạo mới người dùng (snake_case JSON DTO):
```bash
curl -i -X POST http://localhost:8080/users \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"first_name":"John","last_name":"Doe"}'
```

#### Trùng lặp người dùng (409 Conflict):
```bash
curl -i -X POST http://localhost:8080/users \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"first_name":"John","last_name":"Doe"}'
```

#### Thiếu trường bắt buộc (400 Bad Request từ Core validation):
```bash
curl -i -X POST http://localhost:8080/users \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"last_name":"Doe"}'
```

## Kiểm tra chất lượng mã nguồn

```bash
go vet ./...
go build ./...
```
