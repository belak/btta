-- name: ListScores :many
SELECT * FROM scores ORDER BY game_name ASC;

-- name: GetScore :one
SELECT * FROM scores WHERE id = ?;

-- name: CreateScore :one
INSERT INTO scores (game_banner, game_name, player_name, player_score)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: UpdateScore :one
UPDATE scores
SET game_banner  = ?,
    game_name    = ?,
    player_name  = ?,
    player_score = ?,
    updated_at   = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?
RETURNING *;

-- name: DeleteScore :exec
DELETE FROM scores WHERE id = ?;

-- name: ListImages :many
SELECT * FROM images ORDER BY name ASC;

-- name: ListEnabledImages :many
SELECT * FROM images WHERE enabled = TRUE ORDER BY name ASC;

-- name: GetImage :one
SELECT * FROM images WHERE id = ?;

-- name: CreateImage :one
INSERT INTO images (name, image, enabled)
VALUES (?, ?, ?)
RETURNING *;

-- name: UpdateImage :one
UPDATE images
SET name       = ?,
    image      = ?,
    enabled    = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?
RETURNING *;

-- name: DeleteImage :exec
DELETE FROM images WHERE id = ?;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: CreateUser :one
INSERT INTO users (username, password_hash)
VALUES (?, ?)
RETURNING *;

-- name: ListUsers :many
SELECT * FROM users ORDER BY username ASC;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = ?,
    updated_at    = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;
