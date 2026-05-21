package repo

import (
	"context"
	"encoding/json"
	"time"
)

type AuditRow struct {
	ID      int64           `json:"id"`
	TS      time.Time       `json:"ts"`
	ActorID string          `json:"actorId,omitempty"`
	Action  string          `json:"action"`
	Target  string          `json:"target,omitempty"`
	Result  string          `json:"result"`
	IP      string          `json:"ip,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (r *Repo) ListAuditLogs(ctx context.Context, actorID string, p Page) ([]AuditRow, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE ($1::text='' OR actor_id=$1)`,
		actorID).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, ts, COALESCE(actor_id,''), action, COALESCE(target,''), result, COALESCE(host(ip),''), payload
		FROM audit_logs
		WHERE ($1::text='' OR actor_id=$1)
		ORDER BY ts DESC LIMIT $2 OFFSET $3`,
		actorID, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]AuditRow, 0, limit)
	for rows.Next() {
		var a AuditRow
		if err := rows.Scan(&a.ID, &a.TS, &a.ActorID, &a.Action, &a.Target, &a.Result, &a.IP, &a.Payload); err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, a)
	}
	return out, total, nil
}
