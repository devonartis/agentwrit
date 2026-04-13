// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devonartis/agentwrit/internal/audit"
	"github.com/devonartis/agentwrit/internal/authz"
	"github.com/devonartis/agentwrit/internal/revoke"
	"github.com/devonartis/agentwrit/internal/token"
)

// nopRevStoreInternal is a no-op RevocationStore for internal handler tests.
type nopRevStoreInternal struct{}

func (nopRevStoreInternal) SaveRevocation(_, _ string) error { return nil }

func TestReleaseHdl_ReleasesOwnToken(t *testing.T) {
	revSvc := revoke.NewRevSvc(nopRevStoreInternal{})
	auditLog := audit.NewAuditLog(nil)
	hdl := NewReleaseHdl(revSvc, auditLog)

	claims := &token.TknClaims{
		Jti: "jti-release-me", Sub: "agent-1",
		TaskId: "task-1", OrchId: "orch-1",
	}
	ctx := authz.ContextWithClaims(t.Context(), claims)
	req := httptest.NewRequest("POST", "/v1/token/release", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// Verify token is now revoked
	if !revSvc.IsRevoked(claims) {
		t.Fatal("token should be revoked after release")
	}
}

func TestReleaseHdl_DoubleReleaseIdempotent(t *testing.T) {
	revSvc := revoke.NewRevSvc(nopRevStoreInternal{})
	auditLog := audit.NewAuditLog(nil)
	hdl := NewReleaseHdl(revSvc, auditLog)

	claims := &token.TknClaims{
		Jti: "jti-double", Sub: "agent-1",
	}
	ctx := authz.ContextWithClaims(t.Context(), claims)

	// First release
	req1 := httptest.NewRequest("POST", "/v1/token/release", nil).WithContext(ctx)
	rec1 := httptest.NewRecorder()
	hdl.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusNoContent {
		t.Fatalf("first release: expected 204, got %d", rec1.Code)
	}

	// Second release — should also succeed
	req2 := httptest.NewRequest("POST", "/v1/token/release", nil).WithContext(ctx)
	rec2 := httptest.NewRecorder()
	hdl.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("second release: expected 204, got %d", rec2.Code)
	}
}

func TestReleaseHdl_NoClaims401(t *testing.T) {
	revSvc := revoke.NewRevSvc(nopRevStoreInternal{})
	hdl := NewReleaseHdl(revSvc, nil)

	req := httptest.NewRequest("POST", "/v1/token/release", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestReleaseHdl_AuditEventRecorded(t *testing.T) {
	revSvc := revoke.NewRevSvc(nopRevStoreInternal{})
	auditLog := audit.NewAuditLog(nil)
	hdl := NewReleaseHdl(revSvc, auditLog)

	claims := &token.TknClaims{
		Jti: "jti-audit-check", Sub: "agent-2",
		TaskId: "task-2", OrchId: "orch-2",
	}
	ctx := authz.ContextWithClaims(t.Context(), claims)
	req := httptest.NewRequest("POST", "/v1/token/release", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// Verify audit event
	events := auditLog.Events()
	found := false
	for _, e := range events {
		if e.EventType == audit.EventTokenReleased && e.AgentID == "agent-2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected token_released audit event for agent-2")
	}
}
