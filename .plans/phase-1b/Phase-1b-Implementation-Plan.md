# Phase 1b: App-Scoped Launch Tokens — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let apps create their own launch tokens within their scope ceiling, and trace agents back to the originating app.

**Architecture:** Extend the existing launch token endpoint to accept app JWTs alongside admin JWTs. When an app calls the endpoint, enforce that requested scopes are a subset of the app's scope ceiling. Thread `app_id` through launch token → agent record for traceability.

**Tech Stack:** Go, SQLite, Ed25519 JWTs, `internal/authz.ScopeIsSubset` for ceiling enforcement.

**Depends on:** Phase 1a (merged to develop at commit 6130ba5).

**Phase 1a open items carried forward:**
- TD-001: `app_rate_limited` audit event not emitted (fix before Phase 1C, not blocking)
- Story 1: `sk_live_` prefix missing from client_secret (decision pending)

---

## Design Decisions

### D1: One endpoint, two callers — not a separate `/v1/app/launch-tokens` route

The spec says "apps use the existing launch token endpoint, just with different auth." The existing `POST /v1/admin/launch-tokens` handler already extracts claims from context — we change the `RequireScope` check to accept EITHER `admin:launch-tokens:*` OR `app:launch-tokens:*`. This avoids duplicating the handler.

**Implementation:** Replace `RequireScope("admin:launch-tokens:*", ...)` with `RequireAnyScope(["admin:launch-tokens:*", "app:launch-tokens:*"], ...)` — a new middleware method on `ValMw`.

### D2: Scope ceiling enforcement happens in the handler, not the service

The handler has access to the JWT claims (which contain the `sub` field like `app:app-weather-bot-b4065c`). It can extract the `app_id`, look up the `AppRecord`, and enforce the ceiling before calling `AdminSvc.CreateLaunchToken`. This keeps the service layer pure (no store lookups for auth context).

### D3: App JWT scopes stay hard-coded — ceiling is enforced at use-time

Phase 1a issues all app JWTs with `["app:launch-tokens:*", "app:agents:*", "app:audit:read"]` regardless of the app's ceiling. This is correct — those are API-level permissions. The ceiling is a separate concept enforced when the app *uses* those permissions (i.e., when creating a launch token). We do NOT change `AuthenticateApp`.

### D4: `app_id` field is optional (empty string for admin-created tokens/agents)

Admin-created launch tokens and agents predate the app model. Their `AppID` field is `""`. All queries and logic must handle this gracefully. No migration needed for in-memory records (they reset on restart).

---

## Task Breakdown

### Task 1: Add `RequireAnyScope` middleware method

**Files:**
- Modify: `internal/authz/val_mw.go`
- Test: `internal/authz/val_mw_test.go`

**Step 1: Write the failing test**

In `val_mw_test.go`, add a test that verifies `RequireAnyScope` passes when the token has at least one of the listed scopes, and fails when it has none.

```go
func TestRequireAnyScope_PassesWhenTokenHasOneOfListedScopes(t *testing.T) {
	t.Helper()
	mw := newTestValMw(t) // existing helper

	// Token with app:launch-tokens:* should pass when either admin or app scope accepted
	claims := &token.TknClaims{
		Iss:   "agentauth",
		Sub:   "app:app-weather-bot-abc123",
		Scope: []string{"app:launch-tokens:*", "app:agents:*", "app:audit:read"},
		Exp:   time.Now().Add(5 * time.Minute).Unix(),
		Nbf:   time.Now().Unix(),
		Iat:   time.Now().Unix(),
		Jti:   "test-jti-001",
	}

	handler := mw.RequireAnyScope(
		[]string{"admin:launch-tokens:*", "app:launch-tokens:*"},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("POST", "/test", nil)
	req = req.WithContext(ContextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestRequireAnyScope_RejectsWhenTokenHasNone(t *testing.T) {
	mw := newTestValMw(t)

	claims := &token.TknClaims{
		Iss:   "agentauth",
		Sub:   "app:app-weather-bot-abc123",
		Scope: []string{"app:agents:*", "app:audit:read"}, // no launch-tokens scope
		Exp:   time.Now().Add(5 * time.Minute).Unix(),
		Nbf:   time.Now().Unix(),
		Iat:   time.Now().Unix(),
		Jti:   "test-jti-002",
	}

	handler := mw.RequireAnyScope(
		[]string{"admin:launch-tokens:*", "app:launch-tokens:*"},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("POST", "/test", nil)
	req = req.WithContext(ContextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/authz/ -run TestRequireAnyScope -v`
