-- name: ListRecordings :many
SELECT * FROM recordings
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRecordings :one
SELECT COUNT(*) FROM recordings WHERE tenant_id = $1;

-- name: GetRecordingByID :one
SELECT * FROM recordings WHERE id = $1;

-- name: CreateRecording :one
INSERT INTO recordings (id, tenant_id, owner_user_id, phone, duration, size_bytes, verdict, object_key, encryption_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: DeleteRecording :exec
DELETE FROM recordings WHERE id = $1 AND tenant_id = $2;

-- name: GetRecordingPolicy :one
SELECT * FROM recording_policy WHERE tenant_id = $1;

-- name: UpsertRecordingPolicy :one
INSERT INTO recording_policy (tenant_id, upload_enabled)
VALUES ($1, $2)
ON CONFLICT (tenant_id) DO UPDATE
SET upload_enabled = EXCLUDED.upload_enabled, updated_at = now()
RETURNING *;
