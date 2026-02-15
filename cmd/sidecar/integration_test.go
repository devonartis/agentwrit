//go:build !short

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/admin"
	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/handler"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

// startTestBroker spins up a real in-process AgentAuth broker on a random
// port using httptest.Server. It mirrors the wiring in cmd/broker/main.go
// but omits logging middleware and metrics for test clarity.
func startTestBroker(t *testing.T, secret string) *httptest.Server {
	t.Helper()

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}

	c := cfg.Cfg{
		Port:        "0",
		LogLevel:    "quiet",
		TrustDomain: "agentauth.local",
		DefaultTTL:  300,
		AdminSecret: secret,
	}

	sqlStore := store.NewSqlStore()
	auditLog := audit.NewAuditLog()
	tknSvc := token.NewTknSvc(privKey, pubKey, c)
	revSvc := revoke.NewRevSvc()
	idSvc := identity.NewIdSvc(sqlStore, tknSvc, c.TrustDomain, auditLog)
	adminSvc := admin.NewAdminSvc(c.AdminSecret, tknSvc, sqlStore, auditLog)
	valMw := authz.NewValMw(tknSvc, revSvc, auditLog)

	mux := http.NewServeMux()
	mux.Handle("GET /v1/challenge", handler.NewChallengeHdl(sqlStore))
	mux.Handle("POST /v1/register", problemdetails.MaxBytesBody(handler.NewRegHdl(idSvc)))
	mux.Handle("POST /v1/token/validate", problemdetails.MaxBytesBody(handler.NewValHdl(tknSvc, revSvc)))
	mux.Handle("POST /v1/token/renew", problemdetails.MaxBytesBody(valMw.Wrap(handler.NewRenewHdl(tknSvc, auditLog))))
	mux.Handle("POST /v1/token/exchange",
		problemdetails.MaxBytesBody(valMw.Wrap(authz.WithRequiredScope("sidecar:manage:*", handler.NewTokenExchangeHdl(tknSvc, sqlStore, auditLog)))))
	mux.Handle("GET /v1/health", handler.NewHealthHdl("test"))
	admin.NewAdminHdl(adminSvc, valMw, auditLog).RegisterRoutes(mux)

	var rootHandler http.Handler = mux
	rootHandler = problemdetails.RequestIDMiddleware(rootHandler)

	return httptest.NewServer(rootHandler)
}

// brokerAdminAuth authenticates as admin and returns the admin JWT.
func brokerAdminAuth(t *testing.T, brokerURL, secret string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{
		"client_id":     "sidecar",
		"client_secret": secret,
	})

	resp, err := http.Post(brokerURL+"/v1/admin/auth", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("admin auth request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("admin auth returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode admin auth response: %v", err)
	}

	tok, ok := result["access_token"].(string)
	if !ok || tok == "" {
		t.Fatalf("admin auth response missing access_token")
	}
	return tok
}

// brokerCreateLaunchToken creates a launch token via the admin API.
func brokerCreateLaunchToken(t *testing.T, brokerURL, adminToken string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]any{
		"agent_name":    "test-agent",
		"allowed_scope": []string{"read:data:*"},
		"max_ttl":       600,
		"ttl":           600,
	})

	req, _ := http.NewRequest("POST", brokerURL+"/v1/admin/launch-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create launch token request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("create launch token returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode launch token response: %v", err)
	}

	lt, ok := result["launch_token"].(string)
	if !ok || lt == "" {
		t.Fatalf("launch token response missing launch_token")
	}
	return lt
}

// brokerGetChallenge fetches a challenge nonce from the broker.
func brokerGetChallenge(t *testing.T, brokerURL string) string {
	t.Helper()

	resp, err := http.Get(brokerURL + "/v1/challenge")
	if err != nil {
		t.Fatalf("challenge request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("challenge returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode challenge response: %v", err)
	}

	nonce, ok := result["nonce"].(string)
	if !ok || nonce == "" {
		t.Fatalf("challenge response missing nonce")
	}
	return nonce
}

// brokerRegisterAgent performs the full challenge-response registration
// and returns the agent_id (SPIFFE format) from the broker.
func brokerRegisterAgent(t *testing.T, brokerURL, launchToken, nonce string) string {
	t.Helper()

	// Generate agent Ed25519 keypair.
	agentPub, agentPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate agent keypair: %v", err)
	}

	// Decode the hex nonce to bytes, then sign the raw bytes.
	nonceBytes, err := hex.DecodeString(nonce)
	if err != nil {
		t.Fatalf("decode nonce hex: %v", err)
	}

	sig := ed25519.Sign(agentPriv, nonceBytes)

	body, _ := json.Marshal(map[string]any{
		"launch_token":   launchToken,
		"nonce":          nonce,
		"public_key":     base64.StdEncoding.EncodeToString(agentPub),
		"signature":      base64.StdEncoding.EncodeToString(sig),
		"orch_id":        "test-orch",
		"task_id":        "test-task",
		"requested_scope": []string{"read:data:*"},
	})

	resp, err := http.Post(brokerURL+"/v1/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("register returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	agentID, ok := result["agent_id"].(string)
	if !ok || agentID == "" {
		t.Fatalf("register response missing agent_id")
	}

	t.Logf("registered agent_id: %s", agentID)
	return agentID
}

// brokerValidateToken validates a token against the broker and returns
// the parsed response body.
func brokerValidateToken(t *testing.T, brokerURL, tokenStr string) map[string]any {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"token": tokenStr})
	resp, err := http.Post(brokerURL+"/v1/token/validate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("validate token request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("validate token returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode validate response: %v", err)
	}
	return result
}

