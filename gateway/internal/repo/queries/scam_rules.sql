-- name: ListScamRules :many
SELECT * FROM scam_rules
WHERE ($1::text = '' OR category = $1)
ORDER BY weight DESC, category
LIMIT $2 OFFSET $3;

-- name: CountScamRules :one
SELECT COUNT(*) FROM scam_rules
WHERE ($1::text = '' OR category = $1);

-- name: ListEnabledScamRules :many
SELECT * FROM scam_rules WHERE enabled = true ORDER BY weight DESC;

-- name: GetScamRuleByID :one
SELECT * FROM scam_rules WHERE id = $1;

-- name: CreateScamRule :one
INSERT INTO scam_rules (id, category, keyword, weight, enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateScamRule :one
UPDATE scam_rules SET category=$2, keyword=$3, weight=$4, enabled=$5, updated_at=now()
WHERE id=$1
RETURNING *;

-- name: DeleteScamRule :exec
DELETE FROM scam_rules WHERE id = $1;
