// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0

// Package store is the single persistence layer for the broker. It holds
// everything: nonces (challenge-response), launch tokens (agent bootstrapping),
// agent records, app records, audit events, and revocations.
//
// Two storage modes coexist: in-memory maps behind a sync.RWMutex for fast
// lookups (nonces, launch tokens, agents), and SQLite for data that must
// survive restarts (audit events, revocations, app records). Call InitDB
// to enable SQLite; without it, only in-memory storage is available.
//
// The type is named SqlStore for historical reasons — it started as pure
// in-memory and gained SQLite incrementally. All public methods are
// safe for concurrent use.
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devonartis/agentwrit/internal/audit"
	"github.com/devonartis/agentwrit/internal/obs"
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
	ErrAgentNotFound = errors.New("agent not found")
	ErrAppNotFound   = errors.New("app not found")
)

// LaunchTokenRecord represents a pre-authorized launch token created by an
// admin. It binds an agent name to an allowed scope ceiling and optional
// TTL cap. A single-use token is consumed on first registration.
type LaunchTokenRecord struct {
	// Token is the opaque 64-character hex credential presented by agents
	// during registration.
	Token string
	// AgentName is the human-readable label for the agent this token authorizes.
	AgentName string
	// AllowedScope is the scope ceiling: agents registering with this token
	// cannot request scopes beyond this set.
	AllowedScope []string
	// MaxTTL caps the JWT lifetime (in seconds) issued at registration.
	MaxTTL int
	// SingleUse, when true, causes the token to be consumed after one
	// successful registration. Multi-use tokens are never consumed.
	SingleUse bool
	CreatedAt time.Time
	ExpiresAt time.Time
	// ConsumedAt is nil until a single-use token is consumed during
	// agent registration.
	ConsumedAt *time.Time
	// CreatedBy is the JWT subject of the caller who created this token
	// (e.g., "admin" or "app:app-weather-bot-abc123").
	CreatedBy string
	// AppID is the originating app's identifier. Empty when the token was
	// created directly by an admin rather than through app credentials.
	AppID string
}

// AgentRecord stores the persistent state of a registered agent,
// including its SPIFFE-format AgentID, Ed25519 public key, and the
// scope granted at registration time.
type AgentRecord struct {
	// AgentID is the SPIFFE URI assigned at registration
	// (format: spiffe://{trustDomain}/agent/{orchID}/{taskID}/{instanceID}).
	AgentID string
	// PublicKey is the raw 32-byte Ed25519 public key provided during
	// challenge-response registration.
	PublicKey []byte
	// OrchID identifies the orchestrator that launched this agent.
	OrchID string
	// TaskID identifies the specific task this agent was created for.
	TaskID string
	// Scope is the set of permissions granted at registration, always a
	// subset of the launch token's AllowedScope.
	Scope        []string
	RegisteredAt time.Time
	LastSeen     time.Time
	ExpiresAt    time.Time
	Status       string
	// AppID is inherited from the launch token used during registration.
	// Empty for agents registered via admin-created launch tokens.
	AppID string
}

// ErrAgentExpired is returned when an operation is attempted on an expired agent.
var ErrAgentExpired = errors.New("agent has expired")

type nonceRecord struct {
	value     string
	expiresAt time.Time
	consumed  bool
}

// SqlStore is the broker's single storage backend. In-memory maps handle
// ephemeral data (nonces, launch tokens, agents), while SQLite persists
// durable data (audit trail, revocations, app registry). Create with
// NewSqlStore, then call InitDB to enable SQLite persistence.
type SqlStore struct {
	mu             sync.RWMutex
	nonces         map[string]*nonceRecord
	launchTokens   map[string]*LaunchTokenRecord
	agents         map[string]*AgentRecord
	jtiConsumption map[string]time.Time
	db             *sql.DB
}

