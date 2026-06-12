package repo

import (
	"context"
	"encoding/json"
	"time"
)

// AgentConfig 一行 = 一个 (key, value JSONB) 配置项。
type AgentConfig struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

func (r *Repo) ListAgentConfig(ctx context.Context) ([]AgentConfig, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, value, updated_at FROM agent_config ORDER BY key`)
	if err != nil {
		return nil, translateErr(err)
	}
	defer rows.Close()
	out := []AgentConfig{}
	for rows.Next() {
		var a AgentConfig
		if err := rows.Scan(&a.Key, &a.Value, &a.UpdatedAt); err != nil {
			return nil, translateErr(err)
		}
		out = append(out, a)
	}
	return out, nil
}

func (r *Repo) GetAgentConfig(ctx context.Context, key string) (AgentConfig, error) {
	var a AgentConfig
	err := r.pool.QueryRow(ctx,
		`SELECT key, value, updated_at FROM agent_config WHERE key = $1`, key).
		Scan(&a.Key, &a.Value, &a.UpdatedAt)
	return a, translateErr(err)
}

func (r *Repo) UpsertAgentConfig(ctx context.Context, key string, value json.RawMessage) (AgentConfig, error) {
	var a AgentConfig
	err := r.pool.QueryRow(ctx, `
		INSERT INTO agent_config (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
		RETURNING key, value, updated_at`,
		key, []byte(value),
	).Scan(&a.Key, &a.Value, &a.UpdatedAt)
	return a, translateErr(err)
}
