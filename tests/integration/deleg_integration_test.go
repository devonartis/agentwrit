//go:build integration

package integration_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/deleg"
	"github.com/divineartis/agentauth/internal/handler"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

// setupDelegServer creates a full test server with delegation wired in,
// mirroring main.go exactly (with /v1/delegate added).
func setupDelegServer(t *testing.T) (*httptest.Server, *store.SqlStore, *token.TknSvc) {
	t.Helper()
	sqlStore := store.NewSqlStore()
	brokerPub, brokerPriv, _ := identity.GenerateSigningKeyPair()
	c := cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300}
	idSvc := identity.NewIdSvc(sqlStore, brokerPriv, c.TrustDomain)
	tknSvc := token.NewTknSvc(brokerPriv, brokerPub, c)
	revSvc := revoke.NewRevSvc()
	valMw := authz.NewValMw(tknSvc, revSvc)

	// M07: Delegation.
	delegSvc := deleg.NewDelegSvc(tknSvc, brokerPriv, 3)
	delegHdl := handler.NewDelegHdl(delegSvc)

	mux := http.NewServeMux()
	mux.Handle("/v1/challenge", handler.NewChallengeHdl(sqlStore))
	mux.Handle("/v1/register", handler.NewRegHdl(idSvc, tknSvc, c))
	mux.Handle("/v1/token/validate", handler.NewValHdl(tknSvc))
	mux.Handle("/v1/token/renew", handler.NewRenewHdl(tknSvc))
	mux.Handle("/v1/revoke", authz.WithRequiredScope("admin:Broker:*", valMw.Wrap(handler.NewRevokeHdl(revSvc))))
	mux.Handle("/v1/delegate", delegHdl)
	mux.Handle("/v1/protected/customers/12345", authz.WithRequiredScope("read:Customers:12345", valMw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"customer_id":"12345"}`))
	}))))

	srv := httptest.NewServer(mux)
	return srv, sqlStore, tknSvc
}

func TestDelegationHappyPathIntegration(t *testing.T) {
	srv, sqlStore, _ := setupDelegServer(t)
	defer srv.Close()

	// Agent A registers with wildcard scope.
	_, tokenA, _ := registerAgent(t, srv, sqlStore, "orch-deleg", "task-a", []string{"read:Customers:*"})

	// Agent A delegates narrowed scope to Agent B via HTTP.
	delegReq, _ := json.Marshal(map[string]any{
		"delegator_token": tokenA,
		"target_agent_id": "spiffe://agentauth.local/agent/orch-deleg/task-b/instanceB",
		"delegated_scope": []string{"read:Customers:12345"},
		"max_ttl":         60,
	})
	delegRes, err := http.Post(srv.URL+"/v1/delegate", "application/json", bytes.NewReader(delegReq))
	if err != nil {
		t.Fatalf("delegate POST: %v", err)
	}
	defer delegRes.Body.Close()
	if delegRes.StatusCode != http.StatusCreated {
		t.Fatalf("delegate: expected 201, got %d", delegRes.StatusCode)
	}
	var dr map[string]any
	_ = json.NewDecoder(delegRes.Body).Decode(&dr)
	delegToken, _ := dr["delegation_token"].(string)
	chainHash, _ := dr["chain_hash"].(string)
	depth, _ := dr["delegation_depth"].(float64)

	if delegToken == "" {
		t.Fatal("delegation_token missing")
	}
	if chainHash == "" {
		t.Fatal("chain_hash missing")
	}
	if int(depth) != 1 {
		t.Fatalf("expected depth=1, got %d", int(depth))
	}

	// Agent B validates its delegation token.
	valReq, _ := json.Marshal(map[string]any{
		"token":          delegToken,
		"required_scope": "read:Customers:12345",
	})
	valRes, err := http.Post(srv.URL+"/v1/token/validate", "application/json", bytes.NewReader(valReq))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	defer valRes.Body.Close()
	if valRes.StatusCode != http.StatusOK {
		t.Fatalf("validate delegation token: expected 200, got %d", valRes.StatusCode)
	}
	var vr map[string]any
	_ = json.NewDecoder(valRes.Body).Decode(&vr)
	if gotDepth, _ := vr["delegation_depth"].(float64); int(gotDepth) != 1 {
		t.Fatalf("validate delegation token: expected delegation_depth=1, got %v", vr["delegation_depth"])
	}

	// Agent B uses delegation token on protected resource.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/protected/customers/12345", nil)
	req.Header.Set("Authorization", "Bearer "+delegToken)
	protRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("protected: %v", err)
	}
	defer protRes.Body.Close()
	if protRes.StatusCode != http.StatusOK {
		t.Fatalf("protected with delegation token: expected 200, got %d", protRes.StatusCode)
	}
}

func TestDelegationScopeEscalationBlockedIntegration(t *testing.T) {
	srv, sqlStore, _ := setupDelegServer(t)
	defer srv.Close()

	// Agent A registers with a specific (non-wildcard) scope.
	_, tokenA, _ := registerAgent(t, srv, sqlStore, "orch-deleg", "task-a", []string{"read:Customers:12345"})

	// Agent A tries to delegate broader wildcard scope — must fail.
	delegReq, _ := json.Marshal(map[string]any{
		"delegator_token": tokenA,
		"target_agent_id": "spiffe://agentauth.local/agent/orch-deleg/task-b/instanceB",
		"delegated_scope": []string{"read:Customers:*"},
		"max_ttl":         60,
	})
	delegRes, err := http.Post(srv.URL+"/v1/delegate", "application/json", bytes.NewReader(delegReq))
	if err != nil {
		t.Fatalf("delegate POST: %v", err)
	}
	defer delegRes.Body.Close()
	if delegRes.StatusCode != http.StatusForbidden {
		t.Fatalf("scope escalation: expected 403, got %d", delegRes.StatusCode)
	}
	var dr map[string]any
	_ = json.NewDecoder(delegRes.Body).Decode(&dr)
	if dr["type"] != "urn:agentauth:error:scope-escalation" {
		t.Fatalf("expected scope-escalation error, got %v", dr["type"])
	}
}

func TestDelegationDepthLimitIntegration(t *testing.T) {
	// Test depth enforcement using maxDepth=0 to trigger immediately.
	// TknSvc.Issue creates tokens with empty delegation chains, so
	// each delegation from a fresh token always has depth=0 in the chain.
	// With maxDepth=0, the very first delegation attempt is blocked.
	pub, priv, _ := ed25519.GenerateKey(nil)
	c := cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300}
	tknSvc := token.NewTknSvc(priv, pub, c)
	delegSvc := deleg.NewDelegSvc(tknSvc, priv, 0) // max depth = 0

	issResp, err := tknSvc.Issue(token.IssueReq{
		AgentID: "spiffe://agentauth.local/agent/orch/task/agentA",
		OrchID:  "orch", TaskID: "task",
		Scope: []string{"read:Customers:*"}, TTLSecond: 300,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = delegSvc.Delegate(deleg.DelegReq{
		DelegatorToken: issResp.AccessToken,
		TargetAgentId:  "spiffe://agentauth.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	if err == nil {
		t.Fatal("expected depth exceeded error at maxDepth=0")
	}

	// With maxDepth=1, first delegation succeeds and second is blocked.
	delegSvc1 := deleg.NewDelegSvc(tknSvc, priv, 1)
	resp, err := delegSvc1.Delegate(deleg.DelegReq{
		DelegatorToken: issResp.AccessToken,
		TargetAgentId:  "spiffe://agentauth.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	if err != nil {
		t.Fatalf("delegation with maxDepth=1 should succeed: %v", err)
	}
	if resp.DelegationDepth != 1 {
		t.Fatalf("expected depth=1, got %d", resp.DelegationDepth)
	}

	_, err = delegSvc1.Delegate(deleg.DelegReq{
		DelegatorToken: resp.DelegationToken,
		TargetAgentId:  "spiffe://agentauth.local/agent/orch/task/agentC",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         30,
	})
	if err == nil {
		t.Fatal("expected second delegation to fail at maxDepth=1")
	}
	if !errors.Is(err, deleg.ErrDepthExceeded) {
		t.Fatalf("expected depth exceeded, got: %v", err)
	}
}

func TestDelegationReDelegateBroaderScopeBlockedIntegration(t *testing.T) {
	srv, sqlStore, _ := setupDelegServer(t)
	defer srv.Close()

	// Agent A registers with wildcard scope.
	_, tokenA, _ := registerAgent(t, srv, sqlStore, "orch-deleg", "task-a", []string{"read:Customers:*"})

	// A delegates to B with narrowed scope.
	delegReq, _ := json.Marshal(map[string]any{
		"delegator_token": tokenA,
		"target_agent_id": "spiffe://agentauth.local/agent/orch-deleg/task-b/instanceB",
		"delegated_scope": []string{"read:Customers:12345"},
		"max_ttl":         60,
	})
	delegRes, err := http.Post(srv.URL+"/v1/delegate", "application/json", bytes.NewReader(delegReq))
	if err != nil {
		t.Fatalf("delegate: %v", err)
	}
	defer delegRes.Body.Close()
	if delegRes.StatusCode != http.StatusCreated {
		t.Fatalf("first delegation: expected 201, got %d", delegRes.StatusCode)
	}
	var dr map[string]any
	_ = json.NewDecoder(delegRes.Body).Decode(&dr)
	delegTokenB, _ := dr["delegation_token"].(string)

	// B tries to re-delegate with broader scope (read:Customers:*) — must fail.
	reDelegReq, _ := json.Marshal(map[string]any{
		"delegator_token": delegTokenB,
		"target_agent_id": "spiffe://agentauth.local/agent/orch-deleg/task-c/instanceC",
		"delegated_scope": []string{"read:Customers:*"},
		"max_ttl":         30,
	})
	reDelegRes, err := http.Post(srv.URL+"/v1/delegate", "application/json", bytes.NewReader(reDelegReq))
	if err != nil {
		t.Fatalf("re-delegate: %v", err)
	}
	defer reDelegRes.Body.Close()
	if reDelegRes.StatusCode != http.StatusForbidden {
		t.Fatalf("re-delegation with broader scope: expected 403, got %d", reDelegRes.StatusCode)
	}
}
