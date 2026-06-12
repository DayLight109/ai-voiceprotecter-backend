package repo

import "context"

// Permission 一行 = 一个开关。
type Permission struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

func (r *Repo) ListPermissions(ctx context.Context, userID string) ([]Permission, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT key, enabled FROM permissions WHERE user_id = $1 ORDER BY key`, userID)
	if err != nil {
		return nil, translateErr(err)
	}
	defer rows.Close()
	out := []Permission{}
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.Key, &p.Enabled); err != nil {
			return nil, translateErr(err)
		}
		out = append(out, p)
	}
	return out, nil
}

// UpsertPermissions 批量设置；每条 upsert 是幂等的，无需事务。
func (r *Repo) UpsertPermissions(ctx context.Context, userID string, perms []Permission) error {
	for _, p := range perms {
		if _, err := r.pool.Exec(ctx, `
			INSERT INTO permissions (user_id, key, enabled)
			VALUES ($1,$2,$3)
			ON CONFLICT (user_id, key) DO UPDATE SET enabled = EXCLUDED.enabled`,
			userID, p.Key, p.Enabled); err != nil {
			return translateErr(err)
		}
	}
	return nil
}

func (r *Repo) DeletePermission(ctx context.Context, userID, key string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM permissions WHERE user_id=$1 AND key=$2`, userID, key)
	return translateErr(err)
}