// NewSqlStore returns an initialized, empty in-memory store ready for use.
func NewSqlStore() *SqlStore {
	return &SqlStore{
		nonces:         make(map[string]*nonceRecord),
		launchTokens:   make(map[string]*LaunchTokenRecord),
		agents:         make(map[string]*AgentRecord),
		jtiConsumption: make(map[string]time.Time),
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

// ExpireAgents marks active agents as expired when their ExpiresAt time
// has passed. Returns the number of agents expired.
func (s *SqlStore) ExpireAgents() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expired := 0
	for _, rec := range s.agents {
		if rec.Status == "active" && !rec.ExpiresAt.IsZero() && rec.ExpiresAt.Before(now) {
			rec.Status = "expired"
			expired++
		}
	}
	return expired
}

// PruneExpiredJTIs removes JTI consumption entries whose associated token
// has already expired. Returns the number of entries removed.
func (s *SqlStore) PruneExpiredJTIs() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	pruned := 0
	for jti, exp := range s.jtiConsumption {
		if exp.Before(now) {
			delete(s.jtiConsumption, jti)
			pruned++
		}
	}
	return pruned
}

// ---------------------------------------------------------------------------
// SQLite audit persistence
// ---------------------------------------------------------------------------

const createAuditTable = `
CREATE TABLE IF NOT EXISTS audit_events (
	id                TEXT PRIMARY KEY,
	timestamp         TEXT NOT NULL,
	event_type        TEXT NOT NULL,
	agent_id          TEXT NOT NULL DEFAULT '',
	task_id           TEXT NOT NULL DEFAULT '',
	orch_id           TEXT NOT NULL DEFAULT '',
	detail            TEXT NOT NULL DEFAULT '',
	resource          TEXT DEFAULT NULL,
	outcome           TEXT DEFAULT NULL,
	deleg_depth       INTEGER DEFAULT NULL,
	deleg_chain_hash  TEXT DEFAULT NULL,
	bytes_transferred INTEGER DEFAULT NULL,
	hash              TEXT NOT NULL,
	prev_hash         TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audit_events(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_agent_id   ON audit_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp  ON audit_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_outcome    ON audit_events(outcome);
`

const createRevocationsTable = `
CREATE TABLE IF NOT EXISTS revocations (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	level      TEXT NOT NULL,
	target     TEXT NOT NULL,
	revoked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(level, target)
);
`

