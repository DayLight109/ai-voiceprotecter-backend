-- name: ListVoiceModels :many
SELECT * FROM voice_models ORDER BY uploaded_at DESC LIMIT $1 OFFSET $2;

-- name: CountVoiceModels :one
SELECT COUNT(*) FROM voice_models;

-- name: GetVoiceModelByID :one
SELECT * FROM voice_models WHERE id = $1;

-- name: GetActiveVoiceModel :one
SELECT * FROM voice_models WHERE active = true LIMIT 1;

-- name: CreateVoiceModel :one
INSERT INTO voice_models (id, version, accuracy, size_bytes, object_key, active)
VALUES ($1, $2, $3, $4, $5, false)
RETURNING *;

-- name: ActivateVoiceModel :exec
UPDATE voice_models SET active = (id = $1);

-- name: DeleteVoiceModel :exec
DELETE FROM voice_models WHERE id = $1 AND active = false;
