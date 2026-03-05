# TD-006: Per-App Configurable JWT TTL — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make app JWT TTL configurable per-app (stored in DB) with an operator-configurable global default, replacing the hardcoded 5-minute constant.

**Architecture:** Two-layer TTL resolution — global default from `AA_APP_TOKEN_TTL` env var (1800s), per-app override stored in `apps.token_ttl` column. At auth time, `AuthenticateApp()` reads the app's stored TTL. Safety bounds (60s–86400s) enforced at registration and update.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), cobra CLI, standard library `net/http`

**Spec:** `.plans/td-006/TD-006-App-JWT-TTL.md`

---

## Task 0: Write User Stories for Acceptance Criteria

**Files:**
- Create: `tests/td-006/user-stories.md`

**Step 1: Create test directory and write user stories**

Create `tests/td-006/user-stories.md` with the stories from the spec. These are the acceptance criteria that drive all testing. Every story must have a persona, action, and verification.

```markdown
# TD-006: Per-App JWT TTL — User Stories

## Personas and Tools

| Persona | Tool | Stories |
|---------|------|---------|
| Operator | `aactl` | 1–4 |
| Developer | `curl` | 5 |
| Security reviewer | Both | 6–7 |

---

## Operator Stories

### S1 — Operator registers app with default TTL

**As an operator**, I want to register an app without specifying a TTL and have it get the global default (30 min).

**Precondition:** Broker running with default config (no AA_APP_TOKEN_TTL set).

**Acceptance criteria:**
- `aactl app register --name ttl-default --scopes "read:data:*"` returns 201
- Response shows `token_ttl: 1800`
- `aactl app get <app_id>` confirms `token_ttl: 1800`

### S2 — Operator registers app with custom TTL

**As an operator**, I want to specify a TTL when registering an app so that long-running apps get longer tokens.

**Acceptance criteria:**
- `aactl app register --name ttl-custom --scopes "read:data:*" --token-ttl 3600` returns 201
- Response shows `token_ttl: 3600`
- Developer authenticates with this app's credentials → JWT `exp` is ~3600s from `iat`

### S3 — Operator updates existing app's TTL

**As an operator**, I want to change an app's TTL without re-registering it.

**Acceptance criteria:**
- `aactl app update --id <app_id> --token-ttl 7200` succeeds
- `aactl app get <app_id>` shows `token_ttl: 7200`
- Next developer auth uses new TTL
- Audit trail shows `app_updated` event with old and new TTL values

### S4 — Operator is rejected for out-of-bounds TTL

**As an operator**, I want the broker to reject TTL values outside safe bounds.

**Acceptance criteria:**
- `aactl app register --name ttl-low --scopes "read:data:*" --token-ttl 30` → error (< 60s minimum)
- `aactl app register --name ttl-high --scopes "read:data:*" --token-ttl 100000` → error (> 86400s maximum)
- `aactl app update --id <app_id> --token-ttl 5` → error (< 60s minimum)

## Developer Stories

### S5 — Developer gets a token that lasts long enough

**As a developer**, I want my app JWT to reflect the configured TTL.

**Precondition:** App registered with `--token-ttl 3600`.

**Acceptance criteria:**
- `POST /v1/app/auth` with valid credentials returns 200
- Response `expires_in` field is 3600
- JWT `exp` claim is approximately `iat + 3600`

## Security Stories

### S6 — TTL bounds prevent misconfiguration

**As a security reviewer**, I want TTL bounded between 60s and 86400s.

**Acceptance criteria:**
- API rejects `token_ttl: 0` at registration (if explicitly provided; omitted=default is ok)
- API rejects `token_ttl: -1`
- API rejects `token_ttl: 86401`
- API accepts `token_ttl: 60` (minimum)
- API accepts `token_ttl: 86400` (maximum)

### S7 — TTL changes are audited

**As a security reviewer**, I want TTL changes recorded in audit.

**Acceptance criteria:**
- After `aactl app update --token-ttl`, `aactl audit events --event-type app_updated` shows the change
- Audit detail includes old TTL and new TTL values
```

**Step 2: Commit**

```bash
git add tests/td-006/user-stories.md
git commit -m "test: TD-006 user stories for per-app JWT TTL"
```

---

## Task 1: Add `AA_APP_TOKEN_TTL` to Config

**Files:**
- Modify: `internal/cfg/cfg.go:31-44` (Cfg struct), `internal/cfg/cfg.go:49-71` (Load func)
- Modify: `internal/cfg/cfg_test.go` (add tests)

