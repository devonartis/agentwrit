package handler

import (
	"bytes"
	"crypto/rand"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/token"
)

func testTokenSvc(t *testing.T) *token.TknSvc {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return token.NewTknSvc(priv, pub, cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300})
}

func issueTestToken(t *testing.T, svc *token.TknSvc) string {
	t.Helper()
	resp, err := svc.Issue(token.IssueReq{
		AgentID: "spiffe://agentauth.local/agent/orch/task/inst",
		OrchID:  "orch-1",
		TaskID:  "task-1",
		Scope:   []string{"read:Customers:12345"},
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return resp.AccessToken
}

func TestValHdlSuccessAndScopeMismatch(t *testing.T) {
	svc := testTokenSvc(t)
	tokenStr := issueTestToken(t, svc)
	hdl := NewValHdl(svc)

	okBody, _ := json.Marshal(map[string]string{
		"token":          tokenStr,
		"required_scope": "read:Customers:12345",
	})
	okReq := httptest.NewRequest(http.MethodPost, "/v1/token/validate", bytes.NewReader(okBody))
	okRec := httptest.NewRecorder()
	hdl.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", okRec.Code, okRec.Body.String())
	}

	denyBody, _ := json.Marshal(map[string]string{
		"token":          tokenStr,
		"required_scope": "write:Customers:12345",
	})
	denyReq := httptest.NewRequest(http.MethodPost, "/v1/token/validate", bytes.NewReader(denyBody))
	denyRec := httptest.NewRecorder()
	hdl.ServeHTTP(denyRec, denyReq)
	if denyRec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", denyRec.Code, denyRec.Body.String())
	}
}

func TestRenewHdl(t *testing.T) {
	svc := testTokenSvc(t)
	tokenStr := issueTestToken(t, svc)
	hdl := NewRenewHdl(svc)

	reqBody, _ := json.Marshal(map[string]string{"token": tokenStr})
	req := httptest.NewRequest(http.MethodPost, "/v1/token/renew", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}
