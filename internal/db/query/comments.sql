-- name: GetCommentsByPostID :many
SELECT
    c.id,
    c.post_id,
    c.user_id,
    c.content,
    c.created_at,
    u.id AS author_id,
    u.first_name AS author_first_name,
    u.last_name AS author_last_name
FROM comments c
JOIN users u ON u.id = c.user_id
WHERE c.post_id = $1
ORDER BY c.created_at DESC
LIMIT 100;

-- name: CreateComment :one
INSERT INTO comments (
    post_id,
    user_id,
    content
)
VALUES (
    $1,
    $2,
    $3
)
RETURNING
    id,
    post_id,
    user_id,
    content,
    created_at;
