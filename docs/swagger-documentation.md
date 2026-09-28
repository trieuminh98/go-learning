# Swagger / OpenAPI Architecture: Single Source of Truth & Generated Contracts

This document details the architectural principles, mental models, developer workflows, and implementation boundaries for integrating Swagger/OpenAPI documentation in Go backend applications adhering to Clean Architecture.

---

## 1. Core Mental Models

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

### 1.1. Two Supporting Mental Models

1. **`Generated docs ≠ source of truth`**:
   Các file trong thư mục `docs/` (`docs.go`, `swagger.json`, `swagger.yaml`) là **disposable artifacts** (tập tin phát sinh tạm thời, có thể xóa và tái tạo bất cứ lúc nào). Tuyệt đối không chỉnh sửa thủ công các file này. Source of truth duy nhất nằm ở Go handlers, annotations và struct DTOs tại tầng API.

2. **`Change contract → regenerate docs → consumers see the new API`**:
   Bất cứ khi nào endpoint thay đổi (thêm query param, đổi request body, cập nhật status code), lập trình viên chỉ cần chạy `make gen-docs`. Giao diện Swagger UI và client consumers sẽ thấy ngay hợp đồng mới mà không có độ trễ hay sai lệch do nhập liệu thủ công.

> **Nếu rút còn một câu**:
>
> *One contract, many generated representations.*

---

### 1.2. Code-First vs Spec-First

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
| **Handler Annotations & DTOs** | [`internal/api/*.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api) | Khai báo chú thích Swagger mô tả hành vi của từng route. Tái sử dụng 100% các DTOs nghiệp vụ hiện có (`PostResponse`, `CreatePostRequest`, `UserResponse`, `HealthResponse`, `ErrorResponse`). Không tạo duplicate models. |
| **Route Registration** | [`internal/api/router.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/router.go) | Mount Swagger UI handler (`httpSwagger.WrapHandler`) tại `GET /v1/swagger/*` trong nhóm **Public route group**, đảm bảo lập trình viên và người dùng truy cập được tài liệu mà không bị chặn bởi AuthMiddleware. |
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

## 4. Endpoints Documented

Tất cả các tài nguyên và hành vi đã triển khai trong hệ thống đều được tài liệu hóa chi tiết:

1. **System**:
   - `GET /v1/health`: Kiểm tra trạng thái khả dụng, trả về [`HealthResponse`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/health.go#L8-L12).
2. **Posts**:
   - `GET /v1/posts`: Danh sách tất cả bài viết ([`PostResponse`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/post.go#L68-L77)).
   - `POST /v1/posts`: Tạo bài viết mới nhận [`CreatePostRequest`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/post.go#L52-L56).
   - `GET /v1/posts/{postID}`: Chi tiết bài viết kèm bình luận và tác giả ([`PostDetailResponse`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/post.go#L119-L124)).
   - `PATCH /v1/posts/{postID}`: Cập nhật từng phần bài viết với optimistic concurrency control ([`UpdatePostRequest`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/post.go#L61-L65)).
   - `DELETE /v1/posts/{postID}`: Xóa bài viết (trả về 204 No Content).
3. **Users & Relationships**:
   - `GET /v1/users`: Danh sách người dùng ([`UserResponse`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/user.go#L29-L33)).
   - `POST /v1/users`: Đăng ký người dùng mới ([`CreateUserRequest`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/user.go#L23-L26)).
   - `GET /v1/users/{userID}`: Xem hồ sơ người dùng.
   - `PUT /v1/users/{userID}/follow`: Thiết lập quan hệ theo dõi (204 No Content).
   - `PUT /v1/users/{userID}/unfollow`: Xóa quan hệ theo dõi (204 No Content).
4. **Feed**:
   - `GET /v1/feed`: Bảng tin người dùng có phân trang offset (`page`, `page_size`) và lọc đa tiêu chí tùy chọn (`tag`, `search`).
5. **Trucks**:
   - `GET /v1/trucks`, `GET /v1/trucks/{id}`, `POST /v1/trucks`: Các thao tác kiểm thử nghiệp vụ tải trọng.

---

## 5. Security & Authentication Placeholder

Trong metadata chung tại [`cmd/api/main.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/cmd/api/main.go), sơ đồ xác thực Bearer Token được khai báo trước:

```go
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
```

- **Nguyên tắc hợp đồng trung thực**: Mặc dù schema đã được khai báo trên giao diện Swagger UI để chuẩn bị cho giai đoạn xác thực tiếp theo, các endpoint v1 hiện tại **chưa** gắn thẻ `@Security BearerAuth` khi AuthMiddleware chưa thực sự ép buộc JWT token trên môi trường production.
- Ranh giới này đảm bảo tài liệu phản ánh chính xác 100% hành vi runtime hiện tại của hệ thống.
