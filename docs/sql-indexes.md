# Database Indexing Architecture: Access-Pattern-Driven Indexing

This document outlines the indexing strategy, access pattern auditing, and verification practices for PostgreSQL in Go backend services.

---

## 1. Core Mental Models

> **Core mental cốt lõi**:
>
> *Index theo cách dữ liệu được truy cập, không theo cách schema được định nghĩa.*

```text
Query pattern
    ↓
WHERE / JOIN / ORDER BY
    ↓
Candidate index
    ↓
EXPLAIN
    ↓
Giữ / bỏ
```

### Hai ý phụ cần nhớ:

1. **Index = Trade-off**:
   ```text
   Index
   = đổi write/storage cost
     lấy read speed
   ```
   - Mỗi index thêm vào giúp tăng tốc query đọc tương ứng nhưng làm chậm mọi thao tác ghi (`INSERT`, `UPDATE`, `DELETE`).
   - Index thừa chiếm dụng RAM quý giá trong PostgreSQL buffer cache, đẩy các bảng và working set thường dùng ra ngoài disk.

2. **Composite index = Có thứ tự**:
   ```text
   Composite index
   = có thứ tự (Order matters)
   ```
   Ví dụ:
   ```sql
   (user_id, created_at)
   ```
   **không tương đương với**:
   ```sql
   (created_at, user_id)
   ```
   - Luôn đặt **equality columns** (`=`) lên đầu.
   - Đặt **range columns** (`>`, `<`, `BETWEEN`) hoặc **sorting columns** (`ORDER BY`) ở sau.

---

> **Câu ngắn nhất để nhớ**:
> - **Don't index the schema.**
> - **Index the access pattern.**

---

## 2. Access Pattern Audit & Existing Indexes

| Table | Candidate / Existing Index | Matching Query / Access Pattern | Status |
|---|---|---|---|
| `comments` | `idx_comments_post_id ON comments(post_id, created_at DESC)` | `SELECT ... WHERE post_id = $1 ORDER BY created_at DESC LIMIT 100` | Active (created in migration `000004`) |
| `followers` | `PRIMARY KEY (follower_id, followed_user_id)` | `SELECT ... WHERE follower_id = $1` | Active (covers leading column) |
| `followers` | `idx_followers_followed_user_id ON followers(followed_user_id)` | `SELECT ... WHERE followed_user_id = $1` (reverse lookup) | Active (created in migration `000006`) |
| `posts` | `idx_posts_user_id ON posts(user_id)` | `SELECT ... WHERE user_id = $1` + `users` table `ON DELETE CASCADE` | Added in migration `000007` |
| `feed` | *(Composite index e.g. `(user_id, created_at DESC)`)* | Complex multi-user feed query with pagination | **Deferred** until feed query is implemented |

### 2.1. Why `posts(user_id)` is Indexed
- **Foreign Key Cascades**: When a user is deleted from `users`, PostgreSQL must cascade-delete all matching rows in `posts`. Without an index on `posts(user_id)`, this triggers a full table `Seq Scan` on `posts`, locking and scanning the entire table.
- **User Post Lookups**: Prepares the access pattern for retrieving all posts authored by a specific user.

### 2.2. Why the Feed Index is Deferred
- Speculative indexing before a query is written often selects the wrong column sequence or sort direction (e.g. `(user_id, created_at DESC)` vs `(created_at DESC, user_id)` vs `ANY(...)`).
- Wait until the feed endpoint and query contract are confirmed with concrete pagination (`cursor` vs `offset`).

---

## 3. Migration Pattern

All index changes must be managed via versioned migration files rather than manual ad-hoc DDL:

### `migrations/000007_add_indexes.up.sql`
```sql
CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);
```

### `migrations/000007_add_indexes.down.sql`
```sql
DROP INDEX IF EXISTS idx_posts_user_id;
```

---

## 4. Verification with `EXPLAIN`

Verify index selection using PostgreSQL's query execution planner:

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, user_id, title, created_at
FROM posts
WHERE user_id = 1;
```

### Scan Types to Recognize:
- **`Index Scan`**: PostgreSQL traverses the B-Tree index to locate row pointers, then fetches table pages from disk/cache. Optimal for selective point queries.
- **`Index Only Scan`**: All requested columns are present in the index itself (no table heap fetch needed). Maximum read performance.
- **`Bitmap Index Scan` + `Bitmap Heap Scan`**: Collects matching row pointers from index into a bitmap, then fetches table pages in physical disk order. Common for medium-volume results.
- **`Seq Scan`**: Reads every page in the table sequentially from start to finish.

> [!NOTE]
> **Why PostgreSQL May Choose `Seq Scan` on Small Tables**:
> On small tables or development datasets (e.g. < 100 rows), reading 2 sequential disk blocks is significantly faster than reading 3 index blocks + 2 table blocks. The planner deliberately chooses `Seq Scan` based on cost calculations until table volume grows.

---

## 5. Architectural Guardrails

> [!IMPORTANT]
> **Index Design Rules**:
> - **Do not index every column**: Indexes are liabilities on write operations; only create indexes that pay for themselves in read performance.
> - **Do not duplicate unique constraints**: Columns with `PRIMARY KEY` or `UNIQUE` constraints automatically create a B-Tree index under the hood.
> - **Column order in composite indexes**: Strict rule: Equality first, sort/range second.
> - **No speculative indexes**: Build indexes when queries are built, not when tables are created.
