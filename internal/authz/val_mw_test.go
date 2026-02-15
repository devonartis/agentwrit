package authz

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/token"
)

type mockVerifier struct {
	claims *token.TknClaims
	err    error
}

func (m *mockVerifier) Verify(tokenStr string) (*token.TknClaims, error) {
	return m.claims, m.err
}

type mockRevChecker struct {
	revoked bool
}

func (m *mockRevChecker) IsRevoked(claims *token.TknClaims) bool {
	return m.revoked
}

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
}

func TestWrap_NilAuditLog_DoesNotPanic(t *testing.T) {
	mw := NewValMw(&mockVerifier{err: token.ErrInvalidToken}, nil, nil)

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	mw.Wrap(okHandler).ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestWrap_ValidToken_NoAuditEvent(t *testing.T) {
	al := audit.NewAuditLog()
	claims := &token.TknClaims{Sub: "agent-1", Scope: []string{"read:data:*"}}
	mw := NewValMw(&mockVerifier{claims: claims}, &mockRevChecker{revoked: false}, al)

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	mw.Wrap(okHandler).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(al.Events()) != 0 {
		t.Errorf("expected 0 audit events for valid request, got %d", len(al.Events()))
	}
}

func TestRequireScope_InsufficientScope_AuditsEvent(t *testing.T) {
	al := audit.NewAuditLog()
	claims := &token.TknClaims{
		Sub:    "spiffe://test/agent/o/t/a1",
		TaskId: "task-1",
		OrchId: "orch-1",
		Scope:  []string{"read:data:*"},
	}
	mw := NewValMw(&mockVerifier{claims: claims}, nil, al)
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
	if !strings.Contains(events[0].Detail, "write:data:*") {
		t.Errorf("expected detail to contain required scope, got %s", events[0].Detail)
	}
}

func TestRequireScope_SufficientScope_NoAuditEvent(t *testing.T) {
	al := audit.NewAuditLog()
	claims := &token.TknClaims{Sub: "agent-1", Scope: []string{"read:data:*", "write:data:*"}}
	mw := NewValMw(&mockVerifier{claims: claims}, nil, al)
	handler := mw.Wrap(mw.RequireScope("read:data:*", okHandler))

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(al.Events()) != 0 {
		t.Errorf("expected 0 audit events, got %d", len(al.Events()))
	}
}

func TestRequireScope_NilAuditLog_DoesNotPanic(t *testing.T) {
	claims := &token.TknClaims{Sub: "agent-1", Scope: []string{"read:data:*"}}
	mw := NewValMw(&mockVerifier{claims: claims}, nil, nil)
	handler := mw.Wrap(mw.RequireScope("write:data:*", okHandler))

	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
