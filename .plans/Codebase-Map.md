# AgentAuth Codebase Map

**Generated:** 2026-03-02
**Purpose:** Complete mapping of the current codebase architecture so a coding agent can understand WHERE to make changes for each phase. This is the "lay of the land" document.

---

## Directory Structure

```
agentauth/
├── cmd/
│   ├── broker/main.go           ← HTTP server entry point, route wiring
│   ├── aactl/                   ← Operator CLI (Cobra-based)
│   │   ├── main.go
│   │   ├── root.go              ← Root command, --json flag
│   │   ├── client.go            ← HTTP client utilities
│   │   ├── audit.go             ← aactl audit events
│   │   ├── token.go             ← aactl token release
│   │   ├── revoke.go            ← aactl revoke tokens/agents/tasks/chains
│   │   ├── sidecars.go          ← aactl sidecars list/ceiling
│   │   └── output.go            ← Table/JSON formatting
│   ├── sidecar/                 ← Token Proxy binary
│   └── smoketest/               ← Integration smoke tests
├── internal/
│   ├── admin/
│   │   ├── admin_svc.go         ← AdminSvc: auth, launch tokens, sidecar activations
│   │   └── admin_hdl.go         ← AdminHandler: RegisterRoutes(), HTTP handlers
│   ├── audit/
│   │   └── audit_log.go         ← AuditLog: hash-chained event recording
│   ├── authz/
│   │   ├── val_mw.go            ← ValMw: Bearer token validation middleware
│   │   ├── scope.go             ← ParseScope(), ScopeIsSubset()
│   │   └── rate_mw.go           ← RateLimiter: per-IP token bucket
│   ├── cfg/
│   │   └── cfg.go               ← Cfg: env var configuration
│   ├── deleg/
│   │   └── deleg_svc.go         ← DelegSvc: scope-attenuated delegation
│   ├── handler/
│   │   ├── challenge_hdl.go     ← GET /v1/challenge
│   │   ├── reg_hdl.go           ← POST /v1/register
│   │   ├── val_hdl.go           ← POST /v1/token/validate
│   │   ├── renew_hdl.go         ← POST /v1/token/renew
│   │   ├── token_exchange_hdl.go ← POST /v1/token/exchange
│   │   ├── deleg_hdl.go         ← POST /v1/delegate
│   │   ├── revoke_hdl.go        ← POST /v1/revoke
│   │   ├── release_hdl.go       ← POST /v1/token/release
│   │   ├── audit_hdl.go         ← GET /v1/audit/events
│   │   └── health_hdl.go        ← GET /v1/health, GET /v1/metrics
│   ├── identity/
│   │   ├── id_svc.go            ← IdSvc: agent registration flow
│   │   └── spiffe.go            ← SPIFFE ID generation/parsing
│   ├── mutauth/                 ← Mutual auth (discovery, heartbeat)
│   ├── obs/                     ← Observability (logging, metrics)
│   ├── problemdetails/          ← RFC 7807 error responses
│   ├── revoke/
│   │   └── rev_svc.go           ← RevSvc: 4-level revocation
│   ├── store/
│   │   └── sql_store.go         ← SqlStore: in-memory + SQLite persistence
│   └── token/
│       ├── tkn_svc.go           ← TknSvc: EdDSA JWT issue/verify/renew
│       └── tkn_claims.go        ← TknClaims struct
├── scripts/
│   ├── gates.sh                 ← Pre-PR gate checks
│   ├── stack_up.sh              ← Docker stack startup
│   └── live_test.sh             ← Docker live tests
├── docs/                        ← Enterprise documentation
├── CLAUDE.md                    ← Repo rules for AI agents
├── MEMORY.md                    ← Session work log
├── FLOW.md                      ← Decision log
└── CHANGELOG.md                 ← All changes
```

---

## Data Models (internal/store/sql_store.go)

### LaunchTokenRecord
```go
type LaunchTokenRecord struct {
    Token        string        // Opaque 64-char hex token
    AgentName    string        // Pre-authorized agent name
    AllowedScope []string      // Scope ceiling for agent
    MaxTTL       int           // TTL cap in seconds
    SingleUse    bool          // Consume after first use
    CreatedAt    time.Time
    ExpiresAt    time.Time
    ConsumedAt   *time.Time    // Null if unconsumed
    CreatedBy    string        // Admin who created
}
```

