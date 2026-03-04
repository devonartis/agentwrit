# Phase 1a: App Registration & Authentication

**Status:** Spec
**Priority:** P0 — unblocks everything
**Effort estimate:** 1-2 days
**Architecture doc:** `../.plans/CoWork-Architecture-Direct-Broker.md`
**Pattern:** [Ephemeral Agent Credentialing v1.2](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.2.md)

---

## Overview: What We're Building and Why

AgentAuth is a Go broker that issues short-lived scoped JWTs to AI agents via Ed25519 challenge-response. It implements the [Ephemeral Agent Credentialing Pattern v1.2](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.2.md).

**The current architecture has a critical flaw:** the only way to connect an application to the broker is by deploying a Token Proxy (sidecar) that holds the admin master key. This means the master key — the single most powerful credential in the system — gets copied into every app deployment. There's no concept of "an app" as an entity: no app registration, no per-app credentials, no way to list or revoke individual apps. "Adding an app" means deploying infrastructure, and the audit trail can't distinguish one app's activity from another.

**The transformation (Phases 1a through 5) makes apps first-class entities.** Apps register with the broker, receive their own scoped credentials (`client_id` + `client_secret`), and authenticate directly — no master key, no mandatory sidecar. The agent security layer (Ed25519 challenge-response, SPIFFE IDs, scope attenuation, delegation chains, revocation, audit) stays completely untouched. We're changing HOW apps connect to the broker, not how agents prove their identity.

**Phase 1a is the foundation.** It introduces the `AppRecord` data model, the registration and authentication endpoints, per-app rate limiting, and the `aactl app` CLI commands. Without this phase, nothing else works — every subsequent phase depends on apps existing as entities.

**What changes:** New `apps` table, 6 new API endpoints (`/v1/admin/apps` CRUD + `/v1/app/auth`), new `app:` JWT scope family, per-client-id rate limiting, 6 new audit event types, 5 new `aactl app` CLI commands.

**What stays the same:** Every existing endpoint, every agent flow, all admin operations, the sidecar, the audit trail structure, the Ed25519 signing, scope enforcement, delegation, revocation — all untouched.

---

## Problem Statement

Apps don't exist as entities in AgentAuth. The only way to connect an app to the broker is by deploying a Token Proxy (sidecar) with the admin master key baked into its config. This means every app deployment has a copy of the master key, there's no way to list/audit/revoke individual apps, and "adding an app" requires infrastructure deployment instead of an API call.

Phase 1a solves this by making apps first-class entities with their own scoped credentials.

---

## Goals

1. Apps can register with the broker and receive their own `client_id` + `client_secret`
2. Apps authenticate with scoped credentials (not the admin master key)
3. Operators can list, inspect, and manage registered apps via CLI and API
4. Per-app rate limiting prevents credential-stuffing on the app auth endpoint
5. All app operations generate audit events for the hash-chained audit trail

---

## Non-Goals (this phase)

1. **App-scoped launch tokens** — apps creating agents within their ceiling (Phase 1b)
2. **App-level revocation** — revoking all tokens for an app (Phase 1c)
3. **`app_id` in token claims** — attributing agent tokens to apps (Phase 1c)
4. **Secret rotation** — rotating app credentials with grace period (Phase 1c)
5. **Activation token bootstrap** — sidecar using activation token instead of master key (Phase 2)
6. **SDK** — Python client library (Phase 3)

---

## User Stories

### Operator Stories

1. **As an operator**, I want to register a new app with a name and scope ceiling so that a developer can authenticate their app with the broker without needing the master key.

2. **As an operator**, I want to list all registered apps so that I can see what's connected to the broker and manage the fleet.

3. **As an operator**, I want to view details of a specific app (name, scopes, status, created date) so that I can verify its configuration.

4. **As an operator**, I want to update an app's scope ceiling so that I can adjust permissions without re-registering.

5. **As an operator**, I want to deregister an app so that its credentials stop working immediately.

### Developer Stories

6. **As a developer**, I want to authenticate my app with `client_id` + `client_secret` so that I can get a scoped JWT to interact with the broker.

7. **As a developer**, I want a clear error message when my credentials are wrong or my app has been deregistered so that I can diagnose auth failures.

### Security Stories

