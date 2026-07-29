-- name: CreateUser :one
INSERT INTO users (full_name, email, hashed_password, role)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE LOWER(email) = LOWER(sqlc.arg(email));

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users
WHERE (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active'))
ORDER BY full_name
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active'));

-- name: ListApprovers :many
SELECT id, full_name FROM users
WHERE role = 'Approver' AND is_active = true
ORDER BY full_name;

-- name: GetUserForUpdate :one
SELECT * FROM users
WHERE id = $1
FOR UPDATE;

-- name: SetUserActiveStatus :one
UPDATE users
SET is_active = $2, updated_on = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserName :one
UPDATE users 
SET full_name = $2, updated_on = NOW()
WHERE id = $1 
RETURNING *;

-- name: UpdateUserRole :one
UPDATE users 
SET role = $2, updated_on = NOW()
WHERE id = $1 
RETURNING *;