### AgentRecord
```go
type AgentRecord struct {
    AgentID      string        // SPIFFE ID (unique key)
    PublicKey    []byte        // Ed25519 public key
    OrchID       string        // Orchestration ID
    TaskID       string        // Task ID
    Scope        []string      // Granted scope
    RegisteredAt time.Time
    LastSeen     time.Time
}
```

### SidecarRecord
```go
type SidecarRecord struct {
    ID        string
    Ceiling   []string        // JSON-marshaled scope ceiling
    Status    string          // "active", "inactive"
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### RevocationEntry
```go
type RevocationEntry struct {
    Level  string             // "token", "agent", "task", "chain"
    Target string             // JTI / SPIFFE ID / task_id / root delegator
}
```

---

## Token Claims (internal/token/tkn_claims.go)

```go
type TknClaims struct {
    Iss        string              // "agentauth"
    Sub        string              // SPIFFE ID (required)
    Aud        []string            // Audience (optional)
    Exp        int64               // Expiration (unix seconds)
    Nbf        int64               // Not before
    Iat        int64               // Issued at
    Jti        string              // Unique JWT ID (required)
    Sid        string              // Session ID
    SidecarID  string              // Sidecar ID
    Scope      []string            // Granted scopes
    TaskId     string              // Task ID
    OrchId     string              // Orchestration ID
    DelegChain []DelegRecord       // Delegation chain
    ChainHash  string              // Hash of delegation chain
}