8. **As a security reviewer**, I want each app to have its own scoped credentials (not the master key) so that a compromise of one app doesn't compromise the entire system.

9. **As a security reviewer**, I want per-app rate limiting on the auth endpoint so that credential-stuffing against one `client_id` doesn't affect other apps.

10. **As a security reviewer**, I want all app registration and authentication events recorded in the tamper-evident audit trail so that I can trace who did what.

---

## Requirements

### Must-Have (P0)

#### R1: AppRecord Data Model

**Description:** New `apps` table in SQLite with an in-memory index for fast lookups.

**Schema:**
```sql
CREATE TABLE IF NOT EXISTS apps (
    app_id       TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    client_id    TEXT NOT NULL UNIQUE,
    client_secret_hash TEXT NOT NULL,
    scope_ceiling TEXT NOT NULL,        -- JSON array of scope strings
    status       TEXT NOT NULL DEFAULT 'active',  -- 'active' | 'inactive'
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    created_by   TEXT NOT NULL          -- admin SPIFFE ID or "admin"
);
CREATE INDEX idx_apps_client_id ON apps(client_id);
CREATE INDEX idx_apps_status ON apps(status);
```

**Struct (add to `internal/store/sql_store.go`):**
```go
type AppRecord struct {
    AppID            string    // "app-{name}-{random6}" e.g. "app-weather-bot-a1b2c3"
    Name             string    // Human-readable, unique
    ClientID         string    // "{shortname}-{random12}" e.g. "wb-a1b2c3d4e5f6"
    ClientSecretHash string    // bcrypt hash of the client secret
    ScopeCeiling     []string  // Scope ceiling (JSON-marshaled in DB)
    Status           string    // "active" or "inactive"
    CreatedAt        time.Time
    UpdatedAt        time.Time
    CreatedBy        string
}
```

**Acceptance criteria:**
- [ ] Table created on broker startup (in `SqlStore.InitDB()`)
- [ ] `AppRecord` struct defined with all fields
- [ ] `app_id` format: `app-{name}-{random6hex}` (lowercase, hyphenated)
- [ ] `client_id` format: `{2-3 char abbrev}-{random12hex}`
- [ ] `client_secret` is a random 64-char hex string, stored as bcrypt hash
- [ ] `scope_ceiling` stored as JSON array, unmarshaled on read
- [ ] `status` defaults to `"active"`

**Code locations:**
- Table creation: `internal/store/sql_store.go` → `InitDB()` method (add new CREATE TABLE)
- Struct definition: `internal/store/sql_store.go` (alongside existing `SidecarRecord`, `LaunchTokenRecord`, etc.)

---

#### R2: App Store Methods

**Description:** CRUD operations on the `apps` table, following the same patterns as existing `SaveSidecar`, `ListSidecars`, etc.

**Methods to add to `SqlStore`:**
```go
func (s *SqlStore) SaveApp(rec AppRecord) error
func (s *SqlStore) GetAppByClientID(clientID string) (*AppRecord, error)
func (s *SqlStore) GetAppByID(appID string) (*AppRecord, error)
func (s *SqlStore) ListApps() ([]AppRecord, error)
func (s *SqlStore) UpdateAppCeiling(appID string, newCeiling []string) error
func (s *SqlStore) UpdateAppStatus(appID string, status string) error
```

**Acceptance criteria:**
- [ ] `SaveApp` inserts a new app record; returns error if name or client_id already exists
- [ ] `GetAppByClientID` looks up by `client_id` column (used during auth)
- [ ] `GetAppByID` looks up by `app_id` column (used by admin endpoints)
- [ ] `ListApps` returns all apps ordered by `created_at DESC`
- [ ] `UpdateAppCeiling` updates `scope_ceiling` and `updated_at`; returns error if app not found
- [ ] `UpdateAppStatus` sets status to `"active"` or `"inactive"` and `updated_at`; returns error if app not found
- [ ] All methods use parameterized queries (no SQL injection)

**Code location:** `internal/store/sql_store.go`

**Pattern to follow:** Look at `SaveSidecar()`, `ListSidecars()`, `UpdateSidecarCeiling()` for the exact style.

---

#### R3: AppSvc Service

**Description:** New service package `internal/app/` that handles app registration and authentication business logic. Follows the same layered pattern as `internal/admin/admin_svc.go`.