**Step 1: Write the failing tests**

Add to `internal/cfg/cfg_test.go`:

```go
func TestLoad_AppTokenTTLDefault(t *testing.T) {
	os.Unsetenv("AA_APP_TOKEN_TTL")
	c := Load()
	if c.AppTokenTTL != 1800 {
		t.Fatalf("expected default AppTokenTTL 1800, got %d", c.AppTokenTTL)
	}
}

func TestLoad_AppTokenTTLCustom(t *testing.T) {
	os.Setenv("AA_APP_TOKEN_TTL", "3600")
	defer os.Unsetenv("AA_APP_TOKEN_TTL")
	c := Load()
	if c.AppTokenTTL != 3600 {
		t.Fatalf("expected AppTokenTTL 3600, got %d", c.AppTokenTTL)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/cfg/... -run TestLoad_AppTokenTTL -v`
Expected: FAIL — `c.AppTokenTTL` field does not exist

**Step 3: Write minimal implementation**

In `internal/cfg/cfg.go`, add `AppTokenTTL` to the `Cfg` struct:

```go
type Cfg struct {
	Port         string // AA_PORT (default "8080")
	LogLevel     string // AA_LOG_LEVEL (default "verbose")
	TrustDomain  string // AA_TRUST_DOMAIN (default "agentauth.local")
	DefaultTTL   int    // AA_DEFAULT_TTL (default 300 seconds)
	AppTokenTTL  int    // AA_APP_TOKEN_TTL (default 1800 seconds / 30 min)
	AdminSecret  string // AA_ADMIN_SECRET (required for admin auth)
	SeedTokens   bool   // AA_SEED_TOKENS (dev only, default false)
	DBPath       string // AA_DB_PATH (default "./agentauth.db")
	TLSMode      string // AA_TLS_MODE: none|tls|mtls (default "none")
	TLSCert      string // AA_TLS_CERT: path to TLS certificate PEM file
	TLSKey       string // AA_TLS_KEY: path to TLS private key PEM file
	TLSClientCA  string // AA_TLS_CLIENT_CA: path to client CA PEM file (mtls only)
	Audience     string // AA_AUDIENCE: expected token audience (default "agentauth", empty = skip)
}
```

In `Load()`, after the `DefaultTTL` line, add:

```go
AppTokenTTL: envIntOr("AA_APP_TOKEN_TTL", 1800),
```

