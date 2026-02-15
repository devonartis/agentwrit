# Enforcement Layer & Audit Gap Fix — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire audit `Record()` calls into every middleware denial path so that all 401/403 responses produce audit events, convert `WithRequiredScope` to a method on `ValMw` for audit access, add audit recording to sidecar scope ceiling and delegation attenuation denials, and write interim developer enforcement docs.

**Architecture:** The `ValMw` struct already holds an `AuditRecorder` field (`auditLog`) that is never called. We add `Record()` calls at each denial point with nil-safety guards. We convert the standalone `WithRequiredScope` function into a `ValMw.RequireScope()` method so it can access `auditLog`. Route wiring in `cmd/broker/main.go` is updated to use the new method. Delegation scope violations get audit recording in `deleg_svc.go`. Sidecar scope ceiling violations get audit recording via `obs.Warn` detail enrichment (full broker audit reporting deferred to SDK module).

**Tech Stack:** Go 1.24, `net/http`, `internal/audit`, `internal/authz`, `internal/deleg`

**Design doc:** `docs/plans/2026-02-15-enforcement-audit-design.md` (approved)

---

## Task 1: Add New Audit Event Type Constants

**Files:**
- Modify: `internal/audit/audit_log.go:27-45`
- Test: `internal/audit/audit_log_test.go`

**Step 1: Write the failing test**

Add to `internal/audit/audit_log_test.go`:

```go
func TestNewEventTypeConstants_Exist(t *testing.T) {
	// Verify the new event type constants are defined and non-empty.
	constants := []string{
		audit.EventTokenAuthFailed,
		audit.EventTokenRevokedAccess,
		audit.EventScopeViolation,
		audit.EventScopeCeilingExceeded,
		audit.EventDelegationAttenuationViolation,
	}
	for _, c := range constants {
		if c == "" {
			t.Errorf("event type constant should not be empty")
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/... -run TestNewEventTypeConstants_Exist -v`
Expected: FAIL — `audit.EventTokenAuthFailed` (and others) undefined.

**Step 3: Write minimal implementation**

Add these constants to `internal/audit/audit_log.go` inside the existing `const` block, after line 44 (`EventSidecarExchangeDenied`):

```go
	EventTokenAuthFailed                = "token_auth_failed"
	EventTokenRevokedAccess             = "token_revoked_access"
	EventScopeViolation                 = "scope_violation"
	EventScopeCeilingExceeded           = "scope_ceiling_exceeded"
	EventDelegationAttenuationViolation = "delegation_attenuation_violation"
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/audit/... -run TestNewEventTypeConstants_Exist -v`
Expected: PASS

**Step 5: Run all existing tests to check for regressions**

Run: `go test ./internal/audit/... -v`
Expected: All existing tests PASS (no changes to behavior, only new constants).

**Step 6: Commit**

```bash
git add internal/audit/audit_log.go internal/audit/audit_log_test.go
git commit -m "feat(audit): add event type constants for auth failures and scope violations"
```

---

## Task 2: Wire Audit Recording Into `ValMw.Wrap()` Denial Paths

**Files:**
- Modify: `internal/authz/val_mw.go:63-91` (the `Wrap` method)
- Create: `internal/authz/val_mw_test.go`

**Context:** `ValMw.Wrap()` has four denial paths (lines 67, 72, 79, 84) that return HTTP errors but write nothing to the audit trail. We need to add `m.auditLog.Record()` at each one, guarded by a nil check.

**Step 1: Write the failing tests**

Create `internal/authz/val_mw_test.go`:

