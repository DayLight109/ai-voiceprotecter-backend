// Package ops 进程/主机指标采样器，为 /api/v1/ops/* 与 /warroom/overview 提供数据。
//
// 后台 goroutine 每 interval 采样一次（CPU / 内存 / 网卡速率 / 负载 / goroutine 数），
// 写入固定容量环形缓冲。所有读取方法并发安全。
// 平台不支持的指标（如 Windows 的 loadavg）安全降级为 0，不报错。
package ops

import (
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	gnet "github.com/shirou/gopsutil/v3/net"
)

// Sample 一次采样。字段名与前端 ops/health 页的 Series.samples 对齐。
type Sample struct {
	T          time.Time `json:"t"`
	CPUPct     float64   `json:"cpuPct"`
	MemPct     float64   `json:"memPct"`
	NetRxBps   float64   `json:"netRxBps"`
	NetTxBps   float64   `json:"netTxBps"`
	Load1      float64   `json:"load1"`
	Load5      float64   `json:"load5"`
	Load15     float64   `json:"load15"`
	Goroutines int       `json:"goroutines"`
}

type Sampler struct {
	mu   sync.RWMutex
	buf  []Sample
	head int // 下一个写入位置
	n    int // 已写入数量（≤ cap）

	interval  time.Duration
	startedAt time.Time
	stop      chan struct{}
	once      sync.Once

	// 网卡累计字节数上次值，用于差分出速率
	lastRx, lastTx uint64
	lastNetAt      time.Time
}

func NewSampler(interval time.Duration, capacity int) *Sampler {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if capacity <= 0 {
		capacity = 360 // 默认 5s × 360 = 30 分钟窗口
	}
	return &Sampler{
		buf:       make([]Sample, capacity),
		interval:  interval,
		startedAt: time.Now(),
		stop:      make(chan struct{}),
	}
}

// Start 启动后台采样。立即采一次，之后按 interval 周期采样。
func (s *Sampler) Start() {
	go func() {
		// cpu.Percent(0) 与上次调用做差分；先空采一次建立基线
		_, _ = cpu.Percent(0, false)
		s.collect()
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-t.C:
				s.collect()
			}
		}
	}()
}

func (s *Sampler) Stop() { s.once.Do(func() { close(s.stop) }) }

func (s *Sampler) collect() {
	now := time.Now()
	smp := Sample{T: now, Goroutines: runtime.NumGoroutine()}

	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		smp.CPUPct = round1(pct[0])
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		smp.MemPct = round1(vm.UsedPercent)
	}
	if avg, err := load.Avg(); err == nil { // Windows 不支持 → 保持 0
		smp.Load1, smp.Load5, smp.Load15 = round2(avg.Load1), round2(avg.Load5), round2(avg.Load15)
	}
	if io, err := gnet.IOCounters(false); err == nil && len(io) > 0 {
		rx, tx := io[0].BytesRecv, io[0].BytesSent
		if !s.lastNetAt.IsZero() {
			dt := now.Sub(s.lastNetAt).Seconds()
			if dt > 0 && rx >= s.lastRx && tx >= s.lastTx {
				smp.NetRxBps = round1(float64(rx-s.lastRx) / dt)
				smp.NetTxBps = round1(float64(tx-s.lastTx) / dt)
			}
		}
		s.lastRx, s.lastTx, s.lastNetAt = rx, tx, now
	}

	s.mu.Lock()
	s.buf[s.head] = smp
	s.head = (s.head + 1) % len(s.buf)
	if s.n < len(s.buf) {
		s.n++
	}
	s.mu.Unlock()
}

// Snapshot 按时间正序返回全部样本。
func (s *Sampler) Snapshot() []Sample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Sample, 0, s.n)
	start := (s.head - s.n + len(s.buf)) % len(s.buf)
	for i := 0; i < s.n; i++ {
		out = append(out, s.buf[(start+i)%len(s.buf)])
	}
	return out
}

// Latest 返回最近一次采样；尚无样本时 ok=false。
func (s *Sampler) Latest() (Sample, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.n == 0 {
		return Sample{}, false
	}
	idx := (s.head - 1 + len(s.buf)) % len(s.buf)
	return s.buf[idx], true
}

func (s *Sampler) IntervalSec() int      { return int(s.interval / time.Second) }
func (s *Sampler) Capacity() int         { return len(s.buf) }
func (s *Sampler) StartedAt() time.Time  { return s.startedAt }
func (s *Sampler) UptimeSec() int64      { return int64(time.Since(s.startedAt) / time.Second) }

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
