# RBAC Database Migration: Zero-Downtime Schema Evolution & Role Hierarchy

This document details the mental models, architectural boundaries, database migration sequencing, and data integrity guardrails for establishing the foundation of Role-Based Access Control (RBAC).

---

## 1. Core Mental Models

> **Mental core của RBAC Database Migration**:
>
> *Thiết lập một invariant (ràng buộc toàn vẹn) mới lên database đang chạy phải tuân thủ quy trình 3 pha: tạo cấu trúc cho phép giá trị thiếu → migrate/backfill dữ liệu lịch sử → sau cùng mới áp đặt ràng buộc bắt buộc (NOT NULL).*

```text
new invariant (users.role_id NOT NULL)
      ↓
① add column nullable (role_id BIGINT REFERENCES roles)
      ↓
② backfill existing data (dynamic lookup by name)
      ↓
③ enforce constraint (SET NOT NULL)
```

### 1.1. Invariant Sequencing

> [!CRITICAL]
> **Quy tắc bất di bất dịch trong Schema Migration**:
>
> **Never enforce a new invariant before existing data has been migrated to satisfy it.**
>
> Nếu chạy ngay `ALTER TABLE users ADD COLUMN role_id BIGINT NOT NULL` trên production, lệnh migration sẽ lập tức sập vì toàn bộ các hàng người dùng hiện có đều mang giá trị `NULL`.

### 1.2. Triệt tiêu Magic ID trong Database & Code

```text
❌ Assume role ID 1 == "user" (Magic ID)
✅ Dynamic lookup: SELECT id FROM roles WHERE name = 'user'
```

Generated auto-increment primary keys (`SERIAL` / `BIGSERIAL`) không được mang ý nghĩa nghiệp vụ (business meaning). Bất kỳ sự thay đổi thứ tự insert, reset sequence hoặc backup/restore nào cũng có thể tráo đổi ID giữa `admin` và `user`.

Do đó:
- Trong migration backfill: luôn phân giải role ID theo tên định danh ổn định (`name = 'user'`).
- Trong các câu lệnh insert user mới: sử dụng subquery phân giải theo tên role.

### 1.3. Rút gọn một câu để nhớ:

```text
Schema change → Backfill → Constraint.
```

---

## 2. Database Schema & Role Hierarchy

### 2.1. Bảng `roles`

| Cột | Kiểu dữ liệu | Ràng buộc | Mô tả |
| :--- | :--- | :--- | :--- |
| `id` | `BIGSERIAL` | `PRIMARY KEY` | Khóa chính tự tăng |
| `name` | `VARCHAR(50)` | `NOT NULL UNIQUE` | Tên vai trò duy nhất (`user`, `moderator`, `admin`) |
| `level` | `INT` | `NOT NULL` | Cấp bậc phân quyền (thứ bậc số nguyên) |
| `description` | `TEXT` | Nullable | Mô tả chức năng và phạm vi quyền |

### 2.2. Dữ liệu hệ thống mặc định (System Roles)

Các vai trò hệ thống này là dữ liệu bắt buộc để hệ thống vận hành (**required production data**), do đó được khởi tạo ngay trong migration script, không phụ thuộc vào lệnh seed của môi trường phát triển:

```sql
INSERT INTO roles (name, level, description)
VALUES 
    ('user', 1, 'Default user role with standard permissions'),
    ('moderator', 2, 'Moderator role with content moderation permissions'),
    ('admin', 3, 'Administrator role with full system access')
ON CONFLICT (name) DO NOTHING;
```

---

## 3. Migration Sequencing & Dependencies

### 3.1. Up Migration (`migrations/000010_create_roles_and_add_user_role.up.sql`)

```sql
-- Phase 1: Tạo bảng roles và nạp dữ liệu hệ thống
CREATE TABLE IF NOT EXISTS roles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    level INT NOT NULL,
    description TEXT
);

INSERT INTO roles (name, level, description)
VALUES 
    ('user', 1, 'Default user role with standard permissions'),
    ('moderator', 2, 'Moderator role with content moderation permissions'),
    ('admin', 3, 'Administrator role with full system access')
ON CONFLICT (name) DO NOTHING;

-- Phase 2: Bổ sung cột role_id cho phép NULL kèm khóa ngoại
ALTER TABLE users ADD COLUMN IF NOT EXISTS role_id BIGINT REFERENCES roles(id);

-- Phase 3: Backfill toàn bộ user hiện có sang role 'user'
UPDATE users
SET role_id = (SELECT id FROM roles WHERE name = 'user')
WHERE role_id IS NULL;

-- Phase 4: Thiết lập ràng buộc NOT NULL sau khi dữ liệu đã đầy đủ
ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;
```

### 3.2. Down Migration (`migrations/000010_create_roles_and_add_user_role.down.sql`)

> [!WARNING]
> **Thứ tự đảo ngược khóa ngoại**:
>
> Không thể drop bảng `roles` khi cột `users.role_id` vẫn đang trỏ tới nó. Down migration bắt buộc phải gỡ bỏ khóa ngoại/cột ở bảng con trước, sau đó mới drop bảng cha:

```sql
ALTER TABLE users DROP COLUMN IF EXISTS role_id;

DROP TABLE IF EXISTS roles;
```

---

## 4. Application Domain & Persistence Integration

### 4.1. Domain Model (`internal/core/user.go`)

Thực thể `core.Role` được định nghĩa độc lập và nhúng trực tiếp vào `core.User`:

```go
type Role struct {
    ID          int64
    Name        string
    Level       int
    Description string
}

type User struct {
    ID        string
    FirstName string
    LastName  string
    Email     string
    Password  Password
    IsActive  bool
    Role      Role
}
```

### 4.2. Storage Layer & Single-Query JOIN (`internal/db/query/users.sql`)

Để triệt tiêu vấn đề N+1 query khi nạp danh sách user hoặc profile, các câu truy vấn `sqlc` thực hiện `JOIN roles` ngay trong câu lệnh SQL:

```sql
-- name: GetUserByID :one
SELECT 
    u.id, 
    u.first_name, 
    u.last_name, 
    u.email, 
    u.is_active, 
    u.created_at,
    r.id AS role_id,
    r.name AS role_name,
    r.level AS role_level,
    r.description AS role_description
FROM users u
JOIN roles r ON u.role_id = r.id
WHERE u.id = $1;
```

---

## 5. Security & Migration Guardrails

1. **Zero-Loss Data Integrity**: Không xóa hoặc sửa đổi dữ liệu người dùng cũ để làm cho migration vượt qua một cách dễ dàng.
2. **Atomic Execution**: Toàn bộ script migration chạy trong một transaction của migration runner (`golang-migrate`), bảo đảm rollback tự động nếu bất kỳ bước nào gặp sự cố.
3. **No Magic Constants**: Không sử dụng `1`, `2`, `3` trong mã nguồn ứng dụng để định danh vai trò; chỉ kiểm tra qua `user.Role.Level` hoặc `user.Role.Name`.
4. **Separation of Concerns**: Task này chỉ chuẩn bị cấu trúc cơ sở dữ liệu và mapping tầng repository; chưa áp dụng logic authorization middleware (sẽ thực hiện ở lesson kế tiếp).