```go
package authz

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/token"
)

// mockVerifier is a test double for TokenVerifier.
type mockVerifier struct {
	claims *token.TknClaims
	err    error
}

func (m *mockVerifier) Verify(tokenStr string) (*token.TknClaims, error) {
	return m.claims, m.err
}

// mockRevChecker is a test double for RevocationChecker.
type mockRevChecker struct {
	revoked bool
}

func (m *mockRevChecker) IsRevoked(claims *token.TknClaims) bool {
	return m.revoked
}

// okHandler is a trivial handler that responds 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestWrap_MissingAuthHeader_AuditsEvent(t *testing.T) {
	al := audit.NewAuditLog()
	mw := NewValMw(&mockVerifier{}, nil, al)

	req := httptest.NewRequest("GET", "/test/path", nil)
	rec := httptest.NewRecorder()
	mw.Wrap(okHandler).ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	events := al.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].EventType != audit.EventTokenAuthFailed {
		t.Errorf("expected event_type=%s, got %s", audit.EventTokenAuthFailed, events[0].EventType)
	}
	if !strings.Contains(events[0].Detail, "/test/path") {
		t.Errorf("expected detail to contain request path, got %s", events[0].Detail)
	}
}

func TestWrap_InvalidScheme_AuditsEvent(t *testing.T) {
	al := audit.NewAuditLog()
	mw := NewValMw(&mockVerifier{}, nil, al)

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	mw.Wrap(okHandler).ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	events := al.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].EventType != audit.EventTokenAuthFailed {
		t.Errorf("expected event_type=%s, got %s", audit.EventTokenAuthFailed, events[0].EventType)
	}
}

func TestWrap_VerificationFailed_AuditsEvent(t *testing.T) {
	al := audit.NewAuditLog()
	mw := NewValMw(&mockVerifier{err: token.ErrInvalidToken}, nil, al)

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	mw.Wrap(okHandler).ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	events := al.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].EventType != audit.EventTokenAuthFailed {
		t.Errorf("expected event_type=%s, got %s", audit.EventTokenAuthFailed, events[0].EventType)
	}
	if !strings.Contains(events[0].Detail, "token verification failed") {
		t.Errorf("expected detail to contain error reason, got %s", events[0].Detail)
	}
}

func TestWrap_RevokedToken_AuditsEvent(t *testing.T) {
	al := audit.NewAuditLog()
	claims := &token.TknClaims{
		Sub:    "spiffe://test/agent/o/t/a1",
		TaskId: "task-1",
		OrchId: "orch-1",
		Scope:  []string{"read:data:*"},
	}
	mw := NewValMw(&mockVerifier{claims: claims}, &mockRevChecker{revoked: true}, al)

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer valid-but-revoked")
	rec := httptest.NewRecorder()
	mw.Wrap(okHandler).ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	events := al.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].EventType != audit.EventTokenRevokedAccess {
		t.Errorf("expected event_type=%s, got %s", audit.EventTokenRevokedAccess, events[0].EventType)
	}
	if events[0].AgentID != "spiffe://test/agent/o/t/a1" {
		t.Errorf("expected agent_id from claims, got %s", events[0].AgentID)
	}
	if events[0].TaskID != "task-1" {
		t.Errorf("expected task_id=task-1, got %s", events[0].TaskID)
	}
}

func TestWrap_NilAuditLog_DoesNotPanic(t *testing.T) {
	mw := NewValMw(&mockVerifier{err: token.ErrInvalidToken}, nil, nil)

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()

	// Should not panic even without an audit logger.
	mw.Wrap(okHandler).ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestWrap_ValidToken_NoAuditEvent(t *testing.T) {
	al := audit.NewAuditLog()
	claims := &token.TknClaims{
		Sub:   "agent-1",
		Scope: []string{"read:data:*"},
	}
	mw := NewValMw(&mockVerifier{claims: claims}, &mockRevChecker{revoked: false}, al)

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	mw.Wrap(okHandler).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	events := al.Events()
	if len(events) != 0 {
		t.Errorf("expected 0 audit events for valid request, got %d", len(events))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/authz/... -run "TestWrap_.*AuditsEvent" -v`
Expected: FAIL — the audit log will have 0 events (no `Record()` calls in `Wrap` yet).

**Step 3: Implement the audit recording**

Modify `internal/authz/val_mw.go`. Add the `audit` import and `Record()` calls on each denial path in `Wrap()`:

Add to imports:
```go
"github.com/divineartis/agentauth/internal/audit"
```

Replace the `Wrap` method body (lines 63-91) with:

