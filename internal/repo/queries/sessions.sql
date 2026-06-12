-- name: CreateSession :exec
INSERT INTO sessions (jti, user_id, refresh_token_hash, expires_at, user_agent, ip)
VALUES ($1, $2, $3, $4, $5, NULLIF($6,'')::inet);

-- name: GetSession :one
SELECT * FROM sessions WHERE jti = $1;

-- name: RevokeSession :exec
UPDATE sessions SET revoked = true WHERE jti = $1;

-- name: RevokeAllSessionsByUser :exec
UPDATE sessions SET revoked = true WHERE user_id = $1;

-- name: ListActiveSessionsByUser :many
SELECT * FROM sessions
WHERE user_id = $1 AND revoked = false AND expires_at > now()
ORDER BY created_at DESC;

-- name: PurgeExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < now() - interval '30 days';
