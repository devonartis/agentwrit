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
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

func TestIdentityChallengeRegisterAndSingleUseLaunchToken(t *testing.T) {
	sqlStore := store.NewSqlStore()
	brokerPub, brokerPriv, _ := identity.GenerateSigningKeyPair()
	idSvc := identity.NewIdSvc(sqlStore, brokerPriv, "agentauth.local")
	tknSvc := token.NewTknSvc(brokerPriv, brokerPub, cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300})

	mux := http.NewServeMux()
	mux.Handle("/v1/challenge", handler.NewChallengeHdl(sqlStore))
	mux.Handle("/v1/register", handler.NewRegHdl(idSvc, tknSvc, cfg.Cfg{DefaultTTL: 300}))
	mux.Handle("/v1/token/validate", handler.NewValHdl(tknSvc))
	mux.Handle("/v1/token/renew", handler.NewRenewHdl(tknSvc))
	revSvc := revoke.NewRevSvc()
	valMw := authz.NewValMw(tknSvc, revSvc)
	mux.Handle("/v1/revoke", authz.WithRequiredScope("admin:Broker:*", valMw.Wrap(handler.NewRevokeHdl(revSvc))))
	mux.Handle("/v1/protected/customers/12345", authz.WithRequiredScope("read:Customers:12345", valMw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"customer_id":"12345"}`))
	}))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	launch, err := identity.CreateLaunchToken(sqlStore, "orch-456", "task-789", []string{"read:Customers:12345"}, 30*time.Second)
	if err != nil {
		t.Fatalf("create launch token: %v", err)
	}

	chRes, err := http.Get(srv.URL + "/v1/challenge")
	if err != nil {
		t.Fatalf("challenge request failed: %v", err)
	}
	defer chRes.Body.Close()
	if chRes.StatusCode != http.StatusOK {
		t.Fatalf("challenge status: %d", chRes.StatusCode)
	}

	var chBody map[string]string
	if err := json.NewDecoder(chRes.Body).Decode(&chBody); err != nil {
		t.Fatalf("decode challenge body: %v", err)
	}
	nonce := chBody["nonce"]
	if nonce == "" {
		t.Fatalf("missing nonce")
	}

	agentPub, agentPriv, _ := identity.GenerateSigningKeyPair()
	sig := ed25519.Sign(agentPriv, []byte(nonce))
	pubJWK, _ := json.Marshal(map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(agentPub),
	})

	body := map[string]any{
		"launch_token":     launch,
		"nonce":            nonce,
		"agent_public_key": json.RawMessage(pubJWK),
		"signature":        base64.RawURLEncoding.EncodeToString(sig),
		"orchestration_id": "orch-456",
		"task_id":          "task-789",
		"requested_scope":  []string{"read:Customers:12345"},
	}
	raw, _ := json.Marshal(body)

	regRes, err := http.Post(srv.URL+"/v1/register", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("register request failed: %v", err)
	}
	defer regRes.Body.Close()
	if regRes.StatusCode != http.StatusCreated {
		t.Fatalf("register status: %d", regRes.StatusCode)
	}
	var regBody map[string]any
	_ = json.NewDecoder(regRes.Body).Decode(&regBody)
	agentID, _ := regBody["agent_instance_id"].(string)
	accessToken, _ := regBody["access_token"].(string)
	if err := identity.ValidateSpiffeId(agentID); err != nil {
		t.Fatalf("invalid agent_instance_id: %v", err)
	}
	if accessToken == "" {
		t.Fatalf("missing access token")
	}

	// Validate issued token against same required scope.
	validateReq, _ := json.Marshal(map[string]any{
		"token":          accessToken,
		"required_scope": "read:Customers:12345",
	})
	validateRes, err := http.Post(srv.URL+"/v1/token/validate", "application/json", bytes.NewReader(validateReq))
	if err != nil {
		t.Fatalf("token validate request failed: %v", err)
	}
	defer validateRes.Body.Close()
	if validateRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /v1/token/validate, got %d", validateRes.StatusCode)
	}

	// Renew token and verify renewed token still validates.
	renewReq, _ := json.Marshal(map[string]any{
		"token": accessToken,
	})
	renewRes, err := http.Post(srv.URL+"/v1/token/renew", "application/json", bytes.NewReader(renewReq))
	if err != nil {
		t.Fatalf("token renew request failed: %v", err)
	}
	defer renewRes.Body.Close()
	if renewRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /v1/token/renew, got %d", renewRes.StatusCode)
	}
	var renewBody map[string]any
	_ = json.NewDecoder(renewRes.Body).Decode(&renewBody)
	renewedToken, _ := renewBody["access_token"].(string)
	if renewedToken == "" {
		t.Fatalf("missing renewed access token")
	}
	if renewedToken == accessToken {
		t.Fatalf("renewed token should differ from original token")
	}

	// Authz middleware allows matching scoped token.
	okReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/protected/customers/12345", nil)
	okReq.Header.Set("Authorization", "Bearer "+renewedToken)
	okRes, err := http.DefaultClient.Do(okReq)
	if err != nil {
		t.Fatalf("protected route request failed: %v", err)
	}
	defer okRes.Body.Close()
	if okRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on protected route, got %d", okRes.StatusCode)
	}

	// Missing bearer should fail closed.
	denyReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/protected/customers/12345", nil)
	denyRes, err := http.DefaultClient.Do(denyReq)
	if err != nil {
		t.Fatalf("protected route missing-bearer request failed: %v", err)
	}
	defer denyRes.Body.Close()
	if denyRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing bearer, got %d", denyRes.StatusCode)
	}

	// Same launch token must fail because registration consumed it atomically.
	sqlStore.PutNonce(nonce, time.Now().UTC().Add(30*time.Second))
	regRes2, err := http.Post(srv.URL+"/v1/register", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("second register request failed: %v", err)
	}
	defer regRes2.Body.Close()
	if regRes2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on reused launch token, got %d", regRes2.StatusCode)
	}

	// Revoke token, then verify protected route returns 401.
	claims, verifyErr := tknSvc.Verify(renewedToken)
	if verifyErr != nil {
		t.Fatalf("verify renewed token for JTI extraction: %v", verifyErr)
	}
	revokeBody, _ := json.Marshal(map[string]string{
		"level":     "token",
		"target_id": claims.Jti,
		"reason":    "integration test revocation",
	})
	adminResp, err := tknSvc.Issue(token.IssueReq{
		AgentID:   "spiffe://agentauth.local/agent/orch-456/task-789/admin",
		OrchID:    "orch-456",
		TaskID:    "task-789",
		Scope:     []string{"admin:Broker:*"},
		TTLSecond: 300,
	})
	if err != nil {
		t.Fatalf("issue admin token for revoke: %v", err)
	}
	revokeReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/revoke", bytes.NewReader(revokeBody))
	revokeReq.Header.Set("Content-Type", "application/json")
	revokeReq.Header.Set("Authorization", "Bearer "+adminResp.AccessToken)
	revokeRes, err := http.DefaultClient.Do(revokeReq)
	if err != nil {
		t.Fatalf("revoke request failed: %v", err)
	}
	defer revokeRes.Body.Close()
	if revokeRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /v1/revoke, got %d", revokeRes.StatusCode)
	}

	// Protected route should now deny the revoked token.
	revokedReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/protected/customers/12345", nil)
	revokedReq.Header.Set("Authorization", "Bearer "+renewedToken)
	revokedRes, err := http.DefaultClient.Do(revokedReq)
	if err != nil {
		t.Fatalf("protected route with revoked token: %v", err)
	}
	defer revokedRes.Body.Close()
	if revokedRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on revoked token, got %d", revokedRes.StatusCode)
	}
}