**File:** `internal/app/app_svc.go`

**Struct:**
```go
type AppSvc struct {
    store    *store.SqlStore
    tknSvc   *token.TknSvc
    auditLog *audit.AuditLog
    audience string
}
```

**Methods:**
```go
func NewAppSvc(store *store.SqlStore, tknSvc *token.TknSvc, auditLog *audit.AuditLog, audience string) *AppSvc

// RegisterApp creates a new app with generated credentials.
// Returns the AppRecord + plaintext client_secret (only returned once).
func (s *AppSvc) RegisterApp(name string, scopes []string, createdBy string) (*RegisterAppResp, error)

// AuthenticateApp validates client_id + client_secret, returns a scoped JWT.
func (s *AppSvc) AuthenticateApp(clientID, clientSecret string) (*token.IssueResp, error)

// ListApps returns all registered apps.
func (s *AppSvc) ListApps() ([]store.AppRecord, error)

// GetApp returns a single app by ID.
func (s *AppSvc) GetApp(appID string) (*store.AppRecord, error)

// UpdateApp updates an app's scope ceiling.
func (s *AppSvc) UpdateApp(appID string, newScopes []string, updatedBy string) error

// DeregisterApp marks an app as inactive (credentials stop working).
func (s *AppSvc) DeregisterApp(appID string, deregisteredBy string) error
```

**`RegisterAppResp`:**
```go
type RegisterAppResp struct {
    AppID        string   `json:"app_id"`
    ClientID     string   `json:"client_id"`
    ClientSecret string   `json:"client_secret"`  // Plaintext — only returned here
    ScopeCeiling []string `json:"scopes"`
    BrokerURL    string   `json:"broker_url,omitempty"`
}
```

**Authentication logic (`AuthenticateApp`):**
1. Look up app by `client_id` → 401 if not found
2. Check `status == "active"` → 401 if inactive
3. Compare `client_secret` against stored bcrypt hash using `bcrypt.CompareHashAndPassword` (constant-time)
4. Issue JWT with:
   - `sub`: `app:{app_id}`
   - `aud`: configured audience
   - `scope`: `["app:launch-tokens:*", "app:agents:*", "app:audit:read"]`
   - `exp`: 300 seconds (5 min, same as admin TTL)
   - `jti`: new UUID
5. Record audit event `app_authenticated` on success, `app_auth_failed` on failure

