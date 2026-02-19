// Package store provides the persistence layer for nonces, launch tokens,
// and agent records.
//
// The current implementation ([SqlStore]) keeps everything in memory behind
// a [sync.RWMutex]. The type is named SqlStore to ease a future migration to
// a SQL-backed store without changing call sites.
//
// SQLite-backed audit persistence is available via [SqlStore.InitDB],
// [SqlStore.SaveAuditEvent], [SqlStore.LoadAllAuditEvents],
// [SqlStore.QueryAuditEvents], and [SqlStore.Close].
//
// All public methods are safe for concurrent use.
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/obs"
	_ "modernc.org/sqlite" // register sqlite driver
)

// Sentinel errors returned by store operations. Callers can use
// [errors.Is] to match specific failure modes.
var (
	ErrNonceNotFound = errors.New("nonce not found or expired")
	ErrNonceConsumed = errors.New("nonce already consumed")
	ErrTokenNotFound = errors.New("launch token not found")
	ErrTokenExpired  = errors.New("launch token expired")
	ErrTokenConsumed = errors.New("launch token already consumed")
	ErrAgentNotFound    = errors.New("agent not found")
	ErrCeilingNotFound  = errors.New("sidecar ceiling not found")
)

// LaunchTokenRecord represents a pre-authorized launch token created by an
// admin. It binds an agent name to an allowed scope ceiling and optional
// TTL cap. A single-use token is consumed on first registration.
type LaunchTokenRecord struct {
	Token        string
	AgentName    string
	AllowedScope []string
	MaxTTL       int
	SingleUse    bool
	CreatedAt    time.Time
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	CreatedBy    string
}

// AgentRecord stores the persistent state of a registered agent,
// including its SPIFFE-format AgentID, Ed25519 public key, and the
// scope granted at registration time.
type AgentRecord struct {
	AgentID      string
	PublicKey    []byte
	OrchID       string
	TaskID       string
	Scope        []string
	RegisteredAt time.Time
	LastSeen     time.Time
}

// SidecarRecord stores the persistent state of an activated sidecar.
type SidecarRecord struct {
	ID        string
	Ceiling   []string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type nonceRecord struct {
	value     string
	expiresAt time.Time
	consumed  bool
}

// SqlStore provides in-memory storage with read/write mutex protection,
// and optional SQLite-backed audit event persistence. Create one with
// [NewSqlStore] and call [InitDB] to enable persistence.
type SqlStore struct {
	mu             sync.RWMutex
	nonces         map[string]*nonceRecord
	launchTokens   map[string]*LaunchTokenRecord
	agents         map[string]*AgentRecord
	jtiConsumption map[string]time.Time
	ceilings       map[string][]string // sidecar ID → scope ceiling
	db             *sql.DB
}

// NewSqlStore returns an initialized, empty in-memory store ready for use.
func NewSqlStore() *SqlStore {
	return &SqlStore{
		nonces:         make(map[string]*nonceRecord),
		launchTokens:   make(map[string]*LaunchTokenRecord),
		agents:         make(map[string]*AgentRecord),
		jtiConsumption: make(map[string]time.Time),
		ceilings:       make(map[string][]string),
	}
}

// CreateNonce generates a cryptographically random 64-character hex nonce,
// stores it with a 30-second TTL, and returns the nonce string. The nonce
// must be consumed via [SqlStore.ConsumeNonce] before it expires.
func (s *SqlStore) CreateNonce() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	nonce := hex.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonces[nonce] = &nonceRecord{
		value:     nonce,
		expiresAt: time.Now().Add(30 * time.Second),
		consumed:  false,
	}
	return nonce
}

// ConsumeNonce atomically marks a nonce as consumed. It returns
// [ErrNonceNotFound] if the nonce does not exist or has expired, or
// [ErrNonceConsumed] if it was already consumed.
func (s *SqlStore) ConsumeNonce(nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.nonces[nonce]
	if !ok {
		return ErrNonceNotFound
	}
	if rec.consumed {
		return ErrNonceConsumed
	}
	if time.Now().After(rec.expiresAt) {
		delete(s.nonces, nonce)
		return ErrNonceNotFound
	}
	rec.consumed = true
	return nil
}

