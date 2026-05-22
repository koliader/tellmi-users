-- name: CreateUser :one
INSERT INTO "Users" (
  password,
  username
) VALUES (
  $1, $2
) RETURNING *;

-- name: ListUsers :many
SELECT * FROM "Users";
