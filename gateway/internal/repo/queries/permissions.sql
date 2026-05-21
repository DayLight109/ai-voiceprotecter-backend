-- name: ListPermissions :many
SELECT key, enabled FROM permissions WHERE user_id = $1 ORDER BY key;

-- name: UpsertPermission :exec
INSERT INTO permissions (user_id, key, enabled)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, key) DO UPDATE SET enabled = EXCLUDED.enabled;

-- name: DeletePermission :exec
DELETE FROM permissions WHERE user_id = $1 AND key = $2;
