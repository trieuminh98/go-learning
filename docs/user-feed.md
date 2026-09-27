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

### 1.3. Sequence before Chunks: Pagination & Sorting

> **Mental core phân trang & sắp xếp**:
>
> *Sorting defines the sequence. Pagination only slices that sequence.*

```text
Filter/result set
      ↓
stable ORDER BY
      ↓
ordered sequence
      ↓
pagination
      ↓
page/chunk
```

**Hai ý phụ quan trọng**:
1. `No stable ORDER BY → no stable pagination`: Không có thứ tự ổn định thì không thể có phân trang ổn định.
2. Bản chất hai mô hình phân trang:
   - `OFFSET` → skip N rows
   - `Cursor` → continue after a known row

> **Câu ngắn nhất để nhớ**:
>
> *Order first, paginate second.*

**Với Feed hiện tại**:
```text
created_at DESC, id DESC
→ stable order
→ LIMIT/OFFSET now
→ cursor later if needed
```

`id` ở đây đóng vai trò là tie-breaker để tránh page boundary bị mơ hồ khi nhiều post có cùng `created_at`.

### 1.4. Membership before Order & Window: Feed Filtering

> **Mental core lọc dữ liệu**:
>
> *Filter decides membership; sort decides order; pagination decides window.*

```text
Candidate posts
      ↓
Filter
→ post nào được phép ở trong result?
      ↓
Sort
→ chúng đứng theo thứ tự nào?
      ↓
Paginate
→ trả về đoạn nào của sequence đó?
```

> **Nếu rút còn 3 câu**:
> - **Filter = WHAT belongs**
> - **Sort = IN WHAT ORDER**
> - **Pagination = HOW MUCH**

**Ý quan trọng thứ hai cho architecture**:
> *Client expresses filter intent; server owns filter semantics and SQL.*

```text
query params
→ API validate/normalize
→ Logic meaning
→ Repository SQL
```

> **Câu ngắn nhất để nhớ**:
>
> *Narrow first, order second, slice last.*

---

## 2. Architectural Boundaries & Data Flow

Feed adheres strictly to Clean Architecture layer separation. Under no circumstances may the API transport layer communicate directly with the database or bypass business validation.

```text
Client Request: GET /v1/feed?tag=golang&search=context&page=1&page_size=20
      │
      ▼
┌────────────────────────────────────────────────────────┐
│ API Transport Layer (FeedHandler)                      │
│ ─► resolveActorID (request context / dev fallback)     │
│ ─► parse & validate query: page, page_size             │
│ ─► parse & normalize filters: tag, search (trim, nil)  │
│ ─► calculate: offset = (page - 1) * page_size          │
└──────────────────────────┬─────────────────────────────┘
                           │ actorID, FeedFilter, limit, offset
                           ▼
┌────────────────────────────────────────────────────────┐
│ Logic Domain Layer (FeedService)                       │
│ ─► Guardrails: cap maxLimit 100, guard offset < 0      │
│ ─► Owns FeedFilter meaning & use-case invariants       │
│ ─► Guarantee non-nil slice ([]core.Post{})             │
└──────────────────────────┬─────────────────────────────┘
                           │ actorID, FeedFilter, limit, offset
                           ▼
┌────────────────────────────────────────────────────────┐
│ Repository Layer (FeedRepository)                      │
│ ─► Postgres: 2s bounded timeout via context            │
│ ─► Single SQL JOIN: FILTER → ORDER BY → LIMIT/OFFSET   │
│ ─► Memory: Follow graph → Tag/Search filter → Sort → Paging │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────┐
│ Database (PostgreSQL)                                  │
│ ─► WHERE follower_id = $1 AND tag filter AND search    │
│ ─► ORDER BY p.created_at DESC, p.id DESC               │
│ ─► LIMIT $limit OFFSET $offset                         │
└────────────────────────────────────────────────────────┘
```

| Tầng | Thành phần / Contract | Trách nhiệm & Ranh giới |
| :--- | :--- | :--- |
| **API** | `FeedHandler`<br>`GET /v1/feed` | Giải mã `actorID`, trích xuất `tag`, `search`, `page`, `page_size`. Đóng gói thành `logic.FeedFilter`. Serialize kết quả sang `[]PostResponse`. Tuyệt đối không chứa business rules hoặc SQL. |
| **Logic** | `FeedService`<br>`GetFeed(ctx, actorID, filter, limit, offset)` | Thực thi quy tắc nghiệp vụ: sở hữu ý nghĩa `FeedFilter`, chuẩn hóa limit (`default = 20`, `max = 100`, `offset < 0` fallback). Đảm bảo non-nil slice khi không có kết quả. |
| **Repository** | `FeedRepository`<br>`GetUserFeed(ctx, userID, filter, limit, offset)` | Đóng gói câu truy vấn SQL JOIN có lọc theo vị từ tùy chọn (`Tag`, `Search`), sắp xếp và phân trang. Đặt deadline 2 giây (`context.WithTimeout`). Map dữ liệu sang `[]core.Post`. |
| **Database** | `PostgreSQL`<br>`posts`, `followers` | Lưu trữ dữ liệu quan hệ và bài viết. Thực thi lọc, sắp xếp qua `idx_posts_user_created_at` và composite PK của `followers`. |

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
WHERE f.follower_id = @follower_id
  AND (@tag::text = '' OR @tag = ANY(p.tags))
  AND (@search::text = '' OR (p.title ILIKE '%' || @search || '%' OR p.content ILIKE '%' || @search || '%'))