```go
func (m *ValMw) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventTokenAuthFailed, "", "", "", "missing authorization header | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 401, "unauthorized", "missing authorization header", r.URL.Path)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventTokenAuthFailed, "", "", "", "invalid authorization scheme | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 401, "unauthorized", "invalid authorization scheme", r.URL.Path)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := m.tknSvc.Verify(tokenStr)
		if err != nil {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventTokenAuthFailed, "", "", "", "token verification failed: "+err.Error()+" | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 401, "unauthorized", "token verification failed: "+err.Error(), r.URL.Path)
			return
		}

		if m.revSvc != nil && m.revSvc.IsRevoked(claims) {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventTokenRevokedAccess, claims.Sub, claims.TaskId, claims.OrchId, "revoked token used | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 403, "insufficient_scope", "token has been revoked", r.URL.Path)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/authz/... -v`
Expected: All tests PASS (new tests + existing `scope_test.go` + `rate_mw_test.go`).

**Step 5: Commit**

```bash
git add internal/authz/val_mw.go internal/authz/val_mw_test.go
git commit -m "feat(authz): wire audit recording into ValMw.Wrap() denial paths"
```

---

## Task 3: Convert `WithRequiredScope` to `ValMw.RequireScope()` Method

**Files:**
- Modify: `internal/authz/val_mw.go:97-112`
- Modify: `internal/authz/val_mw_test.go` (add tests)

**Context:** `WithRequiredScope` is a standalone function with no access to `auditLog`. We convert it to a method on `ValMw` so it can record scope violations. The old function signature is:
```go
func WithRequiredScope(scope string, next http.Handler) http.Handler
```
The new signature is:
```go
func (m *ValMw) RequireScope(scope string, next http.Handler) http.Handler
```

**Step 1: Write the failing tests**

Add to `internal/authz/val_mw_test.go`:

```go
func TestRequireScope_InsufficientScope_AuditsEvent(t *testing.T) {
	al := audit.NewAuditLog()
	claims := &token.TknClaims{
		Sub:    "spiffe://test/agent/o/t/a1",
		TaskId: "task-1",
		OrchId: "orch-1",
		Scope:  []string{"read:data:*"},
	}
	mw := NewValMw(&mockVerifier{claims: claims}, nil, al)

	// Build a handler chain: Wrap (to set claims in context) -> RequireScope -> ok
	handler := mw.Wrap(mw.RequireScope("write:data:*", okHandler))

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	events := al.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].EventType != audit.EventScopeViolation {
		t.Errorf("expected event_type=%s, got %s", audit.EventScopeViolation, events[0].EventType)
	}
	if events[0].AgentID != "spiffe://test/agent/o/t/a1" {
		t.Errorf("expected agent_id from claims, got %s", events[0].AgentID)
	}
	if !strings.Contains(events[0].Detail, "write:data:*") {
		t.Errorf("expected detail to contain required scope, got %s", events[0].Detail)
	}
	if !strings.Contains(events[0].Detail, "read:data:*") {
		t.Errorf("expected detail to contain actual scope, got %s", events[0].Detail)
	}
}

func TestRequireScope_SufficientScope_NoAuditEvent(t *testing.T) {
	al := audit.NewAuditLog()
	claims := &token.TknClaims{
		Sub:   "agent-1",
		Scope: []string{"read:data:*", "write:data:*"},
	}
	mw := NewValMw(&mockVerifier{claims: claims}, nil, al)

	handler := mw.Wrap(mw.RequireScope("read:data:*", okHandler))

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	events := al.Events()
	if len(events) != 0 {
		t.Errorf("expected 0 audit events for allowed request, got %d", len(events))
	}
}

func TestRequireScope_NilAuditLog_DoesNotPanic(t *testing.T) {
	claims := &token.TknClaims{
		Sub:   "agent-1",
		Scope: []string{"read:data:*"},
	}
	mw := NewValMw(&mockVerifier{claims: claims}, nil, nil)

	handler := mw.Wrap(mw.RequireScope("write:data:*", okHandler))

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()

	// Should not panic even without an audit logger.
	handler.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireScope_NoClaims_Returns401(t *testing.T) {
	al := audit.NewAuditLog()
	mw := NewValMw(&mockVerifier{}, nil, al)

	// Call RequireScope directly WITHOUT Wrap (no claims in context).
	handler := mw.RequireScope("read:data:*", okHandler)

	req := httptest.NewRequest("GET", "/test/path", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/authz/... -run "TestRequireScope_" -v`
