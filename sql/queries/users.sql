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