package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// ops.go — 运维监测端点（前端 /ops/health 页轮询）。
//
//	GET /api/v1/ops/ping    探活（RTT 测量）
//	GET /api/v1/ops/health  检查项明细（Report）
//	GET /api/v1/ops/series  指标时间序列（Series）
//	GET /api/v1/ops/info    构建/主机信息（Info）
//
// 注意：响应是裸 JSON（无 envelope）——前端 ops 页直接 resp.json() 解析。
// 挂载在公开路由组（监控探针/大屏不带 token）；只暴露聚合指标，不含业务数据。
//
// 外部依赖探测通过环境变量 OPS_DEPENDENCIES 配置（格式：name=url,name=url）。

const serviceVersion = "v0.1.0"

func OpsRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/ping", opsPing)
	r.Get("/health", opsHealth(d))
	r.Get("/series", opsSeries(d))
	r.Get("/info", opsInfo(d))
	return r
}

func writeRawJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func opsPing(w http.ResponseWriter, _ *http.Request) {
	writeRawJSON(w, http.StatusOK, map[string]any{"ok": true, "ts": time.Now().UTC().Format(time.RFC3339Nano)})
}

// ── /ops/health ──────────────────────────────────────────────

type opsCheck struct {
	Name      string         `json:"name"`
	Status    string         `json:"status"` // ok | warn | down
	Message   string         `json:"message,omitempty"`
	LatencyMs *int64         `json:"latencyMs,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
}

func opsHealth(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		checks := make([]opsCheck, 0, 12)

		// Go 运行时
		checks = append(checks, opsCheck{
			Name: "runtime", Status: "ok",
			Message: runtime.Version(),
			Detail: map[string]any{
				"goroutines": runtime.NumGoroutine(),
				"numCPU":     runtime.NumCPU(),
			},
		})

		// 采样器指标（cpu / memory / network / load）
		if d.Ops != nil {
			if smp, ok := d.Ops.Latest(); ok {
				checks = append(checks,
					thresholdCheck("cpu", smp.CPUPct, 85, 97, pct(smp.CPUPct)),
					thresholdCheck("memory", smp.MemPct, 85, 95, pct(smp.MemPct)),
					opsCheck{Name: "network", Status: "ok",
						Message: "↓ " + humanBps(smp.NetRxBps) + " · ↑ " + humanBps(smp.NetTxBps),
						Detail:  map[string]any{"rxBps": smp.NetRxBps, "txBps": smp.NetTxBps}},
					loadCheck(smp.Load1, smp.Load5, smp.Load15),
				)
			}
		}

		// 磁盘（取工作目录所在卷）
		if wd, err := os.Getwd(); err == nil {
			if du, err := disk.Usage(wd); err == nil {
				checks = append(checks, thresholdCheck("disk", du.UsedPercent, 85, 95, pct(du.UsedPercent)))
			}
		}

		// 业务计数器
		if d.Store != nil {
			s := d.Store.Snapshot()
			checks = append(checks, opsCheck{
				Name: "store", Status: "ok",
				Message: "拦截 " + itoa(s.InterceptedCalls) + " · 阻断 " + itoa(s.BlockedCalls),
				Detail: map[string]any{
					"interceptedCalls": s.InterceptedCalls, "blockedCalls": s.BlockedCalls,
					"aiCloneDetected": s.AICloneDetected, "scriptHits": s.ScriptHits,
					"defcon": s.Defcon,
				},
			})
		}

		// SSE 扇出
		if d.Hub != nil {
			subs, buffered, lastAt := d.Hub.Stats()
			c := opsCheck{Name: "feedHub", Status: "ok",
				Message: itoa(int64(subs)) + " 订阅者",
				Detail:  map[string]any{"subscribers": subs, "buffered": buffered}}
			if !lastAt.IsZero() {
				c.Detail["lastEventAt"] = lastAt.UTC().Format(time.RFC3339)
			}
			checks = append(checks, c)
		}

		// 分析引擎（/analyze 调用统计）
		if d.Store != nil {
			analyzed, failed, lastAt := d.Store.EngineStats()
			c := opsCheck{Name: "engine", Status: "ok",
				Message: "已分析 " + itoa(analyzed) + " · 失败 " + itoa(failed),
				Detail:  map[string]any{"analyzed": analyzed, "failed": failed}}
			if !lastAt.IsZero() {
				c.Detail["lastAnalyzedAt"] = lastAt.UTC().Format(time.RFC3339)
			}
			if analyzed == 0 && failed > 0 {
				c.Status = "warn"
				c.Message += "（尚无成功判决）"
			}
			checks = append(checks, c)
		}

		// 依赖：DB（必需）/ Redis / MinIO（可降级）
		checks = append(checks, dbCheck(ctx, d), availCheck("redis", d.Redis != nil && d.Redis.Available()),
			availCheck("minio", d.Storage != nil && d.Storage.Available()))

		// 外部依赖（OPS_DEPENDENCIES=name=url,name=url）
		checks = append(checks, externalChecks(ctx)...)

		overall := "ok"
		for _, c := range checks {
			if c.Status == "down" {
				overall = "down"
				break
			}
			if c.Status == "warn" {
				overall = "warn"
			}
		}

		startedAt, uptime := time.Now(), int64(0)
		if d.Ops != nil {
			startedAt, uptime = d.Ops.StartedAt(), d.Ops.UptimeSec()
		}
		writeRawJSON(w, http.StatusOK, map[string]any{
			"service":   "sentinel-gateway",
			"version":   serviceVersion,
			"status":    overall,
			"uptimeSec": uptime,
			"startedAt": startedAt.UTC().Format(time.RFC3339),
			"nowUtc":    time.Now().UTC().Format(time.RFC3339),
			"checks":    checks,
		})
	}
}

func dbCheck(ctx context.Context, d Deps) opsCheck {
	if d.DB == nil {
		return opsCheck{Name: "db", Status: "down", Message: "未配置"}
	}
	start := time.Now()
	pingCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	if err := d.DB.Ping(pingCtx); err != nil {
		return opsCheck{Name: "db", Status: "down", Message: err.Error()}
	}
	ms := time.Since(start).Milliseconds()
	return opsCheck{Name: "db", Status: "ok", Message: "postgres 可达", LatencyMs: &ms}
}

func availCheck(name string, ok bool) opsCheck {
	if ok {
		return opsCheck{Name: name, Status: "ok", Message: "已连接"}
	}
	// Redis / MinIO 设计为可降级 → warn 而非 down
	return opsCheck{Name: name, Status: "warn", Message: "降级模式（未连接）"}
}

func thresholdCheck(name string, v, warnAt, downAt float64, msg string) opsCheck {
	st := "ok"
	if v >= downAt {
		st = "down"
	} else if v >= warnAt {
		st = "warn"
	}
	return opsCheck{Name: name, Status: st, Message: msg, Detail: map[string]any{"pct": v}}
}

func loadCheck(l1, l5, l15 float64) opsCheck {
	st := "ok"
	if n := float64(runtime.NumCPU()); n > 0 && l1 > n {
		st = "warn"
	}
	return opsCheck{Name: "load", Status: st,
		Message: fmtF(l1) + " / " + fmtF(l5) + " / " + fmtF(l15),
		Detail:  map[string]any{"load1": l1, "load5": l5, "load15": l15}}
}

// externalChecks 探测 OPS_DEPENDENCIES（name=url,name=url），2s 超时。
func externalChecks(ctx context.Context) []opsCheck {
	raw := strings.TrimSpace(os.Getenv("OPS_DEPENDENCIES"))
	if raw == "" {
		return nil
	}
	client := &http.Client{Timeout: 2 * time.Second}
	var out []opsCheck
	for _, pair := range strings.Split(raw, ",") {
		name, url, found := strings.Cut(strings.TrimSpace(pair), "=")
		if !found || name == "" || url == "" {
			continue
		}
		start := time.Now()
		c := opsCheck{Name: name, Status: "ok", Message: url}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			c.Status, c.Message = "down", err.Error()
			out = append(out, c)
			continue
		}
		resp, err := client.Do(req)
		ms := time.Since(start).Milliseconds()
		c.LatencyMs = &ms
		if err != nil {
			c.Status, c.Message = "down", err.Error()
		} else {
			_ = resp.Body.Close()
			if resp.StatusCode >= 400 {
				c.Status, c.Message = "down", "HTTP "+resp.Status
			}
		}
		out = append(out, c)
	}
	return out
}

// ── /ops/series ──────────────────────────────────────────────

func opsSeries(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if d.Ops == nil {
			writeRawJSON(w, http.StatusOK, map[string]any{
				"intervalSec": 0, "capacity": 0, "count": 0, "samples": []any{},
			})
			return
		}
		samples := d.Ops.Snapshot()
		writeRawJSON(w, http.StatusOK, map[string]any{
			"intervalSec": d.Ops.IntervalSec(),
			"capacity":    d.Ops.Capacity(),
			"count":       len(samples),
			"samples":     samples,
		})
	}
}

// ── /ops/info ────────────────────────────────────────────────

func opsInfo(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		startedAt, uptime := time.Now(), int64(0)
		if d.Ops != nil {
			startedAt, uptime = d.Ops.StartedAt(), d.Ops.UptimeSec()
		}
		info := map[string]any{
			"service":   "sentinel-gateway",
			"version":   serviceVersion,
			"goVersion": runtime.Version(),
			"numCPU":    runtime.NumCPU(),
			"goarch":    runtime.GOARCH,
			"goos":      runtime.GOOS,
			"pid":       os.Getpid(),
			"startedAt": startedAt.UTC().Format(time.RFC3339),
			"uptimeSec": uptime,
		}
		if hi, err := host.Info(); err == nil {
			info["hostname"] = hi.Hostname
			info["platform"] = hi.Platform
			info["platformVersion"] = hi.PlatformVersion
			info["kernelVersion"] = hi.KernelVersion
			info["kernelArch"] = hi.KernelArch
			info["bootTime"] = time.Unix(int64(hi.BootTime), 0).UTC().Format(time.RFC3339)
			info["hostUptimeSec"] = hi.Uptime
		}
		if vm, err := mem.VirtualMemory(); err == nil {
			info["totalMemMB"] = vm.Total / 1024 / 1024
		}
		if avg, err := load.Avg(); err == nil {
			info["load1"], info["load5"], info["load15"] = avg.Load1, avg.Load5, avg.Load15
		}
		writeRawJSON(w, http.StatusOK, info)
	}
}

// ── 小工具 ───────────────────────────────────────────────────

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func pct(v float64) string  { return fmtF(v) + "%" }
func fmtF(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func humanBps(v float64) string {
	switch {
	case v >= 1024*1024:
		return strconv.FormatFloat(v/1024/1024, 'f', 1, 64) + " MB/s"
	case v >= 1024:
		return strconv.FormatFloat(v/1024, 'f', 1, 64) + " KB/s"
	default:
		return strconv.FormatFloat(v, 'f', 0, 64) + " B/s"
	}
}