Expected: FAIL — `RequireScope` method does not exist on `ValMw`.

**Step 3: Implement `RequireScope` and keep backward compatibility**

In `internal/authz/val_mw.go`, replace the `WithRequiredScope` function (lines 97-112) with the new `RequireScope` method:

```go
// RequireScope returns a handler that checks that the authenticated
// token's scopes cover the given scope string. It must be used after
// [ValMw.Wrap] so that claims are present in the context. If the scope
// check fails it responds with a 403 RFC 7807 problem response and
// records a scope_violation audit event.
func (m *ValMw) RequireScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			problemdetails.WriteProblem(r.Context(), w, 401, "unauthorized", "no claims in context", r.URL.Path)
			return
		}

		if !ScopeIsSubset([]string{scope}, claims.Scope) {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventScopeViolation, claims.Sub, claims.TaskId, claims.OrchId,
					"scope_violation | required="+scope+" | actual="+strings.Join(claims.Scope, ",")+" | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 403, "insufficient_scope", "token lacks required scope: "+scope, r.URL.Path)
			return
		}

		next.ServeHTTP(w, r)
	})
}
```

Delete the old `WithRequiredScope` function entirely. Do NOT keep the old function — it will be replaced in `main.go` in the next task.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/authz/... -v`
Expected: PASS for all `val_mw_test.go` tests. `scope_test.go` and `rate_mw_test.go` should still pass.

**Step 5: Verify the build still compiles (it will fail)**

Run: `go build ./...`
Expected: FAIL — `cmd/broker/main.go` still references `authz.WithRequiredScope`. This is expected; we fix it in Task 4.

**Step 6: Commit (tests pass, build intentionally broken until Task 4)**

```bash
git add internal/authz/val_mw.go internal/authz/val_mw_test.go
git commit -m "feat(authz): convert WithRequiredScope to ValMw.RequireScope() with audit recording"
```

---

## Task 4: Update Route Wiring in `cmd/broker/main.go`

**Files:**
- Modify: `cmd/broker/main.go:113-121`

**Context:** Three routes reference `authz.WithRequiredScope(...)`. Replace each with `valMw.RequireScope(...)`. No new tests needed — existing integration tests and the build check suffice.

**Step 1: Update route wiring**

In `cmd/broker/main.go`, change these three lines:

Line 114 — token exchange:
```go
// Before:
mux.Handle("POST /v1/token/exchange",
    problemdetails.MaxBytesBody(valMw.Wrap(authz.WithRequiredScope("sidecar:manage:*", tokenExchangeHdl))))

// After:
mux.Handle("POST /v1/token/exchange",
    problemdetails.MaxBytesBody(valMw.Wrap(valMw.RequireScope("sidecar:manage:*", tokenExchangeHdl))))
```

Lines 118-119 — revoke:
```go
// Before:
mux.Handle("POST /v1/revoke",
    problemdetails.MaxBytesBody(valMw.Wrap(authz.WithRequiredScope("admin:revoke:*", revokeHdl))))

// After:
mux.Handle("POST /v1/revoke",
    problemdetails.MaxBytesBody(valMw.Wrap(valMw.RequireScope("admin:revoke:*", revokeHdl))))
```

Lines 120-121 — audit events:
```go
// Before:
mux.Handle("GET /v1/audit/events",
    valMw.Wrap(authz.WithRequiredScope("admin:audit:*", auditHdl)))

// After:
mux.Handle("GET /v1/audit/events",
    valMw.Wrap(valMw.RequireScope("admin:audit:*", auditHdl)))
