package authz

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/token"
)

func testSvc(t *testing.T) *token.TknSvc {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return token.NewTknSvc(priv, pub, cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300})
}

func issueToken(t *testing.T, svc *token.TknSvc, scopes []string) string {
	t.Helper()
	resp, err := svc.Issue(token.IssueReq{
		AgentID: "spiffe://agentauth.local/agent/orch/task/inst",
		OrchID:  "orch-1",
		TaskID:  "task-1",
		Scope:   scopes,
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return resp.AccessToken
}

func TestValMwAllowsMatchingScope(t *testing.T) {
	svc := testSvc(t)
	mw := NewValMw(svc, revoke.NewRevSvc())
	protected := WithRequiredScope("read:Customers:12345", mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if AgentIDFromContext(r.Context()) == "" {
			t.Fatalf("expected agent id in context")
		}
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/v1/protected/customers/12345", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, svc, []string{"read:Customers:*"}))
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestValMwDeniesMissingBearer(t *testing.T) {
	svc := testSvc(t)
	mw := NewValMw(svc, revoke.NewRevSvc())
	protected := WithRequiredScope("read:Customers:12345", mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	req := httptest.NewRequest(http.MethodGet, "/v1/protected/customers/12345", nil)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestValMwDeniesScopeMismatch(t *testing.T) {
	svc := testSvc(t)
	mw := NewValMw(svc, revoke.NewRevSvc())
	protected := WithRequiredScope("write:Customers:12345", mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/v1/protected/customers/12345", nil)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, svc, []string{"read:Customers:12345"}))
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestWrapRevokedTokenDenied(t *testing.T) {
	svc := testSvc(t)
	revSvc := revoke.NewRevSvc()
	mw := NewValMw(svc, revSvc)
	protected := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tok := issueToken(t, svc, []string{"read:Customers:12345"})

	// Token works before revocation.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 before revocation, got %d", rec.Code)
	}

	// Extract JTI from token claims for revocation.
	claims, err := svc.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	_ = revSvc.RevokeToken(claims.Jti, "test revocation")

	// Token denied after revocation.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Authorization", "Bearer "+tok)
	rec2 := httptest.NewRecorder()
	protected.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 after revocation, got %d", rec2.Code)
	}
}

