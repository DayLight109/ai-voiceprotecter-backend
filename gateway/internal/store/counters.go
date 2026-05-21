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

	mu     sync.RWMutex
	defcon int
	since  time.Time
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
