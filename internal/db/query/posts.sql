-- name: CreatePost :one
INSERT INTO posts (
    user_id,
    title,
    content,
    tags
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING
    id,
    user_id,
    title,
    content,
    tags,
    created_at,
    updated_at;

-- name: ListPosts :many
SELECT
    id,
    user_id,
    title,
    content,
    tags,
    created_at,
    updated_at
FROM posts
ORDER BY id ASC;

-- name: GetPostByID :one
SELECT
    id,
    user_id,
    title,
    content,
    tags,
    created_at,
    updated_at
FROM posts
WHERE id = $1;

-- name: UpdatePost :one
UPDATE posts
SET
    title = $2,
    content = $3,
    tags = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING
    id,
    user_id,
    title,
    content,
    tags,
    created_at,
    updated_at;

-- name: DeletePost :execrows
DELETE FROM posts
WHERE id = $1;
