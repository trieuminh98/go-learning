# Swagger / OpenAPI Architecture: Single Source of Truth & Generated Contracts

This document details the architectural principles, mental models, developer workflows, and implementation boundaries for integrating Swagger/OpenAPI documentation in Go backend applications adhering to Clean Architecture.

---

## 1. Core Mental Models

### 1.1. Contract Generation & Single Source of Truth

> **Mental core ngắn nhất**:
>
> *Docs không nên là một bản copy thủ công của API; docs nên được generate từ contract.*

```text
API contract
    ↓
source of truth
    ↓
generated OpenAPI
    ↓
Swagger UI
```

#### Hai ý phụ:
1. **`Generated docs ≠ source of truth`**:
   Các file trong thư mục `docs/` (`docs.go`, `swagger.json`, `swagger.yaml`) là **disposable artifacts** (tập tin phát sinh tạm thời, có thể xóa và tái tạo bất cứ lúc nào). Tuyệt đối không chỉnh sửa thủ công các file này. Source of truth duy nhất nằm ở Go handlers, annotations và struct DTOs tại tầng API.

2. **`Change contract → regenerate docs → consumers see the new API`**:
   Bất cứ khi nào endpoint thay đổi (thêm query param, đổi request body, cập nhật status code), lập trình viên chỉ cần chạy `make gen-docs`. Giao diện Swagger UI và client consumers sẽ thấy ngay hợp đồng mới mà không có độ trễ hay sai lệch do nhập liệu thủ công.

> **Nếu rút còn một câu**:
>
> *One contract, many generated representations.*

---

### 1.2. Public Contract vs Internal Model (Fidelity & Envelope Parity)

> **Mental core của endpoint documentation**:
>
> *Document the public contract, not the internal model.*

```text
Internal model
    ↓
API mapping
    ↓
Public request / response
    ↓
Swagger docs
```

#### Hai ý phụ cần nhớ:
1. **`Swagger = contract representation`**, không phải **`Swagger = dump Go structs`**:
   Swagger không phải là công cụ phơi bày nguyên trạng các struct nội bộ của cơ sở dữ liệu hay domain core (`core.Post`, `core.User`). Nó là đại diện của **Public API Contract** — chỉ phản ánh những trường dữ liệu mà client được phép gửi lên và nhận về.

2. **`Runtime response must match documented response`**:
   Payload thực tế trả về qua HTTP dây mạng bắt buộc phải trùng khớp 1:1 với schema mô tả trên tài liệu.
   Nếu hai bên sai lệch:
   ```text
   runtime ≠ docs
   → contract drift
   ```
   Ví dụ: API bọc payload trong `{"data": [...]}` thông qua `jsonResponse()`, nhưng Swagger lại ghi chú `{array} PostResponse` (mảng trần). Sự sai lệch này khiến client SDK parser bị đổ vỡ khi đọc dữ liệu runtime. Giải pháp kiến trúc là sử dụng các **Response Envelopes** (`PostsEnvelope`, `UserEnvelope`, etc.) tại ranh giới API để OpenAPI schema thể hiện chính xác thuộc tính `data`.

> **Câu ngắn nhất**:
>
> *Docs should describe what clients actually send and receive.*

---

### 1.3. Code-First vs Spec-First

| Tiếp cận | Luồng thực thi | Đặc điểm trong dự án hiện tại |
| :--- | :--- | :--- |
| **Code-First (Hiện tại)** | `Go annotations/code → OpenAPI → Swagger UI` | Source of truth là Go code. Dùng `swag CLI` quét declarative comments trên handlers và DTOs để sinh OpenAPI spec. Phù hợp phát triển nhanh, giảm thiểu overhead. |
| **Spec-First (Mở rộng sau này)** | `OpenAPI spec → generated Go types/interfaces` | Source of truth là file OpenAPI YAML/JSON. Dùng generator (như `oapi-codegen`) để sinh interfaces và models trong Go. Phù hợp hệ thống lớn với nhiều team tiêu thụ chung contract. |

> Dù chọn Code-First hay Spec-First, **nguyên tắc bất biến vẫn y chang**:
>
> *Chỉ giữ đúng một source of truth duy nhất để ngăn ngừa triệt để contract drift (sự lệch pha giữa code thực tế và tài liệu).*

