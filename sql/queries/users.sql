-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
  gen_random_uuid(),
  now(),
  now(),
  $1,
  $2
)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE email = $1;

-- name: DeleteAllUsers :exec
DELETE FROM users;

-- name: ActivateChirpyRed :exec
UPDATE users
SET is_chirpy_red = true
WHERE id = $1;