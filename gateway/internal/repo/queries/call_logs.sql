-- name: ListCallLogs :many
SELECT id, tenant_id, user_id, phone, region, duration, verdict, reason, risk_score, created_at
FROM call_logs
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountCallLogs :one
SELECT COUNT(*) FROM call_logs WHERE tenant_id = $1;

-- name: GetCallLogByID :one
SELECT * FROM call_logs WHERE id = $1;

-- name: CreateCallLog :one
INSERT INTO call_logs (id, tenant_id, user_id, phone, region, duration, verdict, reason, risk_score, trace_json, voiceprint_json, script_json)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;
