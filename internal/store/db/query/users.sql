-- name: CreateUser :one
INSERT INTO "Users" (
  password,
  username
) VALUES (
  $1, $2
) RETURNING *;

-- name: ListUsers :many
SELECT * FROM "Users";

-- name: GetUserByUsername :one
SELECT * FROM "Users" 
WHERE Username = $1;

-- name: GetUserById :one
SELECT * FROM "Users" 
WHERE ID = $1;