---

## 2. Architectural Boundaries & Placement

Toàn bộ mối bận tâm về Swagger và tài liệu giao tiếp được cô lập nghiêm ngặt tại ranh giới tầng API.

```text
               ┌────────────────────────────────────────────────────────┐
               │ cmd/api/main.go                                        │
               │ ─► General API metadata (@title, @version, @BasePath)  │
               │ ─► Security scheme (@securityDefinitions BearerAuth)   │
               │ ─► Dynamic Host configuration (docs.SwaggerInfo.Host)  │
               └──────────────────────────┬─────────────────────────────┘
                                          │
                                          ▼
┌───────────────────────────────────────────────────────────────────────┐
│ internal/api                                                          │
│ ─► Handler annotations: @Summary, @Tags, @Param, @Success, @Failure   │
│ ─► Reuses existing DTOs: PostResponse, UserResponse, CreatePostRequest│
│ ─► Typed Public Envelopes: PostsEnvelope, PostEnvelope, UserEnvelope  │
│ ─► Router: Mounts public route GET /v1/swagger/* (httpSwagger)        │
└───────────────────────────────────────────────────────────────────────┘
                                          │
                  ┌───────────────────────┴───────────────────────┐
                  ▼                                               ▼
┌───────────────────────────────────┐           ┌───────────────────────────────────┐
│ docs/ (Generated Artifacts)       │           │ Core / Logic / Repository         │
│ ─► docs.go                        │           │ ─► Zero Swagger annotations       │
│ ─► swagger.json                   │           │ ─► Zero Swagger dependencies      │
│ ─► swagger.yaml                   │           │ ─► Pure domain logic & storage    │
│ (Regenerable via make gen-docs)   │           │ (Strictly out of bounds)          │
└───────────────────────────────────┘           └───────────────────────────────────┘
```

### 2.1. Layer Responsibilities