// TestIntegration_DeveloperFlow tests the complete developer flow:
//
//  1. Start a real in-process broker.
//  2. Register an agent via the challenge-response flow.
//  3. Bootstrap the sidecar against the broker.
//  4. Request a scoped token via the sidecar's POST /v1/token.
//  5. Validate the token at the broker.
//  6. Verify scope escalation is denied.
func TestIntegration_DeveloperFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Shorten the health timeout so bootstrap retries don't waste time.
	origTimeout := defaultHealthTimeout
	defaultHealthTimeout = 5 * time.Second
	defer func() { defaultHealthTimeout = origTimeout }()

	const adminSecret = "integration-test-secret"

	// ---------------------------------------------------------------
	// Step 1: Start a real in-process broker.
	// ---------------------------------------------------------------
	broker := startTestBroker(t, adminSecret)
	defer broker.Close()
	t.Logf("broker running at %s", broker.URL)

	// ---------------------------------------------------------------
	// Step 2: Admin auth to get admin token.
	// ---------------------------------------------------------------
	adminToken := brokerAdminAuth(t, broker.URL, adminSecret)
	t.Logf("admin token obtained (len=%d)", len(adminToken))

	// ---------------------------------------------------------------
	// Step 3: Create a launch token for agent registration.
	// ---------------------------------------------------------------
	launchToken := brokerCreateLaunchToken(t, broker.URL, adminToken)
	t.Logf("launch token created (len=%d)", len(launchToken))

	// ---------------------------------------------------------------
	// Step 4: Get challenge nonce and register the agent.
	// ---------------------------------------------------------------
	nonce := brokerGetChallenge(t, broker.URL)
	t.Logf("challenge nonce obtained: %s", nonce)

	agentID := brokerRegisterAgent(t, broker.URL, launchToken, nonce)
	t.Logf("agent registered: %s", agentID)

	// ---------------------------------------------------------------
	// Step 5: Bootstrap the sidecar against the real broker.
	// ---------------------------------------------------------------
	bc := newBrokerClient(broker.URL)
	sidecarCfg := sidecarConfig{
		AdminSecret:  adminSecret,
		ScopeCeiling: []string{"read:data:*"},
	}

	state, err := bootstrap(bc, sidecarCfg)
	if err != nil {
		t.Fatalf("sidecar bootstrap failed: %v", err)
	}
	t.Logf("sidecar bootstrapped: id=%s, token_len=%d, expires_in=%d",
		state.sidecarID, len(state.sidecarToken), state.expiresIn)

	// ---------------------------------------------------------------
	// Step 6: Request a scoped token via the sidecar's POST /v1/token.
	// ---------------------------------------------------------------
	reg := newAgentRegistry()
	reg.store(agentID, &agentEntry{spiffeID: agentID})
	th := newTokenHandler(bc, state, sidecarCfg.ScopeCeiling, reg, adminSecret)

	// Use the full SPIFFE agent_id as agent_name, leave task_id empty
	// so the handler passes it through to the broker as-is.
	tokenReqBody, _ := json.Marshal(map[string]any{
		"agent_name": agentID,
		"scope":      []string{"read:data:*"},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(tokenReqBody))
	req.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /v1/token returned %d: %s", rr.Code, rr.Body.String())
	}

	var tokenResp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}

	accessToken, ok := tokenResp["access_token"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("token response missing access_token: %v", tokenResp)
	}
	t.Logf("sidecar issued token (len=%d)", len(accessToken))

	// Verify scope is echoed back.
	if scopeRaw, ok := tokenResp["scope"]; ok {
		if scopeArr, ok := scopeRaw.([]any); ok {
			if len(scopeArr) != 1 || scopeArr[0] != "read:data:*" {
				t.Errorf("unexpected scope in response: %v", scopeArr)
			}
		}
	} else {
		t.Errorf("token response missing scope field")
	}

	// ---------------------------------------------------------------
	// Step 7: Validate the issued token at the broker.
	// ---------------------------------------------------------------
	valResult := brokerValidateToken(t, broker.URL, accessToken)

	valid, ok := valResult["valid"].(bool)
	if !ok || !valid {
		t.Fatalf("broker says token is invalid: %v", valResult)
	}
	t.Log("broker validated token as valid")

	// Verify the claims contain the expected subject (the agent_id).
	if claims, ok := valResult["claims"].(map[string]any); ok {
		if sub, ok := claims["sub"].(string); ok {
			if sub != agentID {
				t.Errorf("token sub = %q, want %q", sub, agentID)
			}
		} else {
			t.Error("claims missing sub field")
		}
	} else {
		t.Error("validate response missing claims")
	}

	// ---------------------------------------------------------------
	// Step 8: Verify scope escalation is denied.
	// ---------------------------------------------------------------
	escalationBody, _ := json.Marshal(map[string]any{
		"agent_name": agentID,
		"scope":      []string{"write:data:*"},
	})

	rrEsc := httptest.NewRecorder()
	reqEsc := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(escalationBody))
	reqEsc.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rrEsc, reqEsc)

	if rrEsc.Code != http.StatusForbidden {
		t.Errorf("scope escalation: expected 403, got %d: %s", rrEsc.Code, rrEsc.Body.String())
	}
	t.Log("scope escalation correctly denied with 403")
}