type DelegRecord struct {
    Agent       string
    Scope       []string
    DelegatedAt time.Time
    Signature   string              // Ed25519 signature
}
```

---

## SQLite Schema

### audit_events
```sql
CREATE TABLE IF NOT EXISTS audit_events (
    id                TEXT PRIMARY KEY,
    timestamp         TEXT NOT NULL,
    event_type        TEXT NOT NULL,
    agent_id          TEXT DEFAULT '',
    task_id           TEXT DEFAULT '',
    orch_id           TEXT DEFAULT '',
    detail            TEXT DEFAULT '',
    resource          TEXT,
    outcome           TEXT,
    deleg_depth       INTEGER,
    deleg_chain_hash  TEXT,
    bytes_transferred INTEGER,
    hash              TEXT NOT NULL,
    prev_hash         TEXT NOT NULL
);
-- Indexes: idx_audit_event_type, idx_audit_agent_id, idx_audit_timestamp, idx_audit_outcome
```

### revocations
```sql
CREATE TABLE IF NOT EXISTS revocations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    level      TEXT NOT NULL,
    target     TEXT NOT NULL,
    revoked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(level, target)
);
```

### sidecars
```sql
CREATE TABLE IF NOT EXISTS sidecars (
    id         TEXT PRIMARY KEY,
    ceiling    TEXT NOT NULL,
    status     TEXT DEFAULT 'active',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

---

## Service Layer

### TknSvc (internal/token/tkn_svc.go)
```go
type TknSvc struct {
    signingKey  ed25519.PrivateKey
    pubKey      ed25519.PublicKey
    cfg         cfg.Cfg
}

func NewTknSvc(signingKey ed25519.PrivateKey, pubKey ed25519.PublicKey, c cfg.Cfg) *TknSvc
func (s *TknSvc) Issue(req IssueReq) (*IssueResp, error)
func (s *TknSvc) Verify(tokenStr string) (*TknClaims, error)
func (s *TknSvc) Renew(tokenStr string) (*IssueResp, error)
func (s *TknSvc) PublicKey() ed25519.PublicKey
```

**IssueReq:**
```go
type IssueReq struct {
    Sub, Sid, SidecarID, TaskId, OrchId, ChainHash string
    Aud, Scope []string
    TTL int
    DelegChain []DelegRecord
}
```

**IssueResp:**
```go
type IssueResp struct {
    AccessToken string    // Compact JWT
    ExpiresIn   int       // Effective TTL
    TokenType   string    // "Bearer"
    Claims      *TknClaims
}
```

### IdSvc (internal/identity/id_svc.go)
```go
type IdSvc struct {
    store *store.SqlStore; tknSvc *token.TknSvc; trustDomain string; auditLog AuditRecorder; audience string
}

func NewIdSvc(sqlStore, tknSvc, trustDomain, auditLog, audience) *IdSvc
func (s *IdSvc) Register(req RegisterReq) (*RegisterResp, error)
```

**Registration Flow:**
1. Validate fields → 2. Get + validate launch token → 3. Check scope (requested ⊆ allowed) BEFORE consumption → 4. Consume nonce → 5. Decode Ed25519 public key → 6. Verify nonce signature → 7. Consume launch token → 8. Generate SPIFFE ID → 9. Issue JWT → 10. Save agent record → 11. Audit

### RevSvc (internal/revoke/rev_svc.go)
```go
type RevSvc struct {
    mu sync.RWMutex
    tokens, agents, tasks, chains map[string]bool  // in-memory indexes
    store RevocationStore                           // SQLite persistence
}

func NewRevSvc(store RevocationStore) *RevSvc
func (r *RevSvc) IsRevoked(claims *token.TknClaims) bool   // Checks all 4 levels
func (r *RevSvc) Revoke(level, target string) (int, error)  // "token"|"agent"|"task"|"chain"
func (r *RevSvc) LoadFromEntries(entries) // Bulk load from SQLite
```

### DelegSvc (internal/deleg/deleg_svc.go)
```go
type DelegSvc struct {
    tknSvc *token.TknSvc; store *store.SqlStore; auditLog *audit.AuditLog; signingKey ed25519.PrivateKey
}

func NewDelegSvc(tknSvc, store, auditLog, signingKey) *DelegSvc
func (d *DelegSvc) Delegate(delegatorClaims *token.TknClaims, req DelegReq) (*DelegResp, error)
// Max depth: 5. Scope attenuation enforced. Chain hashed (SHA-256).
```

### AdminSvc (internal/admin/admin_svc.go)
```go
type AdminSvc struct {
    adminSecret []byte; tknSvc *token.TknSvc; store *store.SqlStore; auditLog *audit.AuditLog; audience string
}

func NewAdminSvc(adminSecret, tknSvc, store, auditLog, audience) *AdminSvc
func (a *AdminSvc) Authenticate(clientID, clientSecret string) (*token.IssueResp, error)
func (a *AdminSvc) CreateLaunchToken(req, createdBy) (*CreateLaunchTokenResp, error)
func (a *AdminSvc) CreateSidecarActivationToken(req, createdBy) (*resp, error)
func (a *AdminSvc) ActivateSidecar(req) (*resp, error)
func (a *AdminSvc) ListSidecars() ([]store.SidecarRecord, error)
func (a *AdminSvc) GetSidecarCeiling(sidecarID) ([]string, error)
func (a *AdminSvc) UpdateSidecarCeiling(sidecarID, newCeiling, updatedBy) (*result, error)
func (a *AdminSvc) ValidateLaunchToken(tokenStr) (*store.LaunchTokenRecord, error)
func (a *AdminSvc) ConsumeLaunchToken(tokenStr) error
```

**Admin JWT Scopes:** `["admin:launch-tokens:*", "admin:revoke:*", "admin:audit:*"]`

**TTL Constants:** adminTTL=300 (5 min), sidecarTTL=900 (15 min), defaultTokenTTL=30

### AuditLog (internal/audit/audit_log.go)
```go
type AuditLog struct {
    mu sync.RWMutex; events []AuditEvent; prevHash string; counter int; store AuditStore
}

func NewAuditLog(store AuditStore) *AuditLog
func (a *AuditLog) Record(eventType, agentID, taskID, orchID, detail string, opts ...RecordOption)
func (a *AuditLog) Query(filters QueryFilters) ([]AuditEvent, int)
```

**RecordOption functional options:** `WithResource(r)`, `WithOutcome(o)`, `WithDelegDepth(d)`, `WithDelegChainHash(h)`, `WithBytesTransferred(b)`

**QueryFilters:** `AgentID, TaskID, EventType, Outcome string; Since, Until *time.Time; Limit, Offset int`

---

## Audit Event Constants

```go
EventAdminAuth                      = "admin_auth"
EventAdminAuthFailed                = "admin_auth_failed"
EventLaunchTokenIssued              = "launch_token_issued"
EventLaunchTokenDenied              = "launch_token_denied"
EventSidecarActivationIssued        = "sidecar_activation_issued"
EventSidecarActivated               = "sidecar_activated"
EventSidecarActivationFailed        = "sidecar_activation_failed"
EventAgentRegistered                = "agent_registered"
EventRegistrationViolation          = "registration_policy_violation"
EventTokenIssued                    = "token_issued"
EventTokenRevoked                   = "token_revoked"
EventTokenRenewed                   = "token_renewed"
EventTokenReleased                  = "token_released"
EventTokenRenewalFailed             = "token_renewal_failed"
EventDelegationCreated              = "delegation_created"
EventResourceAccessed               = "resource_accessed"
EventSidecarExchangeSuccess         = "sidecar_exchange_success"
EventSidecarExchangeDenied          = "sidecar_exchange_denied"
EventTokenAuthFailed                = "token_auth_failed"
EventTokenRevokedAccess             = "token_revoked_access"
EventScopeViolation                 = "scope_violation"
EventScopeCeilingExceeded           = "scope_ceiling_exceeded"
EventDelegationAttenuationViolation = "delegation_attenuation_violation"
EventScopesCeilingUpdated           = "scopes_ceiling_updated"
```

---

## HTTP Routes (cmd/broker/main.go)

### Public (no auth)
```
GET  /v1/challenge              → ChallengeHdl
GET  /v1/health                 → HealthHdl
GET  /v1/metrics                → MetricsHdl
POST /v1/token/validate         → MaxBytesBody → ValHdl
POST /v1/register               → MaxBytesBody → RegHdl
```

### Authenticated (Bearer + ValMw)
```
POST /v1/token/renew            → MaxBytesBody → ValMw → RenewHdl
POST /v1/token/exchange         → MaxBytesBody → ValMw → RequireScope("sidecar:manage:*") → TokenExchangeHdl
POST /v1/delegate               → MaxBytesBody → ValMw → DelegHdl
POST /v1/token/release          → MaxBytesBody → ValMw → ReleaseHdl
POST /v1/revoke                 → MaxBytesBody → ValMw → RequireScope("admin:revoke:*") → RevokeHdl
GET  /v1/audit/events           → ValMw → RequireScope("admin:audit:*") → AuditHdl
```

### Admin (registered via AdminHdl.RegisterRoutes)
```
POST /v1/admin/auth             → RateLimiter → handleAuth
POST /v1/admin/launch-tokens    → ValMw → RequireScope("admin:launch-tokens:*") → handleCreateLaunchToken
POST /v1/admin/sidecar-activations → ValMw → RequireScope("admin:launch-tokens:*") → handleCreateSidecarActivation
POST /v1/sidecar/activate       → RateLimiter → handleActivateSidecar
GET  /v1/admin/sidecars/{id}/ceiling → ValMw → RequireScope("admin:launch-tokens:*") → handleGetCeiling
PUT  /v1/admin/sidecars/{id}/ceiling → ValMw → RequireScope("admin:launch-tokens:*") → handleUpdateCeiling
GET  /v1/admin/sidecars         → ValMw → RequireScope("admin:launch-tokens:*") → handleListSidecars
```

### Middleware Stack (global)
1. `RequestIDMiddleware` — adds request ID to context
2. `LoggingMiddleware` — logs requests
3. (per-route) `MaxBytesBody(1MB)` — limits body size
4. (per-route) `ValMw.Wrap()` — Bearer token extraction + validation
5. (per-route) `ValMw.RequireScope(scope)` — scope checking
6. (per-route) `RateLimiter.Wrap()` — per-IP rate limiting

---

## Validation & Authorization (internal/authz/)

### ValMw (val_mw.go)
```go
type ValMw struct {
    tknSvc TokenVerifier; revSvc RevocationChecker; auditLog AuditRecorder; audience string
}

func NewValMw(tknSvc, revSvc, auditLog, audience) *ValMw
func (v *ValMw) Wrap(next http.Handler) http.Handler          // Validates Bearer token, stores claims in context
func (v *ValMw) RequireScope(scope string, next http.Handler) http.Handler  // Checks scope
func (v *ValMw) ClaimsFromContext(ctx) *token.TknClaims       // Retrieves claims from context
func (v *ValMw) TokenFromRequest(r) string                    // Extracts Bearer token
```

**Validation Flow:** Extract Bearer → Verify signature + claims → Check revocation → Validate audience → Store claims in context → Next handler

### Scope (scope.go)
```go
func ParseScope(s string) (action, resource, identifier string, err error)
// Format: action:resource:identifier (e.g., "read:weather:current")

func ScopeIsSubset(requested, allowed []string) bool
// Wildcard: "*" matches any identifier for action:resource pair
```

### RateLimiter (rate_mw.go)
```go
type RateLimiter struct {
    mu sync.Mutex; clients map[string]*bucket; rate float64; burst int
}

func NewRateLimiter(rate float64, burst int) *RateLimiter
func (rl *RateLimiter) Allow(key string) bool
func (rl *RateLimiter) Wrap(next http.Handler) http.Handler   // Keys on IP
```

**Current usage:** Admin auth = `NewRateLimiter(5, 10)` — 5 req/sec, burst 10

---

## Configuration (internal/cfg/cfg.go)

```go
type Cfg struct {
    Port        string    // AA_PORT (default "8080")
    LogLevel    string    // AA_LOG_LEVEL: quiet|standard|verbose|trace (default "verbose")
    TrustDomain string    // AA_TRUST_DOMAIN (default "agentauth.local")
    DefaultTTL  int       // AA_DEFAULT_TTL seconds (default 300)
    AdminSecret string    // AA_ADMIN_SECRET (required)
    SeedTokens  bool      // AA_SEED_TOKENS (default false, dev only)
    DBPath      string    // AA_DB_PATH (default "./agentauth.db")
    TLSMode     string    // AA_TLS_MODE: none|tls|mtls (default "none")
    TLSCert     string    // AA_TLS_CERT path
    TLSKey      string    // AA_TLS_KEY path
    TLSClientCA string    // AA_TLS_CLIENT_CA path (mtls)
    Audience    string    // AA_AUDIENCE (default "agentauth", empty = skip)
}
```

---

## aactl CLI Pattern (cmd/aactl/)

Cobra-based command hierarchy. All commands use:
- `AACTL_BROKER_URL` — broker base URL
- `AACTL_ADMIN_TOKEN` — Bearer token for admin endpoints
- `--json` persistent flag for JSON output
- `printTable()` from `output.go` for formatted tables

**Current command tree:**
```
aactl [--json]
  ├─ audit events [--agent-id, --task-id, --event-type, --since, --until, --limit, --offset, --outcome]
  ├─ revoke tokens|agents|tasks|chains <target>
  ├─ token release --token <jwt>
  └─ sidecars
      ├─ list
      └─ ceiling get|set <sidecar-id> [<scope-csv>]
```

---

## Key Patterns to Follow

1. **New service:** Create `internal/{name}/{name}_svc.go` — follow `internal/admin/admin_svc.go` pattern
2. **New handler:** Create handler in same package as service — follow `internal/admin/admin_hdl.go` with `RegisterRoutes(mux)` method
3. **New store methods:** Add to `internal/store/sql_store.go` — follow `SaveSidecar`/`ListSidecars` pattern
4. **New table:** Add CREATE TABLE to `InitDB()` method in `sql_store.go`
5. **New aactl command:** Create `cmd/aactl/{name}.go` — follow `cmd/aactl/sidecars.go` Cobra pattern
6. **New audit events:** Add constants to `internal/audit/audit_log.go` — follow `Event*` naming
7. **New middleware:** Extend `internal/authz/` — follow `RateLimiter` or `ValMw` patterns
8. **Route wiring:** Register in `cmd/broker/main.go` via `handler.RegisterRoutes(mux)`

---

## SPIFFE ID Format

```
spiffe://{trustDomain}/agent/{orchID}/{taskID}/{instanceID}
```

Generated in `internal/identity/spiffe.go` → `NewSpiffeId()`

---

## Security Invariants (Must Not Break)

1. Every agent has unique Ed25519 keypair + SPIFFE ID
2. Every token signed with broker's Ed25519 key
3. Every token has JTI for individual revocation
4. Scopes only narrow, never widen (attenuation)
5. Delegation depth capped at 5
6. Delegation chains tamper-evident (SHA-256)
7. Revocation immediate and persistent (SQLite)
8. Every operation generates audit event (hash-chained)
9. Nonces single-use, 30s TTL
10. Launch tokens single-use with scope ceilings
