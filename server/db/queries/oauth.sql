-- name: CreateOAuthClient :one
INSERT INTO oauth_clients (name, redirect_uris)
VALUES ($1, $2)
RETURNING *;

-- name: GetOAuthClient :one
SELECT * FROM oauth_clients WHERE id = $1;

-- name: CreateOAuthCode :exec
INSERT INTO oauth_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, scope, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ConsumeOAuthCode :one
DELETE FROM oauth_codes
WHERE code_hash = $1
RETURNING *;

-- name: DeleteExpiredOAuthCodes :exec
DELETE FROM oauth_codes WHERE expires_at < now();

-- name: CreateOAuthToken :one
INSERT INTO api_keys (user_id, name, prefix, key_hash, expires_at, client_id, refresh_hash, refresh_expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetOAuthTokenByRefresh :one
SELECT * FROM api_keys
WHERE refresh_hash = $1
  AND revoked_at IS NULL
  AND refresh_expires_at > now();

-- name: RotateOAuthToken :execrows
UPDATE api_keys
SET key_hash = $2, prefix = $3, expires_at = $4, refresh_hash = $5, refresh_expires_at = $6
WHERE id = $1 AND revoked_at IS NULL AND refresh_hash = sqlc.arg(old_refresh_hash);
