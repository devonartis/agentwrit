// Package audit provides a tamper-evident, hash-chain audit trail with
// automatic PII sanitization.
//
// Every event recorded via [AuditLog.Record] is assigned a monotonically
// increasing ID, timestamped in UTC, sanitized for sensitive keywords
// (secret, password, private_key, token_value), and linked to the
// previous event via a SHA-256 hash chain. The genesis event's PrevHash
// is 64 zero characters.
//
// Events can be queried by agent, task, event type, and time range using
// [AuditLog.Query] with pagination support (offset + limit).
//
// All methods are safe for concurrent use.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
)

// Event type constants used as the eventType argument to [AuditLog.Record].
// Each constant corresponds to a specific broker operation.
const (
	EventAdminAuth               = "admin_auth"
	EventAdminAuthFailed         = "admin_auth_failed"
	EventLaunchTokenIssued       = "launch_token_issued"
	EventLaunchTokenDenied       = "launch_token_denied"
	EventSidecarActivationIssued = "sidecar_activation_issued"
	EventSidecarActivated        = "sidecar_activated"
	EventSidecarActivationFailed = "sidecar_activation_failed"
	EventAgentRegistered         = "agent_registered"
	EventRegistrationViolation   = "registration_policy_violation"
	EventTokenIssued             = "token_issued"
	EventTokenRevoked            = "token_revoked"
	EventTokenRenewed            = "token_renewed"
	EventTokenReleased           = "token_released"
	EventTokenRenewalFailed      = "token_renewal_failed"
	EventDelegationCreated       = "delegation_created"
	EventResourceAccessed         = "resource_accessed"
	EventSidecarExchangeSuccess   = "sidecar_exchange_success"
	EventSidecarExchangeDenied    = "sidecar_exchange_denied"

	EventTokenAuthFailed                = "token_auth_failed"
	EventTokenRevokedAccess             = "token_revoked_access"
	EventScopeViolation                 = "scope_violation"
	EventScopeCeilingExceeded           = "scope_ceiling_exceeded"
	EventDelegationAttenuationViolation = "delegation_attenuation_violation"
	EventScopesCeilingUpdated           = "scopes_ceiling_updated"
)

// AuditEvent is a single immutable entry in the audit trail. The Hash
// field chains to PrevHash of the subsequent event, creating a
// tamper-evident sequence.
type AuditEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	AgentID   string    `json:"agent_id,omitempty"`
	TaskID    string    `json:"task_id,omitempty"`
	OrchID    string    `json:"orch_id,omitempty"`
	Detail           string    `json:"detail"`
	Resource         string    `json:"resource,omitempty"`
	Outcome          string    `json:"outcome,omitempty"`
	DelegDepth       int       `json:"deleg_depth,omitempty"`
	DelegChainHash   string    `json:"deleg_chain_hash,omitempty"`
	BytesTransferred int64     `json:"bytes_transferred,omitempty"`
	Hash             string    `json:"hash"`
	PrevHash         string    `json:"prev_hash"`
}

// RecordOption is a functional option for adding structured fields to an
// audit event. Pass zero or more options to [AuditLog.Record].
type RecordOption func(*AuditEvent)

// WithResource sets the resource field on an audit event.
func WithResource(r string) RecordOption { return func(e *AuditEvent) { e.Resource = r } }

// WithOutcome sets the outcome field (e.g. "success", "denied") on an audit event.
func WithOutcome(o string) RecordOption { return func(e *AuditEvent) { e.Outcome = o } }

// WithDelegDepth sets the delegation depth on an audit event.
func WithDelegDepth(d int) RecordOption { return func(e *AuditEvent) { e.DelegDepth = d } }

// WithDelegChainHash sets the delegation chain hash on an audit event.
func WithDelegChainHash(h string) RecordOption { return func(e *AuditEvent) { e.DelegChainHash = h } }

// WithBytesTransferred sets the bytes transferred on an audit event.
func WithBytesTransferred(b int64) RecordOption {
	return func(e *AuditEvent) { e.BytesTransferred = b }
}

// QueryFilters defines optional filters for [AuditLog.Query]. Zero-value
// fields are ignored. Limit defaults to 100 and is capped at 1000.
type QueryFilters struct {
	AgentID   string
	TaskID    string
	EventType string
	Outcome   string
	Since     *time.Time
	Until     *time.Time
	Limit     int
	Offset    int
}

// AuditStore is the persistence interface for audit events. Implementations
// must be safe for concurrent use.
type AuditStore interface {
	SaveAuditEvent(AuditEvent) error
}

// AuditLog maintains an append-only, hash-chained sequence of
// [AuditEvent] entries in memory. Create one with [NewAuditLog].
type AuditLog struct {
	mu       sync.RWMutex
	events   []AuditEvent
	prevHash string
	counter  int
	store    AuditStore
}

// NewAuditLog returns an empty audit log with an optional persistence store.
// Pass nil for store to use memory-only mode. The genesis PrevHash is
// initialized to 64 zero characters.
func NewAuditLog(store AuditStore) *AuditLog {
	return &AuditLog{
		events:   make([]AuditEvent, 0),
		prevHash: "0000000000000000000000000000000000000000000000000000000000000000",
		store:    store,
	}
}