Update the package godoc to include `AA_APP_TOKEN_TTL`.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/cfg/... -v`
Expected: All tests PASS including the two new ones

**Step 5: Commit**

```bash
git add internal/cfg/cfg.go internal/cfg/cfg_test.go
git commit -m "feat: add AA_APP_TOKEN_TTL config (default 1800s)"
```

---

## Task 2: Add `TokenTTL` to AppRecord and Schema

**Files:**
- Modify: `internal/store/sql_store.go:802-828` (AppRecord struct + createAppsTable)
- Modify: `internal/store/sql_store.go:357-391` (InitDB — add migration)
- Modify: `internal/store/sql_store.go:832-866` (SaveApp)
- Modify: `internal/store/sql_store.go:870-927` (GetAppByClientID, GetAppByID, scanAppRow)
- Modify: `internal/store/sql_store.go:930-985` (ListApps)

**Step 1: Write the failing test**

Create a test in an existing or new store test file that verifies `TokenTTL` round-trips through save and get:

```go
func TestSaveApp_TokenTTL(t *testing.T) {
	st := newTestStore(t) // uses t.TempDir()

	rec := store.AppRecord{
		AppID:            "app-test-ttl-aaa111",
		Name:             "test-ttl",
		ClientID:         "tt-aaa111bbb222",
		ClientSecretHash: "$2a$12$fakehash",
		ScopeCeiling:     []string{"read:data:*"},
		TokenTTL:         3600,
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}
	if err := st.SaveApp(rec); err != nil {
		t.Fatalf("SaveApp: %v", err)
	}

	got, err := st.GetAppByID("app-test-ttl-aaa111")
	if err != nil {
		t.Fatalf("GetAppByID: %v", err)
	}
	if got.TokenTTL != 3600 {
		t.Fatalf("expected TokenTTL 3600, got %d", got.TokenTTL)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestSaveApp_TokenTTL -v`
Expected: FAIL — `TokenTTL` field does not exist on `AppRecord`

**Step 3: Write minimal implementation**

Add `TokenTTL` to `AppRecord`:

```go
type AppRecord struct {
	AppID            string
	Name             string
	ClientID         string
	ClientSecretHash string
	ScopeCeiling     []string
	TokenTTL         int       // JWT TTL in seconds (default 1800)
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CreatedBy        string
}
```

Update `createAppsTable` to include `token_ttl`:

```sql
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
```

Add migration in `InitDB()` — after the `createAppsTable` exec, add:

```go
// Migrate: add token_ttl column to existing apps tables that lack it.
s.migrateAddColumn(db, "apps", "token_ttl", "INTEGER NOT NULL DEFAULT 1800")
```

Add the migration helper:

```go
func (s *SqlStore) migrateAddColumn(db *sql.DB, table, column, colDef string) {
	// Check if column exists using PRAGMA table_info
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
	// Column doesn't exist — add it
	q := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colDef)
	if _, err := db.Exec(q); err != nil {
		obs.Warn("store", "sqlite", "migration failed", "table="+table, "column="+column, "error="+err.Error())
	} else {
		obs.Ok("store", "sqlite", "migration applied", "table="+table, "column="+column)
	}
}
```

Update `SaveApp` INSERT to include `token_ttl`:

```go
const q = `INSERT INTO apps
	(app_id, name, client_id, client_secret_hash, scope_ceiling, token_ttl, status, created_at, updated_at, created_by)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

_, err = s.db.Exec(q,
	rec.AppID, rec.Name, rec.ClientID, rec.ClientSecretHash,
	string(scopeJSON), rec.TokenTTL, rec.Status,
	rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	rec.CreatedBy,
)
```

Update `scanAppRow` to read `token_ttl`:

```go
func (s *SqlStore) scanAppRow(row *sql.Row) (*AppRecord, error) {
	var rec AppRecord
	var scopeStr, createdStr, updatedStr string
	err := row.Scan(
		&rec.AppID, &rec.Name, &rec.ClientID, &rec.ClientSecretHash,
		&scopeStr, &rec.TokenTTL, &rec.Status, &createdStr, &updatedStr, &rec.CreatedBy,
	)
	// ... rest unchanged
}
```

Update `ListApps` row scan similarly — add `&rec.TokenTTL` to the `Scan` call.

Update all SELECT queries in `GetAppByClientID`, `GetAppByID`, and `ListApps` to include `token_ttl` in the column list:

```sql
SELECT app_id, name, client_id, client_secret_hash, scope_ceiling, token_ttl, status, created_at, updated_at, created_by FROM apps ...
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/store/... -run TestSaveApp_TokenTTL -v`
Expected: PASS

Run: `go test ./internal/store/... -v`
Expected: All existing store tests PASS (they'll need AppRecord updates too — set `TokenTTL: 300` or similar in test fixtures)

**Step 5: Commit**

```bash
git add internal/store/sql_store.go
git commit -m "feat: add token_ttl column to apps table with migration"
```

---

## Task 3: Add `UpdateAppTTL` Store Method

**Files:**
- Modify: `internal/store/sql_store.go` (add `UpdateAppTTL` after `UpdateAppCeiling`)

**Step 1: Write the failing test**

```go
func TestUpdateAppTTL(t *testing.T) {
	st := newTestStore(t)

	rec := store.AppRecord{
		AppID: "app-ttl-update-aaa111", Name: "ttl-update",
		ClientID: "tu-aaa111bbb222", ClientSecretHash: "$2a$12$fakehash",
		ScopeCeiling: []string{"read:data:*"}, TokenTTL: 1800,
		Status: "active", CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(), CreatedBy: "admin",
	}
	st.SaveApp(rec)

	if err := st.UpdateAppTTL("app-ttl-update-aaa111", 7200); err != nil {
		t.Fatalf("UpdateAppTTL: %v", err)
	}

	got, _ := st.GetAppByID("app-ttl-update-aaa111")
	if got.TokenTTL != 7200 {
		t.Fatalf("expected 7200, got %d", got.TokenTTL)
	}
}

func TestUpdateAppTTL_NotFound(t *testing.T) {
	st := newTestStore(t)
	err := st.UpdateAppTTL("app-nonexistent-000000", 3600)
	if !errors.Is(err, store.ErrAppNotFound) {
		t.Fatalf("expected ErrAppNotFound, got %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestUpdateAppTTL -v`
Expected: FAIL — `UpdateAppTTL` method does not exist

**Step 3: Write minimal implementation**

```go
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
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/... -run TestUpdateAppTTL -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/sql_store.go
git commit -m "feat: add UpdateAppTTL store method"
```

---

## Task 4: Update AppSvc — Accept TTL at Registration, Use Per-App TTL at Auth

**Files:**
- Modify: `internal/app/app_svc.go:24` (remove constant)
- Modify: `internal/app/app_svc.go:44-50` (AppSvc struct — add defaultTTL)
- Modify: `internal/app/app_svc.go:65-74` (NewAppSvc — accept cfg)
- Modify: `internal/app/app_svc.go:78-123` (RegisterApp — accept TTL param)
- Modify: `internal/app/app_svc.go:128-174` (AuthenticateApp — use rec.TokenTTL)
- Modify: `internal/app/app_svc.go:187-198` (UpdateApp — add TTL support)

**Step 1: Write the failing tests**

Add to `internal/app/app_svc_test.go`:

```go
func TestRegisterApp_DefaultTTL(t *testing.T) {
	svc := newTestAppSvc(t) // helper that creates AppSvc with defaultTTL=1800
	resp, err := svc.RegisterApp("default-ttl", []string{"read:data:*"}, "admin", 0)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}
	// Verify stored TTL is the default
	rec, _ := svc.GetApp(resp.AppID)
	if rec.TokenTTL != 1800 {
		t.Fatalf("expected default TTL 1800, got %d", rec.TokenTTL)
	}
}

func TestRegisterApp_CustomTTL(t *testing.T) {
	svc := newTestAppSvc(t)
	resp, err := svc.RegisterApp("custom-ttl", []string{"read:data:*"}, "admin", 3600)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}
	rec, _ := svc.GetApp(resp.AppID)
	if rec.TokenTTL != 3600 {
		t.Fatalf("expected TTL 3600, got %d", rec.TokenTTL)
	}
}

func TestRegisterApp_TTLTooLow(t *testing.T) {
	svc := newTestAppSvc(t)
	_, err := svc.RegisterApp("ttl-low", []string{"read:data:*"}, "admin", 30)
	if err == nil || !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("expected ErrInvalidTTL, got %v", err)
	}
}

func TestRegisterApp_TTLTooHigh(t *testing.T) {
	svc := newTestAppSvc(t)
	_, err := svc.RegisterApp("ttl-high", []string{"read:data:*"}, "admin", 100000)
	if err == nil || !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("expected ErrInvalidTTL, got %v", err)
	}
}

func TestAuthenticateApp_UsesPerAppTTL(t *testing.T) {
	svc := newTestAppSvc(t)
	resp, _ := svc.RegisterApp("auth-ttl", []string{"read:data:*"}, "admin", 7200)
	// Retrieve the stored client_secret from the registration response
	authResp, err := svc.AuthenticateApp(resp.ClientID, resp.ClientSecret)
	if err != nil {
		t.Fatalf("AuthenticateApp: %v", err)
	}
	if authResp.ExpiresIn != 7200 {
		t.Fatalf("expected ExpiresIn 7200, got %d", authResp.ExpiresIn)
	}
}

func TestUpdateAppTTL_Service(t *testing.T) {
	svc := newTestAppSvc(t)
	resp, _ := svc.RegisterApp("update-ttl", []string{"read:data:*"}, "admin", 1800)
	if err := svc.UpdateAppTTL(resp.AppID, 3600, "admin"); err != nil {
		t.Fatalf("UpdateAppTTL: %v", err)
	}
	rec, _ := svc.GetApp(resp.AppID)
	if rec.TokenTTL != 3600 {
		t.Fatalf("expected 3600, got %d", rec.TokenTTL)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/... -run "TestRegisterApp_DefaultTTL|TestRegisterApp_CustomTTL|TestRegisterApp_TTLToo|TestAuthenticateApp_Uses|TestUpdateAppTTL_Service" -v`
Expected: FAIL — signature mismatch, missing fields

**Step 3: Write minimal implementation**

Remove the constant:
```go
// DELETE: const appTokenTTL = 300
```

Add TTL bounds constants and error:
```go
const (
	minAppTokenTTL = 60     // 1 minute
	maxAppTokenTTL = 86400  // 24 hours
)

var ErrInvalidTTL = errors.New("invalid token TTL")
```

Update `AppSvc` struct and constructor to carry the default TTL:

```go
type AppSvc struct {
	store       *store.SqlStore
	tknSvc      *token.TknSvc
	auditLog    *audit.AuditLog
	audience    string
	defaultTTL  int // default app token TTL from cfg.AppTokenTTL
}

func NewAppSvc(st *store.SqlStore, tknSvc *token.TknSvc, al *audit.AuditLog, audience string, appTokenTTL int) *AppSvc {
	return &AppSvc{
		store:      st,
		tknSvc:     tknSvc,
		auditLog:   al,
		audience:   audience,
		defaultTTL: appTokenTTL,
	}
}
```

Update `RegisterApp` signature and add TTL validation:

```go
func (s *AppSvc) RegisterApp(name string, scopes []string, createdBy string, tokenTTL int) (*RegisterAppResp, error) {
	if err := validateAppName(name); err != nil {
		return nil, err
	}
	if err := validateScopes(scopes); err != nil {
		return nil, err
	}

	// Resolve TTL: 0 means use default, otherwise validate bounds
	ttl := tokenTTL
	if ttl == 0 {
		ttl = s.defaultTTL
	} else if ttl < minAppTokenTTL || ttl > maxAppTokenTTL {
		return nil, fmt.Errorf("%w: must be between %d and %d seconds, got %d",
			ErrInvalidTTL, minAppTokenTTL, maxAppTokenTTL, ttl)
	}

	// ... rest of method, but set rec.TokenTTL = ttl
```

Update `RegisterAppResp` to include TTL:

```go
type RegisterAppResp struct {
	AppID        string   `json:"app_id"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	ScopeCeiling []string `json:"scopes"`
	TokenTTL     int      `json:"token_ttl"`
}
```

Update `AuthenticateApp` to use `rec.TokenTTL`:

```go
resp, err := s.tknSvc.Issue(token.IssueReq{
	Sub:   "app:" + rec.AppID,
	Aud:   aud,
	Scope: []string{"app:launch-tokens:*", "app:agents:*", "app:audit:read"},
	TTL:   rec.TokenTTL,
})
```

Add `UpdateAppTTL` method:

```go
func (s *AppSvc) UpdateAppTTL(appID string, newTTL int, updatedBy string) error {
	if newTTL < minAppTokenTTL || newTTL > maxAppTokenTTL {
		return fmt.Errorf("%w: must be between %d and %d seconds, got %d",
			ErrInvalidTTL, minAppTokenTTL, maxAppTokenTTL, newTTL)
	}

	// Get old TTL for audit trail
	rec, err := s.store.GetAppByID(appID)
	if err != nil {
		return err
	}
	oldTTL := rec.TokenTTL

	if err := s.store.UpdateAppTTL(appID, newTTL); err != nil {
		return err
	}

	s.record(audit.EventAppUpdated, "",
		fmt.Sprintf("app_id=%s token_ttl=%d->%d updated_by=%s", appID, oldTTL, newTTL, updatedBy),
		audit.WithOutcome("success"))
	return nil
}
```

**Step 4: Fix callers**

Update `cmd/broker/main.go` — the `NewAppSvc` call:

```go
appSvc := app.NewAppSvc(sqlStore, tknSvc, auditLog, c.Audience, c.AppTokenTTL)
```

Update all test helpers that call `NewAppSvc` to pass the TTL parameter (use `1800` or similar).

Update all test calls to `RegisterApp` to pass the TTL parameter (use `0` for "use default").

**Step 5: Run all tests**

Run: `go test ./internal/app/... -v`
Expected: All tests PASS

Run: `go test ./... -count=1` (full suite to catch any missed callers)
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/app/app_svc.go internal/app/app_svc_test.go cmd/broker/main.go
git commit -m "feat: per-app JWT TTL — configurable at registration and auth"
```

---

## Task 5: Update HTTP Handler — Registration and Update

**Files:**
- Modify: `internal/app/app_hdl.go:62-66` (registerAppReq — add TokenTTL)
- Modify: `internal/app/app_hdl.go:67-77` (appResp — add TokenTTL)
- Modify: `internal/app/app_hdl.go:84-86` (updateAppReq — add TokenTTL)
- Modify: `internal/app/app_hdl.go:105-151` (handleRegisterApp)
- Modify: `internal/app/app_hdl.go:200-254` (handleUpdateApp)
- Modify: `internal/app/app_hdl.go:358-370` (storeAppToResp)

**Step 1: Write the failing test**

Add to `internal/app/app_hdl_test.go`:

```go
func TestHandleRegisterApp_WithTTL(t *testing.T) {
	mux, _ := newTestAppMux(t)
	adminToken := getAdminToken(t, mux)

	body := `{"name":"ttl-app","scopes":["read:data:*"],"token_ttl":3600}`
	req := httptest.NewRequest("POST", "/v1/admin/apps", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		TokenTTL int `json:"token_ttl"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.TokenTTL != 3600 {
		t.Fatalf("expected token_ttl 3600, got %d", resp.TokenTTL)
	}
}

