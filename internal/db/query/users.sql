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

-- name: GetUserByID :one
SELECT id, first_name, last_name, created_at
FROM users
WHERE id = $1;

-- name: CreateUserWithCredentials :one
INSERT INTO users (
    first_name,
    last_name,
    email,
    password_hash,
    is_active
)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, first_name, last_name, email, is_active, created_at;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, first_name, last_name, email, password_hash, is_active, created_at
FROM users
WHERE email = $1;

-- name: ActivateUser :exec
UPDATE users
SET is_active = true
WHERE id = $1;
