-- name: ListUsers :many
SELECT
    u.id,
    u.first_name,
    u.last_name,
    u.email,
    u.is_active,
    u.created_at,
    r.id AS role_id,
    r.name AS role_name,
    r.level AS role_level,
    r.description AS role_description
FROM users u
JOIN roles r ON u.role_id = r.id
ORDER BY u.id ASC;

-- name: CreateUser :one
INSERT INTO users (
    first_name,
    last_name,
    role_id
)
VALUES (
    $1,
    $2,
    (SELECT id FROM roles WHERE name = 'user')
)
RETURNING id, first_name, last_name, created_at, role_id;

-- name: GetUserByID :one
SELECT
    u.id,
    u.first_name,
    u.last_name,
    u.email,
    u.is_active,
    u.created_at,
    r.id AS role_id,
    r.name AS role_name,
    r.level AS role_level,
    r.description AS role_description
FROM users u
JOIN roles r ON u.role_id = r.id
WHERE u.id = $1;

-- name: CreateUserWithCredentials :one
INSERT INTO users (
    first_name,
    last_name,
    email,
    password_hash,
    is_active,
    role_id
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    (SELECT id FROM roles WHERE name = 'user')
)
RETURNING id, first_name, last_name, email, is_active, created_at, role_id;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT
    u.id,
    u.first_name,
    u.last_name,
    u.email,
    u.password_hash,
    u.is_active,
    u.created_at,
    r.id AS role_id,
    r.name AS role_name,
    r.level AS role_level,
    r.description AS role_description
FROM users u
JOIN roles r ON u.role_id = r.id
WHERE u.email = $1;

-- name: ActivateUser :exec
UPDATE users
SET is_active = true
WHERE id = $1;

-- name: GetRoleByName :one
SELECT id, name, level, description
FROM roles
WHERE name = $1;
