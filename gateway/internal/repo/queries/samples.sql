-- name: ListSamples :many
SELECT * FROM samples
WHERE ($1::text = '' OR status = $1)
ORDER BY received_at DESC
LIMIT $2 OFFSET $3;

-- name: CountSamples :one
SELECT COUNT(*) FROM samples WHERE ($1::text = '' OR status = $1);

-- name: GetSampleByID :one
SELECT * FROM samples WHERE id = $1;

-- name: CreateSample :one
INSERT INTO samples (id, call_id, transcript, duration, origin, classification, status, audio_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateSampleStatus :exec
UPDATE samples SET status = $2, classification = $3 WHERE id = $1;