**Registration logic (`RegisterApp`):**
1. Validate name (non-empty, alphanumeric + hyphens, max 64 chars)
2. Validate scopes (each must parse as `action:resource:identifier`)
3. Generate `app_id`: `app-{name}-{random6hex}`
4. Generate `client_id`: `{first 2-3 chars of name}-{random12hex}`
5. Generate `client_secret`: 64-char random hex via `crypto/rand`
6. Hash secret with `bcrypt.GenerateFromPassword` (cost 12)
7. Save to store
8. Record audit event `app_registered`
9. Return response with plaintext secret (only time it's visible)

**Acceptance criteria:**
- [ ] `RegisterApp` generates cryptographically random credentials
- [ ] `RegisterApp` returns plaintext secret exactly once (never stored unencrypted)
- [ ] `AuthenticateApp` uses constant-time comparison (bcrypt)
- [ ] `AuthenticateApp` returns 401 for unknown client_id, wrong secret, or inactive app
- [ ] `AuthenticateApp` issues JWT with `app:` scopes (not admin scopes)
- [ ] `DeregisterApp` sets status to `"inactive"` (does NOT delete the record)
- [ ] All operations record audit events
- [ ] Scope validation reuses `authz.ParseScope()` from `internal/authz/scope.go`

---

#### R4: App Handler Endpoints

**Description:** New HTTP handler for app-related endpoints. Follows the same pattern as `AdminHandler` in `internal/handler/admin_hdl.go` (which uses `RegisterRoutes` to wire up a group of related endpoints on a mux).

**Note on admin handler location:** The admin handler lives at `internal/admin/admin_hdl.go` (not in `internal/handler/`). The app handler should follow the same pattern — either in `internal/app/app_hdl.go` alongside `app_svc.go`, or as a separate handler file. The recommendation is `internal/app/app_hdl.go` to keep the app package self-contained.

**File:** `internal/app/app_hdl.go` (new file, alongside `app_svc.go`)

**Endpoints:**

| Method | Path | Auth | Scope | Handler Method |
|--------|------|------|-------|----------------|
| `POST` | `/v1/admin/apps` | Bearer (admin) | `admin:launch-tokens:*` | `handleRegisterApp` |
| `GET` | `/v1/admin/apps` | Bearer (admin) | `admin:launch-tokens:*` | `handleListApps` |
| `GET` | `/v1/admin/apps/{id}` | Bearer (admin) | `admin:launch-tokens:*` | `handleGetApp` |
| `PUT` | `/v1/admin/apps/{id}` | Bearer (admin) | `admin:launch-tokens:*` | `handleUpdateApp` |
| `DELETE` | `/v1/admin/apps/{id}` | Bearer (admin) | `admin:launch-tokens:*` | `handleDeregisterApp` |
| `POST` | `/v1/app/auth` | None (rate-limited) | N/A | `handleAppAuth` |

**Request/Response contracts:**

**`POST /v1/admin/apps`** — Register a new app
```json
// Request
{"name": "weather-bot", "scopes": ["read:weather:*", "write:logs:*"]}

// Response (201 Created)
{
    "app_id": "app-weather-bot-a1b2c3",
    "client_id": "wb-a1b2c3d4e5f6",
    "client_secret": "sk_live_...",
    "scopes": ["read:weather:*", "write:logs:*"]
}
```

**`GET /v1/admin/apps`** — List all apps
```json
// Response (200 OK)
{
    "apps": [
        {
            "app_id": "app-weather-bot-a1b2c3",
            "name": "weather-bot",
            "client_id": "wb-a1b2c3d4e5f6",
            "scopes": ["read:weather:*", "write:logs:*"],
            "status": "active",
            "created_at": "2026-03-02T10:00:00Z",
            "updated_at": "2026-03-02T10:00:00Z"
        }
    ],
    "total": 1
}
```
Note: `client_secret_hash` is NEVER returned in any response.

**`GET /v1/admin/apps/{id}`** — Get app details
```json
// Response (200 OK)
{
    "app_id": "app-weather-bot-a1b2c3",
    "name": "weather-bot",
    "client_id": "wb-a1b2c3d4e5f6",
    "scopes": ["read:weather:*", "write:logs:*"],
    "status": "active",
    "created_at": "2026-03-02T10:00:00Z",
    "updated_at": "2026-03-02T10:00:00Z"
}
```

**`PUT /v1/admin/apps/{id}`** — Update app scopes
```json
// Request
{"scopes": ["read:weather:*", "write:logs:*", "read:alerts:*"]}

// Response (200 OK)
{"app_id": "app-weather-bot-a1b2c3", "scopes": ["read:weather:*", "write:logs:*", "read:alerts:*"], "updated_at": "2026-03-02T11:00:00Z"}
```

**`DELETE /v1/admin/apps/{id}`** — Deregister app
```json
// Response (200 OK)
{"app_id": "app-weather-bot-a1b2c3", "status": "inactive", "deregistered_at": "2026-03-02T12:00:00Z"}
```

**`POST /v1/app/auth`** — App authenticates
```json
// Request
{"client_id": "wb-a1b2c3d4e5f6", "client_secret": "sk_live_..."}

// Response (200 OK)
{
    "access_token": "eyJ...",
    "expires_in": 300,
    "token_type": "Bearer",
    "scopes": ["app:launch-tokens:*", "app:agents:*", "app:audit:read"]
}

// Response (401 Unauthorized)
{"type": "unauthorized", "title": "Authentication failed", "detail": "Invalid client credentials"}
```

**Acceptance criteria:**
- [ ] All admin endpoints require Bearer token with `admin:launch-tokens:*` scope
- [ ] `POST /v1/app/auth` has NO Bearer auth requirement (it IS the auth endpoint)
- [ ] `POST /v1/app/auth` has per-client_id rate limiting (see R5)
- [ ] `DELETE` sets status to inactive, does NOT delete the row
- [ ] `client_secret_hash` is never included in any response
- [ ] All responses use RFC 7807 Problem Details for errors (using existing `problemdetails` package)
- [ ] Request body size limited by `MaxBytesBody(1MB)` middleware (same as all other endpoints)
- [ ] `{id}` path parameter extracted from URL path (same pattern as `sidecars/{id}/ceiling`)

**Code locations to wire up routes:**
- Handler definition: `internal/app/app_hdl.go` (new file, alongside `app_svc.go`)
- Route registration: `cmd/broker/main.go` — add `appHdl.RegisterRoutes(mux)` alongside existing `adminHdl.RegisterRoutes(mux)`
- Pattern: Follow `internal/admin/admin_hdl.go` → `RegisterRoutes()` method
- Timestamp format: Use `time.RFC3339` for all response timestamps (matches existing admin handler pattern)

---

#### R5: Per-Client-ID Rate Limiting

**Description:** The `POST /v1/app/auth` endpoint needs rate limiting keyed on `client_id` (not just IP). A compromised `client_id` under brute-force shouldn't DoS the auth endpoint for other apps.

**Implementation:**

Reuse the existing `RateLimiter` from `internal/authz/rate_mw.go` but create a second instance keyed on `client_id` extracted from the request body.

**Option A (recommended):** Add a new middleware method to `RateLimiter`:
```go
// WrapWithKeyExtractor wraps a handler with rate limiting using a custom key.
// keyExtractor reads the request and returns the rate-limit key.
// If extraction fails, falls back to IP-based limiting.
func (rl *RateLimiter) WrapWithKeyExtractor(next http.Handler, keyExtractor func(r *http.Request) string) http.Handler
```

For `POST /v1/app/auth`, the key extractor peeks at the request body to read `client_id`. This requires buffering the body (read, then reset with `io.NopCloser`).

**Rate limit values:** 10 attempts per minute per `client_id` (stricter than the admin auth endpoint's 5/sec because app auth is more exposed).

**Acceptance criteria:**
- [ ] `POST /v1/app/auth` rate-limited per `client_id` (not just per IP)
- [ ] Rate limit: 10 requests/minute per `client_id` (0.167 req/sec, burst 3)
- [ ] When rate-limited: 429 Too Many Requests with `Retry-After` header
- [ ] Unknown/missing `client_id` falls back to IP-based rate limiting
- [ ] Rate limit audit event recorded on trigger
- [ ] Existing admin auth rate limiting (`POST /v1/admin/auth`) unchanged

**Code location:** `internal/authz/rate_mw.go` (extend existing file)

---

#### R6: App JWT Scope Family

**Description:** Define the new `app:` scope namespace that app JWTs carry. These scopes control what authenticated apps can do.

**Scopes:**
```
app:launch-tokens:*    → create launch tokens within app's scope ceiling
app:agents:*           → register agents, manage agent tokens
app:audit:read         → query audit events for own app only
```

**What these unlock (in future phases):**
- `app:launch-tokens:*` — Phase 1b: apps call `POST /v1/admin/launch-tokens` with their own JWT
- `app:agents:*` — Phase 1b: apps register agents within their ceiling
- `app:audit:read` — Phase 1c: apps query their own audit events

**In Phase 1a:** The scopes are DEFINED and ISSUED in the app JWT, but the middleware endpoints that check them are wired in Phase 1b. Phase 1a just ensures the JWT carries the right scopes.

**Acceptance criteria:**
- [ ] App JWT issued by `AuthenticateApp` carries exactly these 3 scopes
- [ ] Scopes parse correctly via existing `authz.ParseScope()`
- [ ] Existing `admin:` scopes unchanged
- [ ] Existing `sidecar:` scopes unchanged

---

#### R7: aactl App Commands

**Description:** New `aactl app` subcommand group for operator management. Follows the same Cobra pattern as `aactl sidecars`, `aactl audit`, `aactl revoke`.

**File:** `cmd/aactl/apps.go` (new file)

**Commands:**
```
aactl app register --name NAME --scopes SCOPE_CSV
aactl app list
aactl app get ID
aactl app update --id ID --scopes SCOPE_CSV
aactl app remove --id ID
```

**Behavior:**

`aactl app register --name weather-bot --scopes "read:weather:*,write:logs:*"`:
- Calls `POST /v1/admin/apps` with admin Bearer token
- Displays: app_id, client_id, client_secret, scopes
- Warns: "Save the client_secret — it cannot be retrieved again"

`aactl app list`:
- Calls `GET /v1/admin/apps` with admin Bearer token
- Table output: NAME | APP_ID | CLIENT_ID | STATUS | SCOPES | CREATED
- JSON output with `--json` flag

`aactl app get <app_id>`:
- Calls `GET /v1/admin/apps/{id}` with admin Bearer token
- Shows full app details

`aactl app update --id <app_id> --scopes "new,scope,list"`:
- Calls `PUT /v1/admin/apps/{id}` with admin Bearer token
- Confirms scope change

`aactl app remove --id <app_id>`:
- Calls `DELETE /v1/admin/apps/{id}` with admin Bearer token
- Confirms deregistration

**Acceptance criteria:**
- [ ] All commands use `AACTL_BROKER_URL` and `AACTL_ADMIN_TOKEN` env vars (same as existing commands)
- [ ] `register` command warns about saving the client_secret
- [ ] `list` command supports `--json` flag (using existing `output.go` utilities)
- [ ] `remove` confirms the app is deregistered (not deleted)
- [ ] Error messages use RFC 7807 format from the broker
- [ ] Help text for each command (`aactl app register --help`)

**Code locations:**
- New file: `cmd/aactl/apps.go`
- Register with root: `cmd/aactl/root.go` → add `rootCmd.AddCommand(appCmd)`
- Pattern: Follow `cmd/aactl/sidecars.go` for Cobra structure, `cmd/aactl/output.go` for table formatting

---

#### R8: Audit Events for App Operations

**Description:** All app operations must generate audit events in the hash-chained audit trail.

**New event type constants (add to `internal/audit/audit_log.go`):**
```go
EventAppRegistered       = "app_registered"
EventAppAuthenticated    = "app_authenticated"
EventAppAuthFailed       = "app_auth_failed"
EventAppUpdated          = "app_updated"
EventAppDeregistered     = "app_deregistered"
EventAppRateLimited      = "app_rate_limited"
```

**Event details:**

| Event | AgentID | Detail | Outcome |
|-------|---------|--------|---------|
| `app_registered` | `""` | `"app={name} client_id={id} scopes=[...]"` | `"success"` |
| `app_authenticated` | `""` | `"client_id={id} app_id={app_id}"` | `"success"` |
| `app_auth_failed` | `""` | `"client_id={id} reason={reason}"` | `"denied"` |
| `app_updated` | `""` | `"app_id={id} scopes=[new scopes]"` | `"success"` |
| `app_deregistered` | `""` | `"app_id={id} name={name}"` | `"success"` |
| `app_rate_limited` | `""` | `"client_id={id}"` | `"denied"` |

Note: `client_secret` is NEVER logged (PII sanitization already strips secrets via existing `sanitizeDetail()` in audit_log.go).

**Acceptance criteria:**
- [ ] All 6 event types defined as constants
- [ ] Every successful and failed app operation generates an audit event
- [ ] Events chain correctly (hash of previous event included)
- [ ] `client_secret` never appears in audit detail (sanitized)
- [ ] Events queryable via existing `GET /v1/audit/events?event_type=app_registered`

**Code location:** Event constants in `internal/audit/audit_log.go`, event recording in `internal/app/app_svc.go`

---

### Nice-to-Have (P1)

#### R9: App Name Validation

- Names must be lowercase alphanumeric + hyphens only
- Max 64 characters
- Must start with a letter
- No consecutive hyphens
- Regex: `^[a-z][a-z0-9](?:-[a-z0-9]+)*$` (max 64)

#### R10: Scope Ceiling Validation on Update

- When updating an app's scope ceiling, log a warning if the new ceiling is WIDER than the old one (scopes added)
- This is informational, not blocking — operators may legitimately need to expand scopes

---

### Future Considerations (P2)

#### R11: App Metadata Fields

- `description`, `contact_email`, `environment` (dev/staging/prod) fields on AppRecord
- Useful for larger deployments but not needed for core functionality

#### R12: App API Key Alternative

- Support API key auth as an alternative to client_id + client_secret
- Single long token instead of a two-part credential
- Simpler for some integration patterns

---

## Task Breakdown

These are the implementation tasks in dependency order:

### Task 1: AppRecord data model + store methods
**Files:** `internal/store/sql_store.go`
- Add `AppRecord` struct
- Add `apps` table in `InitDB()` (note: method is `InitDB`, not `initTables`)
- Implement all 6 store methods (R2)
- Write unit tests

### Task 2: AppSvc service
**Files:** `internal/app/app_svc.go` (new), `internal/app/app_svc_test.go` (new)
- Create `internal/app/` package
- Implement `AppSvc` with all methods (R3)
- Add audit event constants (R8)
- Wire audit recording into all operations
- Write unit tests

### Task 3: App handler endpoints
**Files:** `internal/app/app_hdl.go` (new), `internal/app/app_hdl_test.go` (new)
- Implement `AppHdl` with `RegisterRoutes()`
- Implement all 6 handler methods (R4)
- Wire into `cmd/broker/main.go` routes

### Task 4: Per-client-id rate limiting
**Files:** `internal/authz/rate_mw.go`
- Add `WrapWithKeyExtractor` method (R5)
- Wire rate limiter to `POST /v1/app/auth` in `cmd/broker/main.go`
- Write tests for rate limit behavior

### Task 5: aactl app commands
**Files:** `cmd/aactl/apps.go` (new), `cmd/aactl/root.go` (register commands)
- Implement all 5 CLI commands (R7)
- Table + JSON output
- Register with root command

### Task 6: Integration tests
**Files:** `tests/phase-1a-user-stories.md` (user stories first), then test files
- Write user stories document FIRST
- Docker live test: register app → authenticate → verify JWT scopes
- Docker live test: list apps → verify table output
- Docker live test: deregister → verify auth fails
- Docker live test: rate limiting triggers on burst

---

## Verification Checklist

Before marking Phase 1a complete:

- [ ] `gates.sh` passes (lint, vet, test)
- [ ] Docker stack starts successfully with new code
- [ ] `aactl app register` returns credentials
- [ ] `POST /v1/app/auth` returns JWT with `app:` scopes
- [ ] `aactl app list` shows registered apps
- [ ] `aactl app remove` makes auth fail
- [ ] Rate limiting triggers on burst (10+ rapid auth attempts)
- [ ] Audit events recorded for all operations
- [ ] No master key used in any app auth flow
- [ ] Existing sidecar/admin/agent flows still work (no regressions)
- [ ] `client_secret_hash` never appears in any API response
- [ ] `client_secret` never appears in audit logs

---

## Dependencies

- **None** — Phase 1a has no external dependencies. It adds new tables, new endpoints, and new CLI commands without modifying any existing code paths.
- **Backward compatible** — existing admin auth, sidecar auth, agent registration, and all other flows are untouched.

## SQLite Migration Strategy

Phase 1a only adds the new `apps` table — this is safe as a `CREATE TABLE IF NOT EXISTS` with no impact on existing tables.

Later phases (1b, 1c) will add `app_id` columns to existing tables (`agents`, `launch_tokens`). **SQLite `ALTER TABLE ... ADD COLUMN` requires that new columns have a default value or be nullable** — you cannot add a `NOT NULL` column without a default to an existing table with rows. The strategy:

- All new `app_id` columns on existing tables must be added as `TEXT DEFAULT NULL`
- Existing rows will have `app_id = NULL` (they predate apps, and this is the correct semantic — they are "legacy" / no-app rows)
- Code must handle `app_id = NULL` without error (it means the agent/token was not created via an app)
- Migration is handled by adding `ALTER TABLE` statements to `InitDB()` wrapped in `IF NOT EXISTS` guards (SQLite doesn't natively support `IF NOT EXISTS` on `ALTER TABLE`, so wrap in a check: attempt, catch "duplicate column name" error, continue)

This is documented here so the implementing agent does NOT design a migration system — simple `ALTER TABLE` in `InitDB()` with error-tolerant logic is sufficient.

---

## Security Considerations

1. **Client secret storage:** bcrypt (cost 12), never stored or logged in plaintext
2. **Constant-time comparison:** bcrypt handles this, but verify no early-return on client_id lookup failure (use consistent response time)
3. **Rate limiting:** Per-client_id prevents credential stuffing without affecting other apps
4. **Scope isolation:** App JWT carries `app:` scopes, never `admin:` scopes
5. **Audit trail:** All operations logged, hash-chained, tamper-evident
6. **No master key exposure:** Apps never see or use the admin master key