// InitDB opens the SQLite database at path and creates all required tables
// (audit_events, revocations, apps) if they do not already exist. It must
// be called before any persistence methods are used.
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
	if _, err = db.Exec(createRevocationsTable); err != nil {
		db.Close()
		obs.Fail("store", "sqlite", "failed to create revocations table", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("create_table").Inc()
		return fmt.Errorf("create revocations table: %w", err)
	}
	if _, err = db.Exec(createAppsTable); err != nil {
		db.Close()
		obs.Fail("store", "sqlite", "failed to create apps table", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("create_table").Inc()
		return fmt.Errorf("create apps table: %w", err)
	}
	// Migrate: add token_ttl column to existing apps tables that lack it.
	s.migrateAddColumn(db, "apps", "token_ttl", "INTEGER NOT NULL DEFAULT 1800")
	s.db = db
	obs.Ok("store", "sqlite", "database initialized", "path="+path)
	return nil
}

// migrateAddColumn adds a column to a table if it doesn't already exist.
// Used for schema evolution on existing databases.
func (s *SqlStore) migrateAddColumn(db *sql.DB, table, column, colDef string) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == column {
			return // column already exists
		}
	}
	q := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colDef)
	if _, err := db.Exec(q); err != nil {
		obs.Warn("store", "sqlite", "migration failed", "table="+table, "column="+column, "error="+err.Error())
	} else {
		obs.Ok("store", "sqlite", "migration applied", "table="+table, "column="+column)
	}
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
		(id, timestamp, event_type, agent_id, task_id, orch_id, detail,
		 resource, outcome, deleg_depth, deleg_chain_hash, bytes_transferred,
		 hash, prev_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(q,
		evt.ID,
		evt.Timestamp.UTC().Format(time.RFC3339Nano),
		evt.EventType,
		evt.AgentID,
		evt.TaskID,
		evt.OrchID,
		evt.Detail,
		nullableString(evt.Resource),
		nullableString(evt.Outcome),
		nullableInt(int64(evt.DelegDepth)),
		nullableString(evt.DelegChainHash),
		nullableInt(evt.BytesTransferred),
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
	const q = `SELECT id, timestamp, event_type, agent_id, task_id, orch_id, detail,
		resource, outcome, deleg_depth, deleg_chain_hash, bytes_transferred,
		hash, prev_hash
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
		var resource, outcome, delegChainHash sql.NullString
		var delegDepth, bytesTransferred sql.NullInt64
		if err := rows.Scan(&evt.ID, &tsStr, &evt.EventType, &evt.AgentID, &evt.TaskID,
			&evt.OrchID, &evt.Detail,
			&resource, &outcome, &delegDepth, &delegChainHash, &bytesTransferred,
			&evt.Hash, &evt.PrevHash); err != nil {
			obs.Fail("store", "sqlite", "failed to scan audit event row", "error="+err.Error())
			obs.DBErrorsTotal.WithLabelValues("scan_audit_event").Inc()
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		ts, err := time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			return nil, fmt.Errorf("parse timestamp %q: %w", tsStr, err)
		}
		evt.Timestamp = ts
		evt.Resource = resource.String
		evt.Outcome = outcome.String
		evt.DelegDepth = int(delegDepth.Int64)
		evt.DelegChainHash = delegChainHash.String
		evt.BytesTransferred = bytesTransferred.Int64
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
	if filters.Outcome != "" {
		whereClauses = append(whereClauses, "outcome = ?")
		args = append(args, filters.Outcome)
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

	// #nosec G202 -- `where` is assembled from fixed-template fragments built
	// above (see whereClauses); every user-supplied value is a `?` placeholder
	// bound through queryArgs. No untrusted strings enter the SQL text.
	selectQ := "SELECT id, timestamp, event_type, agent_id, task_id, orch_id, detail, " +
		"resource, outcome, deleg_depth, deleg_chain_hash, bytes_transferred, " +
		"hash, prev_hash FROM audit_events" +
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
		var resource, outcome, delegChainHash sql.NullString
		var delegDepth, bytesTransferred sql.NullInt64
		if err := rows.Scan(&evt.ID, &tsStr, &evt.EventType, &evt.AgentID, &evt.TaskID,
			&evt.OrchID, &evt.Detail,
			&resource, &outcome, &delegDepth, &delegChainHash, &bytesTransferred,
			&evt.Hash, &evt.PrevHash); err != nil {
			obs.Fail("store", "sqlite", "failed to scan audit event", "error="+err.Error())
			obs.DBErrorsTotal.WithLabelValues("scan_audit_event").Inc()
			return nil, 0, fmt.Errorf("scan audit event: %w", err)
		}
		ts, err := time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			return nil, 0, fmt.Errorf("parse timestamp %q: %w", tsStr, err)
		}
		evt.Timestamp = ts
		evt.Resource = resource.String
		evt.Outcome = outcome.String
		evt.DelegDepth = int(delegDepth.Int64)
		evt.DelegChainHash = delegChainHash.String
		evt.BytesTransferred = bytesTransferred.Int64
		events = append(events, evt)
	}
	if err := rows.Err(); err != nil {
		obs.Fail("store", "sqlite", "row iteration error on query", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("iterate_audit_query").Inc()
		return nil, 0, fmt.Errorf("iterate audit query: %w", err)
	}
	return events, total, nil
}

// ---------------------------------------------------------------------------
// SQLite revocation persistence
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// App persistence
// ---------------------------------------------------------------------------

// AppRecord is an app's persistent state in SQLite. Apps are the production
// path for agent credentialing — they authenticate independently and create
// launch tokens within their scope ceiling. The secret hash is never returned
// in API responses.
type AppRecord struct {
	AppID            string   // "app-{name}-{random6hex}"
	Name             string   // Human-readable, unique
	ClientID         string   // "{abbrev}-{random12hex}"
	ClientSecretHash string   // bcrypt hash of the client secret (never returned)
	ScopeCeiling     []string // Scope ceiling; JSON-marshaled in DB
	TokenTTL         int      // JWT TTL in seconds (default 1800)
	Status           string   // "active" | "inactive"
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CreatedBy        string
}

const createAppsTable = `
CREATE TABLE IF NOT EXISTS apps (
	app_id            TEXT PRIMARY KEY,
	name              TEXT NOT NULL UNIQUE,
	client_id         TEXT NOT NULL UNIQUE,
	client_secret_hash TEXT NOT NULL,
	scope_ceiling     TEXT NOT NULL,
	token_ttl         INTEGER NOT NULL DEFAULT 1800,
	status            TEXT NOT NULL DEFAULT 'active',
	created_at        TEXT NOT NULL,
	updated_at        TEXT NOT NULL,
	created_by        TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_apps_client_id ON apps(client_id);
CREATE INDEX IF NOT EXISTS idx_apps_status ON apps(status);
`

// SaveApp inserts a new app record. Returns an error if a record with the
// same name or client_id already exists (UNIQUE constraint violation).
func (s *SqlStore) SaveApp(rec AppRecord) error {
	if s.db == nil {
		return errors.New("database not initialized: call InitDB first")
	}

	scopeJSON, err := json.Marshal(rec.ScopeCeiling)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to marshal scope ceiling", "app_id="+rec.AppID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("marshal_scope_ceiling").Inc()
		return fmt.Errorf("marshal scope ceiling for app %s: %w", rec.AppID, err)
	}

	const q = `INSERT INTO apps
		(app_id, name, client_id, client_secret_hash, scope_ceiling, token_ttl, status, created_at, updated_at, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.Exec(q,
		rec.AppID,
		rec.Name,
		rec.ClientID,
		rec.ClientSecretHash,
		string(scopeJSON),
		rec.TokenTTL,
		rec.Status,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		rec.CreatedBy,
	)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to save app", "app_id="+rec.AppID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("save_app").Inc()
		return fmt.Errorf("save app %s: %w", rec.AppID, err)
	}
	obs.Ok("store", "sqlite", "app saved", "app_id="+rec.AppID)
	return nil
}

// GetAppByClientID looks up an app by client_id. Returns [ErrAppNotFound]
// if no app with that client_id exists.
func (s *SqlStore) GetAppByClientID(clientID string) (*AppRecord, error) {
	if s.db == nil {
		return nil, errors.New("database not initialized: call InitDB first")
	}

	const q = `SELECT app_id, name, client_id, client_secret_hash, scope_ceiling, token_ttl, status, created_at, updated_at, created_by
		FROM apps WHERE client_id = ?`

	return s.scanAppRow(s.db.QueryRow(q, clientID))
}

// GetAppByID looks up an app by app_id. Returns [ErrAppNotFound] if no app
// with that ID exists.
func (s *SqlStore) GetAppByID(appID string) (*AppRecord, error) {
	if s.db == nil {
		return nil, errors.New("database not initialized: call InitDB first")
	}

	const q = `SELECT app_id, name, client_id, client_secret_hash, scope_ceiling, token_ttl, status, created_at, updated_at, created_by
		FROM apps WHERE app_id = ?`

	return s.scanAppRow(s.db.QueryRow(q, appID))
}

// scanAppRow scans a single *sql.Row into an AppRecord.
// Returns [ErrAppNotFound] on sql.ErrNoRows.
func (s *SqlStore) scanAppRow(row *sql.Row) (*AppRecord, error) {
	var rec AppRecord
	var scopeStr, createdStr, updatedStr string
	err := row.Scan(
		&rec.AppID, &rec.Name, &rec.ClientID, &rec.ClientSecretHash,
		&scopeStr, &rec.TokenTTL, &rec.Status, &createdStr, &updatedStr, &rec.CreatedBy,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAppNotFound
	}
	if err != nil {
		obs.Fail("store", "sqlite", "failed to scan app row", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("scan_app").Inc()
		return nil, fmt.Errorf("scan app: %w", err)
	}
	if err := json.Unmarshal([]byte(scopeStr), &rec.ScopeCeiling); err != nil {
		obs.Fail("store", "sqlite", "failed to unmarshal scope ceiling", "app_id="+rec.AppID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("unmarshal_scope_ceiling").Inc()
		return nil, fmt.Errorf("unmarshal scope ceiling for app %s: %w", rec.AppID, err)
	}
	ca, err := time.Parse(time.RFC3339Nano, createdStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdStr, err)
	}
	ua, err := time.Parse(time.RFC3339Nano, updatedStr)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedStr, err)
	}
	rec.CreatedAt = ca
	rec.UpdatedAt = ua
	return &rec, nil
}

// ListApps returns all app records ordered by created_at DESC.
func (s *SqlStore) ListApps() ([]AppRecord, error) {
	if s.db == nil {
		return nil, errors.New("database not initialized: call InitDB first")
	}

	const q = `SELECT app_id, name, client_id, client_secret_hash, scope_ceiling, token_ttl, status, created_at, updated_at, created_by
		FROM apps ORDER BY created_at DESC`

	rows, err := s.db.Query(q)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to list apps", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("list_apps").Inc()
		return nil, fmt.Errorf("list apps: %w", err)
	}
	defer rows.Close()

	var apps []AppRecord
	for rows.Next() {
		var rec AppRecord
		var scopeStr, createdStr, updatedStr string
		if err := rows.Scan(
			&rec.AppID, &rec.Name, &rec.ClientID, &rec.ClientSecretHash,
			&scopeStr, &rec.TokenTTL, &rec.Status, &createdStr, &updatedStr, &rec.CreatedBy,
		); err != nil {
			obs.Fail("store", "sqlite", "failed to scan app row", "error="+err.Error())
			obs.DBErrorsTotal.WithLabelValues("scan_app").Inc()
			return nil, fmt.Errorf("scan app: %w", err)
		}
		if err := json.Unmarshal([]byte(scopeStr), &rec.ScopeCeiling); err != nil {
			obs.Fail("store", "sqlite", "failed to unmarshal scope ceiling", "app_id="+rec.AppID, "error="+err.Error())
			obs.DBErrorsTotal.WithLabelValues("unmarshal_scope_ceiling").Inc()
			return nil, fmt.Errorf("unmarshal scope ceiling for app %s: %w", rec.AppID, err)
		}
		ca, err := time.Parse(time.RFC3339Nano, createdStr)
		if err != nil {
			return nil, fmt.Errorf("parse created_at %q: %w", createdStr, err)
		}
		ua, err := time.Parse(time.RFC3339Nano, updatedStr)
		if err != nil {
			return nil, fmt.Errorf("parse updated_at %q: %w", updatedStr, err)
		}
		rec.CreatedAt = ca
		rec.UpdatedAt = ua
		apps = append(apps, rec)
	}
	if err := rows.Err(); err != nil {
		obs.Fail("store", "sqlite", "row iteration error on apps", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("iterate_apps").Inc()
		return nil, fmt.Errorf("iterate apps: %w", err)
	}
	if apps == nil {
		apps = []AppRecord{}
	}
	obs.Ok("store", "sqlite", "apps listed", fmt.Sprintf("count=%d", len(apps)))
	return apps, nil
}

// UpdateAppCeiling replaces the scope ceiling for an existing app.
// Returns [ErrAppNotFound] if no app with the given app_id exists.
func (s *SqlStore) UpdateAppCeiling(appID string, newCeiling []string) error {
	if s.db == nil {
		return errors.New("database not initialized: call InitDB first")
	}

	scopeJSON, err := json.Marshal(newCeiling)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to marshal scope ceiling", "app_id="+appID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("marshal_scope_ceiling").Inc()
		return fmt.Errorf("marshal scope ceiling for app %s: %w", appID, err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	const q = `UPDATE apps SET scope_ceiling = ?, updated_at = ? WHERE app_id = ?`
	res, err := s.db.Exec(q, string(scopeJSON), now, appID)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to update app ceiling", "app_id="+appID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("update_app_ceiling").Inc()
		return fmt.Errorf("update app ceiling %s: %w", appID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		obs.Fail("store", "sqlite", "failed to get rows affected", "app_id="+appID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("update_app_ceiling").Inc()
		return fmt.Errorf("rows affected for app %s: %w", appID, err)
	}
	if n == 0 {
		return ErrAppNotFound
	}
	obs.Ok("store", "sqlite", "app ceiling updated", "app_id="+appID)
	return nil
}

// UpdateAppTTL sets the token_ttl for an existing app.
// Returns [ErrAppNotFound] if no app with the given app_id exists.
func (s *SqlStore) UpdateAppTTL(appID string, ttl int) error {
	if s.db == nil {
		return errors.New("database not initialized: call InitDB first")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	const q = `UPDATE apps SET token_ttl = ?, updated_at = ? WHERE app_id = ?`
	res, err := s.db.Exec(q, ttl, now, appID)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to update app TTL", "app_id="+appID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("update_app_ttl").Inc()
		return fmt.Errorf("update app TTL %s: %w", appID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		obs.Fail("store", "sqlite", "failed to get rows affected", "app_id="+appID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("update_app_ttl").Inc()
		return fmt.Errorf("rows affected for app %s: %w", appID, err)
	}
	if n == 0 {
		return ErrAppNotFound
	}
	obs.Ok("store", "sqlite", "app TTL updated", "app_id="+appID, fmt.Sprintf("ttl=%d", ttl))
	return nil
}

// UpdateAppStatus sets the status field for an existing app (e.g., "active"
// or "inactive"). Returns [ErrAppNotFound] if no app with the given app_id
// exists.
func (s *SqlStore) UpdateAppStatus(appID string, status string) error {
	if s.db == nil {
		return errors.New("database not initialized: call InitDB first")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	const q = `UPDATE apps SET status = ?, updated_at = ? WHERE app_id = ?`
	res, err := s.db.Exec(q, status, now, appID)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to update app status", "app_id="+appID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("update_app_status").Inc()
		return fmt.Errorf("update app status %s: %w", appID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		obs.Fail("store", "sqlite", "failed to get rows affected", "app_id="+appID, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("update_app_status").Inc()
		return fmt.Errorf("rows affected for app %s: %w", appID, err)
	}
	if n == 0 {
		return ErrAppNotFound
	}
	obs.Ok("store", "sqlite", "app status updated", "app_id="+appID, "status="+status)
	return nil
}

// RevocationEntry represents a single persisted revocation.
type RevocationEntry struct {
	Level  string
	Target string
}

// SaveRevocation persists a revocation entry. The UNIQUE constraint
// makes this idempotent — re-revoking the same level+target is a no-op.
func (s *SqlStore) SaveRevocation(level, target string) error {
	if s.db == nil {
		return errors.New("database not initialized: call InitDB first")
	}
	const q = `INSERT OR IGNORE INTO revocations (level, target) VALUES (?, ?)`
	_, err := s.db.Exec(q, level, target)
	if err != nil {
		obs.DBErrorsTotal.WithLabelValues("save_revocation").Inc()
		return fmt.Errorf("save revocation %s/%s: %w", level, target, err)
	}
	return nil
}

// LoadAllRevocations returns all persisted revocations. Called at broker
// startup to rebuild the in-memory revocation maps.
func (s *SqlStore) LoadAllRevocations() ([]RevocationEntry, error) {
	if s.db == nil {
		return nil, errors.New("database not initialized: call InitDB first")
	}
	rows, err := s.db.Query(`SELECT level, target FROM revocations ORDER BY id ASC`)
	if err != nil {
		obs.DBErrorsTotal.WithLabelValues("load_revocations").Inc()
		return nil, fmt.Errorf("load revocations: %w", err)
	}
	defer rows.Close()

	var entries []RevocationEntry
	for rows.Next() {
		var e RevocationEntry
		if err := rows.Scan(&e.Level, &e.Target); err != nil {
			return nil, fmt.Errorf("scan revocation row: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// nullableString returns a sql.NullString for SQLite nullable text columns.
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullableInt returns a sql.NullInt64 for SQLite nullable integer columns.
func nullableInt(n int64) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: n, Valid: true}
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
	s.db = nil
	obs.Ok("store", "sqlite", "database closed")
	return nil
}
