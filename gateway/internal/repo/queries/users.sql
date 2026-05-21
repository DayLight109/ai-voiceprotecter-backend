-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone = $1;

-- name: ListUsersByTenant :many
SELECT * FROM users
WHERE tenant_id = $1 AND status <> 'suspended'
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountUsersByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND status <> 'suspended';

-- name: CreateUser :one
INSERT INTO users (id, tenant_id, name, phone, email, password_hash, role, status, dept)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET name=$2, dept=$3, role=$4, status=$5
WHERE id=$1
RETURNING *;

-- name: TouchUserLogin :exec
UPDATE users SET last_login_at = now() WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