// TestIntegration_Phase2_LazyRegistration verifies that the sidecar's
// POST /v1/token automatically registers an agent at the broker on the
// first request (lazy registration), caches it, and denies scope escalation.
func TestIntegration_Phase2_LazyRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	origTimeout := defaultHealthTimeout
	defaultHealthTimeout = 5 * time.Second
	defer func() { defaultHealthTimeout = origTimeout }()

	const adminSecret = "integration-test-secret"

	// Step 1: Start broker.
	broker := startTestBroker(t, adminSecret)
	defer broker.Close()

	// Step 2: Bootstrap sidecar.
	bc := newBrokerClient(broker.URL)
	sidecarCfg := sidecarConfig{
		AdminSecret:   adminSecret,
		ScopeCeiling:  []string{"read:data:*"},
		RenewalBuffer: 0.8,
	}

	state, err := bootstrap(bc, sidecarCfg)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	registry := newAgentRegistry()
	th := newTokenHandler(bc, state, sidecarCfg.ScopeCeiling, registry, adminSecret)

	// Step 3: First POST /v1/token — should trigger lazy registration.
	body1, _ := json.Marshal(map[string]any{
		"agent_name": "lazy-agent",
		"task_id":    "task-1",
		"scope":      []string{"read:data:*"},
	})

	rr1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d: %s", rr1.Code, rr1.Body.String())
	}

	var resp1 map[string]any
	_ = json.NewDecoder(rr1.Body).Decode(&resp1)

	token1 := resp1["access_token"].(string)
	agentID := resp1["agent_id"].(string)
	t.Logf("lazy registration: agent_id=%s, token_len=%d", agentID, len(token1))

	if agentID == "" {
		t.Fatal("agent_id should be non-empty (SPIFFE ID from broker)")
	}

	// Verify agent is in registry.
	entry, ok := registry.lookup("lazy-agent:task-1")
	if !ok {
		t.Fatal("agent should be cached in registry")
	}
	if entry.spiffeID != agentID {
		t.Errorf("cached spiffeID = %q, want %q", entry.spiffeID, agentID)
	}
	if entry.privKey == nil {
		t.Error("lazy-registered agent should have sidecar-managed keypair")
	}

	// Validate token at broker.
	valResult := brokerValidateToken(t, broker.URL, token1)
	if valid, _ := valResult["valid"].(bool); !valid {
		t.Fatalf("broker says token is invalid: %v", valResult)
	}

	// Step 4: Second request — should use cached agent (no re-registration).
	body2, _ := json.Marshal(map[string]any{
		"agent_name": "lazy-agent",
		"task_id":    "task-1",
		"scope":      []string{"read:data:*"},
	})

	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var resp2 map[string]any
	_ = json.NewDecoder(rr2.Body).Decode(&resp2)
	if resp2["agent_id"] != agentID {
		t.Errorf("second request agent_id = %v, want %v", resp2["agent_id"], agentID)
	}
	t.Log("second request used cached registration (no re-register)")

	// Step 5: Scope escalation still denied.
	bodyEsc, _ := json.Marshal(map[string]any{
		"agent_name": "lazy-agent",
		"task_id":    "task-1",
		"scope":      []string{"write:data:*"},
	})

	rrEsc := httptest.NewRecorder()
	reqEsc := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(bodyEsc))
	reqEsc.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rrEsc, reqEsc)

	if rrEsc.Code != http.StatusForbidden {
		t.Errorf("scope escalation: expected 403, got %d", rrEsc.Code)
	}
}

