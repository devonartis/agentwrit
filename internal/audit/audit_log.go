package audit

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
)

// AuditLog provides in-memory audit event storage with hash chain integrity.
type AuditLog struct {
	mu       sync.RWMutex
	events   []AuditEvt
	lastHash string
}

// NewAuditLog creates an AuditLog with an empty event chain.
func NewAuditLog() *AuditLog {
	return &AuditLog{}
}

// LogEvent persists an audit event: generates event_id, sets timestamp,
// computes chain hash, sanitizes PII, and appends to the store.
func (a *AuditLog) LogEvent(evt *AuditEvt) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	evt.EventId = randomEventId()
	evt.Timestamp = time.Now().UTC().Format(time.RFC3339)
	evt.PrevHash = a.lastHash

	sanitized := Sanitize(evt)
	sanitized.EventHash = HashEvent(sanitized, sanitized.PrevHash)

	a.events = append(a.events, *sanitized)
	a.lastHash = sanitized.EventHash

	obs.Ok("AUDIT", "AuditLog.LogEvent", "event recorded",
		"event_id="+sanitized.EventId,
		"type="+sanitized.EventType,
		"agent="+sanitized.AgentInstanceId,
	)
	return nil
}

// QueryEvents filters and paginates stored events.
// Returns (events, total, error).
func (a *AuditLog) QueryEvents(filter AuditFilter) ([]AuditEvt, int, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	var matched []AuditEvt
	for _, evt := range a.events {
		if filter.AgentId != "" && evt.AgentInstanceId != filter.AgentId {
			continue
		}
		if filter.TaskId != "" && evt.TaskId != filter.TaskId {
			continue
		}
		if filter.OrchId != "" && evt.OrchId != filter.OrchId {
			continue
		}
		if filter.EventType != "" && evt.EventType != filter.EventType {
			continue
		}
		if filter.From != "" {
			fromT, err := time.Parse(time.RFC3339, filter.From)
			evtT, err2 := time.Parse(time.RFC3339, evt.Timestamp)
			if err == nil && err2 == nil && evtT.Before(fromT) {
				continue
			}
		}
		if filter.To != "" {
			toT, err := time.Parse(time.RFC3339, filter.To)
			evtT, err2 := time.Parse(time.RFC3339, evt.Timestamp)
			if err == nil && err2 == nil && evtT.After(toT) {
				continue
			}
		}
		matched = append(matched, evt)
	}

	total := len(matched)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(matched) {
		return nil, total, nil
	}
	matched = matched[offset:]
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, total, nil
}

func randomEventId() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "evt-fallback"
	}
	return hex.EncodeToString(b)
}