// SaveLaunchToken persists a [LaunchTokenRecord], keyed by its Token field.
// If a record with the same token already exists it is silently overwritten.
func (s *SqlStore) SaveLaunchToken(rec LaunchTokenRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.launchTokens[rec.Token] = &rec
	return nil
}

// GetLaunchToken retrieves a launch token record by its opaque token string.
// It returns [ErrTokenNotFound] if the token does not exist,
// [ErrTokenExpired] if it has passed its ExpiresAt time, or
// [ErrTokenConsumed] if it was already consumed.
func (s *SqlStore) GetLaunchToken(token string) (*LaunchTokenRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.launchTokens[token]
	if !ok {
		return nil, ErrTokenNotFound
	}
	if time.Now().After(rec.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	if rec.ConsumedAt != nil {
		return nil, ErrTokenConsumed
	}
	return rec, nil
}

// ConsumeLaunchToken sets the ConsumedAt timestamp on a launch token,
// preventing it from being used again. It returns [ErrTokenNotFound] if
// the token does not exist or [ErrTokenConsumed] if already consumed.
func (s *SqlStore) ConsumeLaunchToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.launchTokens[token]
	if !ok {
		return ErrTokenNotFound
	}
	if rec.ConsumedAt != nil {
		return ErrTokenConsumed
	}
	now := time.Now()
	rec.ConsumedAt = &now
	return nil
}

// SaveAgent persists an [AgentRecord], keyed by its AgentID (SPIFFE ID).
func (s *SqlStore) SaveAgent(rec AgentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[rec.AgentID] = &rec
	return nil
}

// GetAgent retrieves an [AgentRecord] by its SPIFFE ID. It returns
// [ErrAgentNotFound] if no record exists for agentID.
func (s *SqlStore) GetAgent(agentID string) (*AgentRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.agents[agentID]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return rec, nil
}

// UpdateAgentLastSeen sets the LastSeen field of the agent identified by
// agentID to the current time. It returns [ErrAgentNotFound] if no matching
// record exists.
func (s *SqlStore) UpdateAgentLastSeen(agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.agents[agentID]
	if !ok {
		return ErrAgentNotFound
	}
	rec.LastSeen = time.Now()
	return nil
}

// ConsumeActivationToken marks an activation token's JTI as consumed. It
// returns [ErrTokenConsumed] if the JTI has already been recorded. The exp
// timestamp is provided to allow for future garbage collection of the JTI
// set, though the current in-memory implementation does not yet implement
// automatic pruning.
func (s *SqlStore) ConsumeActivationToken(jti string, exp int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jtiConsumption[jti]; exists {
		return ErrTokenConsumed
	}

	s.jtiConsumption[jti] = time.Unix(exp, 0)
	return nil
}

// SaveCeiling persists a scope ceiling for the given sidecar ID,
// overwriting any existing ceiling.
func (s *SqlStore) SaveCeiling(sidecarID string, ceiling []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(ceiling))
	copy(cp, ceiling)
	s.ceilings[sidecarID] = cp
	return nil
}

// GetCeiling retrieves the stored scope ceiling for the given sidecar ID.
// Returns [ErrCeilingNotFound] if no ceiling has been stored.
func (s *SqlStore) GetCeiling(sidecarID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.ceilings[sidecarID]
	if !ok {
		return nil, ErrCeilingNotFound
	}
	cp := make([]string, len(c))
	copy(cp, c)
	return cp, nil
}

// ---------------------------------------------------------------------------
// SQLite audit persistence
// ---------------------------------------------------------------------------