ORDER BY
    p.created_at DESC,
    p.id DESC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');
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
   Index này cho phép PostgreSQL duyệt trực tiếp theo thứ tự thời gian giảm dần của từng candidate user. Tie-breaker `p.id DESC` được xử lý tự nhiên trên tập con nhỏ đã được lọc và sắp xếp.

---

## 5. Defensive Offset Pagination & Deterministic Sorting

### 5.1. Deterministic Sorting (Tie-Breaker)

Nếu chỉ sắp xếp theo `created_at DESC`, các bài viết có cùng timestamp sẽ có thứ tự bất định. Bắt buộc phải có `p.id DESC` làm khóa phụ:

```sql
ORDER BY
    p.created_at DESC,
    p.id DESC
```

Vì `id` là Primary Key duy nhất tuyệt đối, mọi bản ghi trong dãy dữ liệu đều có vị trí cố định, ngăn ngừa triệt để hiện tượng bài viết bị trôi hoặc lặp giữa các trang.

### 5.2. API Parameter Normalization

Tầng API chịu trách nhiệm chuyển đổi và validate các tham số người dùng trước khi chuyển xuống Logic/Repository:

```go
// Normalization rules tại API layer:
page := 1
if parsed, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && parsed > 0 {
    page = parsed
}

pageSize := 20
if parsed, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && parsed > 0 {
    pageSize = parsed
}

// Giới hạn trần phòng vệ chống DoS / cạn kiệt RAM
if pageSize > 100 {
    pageSize = 100
}

offset := (page - 1) * pageSize
limit := pageSize
```

Lợi ích:
- `offset` luôn không âm (`>= 0`), loại bỏ nguy cơ lỗi cú pháp SQL hoặc tràn số nguyên.
- Không để lọt chuỗi query thô vào tầng Logic hay Repository.
- Giữ dung lượng RAM và thời gian query luôn nằm trong ngưỡng timeout 2 giây của database.

### 5.3. Roadmap: Offset vs Keyset/Cursor Pagination

```text
OFFSET Pagination:
→ skip N rows: (page - 1) * page_size
→ Đơn giản, hỗ trợ jump-to-page, phù hợp quy mô vừa và nhỏ.

CURSOR Pagination (Future):
→ seek sau bản ghi đã biết: WHERE (created_at, id) < ($last_time, $last_id)
→ Hiệu năng O(1) tuyệt đối, chống trôi bài khi dữ liệu được ghi liên tục.
```

---

## 6. Feed Filtering: Intent, Semantics & SQL

### 6.1. The Three Questions of Feed Generation

Mọi feed query chuẩn mực đều phải trả lời lần lượt 3 câu hỏi tách biệt:

1. **Filter (WHAT belongs)**: Bản ghi nào thuộc về tập kết quả?
   - Quan hệ Follower Graph lọc ra các `candidate posts`.
   - Vị từ `tag` loại bỏ các bài viết không chứa thẻ yêu cầu (`$tag = ANY(p.tags)`).
   - Vị từ `search` loại bỏ các bài viết không chứa từ khóa trong tiêu đề hay nội dung (`ILIKE`).
2. **Sort (IN WHAT ORDER)**: Thứ tự sắp xếp là gì?
   - `ORDER BY p.created_at DESC, p.id DESC` tạo ra chuỗi tuần tự ổn định (stable sequence).
3. **Pagination (HOW MUCH)**: Trả về bao nhiêu và từ vị trí nào?
   - `LIMIT` và `OFFSET` cắt một cửa sổ (window) cụ thể trên chuỗi tuần tự đã lọc.

```text
Candidate posts
      ↓
Filter
→ post nào được phép ở trong result?
      ↓
Sort
→ chúng đứng theo thứ tự nào?
      ↓
Paginate
→ trả về đoạn nào của sequence đó?
```

> **Nguyên tắc bất biến**: *Narrow first, order second, slice last.*
>
> Nếu đảo lộn thứ tự này (ví dụ: phân trang trước rồi mới lọc trên bộ nhớ Go), kích thước trang sẽ bị thiếu hụt hoặc trang bị rỗng giả mạo dù cơ sở dữ liệu vẫn còn bản ghi phù hợp.

### 6.2. Separation of Concerns: Client Intent vs Server Semantics

> *Client expresses filter intent; server owns filter semantics and SQL.*

```text
query params
→ API validate/normalize
→ Logic meaning
→ Repository SQL
```

```text
Client (GET /v1/feed?tag=golang&search=context)
  │
  ▼ expresses intent (raw query params)
API Layer (FeedHandler)
  │ validates, trims whitespace, maps empty to nil
  ▼ FeedFilter{Tag: *string, Search: *string}
Logic Layer (FeedService)
  │ owns business invariants (e.g. tag/search scope, default limits)
  ▼ FeedFilter
Repository Layer (postgres.FeedRepository / memory.FeedRepository)
  │ owns SQL predicates & execution
  ▼
PostgreSQL / In-Memory Store
```

- **Client**: Chỉ biểu đạt *ý định* lọc thông qua query params (`tag`, `search`).
- **Server**: Toàn quyền sở hữu *ngữ nghĩa* của bộ lọc và cách hiện thực bằng SQL.
  - API biến đổi `""` thành `nil` (coi như không lọc).
  - Tầng SQL dùng static query `(@tag::text = '' OR @tag = ANY(p.tags))` thay vì nối chuỗi SQL động, bảo vệ hệ thống tuyệt đối khỏi SQL Injection và tận dụng trọn vẹn kiểm tra kiểu tĩnh của `sqlc`.

---

## 7. Pre-Auth Actor Identity Isolation

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
