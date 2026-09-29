# Authorization Architecture: Ownership-First & Role-Based Access Control

This document outlines the architecture, pipeline design, domain policies, and error semantics for protecting resource mutations via ownership and hierarchical role validation.

---

## 1. Core Mental Models

> **Mental core của Authorization**:
>
> *Authentication gives you the actor; authorization checks whether that actor may perform this action on this resource.*

```text
Actor (Authenticated User in context)
  +
Action (PATCH / DELETE mutation)
  +
Resource (Post in context)
  ↓
Policy Evaluation
  ↓
ALLOW / 403 Forbidden
```

### 1.1. Chính sách: Ownership First → Role Fallback

```text
owns resource?
├─ yes → allow
└─ no
    ↓
role >= required role?
├─ yes → allow
└─ no  → 403 Forbidden
```

### 1.2. Phân định Authentication vs Authorization

| Khía cạnh | Authentication | Authorization |
| :--- | :--- | :--- |
| **Câu hỏi cốt lõi** | *Who are you?* (Bạn là ai?) | *Can you do this action on this resource?* (Bạn có được thực hiện hành động này trên tài nguyên này không?) |
| **Trọng tâm** | Token hợp lệ, danh tính actor | Quyền hạn của actor trên tài nguyên mục tiêu |
| **Mã lỗi thất bại** | **`401 Unauthorized`** (chưa authenticate) | **`403 Forbidden`** (đã authenticate nhưng không đủ quyền) |

### 1.3. Mô hình thứ bậc vai trò (RBAC Level Precedence)

```text
Role   → authorization level (user: 1, moderator: 2, admin: 3)
Action → required level       (PATCH: moderator [2], DELETE: admin [3])

currentRole.Level >= requiredRole.Level → allow
```

---

## 2. Request Pipeline & Middleware Sequencing

Thứ tự thực thi của pipeline tại biên mạng (HTTP Gateway) đóng vai trò quyết định tính toàn vẹn và ngăn chặn các lỗ hổng bảo mật:

```text
HTTP Request (PATCH / DELETE /v1/posts/:id)
  ↓
1. JWT Authentication Middleware
   - Đọc Bearer token từ header
   - Xác thực signature + claims + expiration
   - Truy vấn UserRepository để nạp User (đã bao gồm Role)
   - Gắn User vào request context
   - [Lỗi: 401 Unauthorized]
  ↓
2. Post Loader (PostContextMiddleware)
   - Đọc :id từ URL path parameter
   - Truy vấn PostService để nạp Post
   - Gắn Post vào request context
   - [Lỗi: 400 Bad Request / 404 Not Found]
  ↓
3. Authorization Middleware (AuthorizePost("moderator" / "admin"))
   - Lấy User và Post từ context
   - Đánh giá chính sách: Ownership-First → Role-Fallback
   - [Lỗi: 403 Forbidden]
  ↓
4. Handler (UpdatePost / DeletePost)
   - Tiếp nhận User và Post đã được bảo đảm hợp lệ từ context
   - Thực thi cập nhật hoặc xóa mà không cần query lại DB
```

> [!IMPORTANT]
> **Ranh giới nạp tài nguyên (Resource Context Injection)**:
> Middleware phân quyền phụ thuộc vào việc cả **Actor** lẫn **Resource** đã được phân giải thành công và đính kèm trong context. Handler downstream tái sử dụng trực tiếp thực thể này, triệt tiêu hoàn toàn vấn đề N+1 query.

---

## 3. Authorization Policy: Ownership-First + Role-Fallback

Mỗi thao tác mutation áp dụng chính sách ưu tiên tác giả trước, sau đó mới hạ cấp kiểm tra vai trò:

```text
if post.UserID == currentUser.ID:
    allow  // Tác giả luôn có toàn quyền sửa/xóa bài viết của mình
else if currentUser.Role.Level >= requiredRole.Level:
    allow  // Người dùng có vai trò đặc quyền đáp ứng ngưỡng quy định
else:
    deny   // Trả về 403 Forbidden
```

### 3.1. Ma trận phân quyền theo hành động (Action Matrix)

| Hành động | Endpoint | Ngưỡng Role yêu cầu | Quyền hạn của Owner | Quyền hạn của Normal User | Quyền hạn của Moderator (Level 2) | Quyền hạn của Admin (Level 3) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Cập nhật bài viết** | `PATCH /posts/{id}` | `moderator` (Level 2) | Cho phép (200) | Từ chối (403) | Cho phép (200) | Cho phép (200) |
| **Xóa bài viết** | `DELETE /posts/{id}` | `admin` (Level 3) | Cho phép (204) | Từ chối (403) | Từ chối (403) | Cho phép (204) |

### 3.2. Triệt tiêu Magic Numbers trong Route Configuration

Route không bao giờ hardcode giá trị số nguyên (`level = 2` hoặc `level = 3`). Thay vào đó:
- Route định nghĩa yêu cầu bằng tên định danh ngữ nghĩa: `AuthorizePost("moderator")`, `AuthorizePost("admin")`.
- Middleware phân giải vai trò yêu cầu thông qua `RoleRepository.GetByName(ctx, requiredRole)` để lấy cấp độ `Level` thực tế.

---

## 4. Error Semantics: Phân định rạch ròi 401, 403, 404

Hệ thống tuân thủ chặt chẽ ngữ nghĩa mã lỗi HTTP theo tiêu chuẩn RFC:

```text
401 Unauthorized
→ Không có danh tính hợp lệ (thiếu token, token sai, expired, user không tồn tại hoặc inactive).

403 Forbidden
→ Danh tính đã được xác thực hợp lệ, nhưng không có quyền hạn đối với hành động trên tài nguyên cụ thể.

404 Not Found
→ Tài nguyên không tồn tại trên hệ thống (phát hiện sớm ngay tại Post Loader trước khi thẩm định quyền).
```

---

## 5. Security Guardrails

1. **Zero-Trust Client Payloads**: Không bao giờ lấy `user_id` hay `author_id` từ request body hoặc query parameter. Danh tính actor bắt buộc trích xuất từ token JWT đã ký số trong request context.
2. **Strict Typed Comparison**: Chuyển đổi định danh người dùng `currentUser.ID` (chuỗi) sang số nguyên `int64` bằng `strconv.ParseInt` kèm điều kiện `err == nil && currentUserID > 0 && currentUserID == post.UserID`. Không sử dụng so sánh chuỗi lỏng lẻo để tránh false-positives khi ID không hợp lệ.
3. **Fail-Closed Execution**: Nếu thiếu User trong context, middleware trả về `401 Unauthorized`. Nếu thiếu Post trong context, middleware trả về `500 Internal Server Error`. Nếu không thỏa mãn ownership lẫn role level, middleware trả về `403 Forbidden`. Không có nhánh nào bypass âm thầm.
4. **Future Scalability Boundary**: Mô hình Role Hierarchy (`Level >= requiredLevel`) tối ưu cho các hệ thống có thứ bậc tuyến tính (`admin > moderator > user`). Khi hệ thống mở rộng các quyền độc lập (`can_publish`, `can_moderate_comments`, `can_refund`), kiến trúc cần tiến hóa sang RBAC dạng Permission-based (Role-Permission Mapping) thay vì tiếp tục kéo dài cấp bậc level.
