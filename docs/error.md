# Error Handling Architecture & Core Mental Model

Tài liệu này ghi lại triết lý thiết kế và quy tắc xử lý lỗi (Error Handling) xuyên suốt các tầng kiến trúc trong ứng dụng backend Go. Đây là quy chuẩn bắt buộc áp dụng để đảm bảo tính an toàn, bảo mật thông tin và dễ bảo trì.

---

## 1. Core Mental Model: The Layered Error Flow

```text
Database (PostgreSQL / pgx)
      │  (pgx.ErrNoRows, unique_violation, driver errors)
      ▼
Repository Layer
      │  Chuyển đổi lỗi database driver thành Domain Sentinel Error
      │  Nguyên tắc: KHÔNG BAO GIỜ để rò rỉ pgx/sql error lên tầng trên
      ▼
Logic Layer (Service / Use Case)
      │  Thực thi nghiệp vụ, kiểm tra business rules
      │  Nguyên tắc: Chỉ wrap & return error, KHÔNG log, KHÔNG biết về HTTP
      ▼
API Layer (Handler & Transport)
      │  Biên giới cuối cùng (Boundary)
      ├── 400 Bad Request: Lỗi client input (actionable message, KHÔNG log error)
      ├── 404 Not Found: Lỗi không tìm thấy tài nguyên (KHÔNG log error)
      └── 500 Internal Server Error: Lỗi hệ thống bất ngờ (LOG ERROR + Che giấu lỗi thật)
```

---

## 2. Các nguyên tắc cốt lõi (Hard Rules)

### 2.1. "Log Once at the Boundary" (Không duplicate log)
- **Tầng dưới (`Repository`, `Logic`)**: Chỉ bọc lỗi ngữ cảnh (`fmt.Errorf("...: %w", err)`) và `return err`. Tuyệt đối không `log.Printf` trước khi return.
- **Tầng trên cùng (`API Layer`)**: Là nơi duy nhất quyết định việc ghi log khi lỗi được chuyển thành response HTTP.
- **Lợi ích**: Tránh hiện tượng 1 lỗi phát sinh tạo ra 3-4 dòng log rải rác từ Repo lên Service rồi lên Handler.

### 2.2. Phân loại Logging: Tránh Log Noise (Ô nhiễm log)
Không phải lỗi nào cũng là sự cố hệ thống:
- **Lỗi 500 (Internal Server Error)**: Bắt buộc ghi log ở mức `[ERROR]` kèm đầy đủ ngữ cảnh (`HTTP method`, `path`, `request_id`, `actual error`).
- **Lỗi 400 (Bad Request)**: Là lỗi do client gửi sai payload/params. Không log mức ERROR.
- **Lỗi 404 (Not Found)**: Là trường hợp bình thường khi truy vấn dữ liệu không tồn tại. Không log mức ERROR.

### 2.3. Che giấu lỗi hệ thống ở phản hồi 500 (Security & Data Leak Prevention)
- **Tuyệt đối KHÔNG** trả chuỗi `err.Error()` của lỗi nội bộ ra ngoài client khi gặp lỗi 500.
  - *Lý do*: `err.Error()` có thể chứa tên bảng DB, syntax SQL, schema nội bộ, credentials hoặc stack trace.
- **Quy chuẩn**: Luôn trả về một thông điệp chung, an toàn và ổn định:
  ```json
  {
    "error": "the server encountered a problem"
  }
  ```
  Chi tiết lỗi thật sự được ghi lại an toàn trên server log để lập trình viên điều tra qua `request_id`.

### 2.4. Tính nhất quán của Error Envelope
Tất cả các phản hồi lỗi từ API (dù là 400, 401, 403, 404, 409 hay 500) phải tuân theo đúng một định dạng JSON thống nhất:
```json
{
  "error": "mô tả lỗi ngắn gọn"
}
```
Không để mỗi handler tự phát minh cấu trúc JSON lỗi riêng.

---

## 3. Bản đồ ánh xạ lỗi (Error Mapping Summary)

| Loại lỗi | Nguồn phát sinh | HTTP Status | Log Server | Message trả về Client |
| :--- | :--- | :---: | :---: | :--- |
| **Invalid Input** | JSON syntax sai, unknown field, invalid path param | `400 Bad Request` | Không | Thông điệp cụ thể, rõ ràng để client sửa đổi |
| **Business Rule** | Vi phạm điều kiện nghiệp vụ (`ErrInvalidTitle`, `ErrOverloaded`) | `400 Bad Request` | Không | Thông điệp nghiệp vụ cụ thể |
| **Not Found** | `pgx.ErrNoRows` -> `core.ErrNotFound` | `404 Not Found` | Không | `"resource not found"` hoặc `"post not found"` |
| **Conflict** | `23505 unique_violation` -> `core.ErrDuplicate*` | `409 Conflict` | Không | Thông điệp trùng lặp dữ liệu |
| **Unexpected System Error** | Database down, disk full, unhandled error | `500 Internal Server Error` | **CÓ [ERROR]** | `"the server encountered a problem"` |