Expected: FAIL — `RequireAnyScope` method does not exist.

**Step 3: Implement `RequireAnyScope`**

In `val_mw.go`, add alongside existing `RequireScope`:

```go
// RequireAnyScope returns middleware that passes if the token carries at least
// one of the listed scopes. Used when multiple caller types (admin, app) share
// an endpoint.
func (m *ValMw) RequireAnyScope(scopes []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			problemdetails.Write(w, http.StatusUnauthorized, "missing claims", r.URL.Path)
			return
		}

		for _, scope := range scopes {
			if ScopeIsSubset([]string{scope}, claims.Scope) {
				next.ServeHTTP(w, r)
				return
			}
		}

		if m.auditLog != nil {
			m.auditLog.Record(audit.EventScopeViolation, claims.Sub, "", "",
				fmt.Sprintf("token lacks any of required scopes: %v", scopes),
				audit.WithOutcome("denied"))
		}
		problemdetails.Write(w, http.StatusForbidden,
			fmt.Sprintf("insufficient_scope: requires one of %v", scopes), r.URL.Path)
	})
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/authz/ -run TestRequireAnyScope -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/authz/val_mw.go internal/authz/val_mw_test.go
git commit -m "feat(authz): add RequireAnyScope middleware for multi-caller endpoints"
```

---

### Task 2: Add `AppID` field to `LaunchTokenRecord`

**Files:**
- Modify: `internal/store/sql_store.go` (struct definition + `SaveLaunchToken`)
- Test: `internal/store/sql_store_test.go`

**Step 1: Write the failing test**

```go
func TestSaveLaunchToken_PreservesAppID(t *testing.T) {
	s := NewSqlStore()
	rec := LaunchTokenRecord{
		Token:        "abc123",
		AgentName:    "test-agent",
		AllowedScope: []string{"read:data:*"},
		MaxTTL:       300,
		SingleUse:    true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Second),
		CreatedBy:    "app:app-weather-bot-abc123",
		AppID:        "app-weather-bot-abc123",
	}
	if err := s.SaveLaunchToken(rec); err != nil {
		t.Fatalf("SaveLaunchToken: %v", err)
	}
	got, err := s.GetLaunchToken("abc123")
	if err != nil {
		t.Fatalf("GetLaunchToken: %v", err)
	}
	if got.AppID != "app-weather-bot-abc123" {
		t.Errorf("AppID = %q, want %q", got.AppID, "app-weather-bot-abc123")
	}
}

func TestSaveLaunchToken_AdminHasEmptyAppID(t *testing.T) {
	s := NewSqlStore()
	rec := LaunchTokenRecord{
		Token:        "def456",
		AgentName:    "admin-agent",
		AllowedScope: []string{"read:data:*"},
		MaxTTL:       300,
		SingleUse:    true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Second),
		CreatedBy:    "admin",
	}
	if err := s.SaveLaunchToken(rec); err != nil {
		t.Fatalf("SaveLaunchToken: %v", err)
	}
	got, err := s.GetLaunchToken("def456")
	if err != nil {
		t.Fatalf("GetLaunchToken: %v", err)
	}
	if got.AppID != "" {
		t.Errorf("AppID = %q, want empty string", got.AppID)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestSaveLaunchToken_PreservesAppID -v`
Expected: FAIL — `AppID` field does not exist on `LaunchTokenRecord`.

**Step 3: Add `AppID` field**

In `sql_store.go`, add to `LaunchTokenRecord`:

