-- name: ListBlacklist :many
-- 当前租户 + 全局 (tenant_id IS NULL)
SELECT * FROM blacklist
WHERE (tenant_id = $1 OR tenant_id IS NULL)
ORDER BY risk DESC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountBlacklist :one
SELECT COUNT(*) FROM blacklist
WHERE (tenant_id = $1 OR tenant_id IS NULL);

-- name: SearchBlacklist :many
SELECT * FROM blacklist
WHERE (tenant_id = $1 OR tenant_id IS NULL)
  AND (number ILIKE $2 OR reason ILIKE $2 OR category ILIKE $2)
ORDER BY risk DESC
LIMIT $3 OFFSET $4;

-- name: GetBlacklistByNumber :one
SELECT * FROM blacklist
WHERE (tenant_id = $1 OR tenant_id IS NULL) AND number = $2
LIMIT 1;

-- name: CreateBlacklist :one
INSERT INTO blacklist (id, tenant_id, number, reason, category, risk, source, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateBlacklist :one
UPDATE blacklist SET number=$2, reason=$3, category=$4, risk=$5
WHERE id=$1
RETURNING *;

-- name: DeleteBlacklist :exec
DELETE FROM blacklist WHERE id = $1 AND (tenant_id = $2 OR $3::text = 'sysadmin');
