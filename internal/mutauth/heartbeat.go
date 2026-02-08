package mutauth

import (
	"context"
	"sync"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/revoke"
)

const (
	defaultMaxMiss  = 3
	defaultInterval = 30 * time.Second
)

type heartbeatState struct {
	agentID     string
	state       string // "active" | "idle" | "completing"
	lastSeen    time.Time
	missedCount int
}

// HeartbeatMgr tracks agent liveness and optionally auto-revokes agents that
// miss too many heartbeat windows. When revSvc is non-nil, agents exceeding
// maxMiss consecutive misses are revoked at the agent level.
type HeartbeatMgr struct {
	mu       sync.RWMutex
	agents   map[string]*heartbeatState
	revSvc   *revoke.RevSvc // nil = no auto-revocation (investigation-only mode)
	maxMiss  int
	interval time.Duration
}

// NewHeartbeatMgr creates a heartbeat manager. Pass nil for revSvc to disable
// automatic revocation (agents are still flagged via obs.Warn).
func NewHeartbeatMgr(revSvc *revoke.RevSvc) *HeartbeatMgr {
	return &HeartbeatMgr{
		agents:   make(map[string]*heartbeatState),
		revSvc:   revSvc,
		maxMiss:  defaultMaxMiss,
		interval: defaultInterval,
	}
}

// RecordHeartbeat updates an agent's last-seen time and resets its missed count.
// Valid states: "active", "idle", "completing".
func (h *HeartbeatMgr) RecordHeartbeat(agentID, state string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	s, ok := h.agents[agentID]
	if !ok {
		s = &heartbeatState{agentID: agentID}
		h.agents[agentID] = s
	}
	s.state = state
	s.lastSeen = time.Now().UTC()
	s.missedCount = 0

	obs.Trace("MUTAUTH", "Heartbeat.Record", "heartbeat received",
		"agent_id="+agentID, "state="+state)
}

// CheckLiveness reports whether an agent is considered alive based on its
// heartbeat history relative to the configured interval.
func (h *HeartbeatMgr) CheckLiveness(agentID string) (alive bool, missedCount int) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	s, ok := h.agents[agentID]
	if !ok {
		return false, 0
	}

	elapsed := time.Since(s.lastSeen)
	missed := int(elapsed / h.interval)
	total := s.missedCount + missed

	return total < h.maxMiss, total
}

// StartMonitor runs a background goroutine that periodically checks all tracked
// agents for missed heartbeats. Agents exceeding maxMiss are auto-revoked when
// revSvc is configured, otherwise they are logged as warnings for investigation.
// The goroutine exits when ctx is cancelled.
func (h *HeartbeatMgr) StartMonitor(ctx context.Context, interval time.Duration) {
	if interval > 0 {
		h.interval = interval
	}
	go func() {
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				obs.Ok("MUTAUTH", "Heartbeat.Monitor", "monitor stopped")
				return
			case <-ticker.C:
				h.sweep()
			}
		}
	}()
}

func (h *HeartbeatMgr) sweep() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now().UTC()
	for id, s := range h.agents {
		elapsed := now.Sub(s.lastSeen)
		missed := int(elapsed / h.interval)
		if missed <= 0 {
			continue
		}
		s.missedCount += missed
		s.lastSeen = now // reset to avoid double-counting on next sweep

		if s.missedCount >= h.maxMiss {
			if h.revSvc != nil {
				_ = h.revSvc.RevokeAgent(id, "heartbeat: exceeded max missed heartbeats")
				obs.Warn("MUTAUTH", "Heartbeat.Sweep", "agent auto-revoked",
					"agent_id="+id, "missed="+itoa(s.missedCount))
			} else {
				obs.Warn("MUTAUTH", "Heartbeat.Sweep", "agent flagged for investigation",
					"agent_id="+id, "missed="+itoa(s.missedCount))
			}
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 4)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
