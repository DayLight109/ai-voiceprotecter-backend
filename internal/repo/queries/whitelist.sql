-- name: ListWhitelist :many
SELECT * FROM whitelist
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountWhitelist :one
SELECT COUNT(*) FROM whitelist WHERE tenant_id = $1;

-- name: SearchWhitelist :many
SELECT * FROM whitelist
WHERE tenant_id = $1
  AND (number ILIKE $2 OR name ILIKE $2 OR relation ILIKE $2)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetWhitelistByNumber :one
SELECT * FROM whitelist WHERE tenant_id = $1 AND number = $2 LIMIT 1;

-- name: CreateWhitelist :one
INSERT INTO whitelist (id, tenant_id, number, name, relation)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateWhitelist :one
UPDATE whitelist SET number=$3, name=$4, relation=$5
WHERE id=$1 AND tenant_id=$2
RETURNING *;

-- name: DeleteWhitelist :exec
DELETE FROM whitelist WHERE id = $1 AND tenant_id = $2;
