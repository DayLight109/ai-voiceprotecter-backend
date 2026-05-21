-- name: ListCredentialsByUser :many
SELECT * FROM identity_credentials WHERE user_id = $1 ORDER BY kind;

-- name: GetCredentialByKind :one
SELECT * FROM identity_credentials WHERE user_id = $1 AND kind = $2;

-- name: UpsertCredential :one
INSERT INTO identity_credentials (id, user_id, kind, value_hash, verified, verified_at)
VALUES ($1, $2, $3, $4, $5, CASE WHEN $5 THEN now() ELSE NULL END)
ON CONFLICT (user_id, kind) DO UPDATE
SET value_hash = EXCLUDED.value_hash,
    verified = EXCLUDED.verified,
    verified_at = CASE WHEN EXCLUDED.verified THEN now() ELSE identity_credentials.verified_at END
RETURNING *;

-- name: DeleteCredential :exec
DELETE FROM identity_credentials WHERE user_id = $1 AND kind = $2;
