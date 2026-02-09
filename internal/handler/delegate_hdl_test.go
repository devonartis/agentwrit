package handler

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/deleg"
	"github.com/divineartis/agentauth/internal/token"
)

// delegTestKit holds common test dependencies for delegate handler tests.
type delegTestKit struct {
	tknSvc   *token.TknSvc
	delegSvc *deleg.DelegSvc
	hdl      *DelegHdl
}

func newDelegTestKit() *delegTestKit {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	c := cfg.Cfg{TrustDomain: "test.local", DefaultTTL: 300}
	tknSvc := token.NewTknSvc(priv, pub, c)
	delegSvc := deleg.NewDelegSvc(tknSvc, priv, 3)
	return &delegTestKit{
		tknSvc:   tknSvc,
		delegSvc: delegSvc,
		hdl:      NewDelegHdl(delegSvc, nil),
	}
}

func (k *delegTestKit) issueToken(scope []string) string {
	resp, _ := k.tknSvc.Issue(token.IssueReq{
		AgentID:   "spiffe://test.local/agent/orch/task/agentA",
		OrchID:    "orch",
		TaskID:    "task",
		Scope:     scope,
		TTLSecond: 300,
	})
	return resp.AccessToken
}

func TestDelegHdl_Success(t *testing.T) {
	k := newDelegTestKit()
	tkn := k.issueToken([]string{"read:Customers:*"})

	body, _ := json.Marshal(deleg.DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/delegate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	k.hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp deleg.DelegResp
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.DelegationToken == "" {
		t.Error("delegation_token should not be empty")
	}
	if resp.ChainHash == "" {
		t.Error("chain_hash should not be empty")
	}
	if resp.DelegationDepth != 1 {
		t.Errorf("depth = %d, want 1", resp.DelegationDepth)
	}
}

func TestDelegHdl_InvalidToken(t *testing.T) {
	k := newDelegTestKit()
	body, _ := json.Marshal(deleg.DelegReq{
		DelegatorToken: "bad.token.value",
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/delegate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	k.hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["type"] != "urn:agentauth:error:invalid-token" {
		t.Errorf("unexpected error type: %v", resp["type"])
	}
}

func TestDelegHdl_ScopeEscalation(t *testing.T) {
	k := newDelegTestKit()
	tkn := k.issueToken([]string{"read:Customers:12345"})

	body, _ := json.Marshal(deleg.DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:*"},
		MaxTTL:         60,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/delegate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	k.hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["type"] != "urn:agentauth:error:scope-escalation" {
		t.Errorf("unexpected error type: %v", resp["type"])
	}
}

func TestDelegHdl_DepthExceeded(t *testing.T) {
	// Use maxDepth=0 to trigger immediately.
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	c := cfg.Cfg{TrustDomain: "test.local", DefaultTTL: 300}
	tknSvc := token.NewTknSvc(priv, pub, c)
	delegSvc := deleg.NewDelegSvc(tknSvc, priv, 0)
	hdl := NewDelegHdl(delegSvc, nil)

	resp, _ := tknSvc.Issue(token.IssueReq{
		AgentID: "spiffe://test.local/agent/orch/task/agentA",
		OrchID:  "orch", TaskID: "task",
		Scope: []string{"read:Customers:*"}, TTLSecond: 300,
	})

	body, _ := json.Marshal(deleg.DelegReq{
		DelegatorToken: resp.AccessToken,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/delegate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&result)
	if result["type"] != "urn:agentauth:error:delegation-depth-exceeded" {
		t.Errorf("unexpected error type: %v", result["type"])
	}
}

func TestDelegHdl_MethodNotAllowed(t *testing.T) {
	k := newDelegTestKit()
	req := httptest.NewRequest(http.MethodGet, "/v1/delegate", nil)
	rec := httptest.NewRecorder()
	k.hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rec.Code)
	}
}

func TestDelegHdl_MalformedJSON(t *testing.T) {
	k := newDelegTestKit()
	req := httptest.NewRequest(http.MethodPost, "/v1/delegate", bytes.NewReader([]byte(`{bad json`)))
	rec := httptest.NewRecorder()
	k.hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestDelegHdl_EmptyTarget(t *testing.T) {
	k := newDelegTestKit()
	tkn := k.issueToken([]string{"read:Customers:*"})

	body, _ := json.Marshal(deleg.DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/delegate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	k.hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
