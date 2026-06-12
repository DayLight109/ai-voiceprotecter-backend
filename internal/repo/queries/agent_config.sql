-- name: GetAgentConfig :one
SELECT * FROM agent_config WHERE key = $1;

-- name: UpsertAgentConfig :one
INSERT INTO agent_config (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = now()
RETURNING *;

-- name: ListAgentConfig :many
SELECT * FROM agent_config ORDER BY key;
