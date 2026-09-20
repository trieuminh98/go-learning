-- name: ListTrucks :many
SELECT id, capacity, current_load, created_at
FROM trucks
ORDER BY id ASC;

-- name: GetTruckByID :one
SELECT id, capacity, current_load, created_at
FROM trucks
WHERE id = $1;

-- name: CreateTruck :exec
INSERT INTO trucks (
    id,
    capacity,
    current_load
)
VALUES ($1, $2, $3);
