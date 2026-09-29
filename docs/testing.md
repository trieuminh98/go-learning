# Testing Architecture & Mental Models

This document outlines the core mental models, interface boundary design, dependency substitution, and feedback cycles that make automated testing scalable and reliable.

---

## 1. Core Mental Models

> **Mental core của Testing**:
>
> *Tests are the first consumers of your design. If replacing dependencies is easy, testing becomes easy.*
>
> *(Kiểm thử là khách hàng đầu tiên đánh giá chất lượng thiết kế của bạn. Nếu việc thay thế phụ thuộc dễ dàng, việc kiểm thử sẽ trở nên đơn giản và tự nhiên).*

```text
Code under test
     ↓
Interface boundary
     ↓
replace dependency
     ↓
controlled test behavior
```

### 1.1. Production vs. Test Environment

Trong Clean Architecture / Hexagonal Architecture, code nghiệp vụ hoặc tầng Handler không phụ thuộc trực tiếp vào tầng lưu trữ (infrastructure), mà giao tiếp qua một **Interface Boundary** (Port):

```text
Production:
Handler
   ↓
Repository Interface
   ↓
PostgreSQL / Redis

Test:
Handler
   ↓
Repository Interface
   ↓
Fake / Stub / Mock (In-Memory)
```

- **Môi trường Production**: Repository Interface được thực thi bằng PostgreSQL driver thật hoặc Redis connection pool.
- **Môi trường Test**: Repository Interface được thay thế bằng một Test Double (In-Memory Fake, Stub, hoặc Mock), giúp kiểm soát hoàn toàn dữ liệu đầu vào và đầu ra mà không cần khởi động database hay network stack.

### 1.2. Vòng lặp phản hồi TDD (Feedback Loop)

TDD (Test-Driven Development) không phải là sự áp đặt quy trình phức tạp, mà là một vòng phản hồi cực nhanh:

```text
RED
→ fail (viết test định nghĩa mong đợi trước khi có code)
  ↓
GREEN
→ make it work (viết code tối giản vừa đủ để test pass)
  ↓
REFACTOR
→ improve safely (tối ưu hóa cấu trúc code với sự bảo vệ của test)
```

- **RED**: Xác nhận rằng bài test thực sự kiểm tra được logic mới và sẽ thất bại nếu thiếu cài đặt.
- **GREEN**: Đạt được tính đúng đắn chức năng nhanh nhất có thể.
- **REFACTOR**: Nâng cấp thiết kế, loại bỏ trùng lặp, cải thiện hiệu năng mà không sợ làm hỏng hành vi đã được kiểm chứng.

### 1.3. Rút gọn một câu để nhớ

```text
Design code so dependencies can be replaced; then tests become cheap feedback instead of painful setup.
```

*(Hãy thiết kế code sao cho các phụ thuộc có thể thay thế được; khi đó tests sẽ trở thành vòng phản hồi nhanh và rẻ thay vì một gánh nặng cấu hình đau đầu).*

---

## 2. Interface Boundaries & Test Doubles

Hệ thống trong repo này áp dụng nguyên tắc Dependency Inversion:

```text
[Handler / Use-Case] ---> [Interface (e.g. logic.UserRepository)]
                                     ▲
                      ┌──────────────┴──────────────┐
                      │                             │
          [internal/repository/postgres]   [internal/repository/memory]
             (Production Database)           (Fast In-Memory Test Double)
```

### 2.1. Phân loại Test Doubles

| Loại Double | Đặc điểm | Trường hợp sử dụng |
| :--- | :--- | :--- |
| **Fake** | Triển khai hoạt động thật sự nhưng đơn giản hóa (ví dụ: `memory.UserRepository` dùng map in-memory). | Kiểm thử luồng tích hợp, handler test không cần database container. |
| **Stub** | Trả về dữ liệu cố định định sẵn (canned responses) cho một lời gọi cụ thể. | Kiểm tra nhánh rẽ khi repository trả về `core.ErrUserNotFound` hoặc lỗi mạng. |
| **Mock** | Ghi nhận các lời gọi hàm (method, arguments, call count) và xác minh kỳ vọng tương tác. | Kiểm tra xem mailer hay cache invalidation có được gọi đúng tham số hay không. |
| **Dummy** | Giá trị truyền vào chỉ để thỏa mãn chữ ký hàm, không bao giờ được đọc hay gọi. | Khởi tạo struct khi một tham số không liên quan đến ca kiểm thử. |

---

## 3. Best Practices cho Go Testing

1. **Accept Interfaces, Return Structs**:
   - Tầng tiêu thụ (`Handler`, `Service`) chỉ nhận interface đại diện cho hành vi mà nó cần (`logic.UserGetter`, `logic.UserRepository`).
   - Việc này cho phép test thay thế implementation chỉ bằng 1 dòng inject struct trong test setup.
2. **Tránh kiểm thử quá sâu vào chi tiết nội bộ (Test Behavior, Not Implementation)**:
   - Tập trung test các bất biến (invariants), đầu vào (HTTP request, parameters) và đầu ra (status code, JSON response, state thay đổi).
   - Không gắn chặt test vào từng câu lệnh SQL hoặc private helper function.
3. **Giữ cho Tests độc lập và chạy nhanh**:
   - Ưu tiên unit test và in-memory test chạy trong vài millisecond.
   - Các bài test tích hợp phụ thuộc DB container nên được chạy có chọn lọc (ví dụ qua test flags hoặc build tags).