```

**Step 2: Remove unused `authz.WithRequiredScope` import reference**

If the `authz` import in `main.go` is still used for `authz.NewValMw` (it is), keep the import. No changes needed.

**Step 3: Verify the build compiles**

Run: `go build ./...`
Expected: PASS — no more references to the deleted `WithRequiredScope` function.

**Step 4: Run all tests**

Run: `go test ./... -short`
Expected: All PASS.

**Step 5: Commit**

```bash
git add cmd/broker/main.go
git commit -m "refactor(broker): update route wiring to use ValMw.RequireScope()"
```

---

## Task 5: Add Audit Recording to Delegation Attenuation Denial

**Files:**
- Modify: `internal/deleg/deleg_svc.go:107-109`
- Modify: `internal/deleg/deleg_svc_test.go`

**Context:** When delegation scope attenuation fails (requested scope exceeds delegator scope), `DelegSvc.Delegate()` returns `ErrScopeViolation` at line 109 but records nothing to the audit trail. We need to add a `Record()` call before the error return.

**Step 1: Write the failing test**

Add to `internal/deleg/deleg_svc_test.go`:

```go
func TestDelegate_ScopeViolation_AuditsEvent(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tknSvc := token.NewTknSvc(priv, pub, cfg.Cfg{DefaultTTL: 300})
	st := store.NewSqlStore()
	al := audit.NewAuditLog()
	delegSvc := NewDelegSvc(tknSvc, st, al, priv)

	delegator := "spiffe://test/agent/o/t/delegator"
	delegate := "spiffe://test/agent/o/t/delegate"
	registerAgent(t, st, delegator)
	registerAgent(t, st, delegate)

	delegatorClaims := &token.TknClaims{
		Sub:    delegator,
		Scope:  []string{"read:data:*"},
		TaskId: "task-1",
		OrchId: "orch-1",
	}

	// Request scope wider than delegator — should fail + audit.
	_, err = delegSvc.Delegate(delegatorClaims, DelegReq{
		DelegateTo: delegate,
		Scope:      []string{"write:data:*"},
	})
	if !errors.Is(err, ErrScopeViolation) {
		t.Fatalf("expected ErrScopeViolation, got %v", err)
	}

	events := al.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].EventType != audit.EventDelegationAttenuationViolation {
		t.Errorf("expected event_type=%s, got %s", audit.EventDelegationAttenuationViolation, events[0].EventType)
	}
	if events[0].AgentID != delegator {
		t.Errorf("expected agent_id=%s, got %s", delegator, events[0].AgentID)
	}
}
```

You will also need to add `"errors"` and `"github.com/divineartis/agentauth/internal/audit"` and `"github.com/divineartis/agentauth/internal/cfg"` to the test file imports. Check the existing imports first — `cfg` should already be there.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/deleg/... -run TestDelegate_ScopeViolation_AuditsEvent -v`
Expected: FAIL — audit log has 0 events (no `Record()` call on scope violation path).

**Step 3: Implement the audit recording**

In `internal/deleg/deleg_svc.go`, modify the scope attenuation check (lines 107-109):

```go
	// Check scope attenuation — delegated scope MUST be subset of delegator's scope
	if !authz.ScopeIsSubset(req.Scope, delegatorClaims.Scope) {
		if s.auditLog != nil {
			s.auditLog.Record(audit.EventDelegationAttenuationViolation,
				delegatorClaims.Sub, delegatorClaims.TaskId, delegatorClaims.OrchId,
				fmt.Sprintf("delegation_attenuation_violation | delegator=%s | target=%s | requested=%v | allowed=%v",
					delegatorClaims.Sub, req.DelegateTo, req.Scope, delegatorClaims.Scope))
		}
		return nil, ErrScopeViolation
	}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/deleg/... -v`
Expected: All PASS (new test + existing delegation tests).

**Step 5: Commit**

```bash
git add internal/deleg/deleg_svc.go internal/deleg/deleg_svc_test.go
git commit -m "feat(deleg): add audit recording on delegation attenuation violation"
```

---

## Task 6: Add Audit Logging to Sidecar Scope Ceiling Denial

**Files:**
- Modify: `cmd/sidecar/handler.go:77-83`

**Context:** The sidecar scope ceiling check at line 78-82 calls `RecordScopeDenial()` (Prometheus metric) and `obs.Warn()` but does NOT record to the broker's audit trail. The full `POST /v1/audit/report` endpoint is deferred to the SDK module. For now, we enrich the `obs.Warn` log with structured fields matching the audit event format so operators can grep these out. This is explicitly interim — the design doc (Section 3.3) notes this requires the `POST /v1/audit/report` broker endpoint.

**Step 1: Enrich the sidecar scope ceiling denial log**