// NewAuditLogWithEvents rebuilds an audit log from a set of existing events,
// typically loaded from SQLite on broker startup. The counter and prevHash are
// derived from the last event in the slice, so new events continue the chain
// seamlessly. Pass nil for store to use memory-only mode.
func NewAuditLogWithEvents(store AuditStore, events []AuditEvent) *AuditLog {
	al := &AuditLog{
		events: make([]AuditEvent, len(events)),
		store:  store,
	}
	copy(al.events, events)

	if len(events) > 0 {
		last := events[len(events)-1]
		al.prevHash = last.Hash
		al.counter = len(events)
	} else {
		al.prevHash = "0000000000000000000000000000000000000000000000000000000000000000"
	}
	return al
}

// Record appends a new event to the audit log. The detail string is
// automatically sanitized to redact known sensitive keywords. The event's
// Hash is computed as SHA-256(prevHash | id | timestamp | fields) to
// maintain chain integrity.
func (a *AuditLog) Record(eventType, agentID, taskID, orchID, detail string, opts ...RecordOption) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.counter++
	sanitized := sanitizePII(detail)

	evt := AuditEvent{
		ID:        fmt.Sprintf("evt-%06d", a.counter),
		Timestamp: time.Now().UTC(),
		EventType: eventType,
		AgentID:   agentID,
		TaskID:    taskID,
		OrchID:    orchID,
		Detail:    sanitized,
		PrevHash:  a.prevHash,
	}

	// Apply functional options for structured fields
	for _, opt := range opts {
		opt(&evt)
	}

	// Hash chain: H(prev_hash + event_data)
	evt.Hash = computeHash(evt)
	a.prevHash = evt.Hash
	a.events = append(a.events, evt)

	// Count event by type in Prometheus
	obs.AuditEventsTotal.WithLabelValues(eventType).Inc()

	// Write-through persistence (non-blocking on error)
	if a.store != nil {
		if err := a.store.SaveAuditEvent(evt); err != nil {
			obs.Fail("audit", "persist", "failed to persist audit event", "id="+evt.ID, "error="+err.Error())
		}
	}
}

// Query returns audit events matching the given [QueryFilters] and the
// total count of matching events before pagination. Events are returned
// in insertion order. If no events match, the slice is nil and total is 0.
func (a *AuditLog) Query(filters QueryFilters) ([]AuditEvent, int) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var matched []AuditEvent

	for _, evt := range a.events {
		if filters.AgentID != "" && evt.AgentID != filters.AgentID {
			continue
		}
		if filters.TaskID != "" && evt.TaskID != filters.TaskID {
			continue
		}
		if filters.EventType != "" && evt.EventType != filters.EventType {
			continue
		}
		if filters.Outcome != "" && evt.Outcome != filters.Outcome {
			continue
		}
		if filters.Since != nil && evt.Timestamp.Before(*filters.Since) {
			continue
		}
		if filters.Until != nil && evt.Timestamp.After(*filters.Until) {
			continue
		}
		matched = append(matched, evt)
	}

	total := len(matched)

	// Apply offset
	if filters.Offset > 0 {
		if filters.Offset >= len(matched) {
			return nil, total
		}
		matched = matched[filters.Offset:]
	}

	// Apply limit
	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if len(matched) > limit {
		matched = matched[:limit]
	}

	return matched, total
}

// Events returns a copy of all audit events in insertion order. This
// method is primarily intended for testing.
func (a *AuditLog) Events() []AuditEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]AuditEvent, len(a.events))
	copy(result, a.events)
	return result
}

func computeHash(evt AuditEvent) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%d|%s|%d",
		evt.PrevHash, evt.ID, evt.Timestamp.Format(time.RFC3339Nano),
		evt.EventType, evt.AgentID, evt.TaskID, evt.OrchID, evt.Detail,
		evt.Resource, evt.Outcome, evt.DelegDepth, evt.DelegChainHash, evt.BytesTransferred)
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// sanitizePII masks sensitive data in audit detail strings.
func sanitizePII(detail string) string {
	// Mask anything that looks like a secret, token, or key
	for _, keyword := range []string{"secret", "password", "token_value", "private_key"} {
		if strings.Contains(strings.ToLower(detail), keyword) {
			// Mask the value after the keyword
			detail = maskSensitiveValues(detail)
			break
		}
	}
	return detail
}

func maskSensitiveValues(s string) string {
	// Simple approach: mask anything after "=" or ":" that looks like a secret
	for _, sep := range []string{"=", ": "} {
		parts := strings.SplitN(s, sep, 2)
		if len(parts) == 2 {
			key := strings.ToLower(parts[0])
			for _, kw := range []string{"secret", "password", "private_key", "token_value"} {
				if strings.Contains(key, kw) {
					return parts[0] + sep + "***REDACTED***"
				}
			}
		}
	}
	return s
}