// TestIntegration_Phase2_BYOKRegistration verifies the full BYOK (Bring Your
// Own Key) flow: developer gets a challenge via the sidecar proxy, signs it
// with their own key, registers via the sidecar's BYOK endpoint, then
// requests a token.
func TestIntegration_Phase2_BYOKRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	origTimeout := defaultHealthTimeout
	defaultHealthTimeout = 5 * time.Second
	defer func() { defaultHealthTimeout = origTimeout }()

	const adminSecret = "integration-test-secret"

	broker := startTestBroker(t, adminSecret)
	defer broker.Close()

	bc := newBrokerClient(broker.URL)
	sidecarCfg := sidecarConfig{
		AdminSecret:   adminSecret,
		ScopeCeiling:  []string{"read:data:*"},
		RenewalBuffer: 0.8,
	}

	state, err := bootstrap(bc, sidecarCfg)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	registry := newAgentRegistry()

	// Step 1: Get challenge via sidecar proxy.
	ch := newChallengeProxyHandler(bc)
	rrCh := httptest.NewRecorder()
	reqCh := httptest.NewRequest(http.MethodGet, "/v1/challenge", nil)
	ch.ServeHTTP(rrCh, reqCh)

	if rrCh.Code != http.StatusOK {
		t.Fatalf("challenge: expected 200, got %d", rrCh.Code)
	}
	var chResp map[string]any
	_ = json.NewDecoder(rrCh.Body).Decode(&chResp)
	nonce := chResp["nonce"].(string)

	// Step 2: Developer signs the nonce with their own key.
	devPub, devPriv, _ := ed25519.GenerateKey(rand.Reader)
	nonceBytes, _ := hex.DecodeString(nonce)
	sig := ed25519.Sign(devPriv, nonceBytes)

	// Step 3: Register via sidecar BYOK endpoint.
	rh := newRegisterHandler(bc, registry, adminSecret, sidecarCfg.ScopeCeiling)

	regBody, _ := json.Marshal(map[string]any{
		"agent_name": "byok-agent",
		"task_id":    "task-byok",
		"public_key": base64.StdEncoding.EncodeToString(devPub),
		"signature":  base64.StdEncoding.EncodeToString(sig),
		"nonce":      nonce,
	})

	rrReg := httptest.NewRecorder()
	reqReg := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(regBody))
	reqReg.Header.Set("Content-Type", "application/json")
	rh.ServeHTTP(rrReg, reqReg)

	if rrReg.Code != http.StatusOK {
		t.Fatalf("register: expected 200, got %d: %s", rrReg.Code, rrReg.Body.String())
	}

	var regResp map[string]any
	_ = json.NewDecoder(rrReg.Body).Decode(&regResp)
	agentID := regResp["agent_id"].(string)
	t.Logf("BYOK registered: agent_id=%s", agentID)

	// Verify BYOK entry in registry (nil privKey).
	entry, ok := registry.lookup("byok-agent:task-byok")
	if !ok {
		t.Fatal("BYOK agent should be in registry")
	}
	if entry.privKey != nil {
		t.Error("BYOK entry should have nil privKey")
	}

	// Step 4: Token request using cached BYOK registration.
	th := newTokenHandler(bc, state, sidecarCfg.ScopeCeiling, registry, adminSecret)

	tokBody, _ := json.Marshal(map[string]any{
		"agent_name": "byok-agent",
		"task_id":    "task-byok",
		"scope":      []string{"read:data:*"},
	})

	rrTok := httptest.NewRecorder()
	reqTok := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(tokBody))
	reqTok.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rrTok, reqTok)

	if rrTok.Code != http.StatusOK {
		t.Fatalf("token after BYOK: expected 200, got %d: %s", rrTok.Code, rrTok.Body.String())
	}

	var tokResp map[string]any
	_ = json.NewDecoder(rrTok.Body).Decode(&tokResp)
	accessToken := tokResp["access_token"].(string)

	// Validate at broker.
	valResult := brokerValidateToken(t, broker.URL, accessToken)
	if valid, _ := valResult["valid"].(bool); !valid {
		t.Fatalf("broker says BYOK token invalid: %v", valResult)
	}
	t.Log("BYOK token validated at broker")
}
