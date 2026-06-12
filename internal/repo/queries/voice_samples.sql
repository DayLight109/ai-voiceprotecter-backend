-- name: ListVoiceSamples :many
SELECT * FROM voice_samples ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CountVoiceSamples :one
SELECT COUNT(*) FROM voice_samples;

-- name: GetVoiceSampleByID :one
SELECT * FROM voice_samples WHERE id = $1;

-- name: CreateVoiceSample :one
INSERT INTO voice_samples (id, name, size_bytes, tag, object_key)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: DeleteVoiceSample :exec
DELETE FROM voice_samples WHERE id = $1;
