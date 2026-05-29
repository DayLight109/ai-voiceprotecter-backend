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
-- 授权收口：仅本租户条目可改；全局条目 (tenant_id IS NULL) 只命中 sysadmin。
UPDATE blacklist SET number=$4, reason=NULLIF($5,''), category=$6, risk=$7
WHERE id=$1 AND (tenant_id = $2 OR $3::text = 'sysadmin')
RETURNING *;

-- name: DeleteBlacklist :exec
DELETE FROM blacklist WHERE id = $1 AND (tenant_id = $2 OR $3::text = 'sysadmin');