const createAuditTable = `
CREATE TABLE IF NOT EXISTS audit_events (
	id          TEXT PRIMARY KEY,
	timestamp   TEXT NOT NULL,
	event_type  TEXT NOT NULL,
	agent_id    TEXT NOT NULL DEFAULT '',
	task_id     TEXT NOT NULL DEFAULT '',
	orch_id     TEXT NOT NULL DEFAULT '',
	detail      TEXT NOT NULL DEFAULT '',
	hash        TEXT NOT NULL,
	prev_hash   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audit_events(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_agent_id   ON audit_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp  ON audit_events(timestamp);
`

const createSidecarsTable = `
CREATE TABLE IF NOT EXISTS sidecars (
	id         TEXT PRIMARY KEY,
	ceiling    TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`

// InitDB opens the SQLite database at path and creates the audit_events table
// and indexes if they do not already exist. It must be called before any audit
// persistence methods are used.
func (s *SqlStore) InitDB(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to open database", "path="+path, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("open").Inc()
		return fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if _, err = db.Exec(createAuditTable); err != nil {
		db.Close()
		obs.Fail("store", "sqlite", "failed to create audit table", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("create_table").Inc()
		return fmt.Errorf("create audit table: %w", err)
	}
	if _, err = db.Exec(createSidecarsTable); err != nil {
		db.Close()
		obs.Fail("store", "sqlite", "failed to create sidecars table", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("create_table").Inc()
		return fmt.Errorf("create sidecars table: %w", err)
	}
	s.db = db
	obs.Ok("store", "sqlite", "database initialized", "path="+path)
	return nil
}

// SaveAuditEvent persists an [audit.AuditEvent] to the SQLite audit_events
// table. The write duration is recorded in [obs.AuditWriteDuration]. Returns
// an error if InitDB has not been called or the insert fails.
func (s *SqlStore) SaveAuditEvent(evt audit.AuditEvent) error {
	if s.db == nil {
		return errors.New("database not initialized: call InitDB first")
	}
	timer := obs.AuditWriteDuration
	start := time.Now()
	defer func() {
		timer.Observe(time.Since(start).Seconds())
	}()

	const q = `INSERT INTO audit_events
		(id, timestamp, event_type, agent_id, task_id, orch_id, detail, hash, prev_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(q,
		evt.ID,
		evt.Timestamp.UTC().Format(time.RFC3339Nano),
		evt.EventType,
		evt.AgentID,
		evt.TaskID,
		evt.OrchID,
		evt.Detail,
		evt.Hash,
		evt.PrevHash,
	)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to save audit event", "id="+evt.ID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("save_audit_event").Inc()
		return fmt.Errorf("save audit event %s: %w", evt.ID, err)
	}
	return nil
}

// LoadAllAuditEvents returns all audit events ordered by id ascending. This
// is used at broker startup to rebuild the in-memory hash chain from the
// persisted SQLite state.
func (s *SqlStore) LoadAllAuditEvents() ([]audit.AuditEvent, error) {
	if s.db == nil {
		return nil, errors.New("database not initialized: call InitDB first")
	}
	const q = `SELECT id, timestamp, event_type, agent_id, task_id, orch_id, detail, hash, prev_hash
		FROM audit_events ORDER BY id ASC`

	rows, err := s.db.Query(q)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to load audit events", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("load_audit_events").Inc()
		return nil, fmt.Errorf("load audit events: %w", err)
	}
	defer rows.Close()

	var events []audit.AuditEvent
	for rows.Next() {
		var evt audit.AuditEvent
		var tsStr string
		if err := rows.Scan(&evt.ID, &tsStr, &evt.EventType, &evt.AgentID, &evt.TaskID,
			&evt.OrchID, &evt.Detail, &evt.Hash, &evt.PrevHash); err != nil {
			obs.Fail("store", "sqlite", "failed to scan audit event row", "error="+err.Error())
			obs.DBErrorsTotal.WithLabelValues("scan_audit_event").Inc()
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		ts, err := time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			return nil, fmt.Errorf("parse timestamp %q: %w", tsStr, err)
		}
		evt.Timestamp = ts
		events = append(events, evt)
	}
	if err := rows.Err(); err != nil {
		obs.Fail("store", "sqlite", "row iteration error", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("iterate_audit_events").Inc()
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}
	obs.Ok("store", "sqlite", "audit events loaded", fmt.Sprintf("count=%d", len(events)))
	return events, nil
}

// QueryAuditEvents returns audit events matching the given [audit.QueryFilters]
// and the total count of matching rows before pagination. Supports filtering by
// EventType, AgentID, TaskID, Since, and Until. Limit and Offset control
// pagination; Limit defaults to 100 and is capped at 1000.
func (s *SqlStore) QueryAuditEvents(filters audit.QueryFilters) ([]audit.AuditEvent, int, error) {
	if s.db == nil {
		return nil, 0, errors.New("database not initialized: call InitDB first")
	}

	var whereClauses []string
	var args []any

	if filters.EventType != "" {
		whereClauses = append(whereClauses, "event_type = ?")
		args = append(args, filters.EventType)
	}
	if filters.AgentID != "" {
		whereClauses = append(whereClauses, "agent_id = ?")
		args = append(args, filters.AgentID)
	}
	if filters.TaskID != "" {
		whereClauses = append(whereClauses, "task_id = ?")
		args = append(args, filters.TaskID)
	}
	if filters.Since != nil {
		whereClauses = append(whereClauses, "timestamp >= ?")
		args = append(args, filters.Since.UTC().Format(time.RFC3339Nano))
	}
	if filters.Until != nil {
		whereClauses = append(whereClauses, "timestamp <= ?")
		args = append(args, filters.Until.UTC().Format(time.RFC3339Nano))
	}

	where := ""
	if len(whereClauses) > 0 {
		where = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count total matching rows
	var total int
	countQ := "SELECT COUNT(*) FROM audit_events" + where
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		obs.Fail("store", "sqlite", "failed to count audit events", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("count_audit_events").Inc()
		return nil, 0, fmt.Errorf("count audit events: %w", err)
	}

	// Apply limit/offset defaults
	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	selectQ := "SELECT id, timestamp, event_type, agent_id, task_id, orch_id, detail, hash, prev_hash FROM audit_events" +
		where + " ORDER BY id ASC LIMIT ? OFFSET ?"
	queryArgs := append(args, limit, offset)

	rows, err := s.db.Query(selectQ, queryArgs...)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to query audit events", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("query_audit_events").Inc()
		return nil, 0, fmt.Errorf("query audit events: %w", err)
	}
	defer rows.Close()

	var events []audit.AuditEvent
	for rows.Next() {
		var evt audit.AuditEvent
		var tsStr string
		if err := rows.Scan(&evt.ID, &tsStr, &evt.EventType, &evt.AgentID, &evt.TaskID,
			&evt.OrchID, &evt.Detail, &evt.Hash, &evt.PrevHash); err != nil {
			obs.Fail("store", "sqlite", "failed to scan audit event", "error="+err.Error())
			obs.DBErrorsTotal.WithLabelValues("scan_audit_event").Inc()
			return nil, 0, fmt.Errorf("scan audit event: %w", err)
		}
		ts, err := time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			return nil, 0, fmt.Errorf("parse timestamp %q: %w", tsStr, err)
		}
		evt.Timestamp = ts
		events = append(events, evt)
	}
	if err := rows.Err(); err != nil {
		obs.Fail("store", "sqlite", "row iteration error on query", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("iterate_audit_query").Inc()
		return nil, 0, fmt.Errorf("iterate audit query: %w", err)
	}
	return events, total, nil
}

// HasDB reports whether the store has an active SQLite database connection.
func (s *SqlStore) HasDB() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db != nil
}

// Close closes the underlying SQLite database connection. It should be called
// when the store is no longer needed to release file handles.
func (s *SqlStore) Close() error {
	if s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		obs.Fail("store", "sqlite", "failed to close database", "error="+err.Error())
		return err
	}
	obs.Ok("store", "sqlite", "database closed")
	return nil
}
