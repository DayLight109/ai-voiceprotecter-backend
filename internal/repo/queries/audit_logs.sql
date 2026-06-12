-- name: ListAuditLogs :many
SELECT id, ts, actor_id, action, target, result, ip, payload
FROM audit_logs
WHERE ($1::text = '' OR actor_id = $1)
ORDER BY ts DESC
LIMIT $2 OFFSET $3;

-- name: CountAuditLogs :one
SELECT COUNT(*) FROM audit_logs WHERE ($1::text = '' OR actor_id = $1);

-- name: CreateAuditLog :exec
INSERT INTO audit_logs (actor_id, action, target, result, ip, user_agent, payload)
VALUES (NULLIF($1,''), $2, $3, $4, NULLIF($5,'')::inet, $6, $7::jsonb);
