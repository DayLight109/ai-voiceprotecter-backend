-- name: GetRiskLevelState :one
SELECT * FROM risk_level_state WHERE tenant_id = $1;

-- name: UpsertRiskLevelState :one
INSERT INTO risk_level_state (tenant_id, active_level)
VALUES ($1, $2)
ON CONFLICT (tenant_id) DO UPDATE
SET active_level = EXCLUDED.active_level, updated_at = now()
RETURNING *;

-- name: ListRiskLevelRules :many
SELECT * FROM risk_level_rules
WHERE tenant_id = $1 AND ($2::int = 0 OR level = $2)
ORDER BY level, weight DESC
LIMIT $3 OFFSET $4;

-- name: CountRiskLevelRules :one
SELECT COUNT(*) FROM risk_level_rules
WHERE tenant_id = $1 AND ($2::int = 0 OR level = $2);

-- name: CreateRiskLevelRule :one
INSERT INTO risk_level_rules (id, tenant_id, level, keyword, weight, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateRiskLevelRule :one
UPDATE risk_level_rules
SET level=$3, keyword=$4, weight=$5, enabled=$6, updated_at=now()
WHERE id=$1 AND tenant_id=$2
RETURNING *;

-- name: DeleteRiskLevelRule :exec
DELETE FROM risk_level_rules WHERE id = $1 AND tenant_id = $2;
