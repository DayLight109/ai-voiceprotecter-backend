-- name: ListDevices :many
SELECT * FROM devices
WHERE ($1::text = '' OR type = $1)
  AND ($2::text = '' OR tenant_id = $2)
ORDER BY last_seen_at DESC NULLS LAST
LIMIT $3 OFFSET $4;

-- name: CountDevices :one
SELECT COUNT(*) FROM devices
WHERE ($1::text = '' OR type = $1)
  AND ($2::text = '' OR tenant_id = $2);

-- name: GetDeviceByID :one
SELECT * FROM devices WHERE id = $1;

-- name: CreateDevice :one
INSERT INTO devices (id, name, tenant_id, type, status, version, contact)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateDevice :one
UPDATE devices
SET name=$2, status=$3, version=$4, contact=$5
WHERE id=$1
RETURNING *;

-- name: HeartbeatDevice :exec
UPDATE devices SET last_seen_at = now(), status = 'online' WHERE id = $1;

-- name: DeleteDevice :exec
DELETE FROM devices WHERE id = $1;
