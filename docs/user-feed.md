# User Feed Architecture: Fan-out on Read & Graph Materialization

This document details the architectural principles, mental models, indexing strategies, and implementation boundaries for chronological user feeds in Go backend applications adhering to Clean Architecture.

---

## 1. Core Mental Models

> **Mental core cốt lõi**:
>
> *Feed is a query over the relationship graph.*

```text
Follow graph
   ↓
candidate users
   ↓
their posts
   ↓
sort / limit
   ↓
Feed
```

### 1.1. Fan-out Strategy: Start Simple

> **Ý quan trọng thứ hai**:
>
> *Start with fan-out on read; optimize only when scale proves you need more.*

```text
request feed
→ compute from followers + posts
→ return
```

Đừng nhảy sớm sang:
- `precomputed feeds`
- `queues`
- `cache fan-out`
- `ranking system`

### 1.2. Query Shape Drives Indexing

> **Ý quan trọng thứ ba (nối với bài Indexes)**:
>
> *Feed query shape determines indexes.*

```text
Feed query shape
→ determines indexes
```

```text
followers ──► lookup by follower_id        ──► PRIMARY KEY (follower_id, followed_user_id)
posts     ──► lookup by user_id + created_at DESC ──► idx_posts_user_created_at (user_id, created_at DESC)
```

> **Câu ngắn nhất để nhớ**:
> - **Relationship defines candidates.**
> - **Query materializes the feed.**
> - **Scale decides the optimization strategy.**

---

## 2. Architectural Boundaries & Data Flow

Feed adheres strictly to Clean Architecture layer separation. Under no circumstances may the API transport layer communicate directly with the database or bypass business validation.

```text
Client Request: GET /v1/feed?limit=25
      │
      ▼
┌────────────────────────────────────────────────────────┐
│ API Transport Layer (FeedHandler)                      │
│ ─► resolveActorID (request context / dev fallback)     │
│ ─► parse optional query parameter limit                │
└──────────────────────────┬─────────────────────────────┘
                           │ actorID, rawLimit
                           ▼
┌────────────────────────────────────────────────────────┐
│ Logic Domain Layer (FeedService)                       │
│ ─► Bounded limit guardrails: default = 20, max = 100   │
│ ─► Guarantee non-nil slice ([]core.Post{})             │
└──────────────────────────┬─────────────────────────────┘
                           │ actorID, boundedLimit
                           ▼
┌────────────────────────────────────────────────────────┐
│ Repository Layer (FeedRepository)                      │
│ ─► Postgres: 2s bounded timeout via context            │
│ ─► Single SQL JOIN over posts and followers            │
│ ─► Memory: thread-safe candidate filter & sort         │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────┐
│ Database (PostgreSQL)                                  │
│ ─► Index scan via idx_posts_user_created_at            │
│ ─► Primary Key lookup via followers                    │
└────────────────────────────────────────────────────────┘
```

| Tầng | Thành phần / Contract | Trách nhiệm & Ranh giới |
| :--- | :--- | :--- |
| **API** | `FeedHandler`<br>`GET /v1/feed` | Giải mã `actorID` và parse query `limit`. Serialize kết quả sang `[]PostResponse`. Tuyệt đối không chứa business rules hoặc SQL. |
| **Logic** | `FeedService`<br>`GetFeed(ctx, actorID, limit)` | Thực thi quy tắc nghiệp vụ: chuẩn hóa limit (`default = 20`, `max = 100`, `limit <= 0` fallback). Đảm bảo slice rỗng khi không có bài đăng. |
| **Repository** | `FeedRepository`<br>`GetUserFeed(ctx, userID, limit)` | Đóng gói câu truy vấn SQL JOIN. Đặt deadline 2 giây (`context.WithTimeout`). Map dữ liệu sang `[]core.Post`. |
| **Database** | `PostgreSQL`<br>`posts`, `followers` | Lưu trữ dữ liệu quan hệ và bài viết. Tối ưu qua `idx_posts_user_created_at` và composite PK của `followers`. |

---

## 3. Fan-out Comparison: Read vs Write

| Tiêu chí | Fan-out on Read (Lựa chọn v1) | Fan-out on Write |
| :--- | :--- | :--- |
| **Cơ chế** | Tính toán feed khi người dùng gửi request đọc | Đẩy post vào inbox của tất cả followers khi publish |
| **Chi phí ghi (Write)** | O(1) — chỉ ghi 1 bản ghi vào bảng `posts` | O(N) — phải ghi N bản ghi vào inbox tương ứng N followers |
| **Chi phí đọc (Read)** | O(M log K) — JOIN và sort qua index | O(1) — đọc trực tiếp từ inbox table / Redis list |
| **Độ phức tạp hạ tầng** | Tối thiểu — chỉ cần 1 câu SQL và B-Tree index | Cao — đòi hỏi background queues, worker pools, cache eviction |
| **Vấn đề Celebrity** | Không bị ảnh hưởng | Gây nghẽn hệ thống (write explosion khi user có 1M+ followers) |
| **Thời điểm áp dụng** | **Mặc định khi bắt đầu hệ thống** | Chỉ khi scale chứng minh read latency tại DB trở thành bottleneck |

