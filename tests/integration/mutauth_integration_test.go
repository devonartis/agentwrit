//go:build integration

package integration_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/handler"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/mutauth"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

// registerAgent drives the full challenge→register flow for one agent,
// returning its SPIFFE ID, access token, and Ed25519 private key.
func registerAgent(t *testing.T, srv *httptest.Server, sqlStore *store.SqlStore, orchID, taskID string, scope []string) (string, string, ed25519.PrivateKey) {
	t.Helper()

	launch, err := identity.CreateLaunchToken(sqlStore, orchID, taskID, scope, 30*time.Second)
	if err != nil {
		t.Fatalf("create launch token: %v", err)
	}

	chRes, err := http.Get(srv.URL + "/v1/challenge")
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	defer chRes.Body.Close()
	var ch map[string]string
	_ = json.NewDecoder(chRes.Body).Decode(&ch)
	nonce := ch["nonce"]

	agentPub, agentPriv, _ := identity.GenerateSigningKeyPair()
	sig := ed25519.Sign(agentPriv, []byte(nonce))
	pubJWK, _ := json.Marshal(map[string]string{
		"kty": "OKP", "crv": "Ed25519",
		"x": base64.RawURLEncoding.EncodeToString(agentPub),
	})

	body, _ := json.Marshal(map[string]any{
		"launch_token":     launch,
		"nonce":            nonce,
		"agent_public_key": json.RawMessage(pubJWK),
		"signature":        base64.RawURLEncoding.EncodeToString(sig),
		"orchestration_id": orchID,
		"task_id":          taskID,
		"requested_scope":  scope,
	})

	regRes, err := http.Post(srv.URL+"/v1/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer regRes.Body.Close()
	if regRes.StatusCode != http.StatusCreated {
		t.Fatalf("register status: %d", regRes.StatusCode)
	}
	var reg map[string]any
	_ = json.NewDecoder(regRes.Body).Decode(&reg)
	agentID, _ := reg["agent_instance_id"].(string)
	accessToken, _ := reg["access_token"].(string)
	return agentID, accessToken, agentPriv
}

func TestMutualAuthHandshakeIntegration(t *testing.T) {
	sqlStore := store.NewSqlStore()
	brokerPub, brokerPriv, _ := identity.GenerateSigningKeyPair()
	c := cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300}
	idSvc := identity.NewIdSvc(sqlStore, brokerPriv, c.TrustDomain)
	tknSvc := token.NewTknSvc(brokerPriv, brokerPub, c)
	revSvc := revoke.NewRevSvc()
	valMw := authz.NewValMw(tknSvc, revSvc)

	mux := http.NewServeMux()
	mux.Handle("/v1/challenge", handler.NewChallengeHdl(sqlStore))
	mux.Handle("/v1/register", handler.NewRegHdl(idSvc, tknSvc, c))
	mux.Handle("/v1/token/validate", handler.NewValHdl(tknSvc))
	mux.Handle("/v1/token/renew", handler.NewRenewHdl(tknSvc))
	mux.Handle("/v1/revoke", handler.NewRevokeHdl(revSvc))
	mux.Handle("/v1/protected/customers/12345", authz.WithRequiredScope("read:Customers:12345", valMw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"customer_id":"12345"}`))
	}))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Register two agents through full challenge/register flow.
	agentAID, tokenA, privA := registerAgent(t, srv, sqlStore, "orch-int", "task-a", []string{"read:Data:*"})
	agentBID, tokenB, privB := registerAgent(t, srv, sqlStore, "orch-int", "task-b", []string{"write:Data:*"})

	_ = privA // initiator private key not needed in the handshake
	hdl := mutauth.NewMutAuthHdl(tknSvc, sqlStore)

	// Step 1: Agent A initiates handshake with Agent B.
	req, err := hdl.InitiateHandshake(tokenA, agentBID)
	if err != nil {
		t.Fatalf("initiate handshake: %v", err)
	}
	if req.InitiatorID != agentAID {
		t.Fatalf("initiator ID: got %s, want %s", req.InitiatorID, agentAID)
	}

	// Step 2: Agent B responds.
	resp, err := hdl.RespondToHandshake(req, tokenB, privB)
	if err != nil {
		t.Fatalf("respond to handshake: %v", err)
	}
	if resp.ResponderID != agentBID {
		t.Fatalf("responder ID: got %s, want %s", resp.ResponderID, agentBID)
	}

	// Step 3: Agent A completes.
	ok, err := hdl.CompleteHandshake(resp, req.Nonce)
	if err != nil {
		t.Fatalf("complete handshake: %v", err)
	}
	if !ok {
		t.Fatal("handshake should succeed between registered agents")
	}
}

func TestMutualAuthHandshakeInvalidTokenIntegration(t *testing.T) {
	sqlStore := store.NewSqlStore()
	brokerPub, brokerPriv, _ := identity.GenerateSigningKeyPair()
	c := cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300}
	idSvc := identity.NewIdSvc(sqlStore, brokerPriv, c.TrustDomain)
	tknSvc := token.NewTknSvc(brokerPriv, brokerPub, c)
	revSvc := revoke.NewRevSvc()
	valMw := authz.NewValMw(tknSvc, revSvc)

	mux := http.NewServeMux()
	mux.Handle("/v1/challenge", handler.NewChallengeHdl(sqlStore))
	mux.Handle("/v1/register", handler.NewRegHdl(idSvc, tknSvc, c))
	mux.Handle("/v1/token/validate", handler.NewValHdl(tknSvc))
	mux.Handle("/v1/token/renew", handler.NewRenewHdl(tknSvc))
	mux.Handle("/v1/revoke", handler.NewRevokeHdl(revSvc))
	mux.Handle("/v1/protected/customers/12345", authz.WithRequiredScope("read:Customers:12345", valMw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, _, _ = registerAgent(t, srv, sqlStore, "orch-int", "task-a", []string{"read:Data:*"})
	agentBID, _, _ := registerAgent(t, srv, sqlStore, "orch-int", "task-b", []string{"write:Data:*"})

	hdl := mutauth.NewMutAuthHdl(tknSvc, sqlStore)

	// Try to initiate with a bogus token.
	_, err := hdl.InitiateHandshake("invalid.token.xyz", agentBID)
	if err == nil {
		t.Fatal("expected handshake to fail with invalid token")
	}
}

func TestDiscoveryBindingIntegration(t *testing.T) {
	dr := mutauth.NewDiscoveryRegistry()
	agentID := "spiffe://agentauth.local/agent/orch-int/task-a/abcdef"
	endpoint := "https://agent-a.internal:9443"

	if err := dr.Bind(agentID, endpoint); err != nil {
		t.Fatalf("bind: %v", err)
	}
	got, err := dr.Resolve(agentID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != endpoint {
		t.Fatalf("resolve mismatch: got %s, want %s", got, endpoint)
	}

	// Verify that matching identity passes verification.
	ok, err := dr.VerifyBinding(agentID, agentID)
	if err != nil || !ok {
		t.Fatalf("verify binding failed: ok=%v err=%v", ok, err)
	}

	// Verify that mismatched identity is caught.
	ok, err = dr.VerifyBinding(agentID, "spiffe://agentauth.local/agent/orch-int/task-a/impostor")
	if ok || err == nil {
		t.Fatal("expected binding mismatch to be detected")
	}
}
