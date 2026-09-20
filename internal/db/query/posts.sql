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