---

## 4. Query Shape & Index Optimization

### 4.1. The Anti-Pattern: N+1 Application Assembly

Tránh tuyệt đối pattern lấy danh sách follow rồi lặp qua từng user để query posts:

```text
SELECT followed_user_id FROM followers WHERE follower_id = $1;
  ──► Loop over user IDs
      ──► SELECT * FROM posts WHERE user_id = ? ORDER BY created_at DESC;
```

Hành vi này tạo ra N+1 roundtrips mạng, gây nghẽn connection pool và tiêu tốn CPU ứng dụng để sort trong bộ nhớ.

### 4.2. The Optimal Single JOIN Query

Giao việc kết hợp quan hệ và sắp xếp cho engine PostgreSQL:

```sql
SELECT
    p.id,
    p.user_id,
    p.title,
    p.content,
    p.tags,
    p.created_at,
    p.updated_at,
    p.version
FROM posts p
JOIN followers f
  ON f.followed_user_id = p.user_id
WHERE f.follower_id = $1
ORDER BY p.created_at DESC
LIMIT $2;
```

### 4.3. Index Coverage

Để câu query trên chạy nhanh mà không bị Sort Spill ra bộ nhớ tạm (WorkMem) hoặc disk:

1. **`followers(follower_id, followed_user_id)`**:
   Đã được bao phủ bởi Composite Primary Key. Lọc nhanh các candidate users mà `follower_id` đang theo dõi.
2. **`posts(user_id, created_at DESC)`**:
   Thêm thông qua migration `000008_add_feed_index.up.sql`:
   ```sql
   CREATE INDEX IF NOT EXISTS idx_posts_user_created_at ON posts(user_id, created_at DESC);
   ```
   Index này cho phép PostgreSQL duyệt trực tiếp theo thứ tự thời gian giảm dần của từng candidate user mà không cần sort lại tập dữ liệu.

---

## 5. Defensive Pagination Guardrails

Một lỗi phổ biến là cho phép client yêu cầu số lượng bản ghi không giới hạn (`unbounded queries`):

```go
// Tầng Logic bắt buộc phải áp dụng trần an toàn:
const (
    defaultFeedLimit = 20
    maxFeedLimit     = 100
)

func (s *FeedService) GetFeed(ctx context.Context, actorID int64, limit int) ([]core.Post, error) {
    if limit <= 0 {
        limit = defaultFeedLimit
    }
    if limit > maxFeedLimit {
        limit = maxFeedLimit
    }
    ...
}
```

Lợi ích:
- Ngăn chặn tấn công DoS vô tình hay cố ý (client gửi `?limit=1000000`).
- Giữ dung lượng RAM cho mỗi request nhỏ và dự đoán được.
- Đảm bảo thời gian query luôn nằm trong ngưỡng timeout 2 giây của repository.

---

## 6. Pre-Auth Actor Identity Isolation

Trong giai đoạn phát triển khi JWT Middleware chưa cấp phát token đầy đủ, mã định danh người dùng (`actorID`) được giải quyết linh hoạt tại tầng transport:

```go
func resolveActorID(r *http.Request) (int64, error) {
    // 1. Fallback query param cho GET requests (dev/testing)
    if queryID := r.URL.Query().Get("user_id"); queryID != "" {
        if id, err := strconv.ParseInt(queryID, 10, 64); err == nil && id > 0 {
            return id, nil
        }
    }
    // 2. Fallback JSON body cho PUT/POST requests
    if r.Body != nil && r.ContentLength > 0 {
        var input struct { UserID int64 `json:"user_id"` }
        if err := json.NewDecoder(r.Body).Decode(&input); err == nil && input.UserID > 0 {
            return input.UserID, nil
        }
    }
    // 3. Fallback request context
    return UserIDFromContext(r.Context())
}
```

Ranh giới này đảm bảo:
- `FeedService` chỉ làm việc với kiểu dữ liệu nguyên thủy `actorID int64` đã được chuẩn hóa.
- Khi gắn token authentication thực tế, chỉ cần sửa logic trích xuất tại `api.resolveActorID` hoặc request context mà không làm thay đổi bất kỳ dòng code nào ở tầng Logic hay Repository.
