// Package store 维护全局原子计数器。
//
// 启动值全部为 0；真实流量到来后通过 Inc* 方法递增。
// DEFCON 默认 5（PEACE），由 sysadmin 通过 POST /api/v1/defcon 调整。
package store

import (
	"sync"
	"sync/atomic"
	"time"
)

type Counters struct {
	InterceptedCalls atomic.Int64
	BlockedCalls     atomic.Int64
	AICloneDetected  atomic.Int64
	ScriptHits       atomic.Int64
	SmsBlocked       atomic.Int64
	FundsHeldYuan    atomic.Int64

	// 引擎调用计数（/analyze 成功 / 失败），供 ops/warroom 监控页
	EngineAnalyzed atomic.Int64
	EngineFailed   atomic.Int64

	mu             sync.RWMutex
	defcon         int
	since          time.Time
	lastAnalyzedAt time.Time
}

type Store struct{ c *Counters }

type Snapshot struct {
	InterceptedCalls int64     `json:"interceptedCalls"`
	BlockedCalls     int64     `json:"blockedCalls"`
	AICloneDetected  int64     `json:"aiCloneDetected"`
	ScriptHits       int64     `json:"scriptHits"`
	SmsBlocked       int64     `json:"smsBlocked"`
	FundsHeldYuan    int64     `json:"fundsHeldYuan"`
	Defcon           int       `json:"defcon"`
	Since            time.Time `json:"since"`
	NowUTC           time.Time `json:"nowUtc"`
}

func New() *Store {
	c := &Counters{
		defcon: 5, // PEACE
		since:  time.Now(),
	}
	return &Store{c: c}
}

func (s *Store) Snapshot() Snapshot {
	s.c.mu.RLock()
	defcon, since := s.c.defcon, s.c.since
	s.c.mu.RUnlock()
	return Snapshot{
		InterceptedCalls: s.c.InterceptedCalls.Load(),
		BlockedCalls:     s.c.BlockedCalls.Load(),
		AICloneDetected:  s.c.AICloneDetected.Load(),
		ScriptHits:       s.c.ScriptHits.Load(),
		SmsBlocked:       s.c.SmsBlocked.Load(),
		FundsHeldYuan:    s.c.FundsHeldYuan.Load(),
		Defcon:           defcon,
		Since:            since,
		NowUTC:           time.Now().UTC(),
	}
}

func (s *Store) IncIntercepted(n int64) { s.c.InterceptedCalls.Add(n) }
func (s *Store) IncBlocked(n int64)     { s.c.BlockedCalls.Add(n) }
func (s *Store) IncAIClones(n int64)    { s.c.AICloneDetected.Add(n) }
func (s *Store) IncScriptHits(n int64)  { s.c.ScriptHits.Add(n) }

// Seed 用持久层统计回填计数器（启动时调用，重启不丢历史口径）。
func (s *Store) Seed(intercepted, blocked, aiClones, scriptHits int64) {
	s.c.InterceptedCalls.Store(intercepted)
	s.c.BlockedCalls.Store(blocked)
	s.c.AICloneDetected.Store(aiClones)
	s.c.ScriptHits.Store(scriptHits)
}

// ── 引擎调用统计（ops / warroom 监控） ────────────────────────

func (s *Store) IncAnalyzed() {
	s.c.EngineAnalyzed.Add(1)
	s.c.mu.Lock()
	s.c.lastAnalyzedAt = time.Now()
	s.c.mu.Unlock()
}

func (s *Store) IncAnalyzeFailed() { s.c.EngineFailed.Add(1) }

// EngineStats 返回（成功数, 失败数, 最近一次成功分析时间；零值表示尚无流量）。
func (s *Store) EngineStats() (analyzed, failed int64, lastAt time.Time) {
	s.c.mu.RLock()
	lastAt = s.c.lastAnalyzedAt
	s.c.mu.RUnlock()
	return s.c.EngineAnalyzed.Load(), s.c.EngineFailed.Load(), lastAt
}

func (s *Store) Defcon() int {
	s.c.mu.RLock()
	defer s.c.mu.RUnlock()
	return s.c.defcon
}

func (s *Store) SetDefcon(level int) {
	if level < 1 || level > 5 {
		return
	}
	s.c.mu.Lock()
	s.c.defcon = level
	s.c.mu.Unlock()
}
