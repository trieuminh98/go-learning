# Relationship Loading Architecture & Core Mental Model

This document captures the design philosophy and architectural guidelines for querying relational data across the Go backend application. The guiding principle is: **"Relationship exists in the data model, but loading that relationship is a use-case decision."**

---

## 1. Core Mental Model: Data Relationship ≠ Always Load Together

In relational databases (such as PostgreSQL), tables are linked by foreign keys (e.g. `comments.post_id REFERENCES posts(id)`). However, **the existence of a relationship in the schema must never dictate that every query automatically fetches related entities.**

```text
Data Model Relationship (Physical Schema)
       posts (1) ───< (N) comments ───> (1) users
                            ≠
Use-Case Loading (Application Boundary)
       Query shape strictly follows the specific business requirement.
```

### Use-Case Driven Loading Matrix

| Use Case / Endpoint | Data Needed | Relational Query Strategy |
| :--- | :--- | :--- |
| **Patch / Delete Post** | `Post` row only | Single lightweight query: `GetByID`. Zero relation joins. |
| **Feed / Post List** | List of `Post`s + metrics | `ListPosts` with optional `comment_count`. Never eager-load full comments. |
| **Post Detail** | `Post` + full `Comments` + `Authors` | Composed use case: `GetByID` + `GetByPostID(JOIN users)` via Logic layer. |

---

## 2. Architectural Flow: Application-Layer Composition

Instead of building massive monolithic SQL queries (`LEFT JOIN` across multiple tables) that pollute base repository methods, relationships are composed deliberately at the **Logic (Service) layer**:

```text
HTTP Request: GET /v1/posts/{postID}
         │
         ▼
API Layer (PostHandler)
         │ calls service.GetPostDetail(ctx, postID)
         ▼
Logic Layer (PostService)
   ├── 1. PostRepo.GetByID(ctx, postID) ────────────[Not Found]──> core.ErrPostNotFound (404)
   ├── 2. CommentRepo.GetByPostID(ctx, postID) ─────[Zero Rows]──> []core.Comment{} (200 OK)
   └── 3. Compose: core.PostDetail{ Post, Comments }
         │
         ▼
API Layer
         │ Maps to PostDetailResponse
         ▼
HTTP Response: 200 OK
{
  "id": 1,
  "title": "...",
  "content": "...",
  "comments": [ ... ]
}
```

---

## 3. Core Principles (Hard Rules)

### 3.1. Keep Lightweight Use Cases Lightweight
- Never modify base queries like `PostRepository.GetByID` to always fetch related entities.
- If a background worker or update handler only needs to verify post existence or update its title, it should not incur the latency, CPU, and memory overhead of fetching dozens of comments and authors.

### 3.2. Zero Relations Is Not an Error
- A post with zero comments is a completely valid resource state.
- **Rule**: If `CommentRepository.GetByPostID` returns empty, the handler must return `200 OK` with `"comments": []`.
- **JSON Serialization Rule**: Never return `"comments": null`. Always initialize empty slices (`make([]CommentResponse, 0)`) to prevent frontend client crashes.

### 3.3. Prevent N+1 Query Explosions
- When fetching related entities (e.g. comments and their authors), never iterate over comments in Go to execute individual `UserRepo.GetByID` queries.
- **Rule**: Join related static data (such as comment author `first_name`, `last_name`) directly in the repository SQL query:
  ```sql
  SELECT c.id, c.content, u.id, u.first_name, u.last_name
  FROM comments c
  JOIN users u ON u.id = c.user_id
  WHERE c.post_id = $1
  ORDER BY c.created_at DESC
  LIMIT 100;
  ```

### 3.4. Bound Relational Queries (Defense against OOM)
- Viral or heavily discussed posts can accumulate thousands of comments.
- **Rule**: Relational queries must enforce a safe upper bound (e.g. `LIMIT 100`) at the database layer, laying the groundwork for pagination (cursor/offset) and preventing unbounded memory growth.

### 3.5. Decouple Database Models from API Contracts
- `sqlc` generates database row representations (e.g. `GetCommentsByPostIDRow`).
- Do not expose database-generated structs directly in API responses.
- Always map: `sqlc row` → `core.Comment` → `api.CommentResponse`.

---

## 4. Anti-Patterns Checklist

- [ ] **Anti-Pattern 1: The "God Query" (Single monolithic join for everything)**
  *Incorrect*: Writing one giant query that joins posts, users, comments, likes, and tags, reused across feed, detail, and update endpoints.
  *Correct*: Base queries remain minimal; rich use cases compose focused repository queries at the Logic layer.
- [ ] **Anti-Pattern 2: Eager-loading in ORM style**
  *Incorrect*: Adding `Comments []Comment` into `core.Post` and forcing every repository method to populate it.
  *Correct*: `core.Post` represents the post entity. `core.PostDetail` represents the rich aggregate composed specifically for detail viewing.
- [ ] **Anti-Pattern 3: Treating empty relations as 404**
  *Incorrect*: Returning 404 when a post exists but has no comments.
  *Correct*: 404 only occurs if the root entity (`Post`) does not exist. Empty comments return 200 OK with `[]`.
