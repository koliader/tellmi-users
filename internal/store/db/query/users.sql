-- name: CreateUser :one
INSERT INTO users (
  password,
  username
) VALUES (
  $1, $2
) RETURNING *;

-- name: ListUsers :many
SELECT * FROM users;

-- name: GetUserByUsername :one
SELECT * FROM users 
WHERE username = $1;

-- name: GetUserById :one
SELECT * FROM users 
WHERE id = $1;

-- name: UpdateUser :one
UPDATE users
SET username = $2
WHERE id = $1
RETURNING *;
