package repo

import "context"

// Avatar 用户头像（存 Postgres bytea，避免依赖对象存储）。
type Avatar struct {
	ContentType string
	Bytes       []byte
}

// GetAvatar 读取用户头像；无则 ErrNotFound。
func (r *Repo) GetAvatar(ctx context.Context, userID string) (Avatar, error) {
	var a Avatar
	err := r.pool.QueryRow(ctx,
		`SELECT content_type, bytes FROM user_avatars WHERE user_id = $1`, userID,
	).Scan(&a.ContentType, &a.Bytes)
	return a, translateErr(err)
}

// UpsertAvatar 写入/覆盖用户头像。
func (r *Repo) UpsertAvatar(ctx context.Context, userID, contentType string, data []byte) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_avatars (user_id, content_type, bytes, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id)
		DO UPDATE SET content_type = EXCLUDED.content_type,
		              bytes = EXCLUDED.bytes,
		              updated_at = now()`,
		userID, contentType, data)
	return translateErr(err)
}

// DeleteAvatar 删除用户头像；不存在视为成功（幂等）。
func (r *Repo) DeleteAvatar(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM user_avatars WHERE user_id = $1`, userID)
	return translateErr(err)
}