In `cmd/sidecar/handler.go`, modify lines 78-82. The current code is:

```go
	if !scopeIsSubset(req.Scope, h.scopeCeiling) {
		RecordScopeDenial()
		obs.Warn("SIDECAR", "TOKEN", "scope ceiling exceeded", "requested="+strings.Join(req.Scope, ","), "ceiling="+strings.Join(h.scopeCeiling, ","))
		writeError(w, http.StatusForbidden, "requested scope exceeds sidecar ceiling")
		return
	}
```

Replace with:

```go
	if !scopeIsSubset(req.Scope, h.scopeCeiling) {
		RecordScopeDenial()
		obs.Warn("SIDECAR", "TOKEN", "scope ceiling exceeded",
			"event_type=scope_ceiling_exceeded",
			"agent_name="+req.AgentName,
			"task_id="+req.TaskID,
			"requested="+strings.Join(req.Scope, ","),
			"ceiling="+strings.Join(h.scopeCeiling, ","))
		writeError(w, http.StatusForbidden, "requested scope exceeds sidecar ceiling")
		return
	}
```

**Step 2: Verify build compiles**

Run: `go build ./cmd/sidecar/...`
Expected: PASS

**Step 3: Run all tests**

Run: `go test ./... -short`
Expected: All PASS

**Step 4: Commit**

```bash
git add cmd/sidecar/handler.go
git commit -m "feat(sidecar): enrich scope ceiling denial log with structured audit fields"
```

---

## Task 7: Write Interim Developer Enforcement Docs

**Files:**
- Modify: `docs/getting-started-developer.md` (insert new section after "Using Your Token")

**Context:** Per the design doc Section 4, we add a section titled "Enforcing Scopes in Your Resource Server" that teaches developers the three-step pattern: validate token, check scope, act or deny. This goes between the "Using Your Token" section (ends at line 127) and the "Token Renewal" section (starts at line 130).

**Step 1: Insert the new section**

After line 128 (`---`) in `docs/getting-started-developer.md`, insert:

```markdown
## Enforcing Scopes in Your Resource Server

> **This is interim guidance.** When the AgentAuth SDK ships, it replaces these manual checks with a single function call. But the principle never changes: **validate first, check scope second, act third.** Never skip the scope check -- a valid token does not mean the agent is authorized for this specific action.

Every resource server endpoint that accepts agent tokens must do three things, in order:

1. **Validate the token** -- call `POST /v1/token/validate` on the broker
2. **Check the scope** -- verify the token's scopes cover the action
3. **Act or deny** -- if scope doesn't cover, return 403

### Python Example

```python
import os
import requests

BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "https://agentauth.internal.company.com")


def require_scope(request, required_scope):
    """Validate token and check scope. Call this in every endpoint handler."""
    token = request.headers.get("Authorization", "").removeprefix("Bearer ")
    if not token:
        raise HTTPException(401, "missing bearer token")

    # Step 1: Validate token
    resp = requests.post(f"{BROKER}/v1/token/validate", json={"token": token})
    result = resp.json()
    if not result["valid"]:
        raise HTTPException(403, f"invalid token: {result.get('error', 'unknown')}")

    # Step 2: Check scope
    claims = result["claims"]
    if not scope_covers(claims["scope"], required_scope):
        raise HTTPException(403,
            f"scope {claims['scope']} does not cover {required_scope}")

    return claims  # Pass to handler for audit/attribution


def scope_covers(allowed_scopes, required_scope):
    """Check if any allowed scope covers the required scope.
    Uses the same action:resource:identifier matching as the broker."""
    r_parts = required_scope.split(":")
    if len(r_parts) != 3:
        return False
    for allowed in allowed_scopes:
        a_parts = allowed.split(":")
        if len(a_parts) != 3:
            continue
        if a_parts[0] == r_parts[0] and a_parts[1] == r_parts[1]:
            if a_parts[2] == "*" or a_parts[2] == r_parts[2]:
                return True
    return False
