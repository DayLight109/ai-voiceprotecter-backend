package repo

import (
	"context"
	"encoding/json"
	"strings"
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

// AuditFilter 审计日志高级检索条件。空值 = 不过滤。
type AuditFilter struct {
	ActorID string
	Action  string    // 精确匹配
	Result  string    // 精确匹配（成功 / 失败）
	From    time.Time // ts >= From
	To      time.Time // ts < To
	Q       string    // 模糊匹配 action / target / actor_id
}

// buildAuditWhere 构造动态 WHERE。返回 (sql, args)；sql 已带 " WHERE "（或空字符串）。
func buildAuditWhere(f AuditFilter) (string, []any) {
	conds := make([]string, 0, 6)
	args := make([]any, 0, 6)
	ph := func(v any) string {
		args = append(args, v)
		return placeholder(len(args))
	}
	if f.ActorID != "" {
		conds = append(conds, "actor_id = "+ph(f.ActorID))
	}
	if f.Action != "" {
		conds = append(conds, "action = "+ph(f.Action))
	}
	if f.Result != "" {
		conds = append(conds, "result = "+ph(f.Result))
	}
	if !f.From.IsZero() {
		conds = append(conds, "ts >= "+ph(f.From))
	}
	if !f.To.IsZero() {
		conds = append(conds, "ts < "+ph(f.To))
	}
	if f.Q != "" {
		// 同一个值复用 3 个 placeholder
		p := ph("%" + f.Q + "%")
		conds = append(conds,
			"(action ILIKE "+p+" OR COALESCE(target,'') ILIKE "+p+" OR COALESCE(actor_id,'') ILIKE "+p+")")
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func placeholder(i int) string {
	return "$" + itoa(i)
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

func (r *Repo) ListAuditLogs(ctx context.Context, f AuditFilter, p Page) ([]AuditRow, int64, error) {
	limit, offset := p.Clamp()
	where, args := buildAuditWhere(f)

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs`+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}

	// 把 limit/offset 拼到 args 尾部
	limArgs := append(append([]any{}, args...), limit, offset)
	limIdx := placeholder(len(args) + 1)
	offIdx := placeholder(len(args) + 2)

	rows, err := r.pool.Query(ctx, `
		SELECT id, ts, COALESCE(actor_id,''), action, COALESCE(target,''), result, COALESCE(host(ip),''), payload
		FROM audit_logs`+where+`
		ORDER BY ts DESC LIMIT `+limIdx+` OFFSET `+offIdx,
		limArgs...)
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
