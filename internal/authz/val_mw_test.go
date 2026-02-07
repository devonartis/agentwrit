package authz

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divineartis/agentauth/internal/cfg"
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
	mw := NewValMw(svc)
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
	mw := NewValMw(svc)
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
	mw := NewValMw(svc)
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