```

### Go Example

```go
func requireScope(brokerURL, token, required string) (*Claims, error) {
    // Step 1: Validate
    resp, err := http.Post(brokerURL+"/v1/token/validate",
        "application/json", tokenBody(token))
    if err != nil {
        return nil, fmt.Errorf("validation request failed: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        Valid  bool   `json:"valid"`
        Error  string `json:"error,omitempty"`
        Claims Claims `json:"claims"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }
    if !result.Valid {
        return nil, fmt.Errorf("invalid token: %s", result.Error)
    }

    // Step 2: Check scope
    if !scopeCovers(result.Claims.Scope, required) {
        return nil, fmt.Errorf("scope %v does not cover %s", result.Claims.Scope, required)
    }

    return &result.Claims, nil
}
```

### TypeScript Example

```typescript
const BROKER = process.env.AGENTAUTH_BROKER_URL || "https://agentauth.internal.company.com";

async function requireScope(request: Request, requiredScope: string) {
  const token = request.headers.get("Authorization")?.replace("Bearer ", "");
  if (!token) throw new Error("missing bearer token");

  // Step 1: Validate token
  const resp = await fetch(`${BROKER}/v1/token/validate`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
  const result = await resp.json();
  if (!result.valid) throw new Error(`invalid token: ${result.error}`);

  // Step 2: Check scope
  const claims = result.claims;
  if (!scopeCovers(claims.scope, requiredScope)) {
    throw new Error(`scope ${claims.scope} does not cover ${requiredScope}`);
  }

  return claims;
}

function scopeCovers(allowed: string[], required: string): boolean {
  const [rAct, rRes, rId] = required.split(":");
  if (!rAct || !rRes || !rId) return false;
  return allowed.some((a) => {
    const [aAct, aRes, aId] = a.split(":");
    return aAct === rAct && aRes === rRes && (aId === "*" || aId === rId);
  });
}
```

---
```

**Step 2: Verify the document renders correctly**

Read back the file to spot-check formatting. No automated test needed for docs.

**Step 3: Commit**

```bash
git add docs/getting-started-developer.md
git commit -m "docs(developer): add interim scope enforcement guidance for resource servers"
```

---

## Task 8: Run Full Gate Check

**Files:** None (verification only)

**Step 1: Run the task gate**

Run: `./scripts/gates.sh task`
Expected: All gates PASS (build, lint, unit tests, doc checks, gitflow).

**Step 2: Run unit tests with verbose output**

Run: `go test ./... -short -v 2>&1 | tail -30`
Expected: All PASS, including the new `val_mw_test.go` and updated `deleg_svc_test.go`.

**Step 3: If any gate fails, fix the issue and re-run**

Common issues:
- Lint: unused imports, missing doc comments
- Build: stale references to `WithRequiredScope`
- Tests: import cycles or missing test helpers

**Step 4: Commit any gate-fix changes**

```bash
git add -A && git commit -m "fix: address gate check findings"
```

---

## Summary of Changes

| File | Change |
|------|--------|
| `internal/audit/audit_log.go` | 5 new event type constants |
| `internal/audit/audit_log_test.go` | 1 new test for constants |
| `internal/authz/val_mw.go` | `Record()` calls in `Wrap()` + new `RequireScope()` method, deleted `WithRequiredScope` |
| `internal/authz/val_mw_test.go` | New file: 10 tests covering all denial paths + nil safety |
| `cmd/broker/main.go` | 3 route wiring changes: `WithRequiredScope` → `valMw.RequireScope` |
| `internal/deleg/deleg_svc.go` | `Record()` call on scope attenuation violation |
| `internal/deleg/deleg_svc_test.go` | 1 new test for attenuation audit |
| `cmd/sidecar/handler.go` | Enriched scope ceiling denial log with structured fields |
| `docs/getting-started-developer.md` | New "Enforcing Scopes" section with Python/Go/TS examples |

**What this achieves:** After these changes, every 401 and 403 response from the broker produces an audit event with the agent's identity (when available). Delegation attenuation violations are also recorded. Developers have interim guidance on scope checking. The sidecar's scope ceiling denial logs are enriched for structured querying.

**What's deferred to the next module (SDK):**
- `POST /v1/audit/report` broker endpoint
- Python/Go enforcement SDK (`agentauth` package)
- FastAPI/Flask/net/http framework middleware
- Sidecar → broker audit trail reporting (requires the new endpoint)