func TestHandleRegisterApp_TTLOutOfBounds(t *testing.T) {
	mux, _ := newTestAppMux(t)
	adminToken := getAdminToken(t, mux)

	body := `{"name":"ttl-bad","scopes":["read:data:*"],"token_ttl":30}`
	req := httptest.NewRequest("POST", "/v1/admin/apps", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/... -run "TestHandleRegisterApp_WithTTL|TestHandleRegisterApp_TTLOutOfBounds" -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Update request/response types:

```go
type registerAppReq struct {
	Name     string   `json:"name"`
	Scopes   []string `json:"scopes"`
	TokenTTL int      `json:"token_ttl,omitempty"` // 0 = use default
}

type appResp struct {
	AppID          string   `json:"app_id"`
	Name           string   `json:"name"`
	ClientID       string   `json:"client_id"`
	ClientSecret   string   `json:"client_secret,omitempty"`
	Scopes         []string `json:"scopes"`
	TokenTTL       int      `json:"token_ttl"`
	Status         string   `json:"status"`
	CreatedAt      string   `json:"created_at,omitempty"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
	DeregisteredAt string   `json:"deregistered_at,omitempty"`
}

type updateAppReq struct {
	Scopes   []string `json:"scopes,omitempty"`
	TokenTTL *int     `json:"token_ttl,omitempty"` // pointer to distinguish absent from zero
}
```

Update `handleRegisterApp` to pass TTL:

```go
resp, err := h.appSvc.RegisterApp(req.Name, req.Scopes, createdBy, req.TokenTTL)
```

Add `ErrInvalidTTL` to the error switch:

```go
case errors.Is(err, ErrInvalidTTL):
	problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_ttl", err.Error(), r.URL.Path)
```

Update the registration response to include TTL:

```go
json.NewEncoder(w).Encode(appResp{
	AppID:        resp.AppID,
	ClientID:     resp.ClientID,
	ClientSecret: resp.ClientSecret,
	Scopes:       resp.ScopeCeiling,
	TokenTTL:     resp.TokenTTL,
})
```

Update `handleUpdateApp` to handle TTL updates:

```go
func (h *AppHdl) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	// ... existing appID extraction and body parse ...

	var req updateAppReq
	// ... decode ...

	claims := authz.ClaimsFromContext(r.Context())
	updatedBy := ""
	if claims != nil {
		updatedBy = claims.Sub
	}

	// Update scopes if provided
	if len(req.Scopes) > 0 {
		if err := h.appSvc.UpdateApp(appID, req.Scopes, updatedBy); err != nil {
			// ... existing error handling ...
		}
	}

	// Update TTL if provided
	if req.TokenTTL != nil {
		if err := h.appSvc.UpdateAppTTL(appID, *req.TokenTTL, updatedBy); err != nil {
			switch {
			case errors.Is(err, store.ErrAppNotFound):
				problemdetails.WriteProblem(r.Context(), w, http.StatusNotFound, "not_found", "app not found", r.URL.Path)
			case errors.Is(err, ErrInvalidTTL):
				problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_ttl", err.Error(), r.URL.Path)
			default:
				obs.Fail(hdlMod, hdlCmp, "update app TTL failed", "app_id="+appID, "err="+err.Error())
				problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to update app TTL", r.URL.Path)
			}
			return
		}
	}

	// Require at least one update field
	if len(req.Scopes) == 0 && req.TokenTTL == nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "at least one of scopes or token_ttl must be provided", r.URL.Path)
		return
	}

	// Return updated record
	// ... existing get-and-respond pattern ...
}
```

Update `storeAppToResp` to include TTL:

```go
func storeAppToResp(a store.AppRecord) appResp {
	return appResp{
		AppID:     a.AppID,
		Name:      a.Name,
		ClientID:  a.ClientID,
		Scopes:    a.ScopeCeiling,
		TokenTTL:  a.TokenTTL,
		Status:    a.Status,
		CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
```

**Step 4: Run all handler tests**

Run: `go test ./internal/app/... -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/app/app_hdl.go internal/app/app_hdl_test.go
git commit -m "feat: HTTP handler support for per-app token TTL"
```

---

## Task 6: Update aactl CLI — `--token-ttl` Flag

**Files:**
- Modify: `cmd/aactl/apps.go:29-32` (register flags)
- Modify: `cmd/aactl/apps.go:36-86` (appRegisterCmd)
- Modify: `cmd/aactl/apps.go:186-234` (appUpdateCmd)
- Modify: `cmd/aactl/apps.go:88-136` (appListCmd — add TTL column)
- Modify: `cmd/aactl/apps.go:138-184` (appGetCmd — add TTL row)
- Modify: `cmd/aactl/apps.go:279-290` (init — register flag)

**Step 1: Add the flag variables and update register command**

Add flag variable:
```go
var (
	appRegisterName     string
	appRegisterScopes   string
	appRegisterTokenTTL int
)
```

Update `appRegisterCmd.RunE` to include TTL in payload:

```go
payload := map[string]any{
	"name":   appRegisterName,
	"scopes": scopes,
}
if appRegisterTokenTTL > 0 {
	payload["token_ttl"] = appRegisterTokenTTL
}
```

Update register response struct to include TTL:

```go
var resp struct {
	AppID        string   `json:"app_id"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Scopes       []string `json:"scopes"`
	TokenTTL     int      `json:"token_ttl"`
}
```

Add TTL to table output:

```go
{"TOKEN_TTL", fmt.Sprintf("%ds", resp.TokenTTL)},
```

**Step 2: Update the update command to accept `--token-ttl`**

Add flag variable:
```go
var (
	appUpdateID       string
	appUpdateScopes   string
	appUpdateTokenTTL int
)
```

Update `appUpdateCmd.RunE` — scopes is now optional when TTL is provided:

```go
RunE: func(cmd *cobra.Command, args []string) error {
	if appUpdateID == "" {
		return fmt.Errorf("--id is required")
	}
	if appUpdateScopes == "" && appUpdateTokenTTL == 0 {
		return fmt.Errorf("at least one of --scopes or --token-ttl is required")
	}

	c, err := newClient()
	if err != nil {
		return err
	}

	payload := map[string]any{}
	if appUpdateScopes != "" {
		payload["scopes"] = strings.Split(appUpdateScopes, ",")
	}
	if appUpdateTokenTTL > 0 {
		payload["token_ttl"] = appUpdateTokenTTL
	}

	data, err := c.doPut("/v1/admin/apps/"+appUpdateID, payload)
	// ... rest of response handling, add TokenTTL to output
```

**Step 3: Update list and get output to show TTL**

In `appListCmd`, add TTL to the response struct and table columns:

```go
var resp struct {
	Apps []struct {
		AppID     string   `json:"app_id"`
		Name      string   `json:"name"`
		ClientID  string   `json:"client_id"`
		Scopes    []string `json:"scopes"`
		TokenTTL  int      `json:"token_ttl"`
		Status    string   `json:"status"`
		CreatedAt string   `json:"created_at"`
	} `json:"apps"`
	Total int `json:"total"`
}

// Add to rows:
rows[i] = []string{
	a.Name, a.AppID, a.ClientID, a.Status,
	strings.Join(a.Scopes, ","),
	fmt.Sprintf("%ds", a.TokenTTL),
	a.CreatedAt,
}
printTable([]string{"NAME", "APP_ID", "CLIENT_ID", "STATUS", "SCOPES", "TOKEN_TTL", "CREATED"}, rows)
```

In `appGetCmd`, add TTL row:

```go
{"TOKEN_TTL", fmt.Sprintf("%ds", resp.TokenTTL)},
```

**Step 4: Register flags in init()**

```go
appRegisterCmd.Flags().IntVar(&appRegisterTokenTTL, "token-ttl", 0, "app JWT TTL in seconds (default: global AA_APP_TOKEN_TTL)")
appUpdateCmd.Flags().IntVar(&appUpdateTokenTTL, "token-ttl", 0, "new app JWT TTL in seconds")
```

Make `--scopes` optional on update (remove the required check — it's now "at least one of"):

```go
// In appUpdateCmd, change from:
// if appUpdateScopes == "" { return error }
// To the check above: if scopes == "" && ttl == 0 { return error }
```

**Step 5: Commit**

```bash
git add cmd/aactl/apps.go
git commit -m "feat: aactl --token-ttl flag for app register and update"
```

---

## Task 7: Run Gate Checks

**Step 1: Run gates**

Run: `./scripts/gates.sh task`
Expected: BUILD OK, lint OK, unit OK

**Step 2: Fix any issues**

If lint or tests fail, fix and re-run.

**Step 3: Commit any fixes**

```bash
git add -A
git commit -m "fix: gate check fixes for TD-006"
```

---

## Task 8: Update CHANGELOG and Docs

**Files:**
- Modify: `CHANGELOG.md` (add TD-006 entry under Unreleased)
- Modify: `docker-compose.yml` (add `AA_APP_TOKEN_TTL` env var passthrough)

**Step 1: Add CHANGELOG entry**

Add under `## [Unreleased]`, before the Phase 1B section:

```markdown
### Fixed — TD-006: Per-App Configurable JWT TTL

**Summary:** App JWT TTL was hardcoded to 5 minutes (`const appTokenTTL = 300`).
Now configurable per-app with a global default of 30 minutes. Operators set the
global default via `AA_APP_TOKEN_TTL` and override per-app at registration
(`aactl app register --token-ttl`) or runtime (`aactl app update --token-ttl`).
Safety bounds: 60s minimum, 86400s (24h) maximum.

#### Modified: `internal/cfg/cfg.go`
- New `AppTokenTTL` field on `Cfg` struct, loaded from `AA_APP_TOKEN_TTL` (default 1800)

#### Modified: `internal/store/sql_store.go`
- `AppRecord.TokenTTL` field added
- `apps` table: `token_ttl INTEGER NOT NULL DEFAULT 1800` column
- Schema migration for existing databases (auto-detect and `ALTER TABLE`)
- `UpdateAppTTL()` method for updating per-app TTL
- All app queries updated to include `token_ttl`

#### Modified: `internal/app/app_svc.go`
- Removed `const appTokenTTL = 300`
- `RegisterApp()` accepts optional TTL parameter (0 = use global default)
- `AuthenticateApp()` uses per-app `rec.TokenTTL` instead of constant
- New `UpdateAppTTL()` method with bounds validation and audit trail
- New `ErrInvalidTTL` sentinel error

#### Modified: `internal/app/app_hdl.go`
- `registerAppReq` accepts optional `token_ttl` field
- `updateAppReq` accepts optional `token_ttl` field (pointer for absent vs zero)
- `appResp` includes `token_ttl` in all responses
- `handleUpdateApp` supports TTL-only updates (scopes no longer required)

#### Modified: `cmd/aactl/apps.go`
- `aactl app register --token-ttl N` — optional TTL at registration
- `aactl app update --id ID --token-ttl N` — update existing app TTL
- `aactl app list` — TTL column in table output
- `aactl app get` — TTL row in detail output
```

**Step 2: Update docker-compose.yml**

Add `AA_APP_TOKEN_TTL` to broker service environment:

```yaml
- AA_APP_TOKEN_TTL=${AA_APP_TOKEN_TTL:-1800}
```

**Step 3: Commit**

```bash
git add CHANGELOG.md docker-compose.yml
git commit -m "docs: CHANGELOG and docker-compose for TD-006"
```

---

## Task 9: Mark TD-006 as Resolved

**Files:**
- Modify: `TECH-DEBT.md`

**Step 1: Mark TD-006 resolved**

Change the TD-006 row to include RESOLVED status:

```
| TD-006 | **RESOLVED 2026-03-05** — ... |
```

**Step 2: Commit**

```bash
git add TECH-DEBT.md
git commit -m "docs: mark TD-006 as resolved"
```

---

## Summary

| Task | What | Files | Estimated |
|------|------|-------|-----------|
| 0 | Write user stories | `tests/td-006/user-stories.md` | 5 min |
| 1 | Config: `AA_APP_TOKEN_TTL` | `cfg.go`, `cfg_test.go` | 5 min |
| 2 | Store: `token_ttl` column + migration | `sql_store.go` | 10 min |
| 3 | Store: `UpdateAppTTL` method | `sql_store.go` | 5 min |
| 4 | Service: TTL at register + auth | `app_svc.go`, `app_svc_test.go`, `main.go` | 15 min |
| 5 | Handler: HTTP endpoint support | `app_hdl.go`, `app_hdl_test.go` | 10 min |
| 6 | CLI: `aactl --token-ttl` | `apps.go` | 10 min |
| 7 | Gate checks | — | 5 min |
| 8 | CHANGELOG + docker-compose | `CHANGELOG.md`, `docker-compose.yml` | 5 min |
| 9 | Mark TD-006 resolved | `TECH-DEBT.md` | 2 min |