| Tầng / Thành phần | Vị trí tập tin | Trách nhiệm kiến trúc |
| :--- | :--- | :--- |
| **General Info & Entrypoint** | [`cmd/api/main.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/cmd/api/main.go) | Định nghĩa metadata toàn cục của API và security definition. Thiết lập `docs.SwaggerInfo.Host` động từ môi trường (`API_HOST` / `SERVER_ADDR`) thay vì hardcode localhost. |
| **Handler Annotations & DTOs** | [`internal/api/*.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api) | Khai báo chú thích Swagger mô tả hành vi của từng route. Tái sử dụng các DTOs nghiệp vụ và khai báo Typed Response Envelopes (`PostEnvelope`, `PostsEnvelope`, `UserEnvelope`, `UsersEnvelope`, `TruckEnvelope`, `TrucksEnvelope`, `ErrorResponse`). Không rò rỉ domain model. |
| **Route Registration** | [`internal/api/router.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/router.go) | Mount Swagger UI handler (`httpSwagger.WrapHandler`) tại `GET /v1/swagger/*` trong nhóm **Public route group** (không cần auth), phân định rõ với **Private route group** (`gateway.AuthMiddleware`). |
| **Generated Artifacts** | `docs/docs.go`<br>`docs/swagger.json`<br>`docs/swagger.yaml` | Kết quả được sinh tự động bởi `swag CLI`. Được xem là disposable artifacts — không bao giờ chỉnh sửa trực tiếp bằng tay. |
| **Domain & Storage** | [`internal/core`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/core)<br>[`internal/logic`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/logic)<br>[`internal/repository`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/repository) | **Vùng cấm tuyệt đối**: Không chứa bất kỳ chú thích `@`, cấu trúc hay package nào liên quan đến Swagger. |

---

## 3. Developer Workflow & Reproducibility

Quy trình phát triển duy trì sự đồng bộ tự động giữa code và tài liệu:

```text
Change API Handler / DTO (internal/api)
             │
             ▼
      make gen-docs
             │
      ┌──────┴───────────────────────────────────────────────────────┐
      │ 1. $(SWAG) fmt -d ./internal/api,./cmd/api                   │
      │    ──► Chuẩn hóa formatting của các khối chú thích Go        │
      │ 2. $(SWAG) init -g ./cmd/api/main.go -o ./docs ...           │
      │    ──► Quét dependencies và sinh lại docs/                   │
      └──────┬───────────────────────────────────────────────────────┘
             │
             ▼
   Reload server / restart
             │
             ▼
Swagger UI (GET /v1/swagger/index.html) updated immediately
```

### 3.1. Makefile Automation

Lệnh `make gen-docs` được tích hợp sẵn trong [`Makefile`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/Makefile) với cơ chế fallback tự động tìm binary `swag` trong `$PATH` hoặc `$GOPATH/bin`:

```makefile
SWAG ?= $(shell which swag 2>/dev/null || echo $(shell go env GOPATH)/bin/swag)

gen-docs:
	$(SWAG) fmt -d ./internal/api,./cmd/api
	$(SWAG) init -g ./cmd/api/main.go -o ./docs --parseDependency --parseInternal
```

---

## 4. Endpoints Documented & Envelopes

Tất cả các tài nguyên và hành vi đã triển khai trong hệ thống đều được tài liệu hóa chi tiết với envelope responses và security requirements tương ứng:

1. **System** (Public — No Auth):
   - `GET /v1/health`: Kiểm tra trạng thái khả dụng, trả về [`HealthResponse`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/health.go#L8-L12).
2. **Posts** (Private — `@Security BearerAuth`):
   - `GET /v1/posts`: Danh sách tất cả bài viết (`200` [`PostsEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go)).
   - `POST /v1/posts`: Tạo bài viết mới nhận [`CreatePostRequest`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/post.go) (`201` [`PostEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go)).
   - `GET /v1/posts/{postID}`: Chi tiết bài viết kèm bình luận và tác giả (`200` [`PostDetailEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go)).
   - `PATCH /v1/posts/{postID}`: Cập nhật từng phần bài viết với optimistic concurrency control (`200` [`PostEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go)).
   - `DELETE /v1/posts/{postID}`: Xóa bài viết (`204` No Content).
3. **Users & Relationships** (Private — `@Security BearerAuth`):
   - `GET /v1/users`: Danh sách người dùng (`200` [`UsersEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go)).
   - `POST /v1/users`: Đăng ký người dùng mới (`201` [`UserEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go)).
   - `GET /v1/users/{userID}`: Xem hồ sơ người dùng (`200` [`UserEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go)).
   - `PUT /v1/users/{userID}/follow`: Thiết lập quan hệ theo dõi (`204` No Content).
   - `PUT /v1/users/{userID}/unfollow`: Xóa quan hệ theo dõi (`204` No Content).
4. **Feed** (Private — `@Security BearerAuth`):
   - `GET /v1/feed`: Bảng tin người dùng có phân trang offset (`page`, `page_size`) và lọc đa tiêu chí tùy chọn (`tag`, `search`), trả về `200` [`PostsEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go).
5. **Trucks** (Private — `@Security BearerAuth`):
   - `GET /v1/trucks`: Danh sách xe tải (`200` [`TrucksEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go)).
   - `GET /v1/trucks/{id}`: Chi tiết xe tải (`200` [`TruckEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go)).
   - `POST /v1/trucks`: Tạo xe tải mới (`201` [`TruckEnvelope`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/json.go)).

---

## 5. Security & Authentication Parity

Trong metadata chung tại [`cmd/api/main.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/cmd/api/main.go), sơ đồ xác thực Bearer Token được khai báo:

```go
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and token.
```

- **Ranh giới bảo mật đối chiếu 1:1**:
  - Nhóm **Public** (`/health`, `/v1/swagger/*`): Hoàn toàn không gắn `@Security BearerAuth`, cho phép gọi trực tiếp mà không cần header Authorization.
  - Nhóm **Private** (`/posts`, `/users`, `/feed`, `/trucks`): Được bảo vệ runtime bởi `gateway.AuthMiddleware`. Do đó, trên Swagger documentation, toàn bộ các endpoints này đều được gắn `@Security BearerAuth` và mô tả phản hồi `401 Unauthorized` (`ErrorResponse`).
- Người dùng trên Swagger UI có thể bấm nút **Authorize** ở góc phải màn hình, nhập token hợp lệ (ví dụ: `Bearer secret-token`) để thực hiện các request trực tiếp đến các endpoint được bảo vệ.
