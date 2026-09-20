-- name: ListUsers :many
SELECT id, first_name, last_name, created_at
FROM users
ORDER BY id ASC;

-- name: CreateUser :one
INSERT INTO users (
    first_name,
    last_name
)
VALUES ($1, $2)
RETURNING id, first_name, last_name, created_at;
