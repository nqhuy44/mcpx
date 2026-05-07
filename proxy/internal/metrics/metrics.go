package metrics

import (
	"sync"
	"time"
)

// ServerStats holds live counters for one domain server.
type ServerStats struct {
	mu         sync.RWMutex
	Name       string
	Status     string
	CallCount  int64
	ErrorCount int64
	TokensIn   int64
	TokensOut  int64
	LastCallAt time.Time
}

func (s *ServerStats) SetStatus(status string) {
	s.mu.Lock()
	s.Status = status
	s.mu.Unlock()
}

func (s *ServerStats) GetStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

func (s *ServerStats) RecordCall(tokensIn, tokensOut int64, isError bool) {
	s.mu.Lock()
	s.CallCount++
	s.TokensIn += tokensIn
	s.TokensOut += tokensOut
	s.LastCallAt = time.Now()
	if isError {
		s.ErrorCount++
	}
	s.mu.Unlock()
}

// ServerSnapshot is the JSON-serialisable view of ServerStats.
type ServerSnapshot struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	CallCount  int64     `json:"call_count"`
	ErrorCount int64     `json:"error_count"`
	TokensIn   int64     `json:"tokens_in"`
	TokensOut  int64     `json:"tokens_out"`
	LastCallAt time.Time `json:"last_call_at"`
}

func (s *ServerStats) snapshot() ServerSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ServerSnapshot{
		Name:       s.Name,
		Status:     s.Status,
		CallCount:  s.CallCount,
		ErrorCount: s.ErrorCount,
		TokensIn:   s.TokensIn,
		TokensOut:  s.TokensOut,
		LastCallAt: s.LastCallAt,
	}
}

// SessionStats tracks one MCP client session.
type SessionStats struct {
	mu        sync.RWMutex
	ID        string
	StartTime time.Time
	CallCount int64
}

func (s *SessionStats) RecordCall() {
	s.mu.Lock()
	s.CallCount++
	s.mu.Unlock()
}

// SessionSnapshot is the JSON-serialisable view of SessionStats.
type SessionSnapshot struct {
	ID        string    `json:"id"`
	StartTime time.Time `json:"start_time"`
	CallCount int64     `json:"call_count"`
}

func (s *SessionStats) snapshot() SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SessionSnapshot{ID: s.ID, StartTime: s.StartTime, CallCount: s.CallCount}
}

// Collector is the central registry for all runtime metrics.
type Collector struct {
	mu        sync.RWMutex
	servers   map[string]*ServerStats
	sessions  map[string]*SessionStats
	startTime time.Time
}

func New() *Collector {
	return &Collector{
		servers:   make(map[string]*ServerStats),
		sessions:  make(map[string]*SessionStats),
		startTime: time.Now(),
	}
}

func (c *Collector) RegisterServer(name string) *ServerStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := &ServerStats{Name: name, Status: "connecting"}
	c.servers[name] = s
	return s
}

func (c *Collector) Server(name string) *ServerStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.servers[name]
}

func (c *Collector) TrackSession(id string) *SessionStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := &SessionStats{ID: id, StartTime: time.Now()}
	c.sessions[id] = s
	return s
}

func (c *Collector) EndSession(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, id)
}

// StatusSnapshot is the top-level payload served by /api/status.
type StatusSnapshot struct {
	Uptime         string            `json:"uptime"`
	TotalCalls     int64             `json:"total_calls"`
	TotalTokensIn  int64             `json:"total_tokens_in"`
	TotalTokensOut int64             `json:"total_tokens_out"`
	Servers        []ServerSnapshot  `json:"servers"`
	Sessions       []SessionSnapshot `json:"sessions"`
}

func (c *Collector) Snapshot() StatusSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snap := StatusSnapshot{
		Uptime:   time.Since(c.startTime).Round(time.Second).String(),
		Servers:  make([]ServerSnapshot, 0, len(c.servers)),
		Sessions: make([]SessionSnapshot, 0, len(c.sessions)),
	}

	for _, s := range c.servers {
		ss := s.snapshot()
		snap.Servers = append(snap.Servers, ss)
		snap.TotalCalls += ss.CallCount
		snap.TotalTokensIn += ss.TokensIn
		snap.TotalTokensOut += ss.TokensOut
	}

	for _, s := range c.sessions {
		snap.Sessions = append(snap.Sessions, s.snapshot())
	}

	return snap
}
