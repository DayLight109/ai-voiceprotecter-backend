package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sentinel/gateway/internal/domain"
)

func scanCallLog(r interface {
	Scan(dest ...any) error
}) (domain.CallLog, error) {
	var c domain.CallLog
	var userID, region, duration, reason *string
	err := r.Scan(&c.ID, &c.TenantID, &userID, &c.Phone, &region, &duration, &c.Verdict, &reason, &c.RiskScore, &c.CreatedAt)
	if err != nil {
		return c, err
	}
	if userID != nil {
		c.UserID = *userID
	}
	if region != nil {
		c.Region = *region
	}
	if duration != nil {
		c.Duration = *duration
	}
	if reason != nil {
		c.Reason = *reason
	}
	return c, nil
}

const callLogCols = `id, tenant_id, user_id, phone, region, duration, verdict, reason, COALESCE(risk_score,0), created_at`

func (r *Repo) ListCallLogs(ctx context.Context, tenantID string, p Page) ([]domain.CallLog, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM call_logs WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+callLogCols+` FROM call_logs WHERE tenant_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.CallLog, 0, limit)
	for rows.Next() {
		c, err := scanCallLog(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, c)
	}
	return out, total, nil
}

// ListAllCallLogs 不带 tenant 过滤，供 sysadmin 看全局通话用。
func (r *Repo) ListAllCallLogs(ctx context.Context, p Page) ([]domain.CallLog, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM call_logs`).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+callLogCols+` FROM call_logs
		 ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.CallLog, 0, limit)
	for rows.Next() {
		c, err := scanCallLog(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, c)
	}
	return out, total, nil
}

// CallLogDetail 含完整 jsonb
type CallLogDetail struct {
	domain.CallLog
	TraceJSON      json.RawMessage `json:"trace,omitempty"`
	VoiceprintJSON json.RawMessage `json:"voiceprint,omitempty"`
	ScriptJSON     json.RawMessage `json:"script,omitempty"`
}

func (r *Repo) GetCallLogByID(ctx context.Context, id string) (CallLogDetail, error) {
	var d CallLogDetail
	var userID, region, duration, reason *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, phone, region, duration, verdict, reason,
		       COALESCE(risk_score,0), created_at, trace_json, voiceprint_json, script_json
		FROM call_logs WHERE id = $1`, id).
		Scan(&d.ID, &d.TenantID, &userID, &d.Phone, &region, &duration, &d.Verdict, &reason,
			&d.RiskScore, &d.CreatedAt, &d.TraceJSON, &d.VoiceprintJSON, &d.ScriptJSON)
	if err != nil {
		return d, translateErr(err)
	}
	if userID != nil {
		d.UserID = *userID
	}
	if region != nil {
		d.Region = *region
	}
	if duration != nil {
		d.Duration = *duration
	}
	if reason != nil {
		d.Reason = *reason
	}
	return d, nil
}

type CreateCallLogParams struct {
	ID, TenantID, UserID, Phone, Region, Duration, Verdict, Reason string
	RiskScore                                                      int
	TraceJSON, VoiceprintJSON, ScriptJSON                          []byte
}

func (r *Repo) CreateCallLog(ctx context.Context, p CreateCallLogParams) (domain.CallLog, error) {
	var owner any = p.UserID
	if p.UserID == "" {
		owner = nil
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO call_logs (id, tenant_id, user_id, phone, region, duration, verdict, reason, risk_score, trace_json, voiceprint_json, script_json)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,NULLIF($8,''),$9,$10::jsonb,$11::jsonb,$12::jsonb)
		RETURNING `+callLogCols,
		p.ID, p.TenantID, owner, p.Phone, p.Region, p.Duration, p.Verdict, p.Reason, p.RiskScore,
		p.TraceJSON, p.VoiceprintJSON, p.ScriptJSON,
	)
	c, err := scanCallLog(row)
	return c, translateErr(err)
}

// LatestVoiceprintLog warroom「最近一次声纹判决」视图。
type LatestVoiceprintLog struct {
	ID             string
	CreatedAt      time.Time
	RiskScore      int
	Verdict        string // call_logs 中文判定：拦截/预警/通过
	VoiceprintJSON json.RawMessage
}

// GetLatestVoiceprintCallLog 取最近一条带声纹结果的通话。
// tenantID 为空 = 不过滤（sysadmin）。无记录返回 ErrNotFound。
func (r *Repo) GetLatestVoiceprintCallLog(ctx context.Context, tenantID string) (LatestVoiceprintLog, error) {
	var v LatestVoiceprintLog
	err := r.pool.QueryRow(ctx, `
		SELECT id, created_at, COALESCE(risk_score,0), verdict, voiceprint_json
		FROM call_logs
		WHERE voiceprint_json IS NOT NULL AND voiceprint_json::text <> 'null'
		  AND ($1::text = '' OR tenant_id = $1)
		ORDER BY created_at DESC LIMIT 1`, tenantID).
		Scan(&v.ID, &v.CreatedAt, &v.RiskScore, &v.Verdict, &v.VoiceprintJSON)
	return v, translateErr(err)
}

// CallLogStats 启动时回填内存计数器用的全局统计。
type CallLogStats struct {
	Total      int64 // 总分析次数
	Blocked    int64 // 判定为拦截
	AIClones   int64 // 声纹判 SYNTH
	ScriptHits int64 // 话术层有命中
}

func (r *Repo) CountCallLogStats(ctx context.Context) (CallLogStats, error) {
	var s CallLogStats
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE verdict = '拦截'),
		       COUNT(*) FILTER (WHERE voiceprint_json->>'verdict' = 'SYNTH'),
		       COUNT(*) FILTER (WHERE jsonb_array_length(COALESCE(script_json->'hits','[]'::jsonb)) > 0)
		FROM call_logs`).
		Scan(&s.Total, &s.Blocked, &s.AIClones, &s.ScriptHits)
	return s, translateErr(err)
}