```go
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
	AppID        string // empty for admin-created tokens
}
```

No changes to `SaveLaunchToken` or `GetLaunchToken` needed — the in-memory map stores the full struct by pointer.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestSaveLaunchToken -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/sql_store.go internal/store/sql_store_test.go
git commit -m "feat(store): add AppID field to LaunchTokenRecord"
```

---

### Task 3: Add `AppID` field to `AgentRecord`

**Files:**
- Modify: `internal/store/sql_store.go` (struct definition)
- Test: `internal/store/sql_store_test.go`

**Step 1: Write the failing test**

```go
func TestSaveAgent_PreservesAppID(t *testing.T) {
	s := NewSqlStore()
	rec := AgentRecord{
		AgentID:      "spiffe://example/agent/orch1/task1/inst1",
		PublicKey:    make([]byte, 32),
		OrchID:       "orch1",
		TaskID:       "task1",
		Scope:        []string{"read:data:*"},
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
		AppID:        "app-weather-bot-abc123",
	}
	if err := s.SaveAgent(rec); err != nil {
		t.Fatalf("SaveAgent: %v", err)
	}
	got, err := s.GetAgent(rec.AgentID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.AppID != "app-weather-bot-abc123" {
		t.Errorf("AppID = %q, want %q", got.AppID, "app-weather-bot-abc123")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestSaveAgent_PreservesAppID -v`
Expected: FAIL — `AppID` field does not exist on `AgentRecord`.

**Step 3: Add `AppID` field**

In `sql_store.go`, modify `AgentRecord`:

```go
type AgentRecord struct {
	AgentID      string
	PublicKey    []byte
	OrchID       string
	TaskID       string
	Scope        []string
	RegisteredAt time.Time
	LastSeen     time.Time
	AppID        string // empty for agents not created via an app
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestSaveAgent_PreservesAppID -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/sql_store.go internal/store/sql_store_test.go
git commit -m "feat(store): add AppID field to AgentRecord"
```

---

### Task 4: Wire app scope ceiling enforcement in the launch token handler

**Files:**
- Modify: `internal/admin/admin_hdl.go` (route registration + handler logic)
- Modify: `internal/admin/admin_svc.go` (`CreateLaunchToken` signature — add `appID` param)
- Dependency: `internal/store/sql_store.go` (`GetAppByID` already exists)
- Test: `internal/admin/admin_hdl_test.go`

This is the core task. The handler must:
1. Accept both admin and app JWTs (using `RequireAnyScope`)
2. If the caller is an app (`sub` starts with `"app:"`): look up the `AppRecord`, enforce scope ceiling, and set `AppID` on the launch token
3. If the caller is admin: skip ceiling check, no `AppID`

**Step 1: Write the failing tests**

Add to `admin_hdl_test.go` (or create a new test file `admin_hdl_app_launch_test.go`):

```go
func TestCreateLaunchToken_AppCallerWithinCeiling(t *testing.T) {
	// Setup: register an app with scope ceiling ["read:weather:*"]
	// Authenticate as that app to get an app JWT
	// Call POST /v1/admin/launch-tokens with allowed_scope: ["read:weather:current"]
	// Expect: 201 Created
}

func TestCreateLaunchToken_AppCallerExceedsCeiling(t *testing.T) {
	// Setup: register an app with scope ceiling ["read:weather:*"]
	// Authenticate as that app
	// Call POST /v1/admin/launch-tokens with allowed_scope: ["write:data:all"]
	// Expect: 403 Forbidden with clear error about ceiling violation
}

func TestCreateLaunchToken_AppCallerTokenCarriesAppID(t *testing.T) {
	// Setup: register app, authenticate, create launch token within ceiling
	// Retrieve the launch token record from store
	// Expect: AppID matches the app's app_id
}

func TestCreateLaunchToken_AdminCallerNoCeilingCheck(t *testing.T) {
	// Setup: authenticate as admin
	// Call POST /v1/admin/launch-tokens with any scope
	// Expect: 201 Created, no ceiling enforcement
	// Expect: AppID is empty on the launch token record
}

func TestCreateLaunchToken_AdminCallerStillWorks(t *testing.T) {
	// Regression: existing admin flow unchanged
	// Authenticate as admin, create launch token
	// Expect: 201 Created (backward compatible)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/admin/ -run TestCreateLaunchToken_App -v`
Expected: FAIL — app JWT is rejected by current `RequireScope("admin:launch-tokens:*", ...)`

**Step 3: Implement the changes**

**3a. Update route registration in `admin_hdl.go`:**

Change from:
```go
mux.Handle("POST /v1/admin/launch-tokens",
    h.valMw.Wrap(h.valMw.RequireScope("admin:launch-tokens:*",
        http.HandlerFunc(h.handleCreateLaunchToken))))
```

To:
```go
mux.Handle("POST /v1/admin/launch-tokens",
    h.valMw.Wrap(h.valMw.RequireAnyScope(
        []string{"admin:launch-tokens:*", "app:launch-tokens:*"},
        http.HandlerFunc(h.handleCreateLaunchToken))))
```

**3b. Add store field to `AdminHdl`:**

The handler needs access to the store to look up `AppRecord.ScopeCeiling`. Add a `store` field:

```go
type AdminHdl struct {
    adminSvc    *AdminSvc
    valMw       *authz.ValMw
    revSvc      *revoke.RevSvc
    auditLog    *audit.AuditLog
    rateLimiter *authz.RateLimiter
    store       *store.SqlStore // added for app ceiling lookups
}
```

Update `NewAdminHdl` to accept the store parameter.

**3c. Update `handleCreateLaunchToken` to enforce ceiling:**

```go
func (h *AdminHdl) handleCreateLaunchToken(w http.ResponseWriter, r *http.Request) {
    claims := authz.ClaimsFromContext(r.Context())
    if claims == nil {
        problemdetails.Write(w, http.StatusUnauthorized, "missing credentials", r.URL.Path)
        return
    }

    r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
    var req CreateLaunchTokenReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        problemdetails.Write(w, http.StatusBadRequest, "invalid request body", r.URL.Path)
        return
    }

    var appID string

    // If caller is an app, enforce scope ceiling
    if strings.HasPrefix(claims.Sub, "app:") {
        appID = strings.TrimPrefix(claims.Sub, "app:")
        appRec, err := h.store.GetAppByID(appID)
        if err != nil {
            problemdetails.Write(w, http.StatusForbidden, "app not found", r.URL.Path)
            return
        }
        if !authz.ScopeIsSubset(req.AllowedScope, appRec.ScopeCeiling) {
            h.auditLog.Record(audit.EventScopeCeilingExceeded, claims.Sub, "", "",
                fmt.Sprintf("app=%s requested=%v ceiling=%v", appID, req.AllowedScope, appRec.ScopeCeiling),
                audit.WithOutcome("denied"))
            problemdetails.Write(w, http.StatusForbidden,
                fmt.Sprintf("requested scopes exceed app ceiling; allowed: %v", appRec.ScopeCeiling),
                r.URL.Path)
            return
        }
    }

    resp, err := h.adminSvc.CreateLaunchToken(req, claims.Sub, appID)
    if err != nil {
        // existing error handling...
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(resp)
}
```

**3d. Update `CreateLaunchToken` in `admin_svc.go` to accept `appID`:**

Change signature from:
```go
func (s *AdminSvc) CreateLaunchToken(req CreateLaunchTokenReq, createdBy string) (*CreateLaunchTokenResp, error)
```

To:
```go
func (s *AdminSvc) CreateLaunchToken(req CreateLaunchTokenReq, createdBy, appID string) (*CreateLaunchTokenResp, error)
```

And set `AppID` on the record:
```go
rec := &store.LaunchTokenRecord{
    Token:        tokenStr,
    AgentName:    req.AgentName,
    AllowedScope: req.AllowedScope,
    MaxTTL:       maxTTL,
    SingleUse:    singleUse,
    CreatedAt:    now,
    ExpiresAt:    expiresAt,
    CreatedBy:    createdBy,
    AppID:        appID,
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/admin/ -v`
Expected: ALL PASS

**Step 5: Run full test suite to check for regressions**

Run: `go test ./...`
Expected: ALL PASS (the `CreateLaunchToken` call sites in existing code need the new `""` appID param)

**Step 6: Commit**

```bash
git add internal/admin/admin_hdl.go internal/admin/admin_svc.go internal/admin/admin_hdl_test.go
git commit -m "feat(admin): apps can create launch tokens within scope ceiling"
```

---

### Task 5: Flow `AppID` from launch token to agent record during registration

**Files:**
- Modify: `internal/identity/id_svc.go` (pass `ltRec.AppID` to `SaveAgent`)
- Test: `internal/identity/id_svc_test.go`

**Step 1: Write the failing test**

```go
func TestRegister_AgentInheritsAppIDFromLaunchToken(t *testing.T) {
	// Setup: create a launch token with AppID = "app-weather-bot-abc123"
	// Register an agent using that launch token
	// Retrieve agent record from store
	// Expect: agent.AppID == "app-weather-bot-abc123"
}

func TestRegister_AdminLaunchTokenAgentHasNoAppID(t *testing.T) {
	// Setup: create a launch token with AppID = "" (admin-created)
	// Register an agent
	// Expect: agent.AppID == ""
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/ -run TestRegister_AgentInherits -v`
Expected: FAIL — `AgentRecord` doesn't have `AppID` set during registration.

**Step 3: Implement**

In `id_svc.go`, update the `SaveAgent` call (around line 222):

```go
s.store.SaveAgent(store.AgentRecord{
    AgentID:      agentID,
    PublicKey:    pubKeyBytes,
    OrchID:       req.OrchID,
    TaskID:       req.TaskID,
    Scope:        req.RequestedScope,
    RegisteredAt: now,
    LastSeen:     now,
    AppID:        ltRec.AppID, // inherit from launch token
})
```

**Step 4: Run tests**

Run: `go test ./internal/identity/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/identity/id_svc.go internal/identity/id_svc_test.go
git commit -m "feat(identity): agent inherits AppID from launch token on registration"
```

---

### Task 6: Add `app_id` to audit events for app-triggered operations

**Files:**
- Modify: `internal/admin/admin_hdl.go` (launch token audit events include app_id)
- Modify: `internal/identity/id_svc.go` (agent registration audit events include app_id)
- Test: existing audit assertions in handler/identity tests

**Step 1: Write the failing test**

```go
func TestCreateLaunchToken_AuditIncludesAppID(t *testing.T) {
	// Setup: app creates launch token
	// Check audit events for EventLaunchTokenIssued
	// Expect: detail contains "app_id=app-weather-bot-abc123"
}

func TestRegister_AuditIncludesAppID(t *testing.T) {
	// Setup: register agent via app-created launch token
	// Check audit events for EventAgentRegistered
	// Expect: detail contains "app_id=app-weather-bot-abc123"
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL — no `app_id` in audit detail strings.

**Step 3: Implement**

In `admin_svc.go` `CreateLaunchToken`, add `app_id` to the audit detail string when appID is non-empty:

```go
detail := fmt.Sprintf("agent=%s scope=%v max_ttl=%d created_by=%s", req.AgentName, req.AllowedScope, maxTTL, createdBy)
if appID != "" {
    detail += fmt.Sprintf(" app_id=%s", appID)
}
s.record(audit.EventLaunchTokenIssued, "", detail, audit.WithOutcome("success"))
```

In `id_svc.go` `Register`, add `app_id` to agent_registered detail when present:

```go
detail := fmt.Sprintf("agent_id=%s orch_id=%s task_id=%s", agentID, req.OrchID, req.TaskID)
if ltRec.AppID != "" {
    detail += fmt.Sprintf(" app_id=%s", ltRec.AppID)
}
s.auditLog.Record(audit.EventAgentRegistered, agentID, req.TaskID, req.OrchID, detail, audit.WithOutcome("success"))
```

**Step 4: Run tests**

Run: `go test ./internal/admin/ ./internal/identity/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/admin/admin_svc.go internal/identity/id_svc.go
git commit -m "feat(audit): include app_id in launch token and agent registration events"
```

---

### Task 7: Update `cmd/broker/main.go` wiring

**Files:**
- Modify: `cmd/broker/main.go` (pass store to `NewAdminHdl`)

**Step 1: Update the constructor call**

Change from:
```go
adminHdl := admin.NewAdminHdl(adminSvc, valMw, auditLog, revSvc)
```

To:
```go
adminHdl := admin.NewAdminHdl(adminSvc, valMw, auditLog, revSvc, sqlStore)
```

**Step 2: Verify compilation**

Run: `go build ./cmd/broker/`
Expected: SUCCESS

**Step 3: Run full test suite**

Run: `go test ./...`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add cmd/broker/main.go
git commit -m "wire: pass store to AdminHdl for app ceiling lookups"
```

---

### Task 8: Write user stories and run `gates.sh`

**Files:**
- Create: `tests/phase-1b/user-stories.md`
- Create: `tests/phase-1b/env.sh`

**Step 1: Extract user stories from the Phase 1b spec**

Write `tests/phase-1b/user-stories.md` with stories from `.plans/phase-1b/Phase-1b-App-Scoped-Launch-Tokens.md` section "User Stories" plus Phase 1a regression stories carried forward.

**Step 2: Create env.sh**

```bash
#!/usr/bin/env bash
# Phase 1b test environment — source this once before running live test stories.
export AACTL_BROKER_URL=http://127.0.0.1:8080
export AACTL_ADMIN_SECRET=change-me-in-production
echo "Phase 1b env loaded. Broker: $AACTL_BROKER_URL"
```

**Step 3: Run gates.sh**

Run: `./scripts/gates.sh task`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add tests/phase-1b/
git commit -m "test: add Phase 1b user stories and test env"
```

---

### Task 9: Docker live test

**Prerequisite:** All unit tests pass, gates.sh passes.

**Step 1: Build aactl**

```bash
go build -o ./bin/aactl ./cmd/aactl
```

**Step 2: Start Docker stack**

```bash
./scripts/stack_up.sh
```

**Step 3: Verify health**

```bash
curl http://127.0.0.1:8080/v1/health
```

**Step 4: Run each user story against the live stack**

Execute each story from `tests/phase-1b/user-stories.md`:
1. Register app with ceiling → create launch token within ceiling → verify success
2. Create launch token exceeding ceiling → verify 403
3. Register agent using app-created launch token → verify app_id in agent record
4. Admin creates launch token → verify no regression
5. Check audit trail for app_id attribution

**Step 5: Save evidence**

Create `tests/phase-1b/evidence/` with per-story evidence files and README verdict table.

**Step 6: Tear down**

```bash
docker compose down -v
```

**Step 7: Commit evidence**

```bash
git add tests/phase-1b/evidence/
git commit -m "test: Phase 1b Docker live test evidence"
```

---

## Batch Execution Order

**Batch 1 (foundation):** Tasks 1, 2, 3 — independent, can be done in parallel
**Batch 2 (core logic):** Task 4 — depends on Tasks 1-3
**Batch 3 (propagation):** Tasks 5, 6 — depends on Task 4
**Batch 4 (wiring + tests):** Tasks 7, 8 — depends on all above
**Batch 5 (live test):** Task 9 — depends on all above

---

## Phase 1a Regression Stories to Carry Forward

From `tests/phase-1a/evidence/README.md`, verify these still pass during Phase 1b live testing:
- Story 6: Developer authenticates with client_id + client_secret → gets JWT
- Story 8: App JWT cannot access admin endpoints (403)
- Story 11: Admin auth, audit flows unchanged
- NEW: Admin launch token creation still works (no AppID, no ceiling check)
