package handler_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/admin"
	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/deleg"
	"github.com/divineartis/agentauth/internal/handler"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

const testAdminSecret = "integration-test-secret-32bytes!"

// testBroker sets up a full HTTP mux identical to cmd/broker/main.go,
// including RequestIDMiddleware wrapping.
type testBroker struct {
	handler  http.Handler
	tknSvc   *token.TknSvc
	store    *store.SqlStore
	auditLog *audit.AuditLog
	adminSvc *admin.AdminSvc
}

func newTestBroker(t *testing.T) *testBroker {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	c := cfg.Cfg{
		DefaultTTL:  300,
		TrustDomain: "test.local",
		AdminSecret: testAdminSecret,
	}

	sqlStore := store.NewSqlStore()
	auditLog := audit.NewAuditLog()
	tknSvc := token.NewTknSvc(priv, pub, c)
	revSvc := revoke.NewRevSvc()
	idSvc := identity.NewIdSvc(sqlStore, tknSvc, c.TrustDomain, auditLog)
	delegSvc := deleg.NewDelegSvc(tknSvc, sqlStore, auditLog, priv)
	adminSvc := admin.NewAdminSvc(c.AdminSecret, tknSvc, sqlStore, auditLog)

	valMw := authz.NewValMw(tknSvc, revSvc, auditLog)

	challengeHdl := handler.NewChallengeHdl(sqlStore)
	regHdl := handler.NewRegHdl(idSvc)
	valHdl := handler.NewValHdl(tknSvc, revSvc)
	renewHdl := handler.NewRenewHdl(tknSvc, auditLog)
	revokeHdl := handler.NewRevokeHdl(revSvc, auditLog)
	delegHdl := handler.NewDelegHdl(delegSvc)
	tokenExchangeHdl := handler.NewTokenExchangeHdl(tknSvc, sqlStore, auditLog)
	auditHdl := handler.NewAuditHdl(auditLog)
	healthHdl := handler.NewHealthHdl("test")
	metricsHdl := handler.NewMetricsHdl()
	adminHdl := admin.NewAdminHdl(adminSvc, valMw, auditLog)

	mux := http.NewServeMux()
	mux.Handle("GET /v1/challenge", challengeHdl)
	mux.Handle("GET /v1/health", healthHdl)
	mux.Handle("GET /v1/metrics", metricsHdl)
	mux.Handle("POST /v1/token/validate", problemdetails.MaxBytesBody(valHdl))
	mux.Handle("POST /v1/register", problemdetails.MaxBytesBody(regHdl))
	mux.Handle("POST /v1/token/renew", problemdetails.MaxBytesBody(valMw.Wrap(renewHdl)))
	mux.Handle("POST /v1/token/exchange",
		problemdetails.MaxBytesBody(valMw.Wrap(valMw.RequireScope("sidecar:manage:*", tokenExchangeHdl))))
	mux.Handle("POST /v1/delegate", problemdetails.MaxBytesBody(valMw.Wrap(delegHdl)))
	mux.Handle("POST /v1/revoke",
		problemdetails.MaxBytesBody(valMw.Wrap(valMw.RequireScope("admin:revoke:*", revokeHdl))))
	mux.Handle("GET /v1/audit/events",
		valMw.Wrap(valMw.RequireScope("admin:audit:*", auditHdl)))
	adminHdl.RegisterRoutes(mux)

	// Wrap with global middleware matching cmd/broker/main.go ordering.
	var root http.Handler = mux
	root = handler.LoggingMiddleware(root)
	root = problemdetails.RequestIDMiddleware(root)

	return &testBroker{
		handler:  root,
		tknSvc:   tknSvc,
		store:    sqlStore,
		auditLog: auditLog,
		adminSvc: adminSvc,
	}
}

func (b *testBroker) do(req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	b.handler.ServeHTTP(rr, req)
	return rr
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(v); err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return buf
}

// --- Health ---

func TestHealth(t *testing.T) {
	b := newTestBroker(t)

	req := httptest.NewRequest("GET", "/v1/health", nil)
	rr := b.do(req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck // test assertion
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", resp["status"])
	}
}

// --- Challenge ---

func TestChallenge(t *testing.T) {
	b := newTestBroker(t)

	req := httptest.NewRequest("GET", "/v1/challenge", nil)
	rr := b.do(req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck // test assertion
	nonce, ok := resp["nonce"].(string)
	if !ok || nonce == "" {
		t.Error("expected non-empty nonce in challenge response")
	}
	if resp["expires_in"] != float64(30) {
		t.Errorf("expected expires_in=30, got %v", resp["expires_in"])
	}
}

// --- Admin Auth ---

func TestAdminAuth_Success(t *testing.T) {
	b := newTestBroker(t)

	body := jsonBody(t, map[string]string{
		"client_id":     "admin",
		"client_secret": testAdminSecret,
	})
	req := httptest.NewRequest("POST", "/v1/admin/auth", body)
	rr := b.do(req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck // test assertion
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("expected non-empty access_token")
	}
}

func TestAdminAuth_WrongSecret(t *testing.T) {
	b := newTestBroker(t)

	body := jsonBody(t, map[string]string{
		"client_id":     "admin",
		"client_secret": "wrong",
	})
	req := httptest.NewRequest("POST", "/v1/admin/auth", body)
	rr := b.do(req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// --- Full registration flow ---

func getAdminToken(t *testing.T, b *testBroker) string {
	t.Helper()
	body := jsonBody(t, map[string]string{
		"client_id":     "admin",
		"client_secret": testAdminSecret,
	})
	req := httptest.NewRequest("POST", "/v1/admin/auth", body)
	rr := b.do(req)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin auth failed: %d %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck // test helper
	return resp["access_token"].(string)
}

func createLaunchToken(t *testing.T, b *testBroker, adminToken string) string {
	t.Helper()
	body := jsonBody(t, map[string]any{
		"agent_name":    "test-agent",
		"allowed_scope": []string{"read:data:*", "write:data:*"},
		"max_ttl":       300,
		"ttl":           60,
	})
	req := httptest.NewRequest("POST", "/v1/admin/launch-tokens", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := b.do(req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create launch token failed: %d %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck // test helper
	return resp["launch_token"].(string)
}

func getNonce(t *testing.T, b *testBroker) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/v1/challenge", nil)
	rr := b.do(req)
	if rr.Code != http.StatusOK {
		t.Fatalf("challenge failed: %d", rr.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck // test helper
	return resp["nonce"].(string)
}

func TestFullRegistrationFlow(t *testing.T) {
	b := newTestBroker(t)

	// Step 1: Admin auth.
	adminToken := getAdminToken(t, b)

	// Step 2: Create launch token.
	launchToken := createLaunchToken(t, b, adminToken)

	// Step 3: Challenge.
	nonce := getNonce(t, b)

	// Step 4: Agent generates Ed25519 key pair and signs the nonce.
	agentPub, agentPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate agent key: %v", err)
	}
	nonceBytes, err := hex.DecodeString(nonce)
	if err != nil {
		t.Fatalf("decode nonce hex: %v", err)
	}
	sig := ed25519.Sign(agentPriv, nonceBytes)

	// Step 5: Register.
	regBody := jsonBody(t, map[string]any{
		"launch_token":    launchToken,
		"nonce":           nonce,
		"public_key":      base64.StdEncoding.EncodeToString(agentPub),
		"signature":       base64.StdEncoding.EncodeToString(sig),
		"orch_id":         "orch-1",
		"task_id":         "task-1",
		"requested_scope": []string{"read:data:*"},
	})
	regReq := httptest.NewRequest("POST", "/v1/register", regBody)
	regRR := b.do(regReq)
	if regRR.Code != http.StatusOK {
		t.Fatalf("register failed: %d %s", regRR.Code, regRR.Body.String())
	}

	var regResp map[string]any
	_ = json.NewDecoder(regRR.Body).Decode(&regResp) //nolint:errcheck // test assertion
	agentToken := regResp["access_token"].(string)
	agentID := regResp["agent_id"].(string)
	if agentToken == "" {
		t.Fatal("expected non-empty agent access_token")
	}
	if agentID == "" {
		t.Fatal("expected non-empty agent_id")
	}

	// Step 6: Validate the agent token.
	valBody := jsonBody(t, map[string]string{"token": agentToken})
	valReq := httptest.NewRequest("POST", "/v1/token/validate", valBody)
	valRR := b.do(valReq)
	if valRR.Code != http.StatusOK {
		t.Fatalf("validate failed: %d %s", valRR.Code, valRR.Body.String())
	}
	var valResp map[string]any
	_ = json.NewDecoder(valRR.Body).Decode(&valResp) //nolint:errcheck // test assertion
	if valResp["valid"] != true {
		t.Errorf("expected valid=true, got %v", valResp["valid"])
	}

	// Step 7: Renew the agent token (requires Bearer auth).
	renewReq := httptest.NewRequest("POST", "/v1/token/renew", nil)
	renewReq.Header.Set("Authorization", "Bearer "+agentToken)
	renewRR := b.do(renewReq)
	if renewRR.Code != http.StatusOK {
		t.Fatalf("renew failed: %d %s", renewRR.Code, renewRR.Body.String())
	}
	var renewResp map[string]any
	_ = json.NewDecoder(renewRR.Body).Decode(&renewResp) //nolint:errcheck // test assertion
	if renewResp["access_token"] == nil || renewResp["access_token"] == "" {
		t.Error("expected non-empty renewed access_token")
	}

	// Step 8: Revoke (requires admin scope).
	revokeBody := jsonBody(t, map[string]string{
		"level":  "token",
		"target": "some-jti",
	})
	revokeReq := httptest.NewRequest("POST", "/v1/revoke", revokeBody)
	revokeReq.Header.Set("Authorization", "Bearer "+adminToken)
	revokeRR := b.do(revokeReq)
	if revokeRR.Code != http.StatusOK {
		t.Fatalf("revoke failed: %d %s", revokeRR.Code, revokeRR.Body.String())
	}
	var revokeResp map[string]any
	_ = json.NewDecoder(revokeRR.Body).Decode(&revokeResp) //nolint:errcheck // test assertion
	if revokeResp["revoked"] != true {
		t.Errorf("expected revoked=true, got %v", revokeResp["revoked"])
	}

	// Step 9: Audit (requires admin scope).
	auditReq := httptest.NewRequest("GET", "/v1/audit/events", nil)
	auditReq.Header.Set("Authorization", "Bearer "+adminToken)
	auditRR := b.do(auditReq)
	if auditRR.Code != http.StatusOK {
		t.Fatalf("audit failed: %d %s", auditRR.Code, auditRR.Body.String())
	}
	var auditResp map[string]any
	_ = json.NewDecoder(auditRR.Body).Decode(&auditResp) //nolint:errcheck // test assertion
	total, _ := auditResp["total"].(float64)
	if total == 0 {
		t.Error("expected audit events after registration flow")
	}
}

// --- Token validate: invalid token ---

func TestValidate_InvalidToken(t *testing.T) {
	b := newTestBroker(t)

	body := jsonBody(t, map[string]string{"token": "not-a-valid-token"})
	req := httptest.NewRequest("POST", "/v1/token/validate", body)
	rr := b.do(req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (with valid=false), got %d", rr.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck // test assertion
	if resp["valid"] != false {
		t.Errorf("expected valid=false for invalid token, got %v", resp["valid"])
	}
}

func TestValidate_MissingToken(t *testing.T) {
	b := newTestBroker(t)

	body := jsonBody(t, map[string]string{"token": ""})
	req := httptest.NewRequest("POST", "/v1/token/validate", body)
	rr := b.do(req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- Renew without auth ---

func TestRenew_NoAuth(t *testing.T) {
	b := newTestBroker(t)

	req := httptest.NewRequest("POST", "/v1/token/renew", nil)
	rr := b.do(req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// --- Revoke without admin scope ---

func TestRevoke_InsufficientScope(t *testing.T) {
	b := newTestBroker(t)

	// Issue a token with non-admin scope.
	issResp, _ := b.tknSvc.Issue(token.IssueReq{
		Sub:   "spiffe://test/agent/o/t/i",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})

	body := jsonBody(t, map[string]string{"level": "token", "target": "jti"})
	req := httptest.NewRequest("POST", "/v1/revoke", body)
	req.Header.Set("Authorization", "Bearer "+issResp.AccessToken)
	rr := b.do(req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- Audit without admin scope ---

func TestAudit_InsufficientScope(t *testing.T) {
	b := newTestBroker(t)

	issResp, _ := b.tknSvc.Issue(token.IssueReq{
		Sub:   "spiffe://test/agent/o/t/i",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})

	req := httptest.NewRequest("GET", "/v1/audit/events", nil)
	req.Header.Set("Authorization", "Bearer "+issResp.AccessToken)
	rr := b.do(req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- Delegation via HTTP ---

func TestDelegateHTTP_Success(t *testing.T) {
	b := newTestBroker(t)

	// Register two agents via the full flow.
	adminToken := getAdminToken(t, b)

	// Register agent 1 (delegator).
	lt1 := createLaunchToken(t, b, adminToken)
	nonce1 := getNonce(t, b)
	pub1, priv1, _ := ed25519.GenerateKey(rand.Reader)
	nonceBytes1, _ := hex.DecodeString(nonce1)
	sig1 := ed25519.Sign(priv1, nonceBytes1)
	regBody1 := jsonBody(t, map[string]any{
		"launch_token":    lt1,
		"nonce":           nonce1,
		"public_key":      base64.StdEncoding.EncodeToString(pub1),
		"signature":       base64.StdEncoding.EncodeToString(sig1),
		"orch_id":         "orch-1",
		"task_id":         "task-1",
		"requested_scope": []string{"read:data:*", "write:data:*"},
	})
	regReq1 := httptest.NewRequest("POST", "/v1/register", regBody1)
	regRR1 := b.do(regReq1)
	if regRR1.Code != http.StatusOK {
		t.Fatalf("register agent 1: %d %s", regRR1.Code, regRR1.Body.String())
	}
	var regResp1 map[string]any
	_ = json.NewDecoder(regRR1.Body).Decode(&regResp1) //nolint:errcheck // test setup
	agent1Token := regResp1["access_token"].(string)

	// Register agent 2 (delegate).
	lt2 := createLaunchToken(t, b, adminToken)
	nonce2 := getNonce(t, b)
	pub2, priv2, _ := ed25519.GenerateKey(rand.Reader)
	nonceBytes2, _ := hex.DecodeString(nonce2)
	sig2 := ed25519.Sign(priv2, nonceBytes2)
	regBody2 := jsonBody(t, map[string]any{
		"launch_token":    lt2,
		"nonce":           nonce2,
		"public_key":      base64.StdEncoding.EncodeToString(pub2),
		"signature":       base64.StdEncoding.EncodeToString(sig2),
		"orch_id":         "orch-1",
		"task_id":         "task-1",
		"requested_scope": []string{"read:data:*"},
	})
	regReq2 := httptest.NewRequest("POST", "/v1/register", regBody2)
	regRR2 := b.do(regReq2)
	if regRR2.Code != http.StatusOK {
		t.Fatalf("register agent 2: %d %s", regRR2.Code, regRR2.Body.String())
	}
	var regResp2 map[string]any
	_ = json.NewDecoder(regRR2.Body).Decode(&regResp2) //nolint:errcheck // test setup
	agent2ID := regResp2["agent_id"].(string)

	// Agent 1 delegates to agent 2.
	delegBody := jsonBody(t, map[string]any{
		"delegate_to": agent2ID,
		"scope":       []string{"read:data:*"},
		"ttl":         60,
	})
	delegReq := httptest.NewRequest("POST", "/v1/delegate", delegBody)
	delegReq.Header.Set("Authorization", "Bearer "+agent1Token)
	delegRR := b.do(delegReq)
	if delegRR.Code != http.StatusOK {
		t.Fatalf("delegate: %d %s", delegRR.Code, delegRR.Body.String())
	}

	var delegResp map[string]any
	_ = json.NewDecoder(delegRR.Body).Decode(&delegResp) //nolint:errcheck // test assertion
	if delegResp["access_token"] == nil || delegResp["access_token"] == "" {
		t.Error("expected non-empty delegated access_token")
	}
	chain, ok := delegResp["delegation_chain"].([]any)
	if !ok || len(chain) == 0 {
		t.Error("expected non-empty delegation_chain")
	}
}

func TestTokenExchange_Success_SidecarIDBrokerDerived(t *testing.T) {
	b := newTestBroker(t)

	adminToken := getAdminToken(t, b)
	launchToken := createLaunchToken(t, b, adminToken)
	nonce := getNonce(t, b)

	agentPub, agentPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate agent key: %v", err)
	}
	nonceBytes, _ := hex.DecodeString(nonce)
	sig := ed25519.Sign(agentPriv, nonceBytes)

	regBody := jsonBody(t, map[string]any{
		"launch_token":    launchToken,
		"nonce":           nonce,
		"public_key":      base64.StdEncoding.EncodeToString(agentPub),
		"signature":       base64.StdEncoding.EncodeToString(sig),
		"orch_id":         "orch-sidecar",
		"task_id":         "task-sidecar",
		"requested_scope": []string{"read:data:*"},
	})
	regReq := httptest.NewRequest("POST", "/v1/register", regBody)
	regRR := b.do(regReq)
	if regRR.Code != http.StatusOK {
		t.Fatalf("register failed: %d %s", regRR.Code, regRR.Body.String())
	}
	var regResp map[string]any
	_ = json.NewDecoder(regRR.Body).Decode(&regResp) //nolint:errcheck // test setup
	agentID := regResp["agent_id"].(string)

	// Sidecar token with sidecar:manage + scope ceiling.
	sidecarResp, err := b.tknSvc.Issue(token.IssueReq{
		Sub:   "sidecar:abc123",
		Sid:   "abc123",
		Scope: []string{"sidecar:manage:*", "sidecar:scope:read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue sidecar token: %v", err)
	}

	exBody := jsonBody(t, map[string]any{
		"agent_id":   agentID,
		"scope":      []string{"read:data:*"},
		"ttl":        90,
		"sidecar_id": "spoofed-client-value",
	})
	exReq := httptest.NewRequest("POST", "/v1/token/exchange", exBody)
	exReq.Header.Set("Authorization", "Bearer "+sidecarResp.AccessToken)
	exReq.Header.Set("Content-Type", "application/json")
	exRR := b.do(exReq)
	if exRR.Code != http.StatusOK {
		t.Fatalf("token exchange failed: %d %s", exRR.Code, exRR.Body.String())
	}

	var exResp map[string]any
	_ = json.NewDecoder(exRR.Body).Decode(&exResp) //nolint:errcheck // test assertion
	if exResp["sidecar_id"] != "abc123" {
		t.Fatalf("expected broker-derived sidecar_id=abc123, got %v", exResp["sidecar_id"])
	}

	issuedToken, _ := exResp["access_token"].(string)
	claims, err := b.tknSvc.Verify(issuedToken)
	if err != nil {
		t.Fatalf("verify exchanged token: %v", err)
	}
	if claims.Sid != "abc123" {
		t.Fatalf("expected sid=abc123, got %s", claims.Sid)
	}
	if claims.SidecarID != "abc123" {
		t.Fatalf("expected sidecar_id=abc123, got %s", claims.SidecarID)
	}
	if claims.Sub != agentID {
		t.Fatalf("expected sub=%s, got %s", agentID, claims.Sub)
	}
}

func TestTokenExchange_SidFallbackToSub(t *testing.T) {
	b := newTestBroker(t)

	adminToken := getAdminToken(t, b)
	_, agentID := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	// Issue a sidecar token with NO Sid field — only Sub is set.
	// The handler should fall back to claims.Sub for sidecar_id derivation.
	sidecarResp, err := b.tknSvc.Issue(token.IssueReq{
		Sub:   "sidecar:fallback-sub",
		Scope: []string{"sidecar:manage:*", "sidecar:scope:read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue sidecar token: %v", err)
	}

	exBody := jsonBody(t, map[string]any{
		"agent_id":   agentID,
		"scope":      []string{"read:data:*"},
		"ttl":        60,
		"sidecar_id": "spoofed-value",
	})
	exReq := httptest.NewRequest("POST", "/v1/token/exchange", exBody)
	exReq.Header.Set("Authorization", "Bearer "+sidecarResp.AccessToken)
	exReq.Header.Set("Content-Type", "application/json")
	exRR := b.do(exReq)
	if exRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", exRR.Code, exRR.Body.String())
	}

	var exResp map[string]any
	if err := json.NewDecoder(exRR.Body).Decode(&exResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Response sidecar_id should be the Sub value (fallback), NOT the spoofed value
	if exResp["sidecar_id"] != "sidecar:fallback-sub" {
		t.Fatalf("expected sidecar_id=sidecar:fallback-sub, got %v", exResp["sidecar_id"])
	}

	// Verify token claims
	issuedToken, _ := exResp["access_token"].(string)
	claims, err := b.tknSvc.Verify(issuedToken)
	if err != nil {
		t.Fatalf("verify exchanged token: %v", err)
	}
	if claims.Sid != "sidecar:fallback-sub" {
		t.Fatalf("expected sid=sidecar:fallback-sub, got %s", claims.Sid)
	}
	if claims.SidecarID != "sidecar:fallback-sub" {
		t.Fatalf("expected sidecar_id=sidecar:fallback-sub, got %s", claims.SidecarID)
	}
	if claims.Sub != agentID {
		t.Fatalf("expected sub=%s (agent), got %s", agentID, claims.Sub)
	}
}

func TestTokenExchange_AntiSpoof_ClientSidecarIDIgnored(t *testing.T) {
	b := newTestBroker(t)

	adminToken := getAdminToken(t, b)
	_, agentID := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	// Issue sidecar token with explicit Sid
	sidecarResp, err := b.tknSvc.Issue(token.IssueReq{
		Sub:   "sidecar:real-sidecar",
		Sid:   "real-sid-value",
		Scope: []string{"sidecar:manage:*", "sidecar:scope:read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue sidecar token: %v", err)
	}

	// Client tries multiple spoof values
	spoofValues := []string{"attacker-sidecar", "admin:sidecar", "", "sidecar:manage:*"}
	for _, spoof := range spoofValues {
		t.Run("spoof="+spoof, func(t *testing.T) {
			exBody := jsonBody(t, map[string]any{
				"agent_id":   agentID,
				"scope":      []string{"read:data:*"},
				"ttl":        60,
				"sidecar_id": spoof,
			})
			exReq := httptest.NewRequest("POST", "/v1/token/exchange", exBody)
			exReq.Header.Set("Authorization", "Bearer "+sidecarResp.AccessToken)
			exReq.Header.Set("Content-Type", "application/json")
			exRR := b.do(exReq)
			if exRR.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", exRR.Code, exRR.Body.String())
			}

			var exResp map[string]any
			if err := json.NewDecoder(exRR.Body).Decode(&exResp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if exResp["sidecar_id"] != "real-sid-value" {
				t.Fatalf("anti-spoof failed: expected sidecar_id=real-sid-value, got %v (spoof=%s)",
					exResp["sidecar_id"], spoof)
			}

			issuedToken, _ := exResp["access_token"].(string)
			claims, err := b.tknSvc.Verify(issuedToken)
			if err != nil {
				t.Fatalf("verify exchanged token: %v", err)
			}
			if claims.Sid != "real-sid-value" {
				t.Fatalf("anti-spoof failed: expected sid=real-sid-value, got %s (spoof=%s)",
					claims.Sid, spoof)
			}
			if claims.SidecarID != "real-sid-value" {
				t.Fatalf("anti-spoof failed: expected sidecar_id=real-sid-value, got %s (spoof=%s)",
					claims.SidecarID, spoof)
			}
		})
	}
}

func TestTokenExchange_ScopeEscalationDenied(t *testing.T) {
	b := newTestBroker(t)

	sidecarResp, err := b.tknSvc.Issue(token.IssueReq{
		Sub:   "sidecar:abc123",
		Sid:   "abc123",
		Scope: []string{"sidecar:manage:*", "sidecar:scope:read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue sidecar token: %v", err)
	}

	body := jsonBody(t, map[string]any{
		"agent_id": "spiffe://agentauth.local/agent/o/t/x",
		"scope":    []string{"write:data:*"},
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+sidecarResp.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}

	var problem map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem["error_code"] != "scope_escalation_denied" {
		t.Fatalf("expected scope_escalation_denied, got %v", problem["error_code"])
	}
}

func TestTokenExchange_MalformedScope_Returns400(t *testing.T) {
	b := newTestBroker(t)

	sidecarResp, err := b.tknSvc.Issue(token.IssueReq{
		Sub:   "sidecar:abc123",
		Sid:   "abc123",
		Scope: []string{"sidecar:manage:*", "sidecar:scope:read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue sidecar token: %v", err)
	}

	tests := []struct {
		name  string
		scope []string
	}{
		{"two-segment scope", []string{"read:data"}},
		{"single-segment scope", []string{"read"}},
		{"empty-segment scope", []string{"read::*"}},
		{"scope with leading colon", []string{":data:*"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := jsonBody(t, map[string]any{
				"agent_id": "spiffe://test.local/agent/o/t/x",
				"scope":    tc.scope,
			})
			req := httptest.NewRequest("POST", "/v1/token/exchange", body)
			req.Header.Set("Authorization", "Bearer "+sidecarResp.AccessToken)
			req.Header.Set("Content-Type", "application/json")
			rr := b.do(req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}

			var problem map[string]any
			if err := json.NewDecoder(rr.Body).Decode(&problem); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if problem["error_code"] != "invalid_scope_format" {
				t.Fatalf("expected invalid_scope_format, got %v", problem["error_code"])
			}
		})
	}
}

func TestTokenExchange_WildcardIdentifierCeiling_CoversScope(t *testing.T) {
	b := newTestBroker(t)

	adminToken := getAdminToken(t, b)
	agentToken, agentID := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})
	_ = agentToken

	// Sidecar with wildcard identifier ceiling: sidecar:scope:read:data:*
	// covers read:data:project-42 because wildcard is in identifier position
	sidecarResp, err := b.tknSvc.Issue(token.IssueReq{
		Sub:   "sidecar:wildcard",
		Sid:   "wildcard-sid",
		Scope: []string{"sidecar:manage:*", "sidecar:scope:read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue sidecar token: %v", err)
	}

	body := jsonBody(t, map[string]any{
		"agent_id": agentID,
		"scope":    []string{"read:data:project-42"},
		"ttl":      60,
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+sidecarResp.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["sidecar_id"] != "wildcard-sid" {
		t.Errorf("expected sidecar_id=wildcard-sid, got %v", resp["sidecar_id"])
	}
}

func TestTokenExchange_TripleWildcardCeiling_DeniesScope(t *testing.T) {
	b := newTestBroker(t)

	// sidecar:scope:*:*:* does NOT cover read:data:* because ScopeIsSubset
	// requires exact action:resource match; wildcard only applies to identifier.
	sidecarResp, err := b.tknSvc.Issue(token.IssueReq{
		Sub:   "sidecar:wildcard",
		Sid:   "wildcard-sid",
		Scope: []string{"sidecar:manage:*", "sidecar:scope:*:*:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue sidecar token: %v", err)
	}

	body := jsonBody(t, map[string]any{
		"agent_id": "spiffe://test.local/agent/o/t/x",
		"scope":    []string{"read:data:*"},
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+sidecarResp.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}

	var problem map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem["error_code"] != "scope_escalation_denied" {
		t.Fatalf("expected scope_escalation_denied, got %v", problem["error_code"])
	}
}

func TestTokenExchange_MissingContentType_Returns400(t *testing.T) {
	b := newTestBroker(t)

	sidecarResp, err := b.tknSvc.Issue(token.IssueReq{
		Sub:   "sidecar:abc123",
		Sid:   "abc123",
		Scope: []string{"sidecar:manage:*", "sidecar:scope:read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue sidecar token: %v", err)
	}

	body := jsonBody(t, map[string]any{
		"agent_id": "spiffe://test.local/agent/o/t/x",
		"scope":    []string{"read:data:*"},
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+sidecarResp.AccessToken)
	// Deliberately NOT setting Content-Type
	rr := b.do(req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var problem map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem["error_code"] != "invalid_content_type" {
		t.Fatalf("expected invalid_content_type, got %v", problem["error_code"])
	}
}

func TestTokenExchange_SidecarScopeCeilingMissing_Returns403(t *testing.T) {
	b := newTestBroker(t)

	// Sidecar token with manage authority but NO sidecar:scope:* entries
	sidecarResp, err := b.tknSvc.Issue(token.IssueReq{
		Sub:   "sidecar:no-ceiling",
		Sid:   "no-ceiling-sid",
		Scope: []string{"sidecar:manage:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue sidecar token: %v", err)
	}

	body := jsonBody(t, map[string]any{
		"agent_id": "spiffe://test.local/agent/o/t/x",
		"scope":    []string{"read:data:*"},
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+sidecarResp.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}

	var problem map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem["error_code"] != "sidecar_scope_missing" {
		t.Fatalf("expected sidecar_scope_missing, got %v", problem["error_code"])
	}
}

// --- P4-T04: Comprehensive exchange validation tests ---

func issueSidecarToken(t *testing.T, b *testBroker, sid string, scopes []string) string {
	t.Helper()
	resp, err := b.tknSvc.Issue(token.IssueReq{
		Sub:   "sidecar:" + sid,
		Sid:   sid,
		Scope: scopes,
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue sidecar token: %v", err)
	}
	return resp.AccessToken
}

func TestTokenExchange_MissingAgentID_Returns400(t *testing.T) {
	b := newTestBroker(t)
	tok := issueSidecarToken(t, b, "s1", []string{"sidecar:manage:*", "sidecar:scope:read:data:*"})

	body := jsonBody(t, map[string]any{
		"scope": []string{"read:data:*"},
		"ttl":   60,
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	var p map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p["error_code"] != "missing_field" {
		t.Fatalf("expected missing_field, got %v", p["error_code"])
	}
}

func TestTokenExchange_EmptyScope_Returns400(t *testing.T) {
	b := newTestBroker(t)
	tok := issueSidecarToken(t, b, "s1", []string{"sidecar:manage:*", "sidecar:scope:read:data:*"})

	body := jsonBody(t, map[string]any{
		"agent_id": "spiffe://test.local/agent/o/t/x",
		"scope":    []string{},
		"ttl":      60,
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	var p map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p["error_code"] != "missing_field" {
		t.Fatalf("expected missing_field, got %v", p["error_code"])
	}
}

func TestTokenExchange_TTLTooHigh_Returns400(t *testing.T) {
	b := newTestBroker(t)
	tok := issueSidecarToken(t, b, "s1", []string{"sidecar:manage:*", "sidecar:scope:read:data:*"})

	body := jsonBody(t, map[string]any{
		"agent_id": "spiffe://test.local/agent/o/t/x",
		"scope":    []string{"read:data:*"},
		"ttl":      901,
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	var p map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p["error_code"] != "invalid_ttl" {
		t.Fatalf("expected invalid_ttl, got %v", p["error_code"])
	}
}

func TestTokenExchange_NegativeTTL_Returns400(t *testing.T) {
	b := newTestBroker(t)
	tok := issueSidecarToken(t, b, "s1", []string{"sidecar:manage:*", "sidecar:scope:read:data:*"})

	body := jsonBody(t, map[string]any{
		"agent_id": "spiffe://test.local/agent/o/t/x",
		"scope":    []string{"read:data:*"},
		"ttl":      -1,
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	var p map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p["error_code"] != "invalid_ttl" {
		t.Fatalf("expected invalid_ttl, got %v", p["error_code"])
	}
}

func TestTokenExchange_MissingBearer_Returns401(t *testing.T) {
	b := newTestBroker(t)

	body := jsonBody(t, map[string]any{
		"agent_id": "spiffe://test.local/agent/o/t/x",
		"scope":    []string{"read:data:*"},
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header
	rr := b.do(req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestTokenExchange_RevokedSidecarToken_Returns403(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)
	_, agentID := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	tok := issueSidecarToken(t, b, "revoke-me", []string{"sidecar:manage:*", "sidecar:scope:read:data:*"})

	// Verify the token works first
	claims, err := b.tknSvc.Verify(tok)
	if err != nil {
		t.Fatalf("verify sidecar token: %v", err)
	}

	// Revoke by JTI
	revokeBody := jsonBody(t, map[string]string{"level": "token", "target": claims.Jti})
	revokeReq := httptest.NewRequest("POST", "/v1/revoke", revokeBody)
	revokeReq.Header.Set("Authorization", "Bearer "+adminToken)
	revokeRR := b.do(revokeReq)
	if revokeRR.Code != http.StatusOK {
		t.Fatalf("revoke failed: %d %s", revokeRR.Code, revokeRR.Body.String())
	}

	// Attempt exchange with revoked token -> 403 from ValMw
	exBody := jsonBody(t, map[string]any{
		"agent_id": agentID,
		"scope":    []string{"read:data:*"},
		"ttl":      60,
	})
	exReq := httptest.NewRequest("POST", "/v1/token/exchange", exBody)
	exReq.Header.Set("Authorization", "Bearer "+tok)
	exReq.Header.Set("Content-Type", "application/json")
	exRR := b.do(exReq)

	if exRR.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for revoked sidecar token, got %d: %s", exRR.Code, exRR.Body.String())
	}
}

func TestTokenExchange_LackingSidecarManageScope_Returns403(t *testing.T) {
	b := newTestBroker(t)

	// Issue token WITHOUT sidecar:manage:* — has scope ceiling but not management authority
	tok := issueSidecarToken(t, b, "no-manage", []string{"sidecar:scope:read:data:*"})

	body := jsonBody(t, map[string]any{
		"agent_id": "spiffe://test.local/agent/o/t/x",
		"scope":    []string{"read:data:*"},
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)

	// Should be rejected by WithRequiredScope("sidecar:manage:*") middleware
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestTokenExchange_NonExistentAgent_Returns404(t *testing.T) {
	b := newTestBroker(t)
	tok := issueSidecarToken(t, b, "s1", []string{"sidecar:manage:*", "sidecar:scope:read:data:*"})

	body := jsonBody(t, map[string]any{
		"agent_id": "spiffe://test.local/agent/no/such/agent",
		"scope":    []string{"read:data:*"},
		"ttl":      60,
	})
	req := httptest.NewRequest("POST", "/v1/token/exchange", body)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	var p map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p["error_code"] != "agent_not_found" {
		t.Fatalf("expected agent_not_found, got %v", p["error_code"])
	}
}

func TestTokenExchange_MalformedJSON_Returns400(t *testing.T) {
	b := newTestBroker(t)
	tok := issueSidecarToken(t, b, "s1", []string{"sidecar:manage:*", "sidecar:scope:read:data:*"})

	req := httptest.NewRequest("POST", "/v1/token/exchange", strings.NewReader("{invalid json"))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := b.do(req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	var p map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p["error_code"] != "malformed_body" {
		t.Fatalf("expected malformed_body, got %v", p["error_code"])
	}
}

// TestTokenExchange_FullIntegration_AdminToExchange exercises the complete
// sidecar lifecycle: admin auth -> create sidecar activation -> activate ->
// exchange -> verify issued token claims.
func TestTokenExchange_FullIntegration_AdminToExchange(t *testing.T) {
	b := newTestBroker(t)

	// Step 1: Admin auth
	adminToken := getAdminToken(t, b)

	// Step 2: Register an agent
	_, agentID := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	// Step 3: Create sidecar activation token
	actBody := jsonBody(t, map[string]any{
		"allowed_scopes": []string{"read:data:*"},
		"ttl":            120,
	})
	actReq := httptest.NewRequest("POST", "/v1/admin/sidecar-activations", actBody)
	actReq.Header.Set("Authorization", "Bearer "+adminToken)
	actReq.Header.Set("Content-Type", "application/json")
	actRR := b.do(actReq)
	if actRR.Code != http.StatusCreated {
		t.Fatalf("create sidecar activation failed: %d %s", actRR.Code, actRR.Body.String())
	}
	var actResp map[string]any
	if err := json.NewDecoder(actRR.Body).Decode(&actResp); err != nil {
		t.Fatalf("decode activation resp: %v", err)
	}
	activationToken, _ := actResp["activation_token"].(string)
	if activationToken == "" {
		t.Fatal("expected non-empty activation_token")
	}

	// Step 4: Activate sidecar (exchange activation token for bearer)
	sidecarBody := jsonBody(t, map[string]any{
		"sidecar_activation_token": activationToken,
	})
	sidecarReq := httptest.NewRequest("POST", "/v1/sidecar/activate", sidecarBody)
	sidecarReq.Header.Set("Content-Type", "application/json")
	sidecarRR := b.do(sidecarReq)
	if sidecarRR.Code != http.StatusOK {
		t.Fatalf("sidecar activation failed: %d %s", sidecarRR.Code, sidecarRR.Body.String())
	}
	var sidecarResp map[string]any
	if err := json.NewDecoder(sidecarRR.Body).Decode(&sidecarResp); err != nil {
		t.Fatalf("decode sidecar resp: %v", err)
	}
	sidecarToken, _ := sidecarResp["access_token"].(string)
	sidecarID, _ := sidecarResp["sidecar_id"].(string)
	if sidecarToken == "" || sidecarID == "" {
		t.Fatalf("expected non-empty sidecar token and sidecar_id, got token=%q id=%q", sidecarToken, sidecarID)
	}

	// Step 5: Exchange sidecar token for agent token
	exBody := jsonBody(t, map[string]any{
		"agent_id":   agentID,
		"scope":      []string{"read:data:*"},
		"ttl":        60,
		"sidecar_id": "spoofed-ignored",
	})
	exReq := httptest.NewRequest("POST", "/v1/token/exchange", exBody)
	exReq.Header.Set("Authorization", "Bearer "+sidecarToken)
	exReq.Header.Set("Content-Type", "application/json")
	exRR := b.do(exReq)
	if exRR.Code != http.StatusOK {
		t.Fatalf("token exchange failed: %d %s", exRR.Code, exRR.Body.String())
	}
	var exResp map[string]any
	if err := json.NewDecoder(exRR.Body).Decode(&exResp); err != nil {
		t.Fatalf("decode exchange resp: %v", err)
	}

	// Step 6: Verify exchanged token claims
	issuedToken, _ := exResp["access_token"].(string)
	claims, err := b.tknSvc.Verify(issuedToken)
	if err != nil {
		t.Fatalf("verify exchanged token: %v", err)
	}
	if claims.Sub != agentID {
		t.Errorf("expected sub=%s, got %s", agentID, claims.Sub)
	}
	if claims.Sid == "" {
		t.Error("expected non-empty sid (broker-derived sidecar_id)")
	}
	if claims.SidecarID == "" {
		t.Error("expected non-empty sidecar_id claim")
	}
	if claims.Sid != claims.SidecarID {
		t.Errorf("expected sid == sidecar_id, got sid=%s sidecar_id=%s", claims.Sid, claims.SidecarID)
	}
	// Anti-spoof: sidecar_id should be broker-derived, not "spoofed-ignored"
	if exResp["sidecar_id"] == "spoofed-ignored" {
		t.Fatal("anti-spoof failed: client sidecar_id was used")
	}
	if exResp["token_type"] != "Bearer" {
		t.Errorf("expected token_type=Bearer, got %v", exResp["token_type"])
	}
	expiresIn, _ := exResp["expires_in"].(float64)
	if expiresIn <= 0 || expiresIn > 900 {
		t.Errorf("expected expires_in in (0,900], got %.0f", expiresIn)
	}

	// Step 7: Use exchanged token against a protected endpoint (token renew)
	renewReq := httptest.NewRequest("POST", "/v1/token/renew", nil)
	renewReq.Header.Set("Authorization", "Bearer "+issuedToken)
	renewRR := b.do(renewReq)
	if renewRR.Code != http.StatusOK {
		t.Fatalf("exchanged token rejected by protected endpoint: %d %s", renewRR.Code, renewRR.Body.String())
	}
	var renewResp map[string]any
	if err := json.NewDecoder(renewRR.Body).Decode(&renewResp); err != nil {
		t.Fatalf("decode renew resp: %v", err)
	}
	if renewResp["access_token"] == nil || renewResp["access_token"] == "" {
		t.Error("expected renewed token from protected endpoint")
	}
}

// TestSidecarActivation_ReplayDenied verifies that a sidecar activation
// token cannot be used twice (single-use enforcement).
func TestSidecarActivation_ReplayDenied(t *testing.T) {
	b := newTestBroker(t)

	adminToken := getAdminToken(t, b)

	// Create sidecar activation token
	actBody := jsonBody(t, map[string]any{
		"allowed_scopes": []string{"read:data:*"},
		"ttl":            120,
	})
	actReq := httptest.NewRequest("POST", "/v1/admin/sidecar-activations", actBody)
	actReq.Header.Set("Authorization", "Bearer "+adminToken)
	actReq.Header.Set("Content-Type", "application/json")
	actRR := b.do(actReq)
	if actRR.Code != http.StatusCreated {
		t.Fatalf("create activation: %d %s", actRR.Code, actRR.Body.String())
	}
	var actResp map[string]any
	if err := json.NewDecoder(actRR.Body).Decode(&actResp); err != nil {
		t.Fatalf("decode activation resp: %v", err)
	}
	activationToken := actResp["activation_token"].(string)

	// First activation should succeed
	body1 := jsonBody(t, map[string]any{"sidecar_activation_token": activationToken})
	req1 := httptest.NewRequest("POST", "/v1/sidecar/activate", body1)
	req1.Header.Set("Content-Type", "application/json")
	rr1 := b.do(req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first activation failed: %d %s", rr1.Code, rr1.Body.String())
	}

	// Second activation with same token should be rejected (replay)
	body2 := jsonBody(t, map[string]any{"sidecar_activation_token": activationToken})
	req2 := httptest.NewRequest("POST", "/v1/sidecar/activate", body2)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := b.do(req2)
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for activation replay, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var problem map[string]any
	if err := json.NewDecoder(rr2.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem["error_code"] != "activation_token_replayed" {
		t.Fatalf("expected activation_token_replayed, got %v", problem["error_code"])
	}
}

// --- Metrics endpoint ---

func TestMetrics(t *testing.T) {
	b := newTestBroker(t)

	req := httptest.NewRequest("GET", "/v1/metrics", nil)
	rr := b.do(req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct == "" {
		t.Error("expected Content-Type header on metrics response")
	}
}

// --- Register with bad nonce ---

func TestRegister_BadNonce(t *testing.T) {
	b := newTestBroker(t)

	adminToken := getAdminToken(t, b)
	lt := createLaunchToken(t, b, adminToken)

	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	regBody := jsonBody(t, map[string]any{
		"launch_token":    lt,
		"nonce":           "bad-nonce-not-in-store",
		"public_key":      base64.StdEncoding.EncodeToString(pub),
		"signature":       base64.StdEncoding.EncodeToString([]byte("fake-sig")),
		"orch_id":         "orch-1",
		"task_id":         "task-1",
		"requested_scope": []string{"read:data:*"},
	})
	req := httptest.NewRequest("POST", "/v1/register", regBody)
	rr := b.do(req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- Audit query params ---

func TestAudit_QueryParams(t *testing.T) {
	b := newTestBroker(t)

	// Record some events directly.
	b.auditLog.Record(audit.EventTokenIssued, "agent-x", "task-1", "orch-1", "issued")
	b.auditLog.Record(audit.EventTokenRevoked, "agent-y", "task-2", "orch-2", "revoked")

	adminToken := getAdminToken(t, b)

	// Query by event_type.
	req := httptest.NewRequest("GET", "/v1/audit/events?event_type=token_issued", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := b.do(req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck // test assertion
	events, _ := resp["events"].([]any)
	// Should include the token_issued event and possibly events from admin auth.
	found := false
	for _, e := range events {
		evt := e.(map[string]any)
		if evt["event_type"] == "token_issued" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find token_issued event in filtered results")
	}

	// Query by agent_id.
	req2 := httptest.NewRequest("GET", "/v1/audit/events?agent_id=agent-x", nil)
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	rr2 := b.do(req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr2.Code)
	}
	var resp2 map[string]any
	_ = json.NewDecoder(rr2.Body).Decode(&resp2) //nolint:errcheck // test assertion
	total2, _ := resp2["total"].(float64)
	if total2 < 1 {
		t.Errorf("expected at least 1 event for agent-x, got %.0f", total2)
	}

	// Query with time range (since far future should return 0 matching events).
	future := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	req3 := httptest.NewRequest("GET", "/v1/audit/events?since="+future, nil)
	req3.Header.Set("Authorization", "Bearer "+adminToken)
	rr3 := b.do(req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr3.Code)
	}
	var resp3 map[string]any
	_ = json.NewDecoder(rr3.Body).Decode(&resp3) //nolint:errcheck // test assertion
	total3, _ := resp3["total"].(float64)
	if total3 != 0 {
		t.Errorf("expected 0 events in far future, got %.0f", total3)
	}
}

// --- Layer 1 Security: Nonce replay rejected (HTTP level) ---

func TestRegister_NonceReplay(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)

	// Get a nonce and register an agent successfully.
	lt1 := createLaunchToken(t, b, adminToken)
	nonce := getNonce(t, b)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	nonceBytes, _ := hex.DecodeString(nonce)
	sig := ed25519.Sign(priv, nonceBytes)

	regBody := jsonBody(t, map[string]any{
		"launch_token":    lt1,
		"nonce":           nonce,
		"public_key":      base64.StdEncoding.EncodeToString(pub),
		"signature":       base64.StdEncoding.EncodeToString(sig),
		"orch_id":         "orch-1",
		"task_id":         "task-1",
		"requested_scope": []string{"read:data:*"},
	})
	regReq := httptest.NewRequest("POST", "/v1/register", regBody)
	regRR := b.do(regReq)
	if regRR.Code != http.StatusOK {
		t.Fatalf("first register: %d %s", regRR.Code, regRR.Body.String())
	}

	// Replay the same nonce with a new launch token.
	lt2 := createLaunchToken(t, b, adminToken)
	pub2, priv2, _ := ed25519.GenerateKey(rand.Reader)
	sig2 := ed25519.Sign(priv2, nonceBytes)
	regBody2 := jsonBody(t, map[string]any{
		"launch_token":    lt2,
		"nonce":           nonce,
		"public_key":      base64.StdEncoding.EncodeToString(pub2),
		"signature":       base64.StdEncoding.EncodeToString(sig2),
		"orch_id":         "orch-1",
		"task_id":         "task-2",
		"requested_scope": []string{"read:data:*"},
	})
	replayReq := httptest.NewRequest("POST", "/v1/register", regBody2)
	replayRR := b.do(replayReq)

	if replayRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for nonce replay, got %d: %s", replayRR.Code, replayRR.Body.String())
	}
}

// --- Layer 1 Security: Launch token replay rejected (HTTP level) ---

func TestRegister_LaunchTokenReplay(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)

	// Create a single-use launch token and register with it.
	lt := createLaunchToken(t, b, adminToken)
	nonce1 := getNonce(t, b)
	pub1, priv1, _ := ed25519.GenerateKey(rand.Reader)
	nonceBytes1, _ := hex.DecodeString(nonce1)
	sig1 := ed25519.Sign(priv1, nonceBytes1)
	regBody1 := jsonBody(t, map[string]any{
		"launch_token":    lt,
		"nonce":           nonce1,
		"public_key":      base64.StdEncoding.EncodeToString(pub1),
		"signature":       base64.StdEncoding.EncodeToString(sig1),
		"orch_id":         "orch-1",
		"task_id":         "task-1",
		"requested_scope": []string{"read:data:*"},
	})
	regReq1 := httptest.NewRequest("POST", "/v1/register", regBody1)
	regRR1 := b.do(regReq1)
	if regRR1.Code != http.StatusOK {
		t.Fatalf("first register: %d %s", regRR1.Code, regRR1.Body.String())
	}

	// Try to reuse the same launch token with a new nonce.
	nonce2 := getNonce(t, b)
	pub2, priv2, _ := ed25519.GenerateKey(rand.Reader)
	nonceBytes2, _ := hex.DecodeString(nonce2)
	sig2 := ed25519.Sign(priv2, nonceBytes2)
	regBody2 := jsonBody(t, map[string]any{
		"launch_token":    lt,
		"nonce":           nonce2,
		"public_key":      base64.StdEncoding.EncodeToString(pub2),
		"signature":       base64.StdEncoding.EncodeToString(sig2),
		"orch_id":         "orch-1",
		"task_id":         "task-2",
		"requested_scope": []string{"read:data:*"},
	})
	replayReq := httptest.NewRequest("POST", "/v1/register", regBody2)
	replayRR := b.do(replayReq)

	if replayRR.Code == http.StatusOK {
		t.Fatal("expected non-200 for launch token replay, but got 200")
	}
}

// --- Layer 1 Security: RFC 7807 problem response format ---

func TestProblemResponseFormat(t *testing.T) {
	b := newTestBroker(t)

	// Trigger a 400 by sending an empty token to validate.
	body := jsonBody(t, map[string]string{"token": ""})
	req := httptest.NewRequest("POST", "/v1/token/validate", body)
	rr := b.do(req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("expected Content-Type=application/problem+json, got %s", ct)
	}

	var problem map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem response: %v", err)
	}

	// Verify required RFC 7807 fields.
	for _, field := range []string{"type", "title", "status", "detail", "instance"} {
		if problem[field] == nil {
			t.Errorf("RFC 7807: missing required field %q", field)
		}
	}
	if status, ok := problem["status"].(float64); !ok || int(status) != http.StatusBadRequest {
		t.Errorf("expected status=400 in body, got %v", problem["status"])
	}
}

// --- Layer 2: Delegation rejected for over-scope (HTTP) ---

func TestDelegateHTTP_OverScope(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)

	// Register agent 1 with read:data:* scope only.
	agent1Token, _ := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	// Register agent 2.
	_, agent2ID := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	// Try to delegate write:data:* (agent 1 only has read:data:*).
	delegBody := jsonBody(t, map[string]any{
		"delegate_to": agent2ID,
		"scope":       []string{"write:data:*"},
		"ttl":         60,
	})
	delegReq := httptest.NewRequest("POST", "/v1/delegate", delegBody)
	delegReq.Header.Set("Authorization", "Bearer "+agent1Token)
	delegRR := b.do(delegReq)

	if delegRR.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for over-scope delegation, got %d: %s", delegRR.Code, delegRR.Body.String())
	}
}

// --- Layer 2: Delegation rejected for over-depth (HTTP) ---

func TestDelegateHTTP_OverDepth(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)

	// Register 7 agents (we need initial + 5 delegates to hit depth 5, then one more).
	agents := make([]struct {
		token string
		id    string
	}, 7)

	for i := range agents {
		token, id := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})
		agents[i].token = token
		agents[i].id = id
	}

	// Chain: agent0 -> agent1 -> agent2 -> agent3 -> agent4 -> agent5
	// Max depth is 5, so delegation from agent4 to agent5 should succeed
	// but agent5 to agent6 should fail.
	currentToken := agents[0].token
	for i := 0; i < 5; i++ {
		delegBody := jsonBody(t, map[string]any{
			"delegate_to": agents[i+1].id,
			"scope":       []string{"read:data:*"},
			"ttl":         60,
		})
		delegReq := httptest.NewRequest("POST", "/v1/delegate", delegBody)
		delegReq.Header.Set("Authorization", "Bearer "+currentToken)
		delegRR := b.do(delegReq)
		if delegRR.Code != http.StatusOK {
			t.Fatalf("delegation %d->%d failed: %d %s", i, i+1, delegRR.Code, delegRR.Body.String())
		}
		var delegResp map[string]any
		_ = json.NewDecoder(delegRR.Body).Decode(&delegResp) //nolint:errcheck // test setup
		currentToken = delegResp["access_token"].(string)
	}

	// This delegation (depth=6) should fail.
	delegBody := jsonBody(t, map[string]any{
		"delegate_to": agents[6].id,
		"scope":       []string{"read:data:*"},
		"ttl":         60,
	})
	delegReq := httptest.NewRequest("POST", "/v1/delegate", delegBody)
	delegReq.Header.Set("Authorization", "Bearer "+currentToken)
	delegRR := b.do(delegReq)

	if delegRR.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for over-depth delegation, got %d: %s", delegRR.Code, delegRR.Body.String())
	}
}

// --- Layer 2: Revocation at all 4 levels + denial on protected endpoints ---

func TestRevocation_TokenLevel_DeniesAccess(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)

	// Register an agent.
	agentToken, _ := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	// Get the agent's JTI by validating the token.
	valBody := jsonBody(t, map[string]string{"token": agentToken})
	valReq := httptest.NewRequest("POST", "/v1/token/validate", valBody)
	valRR := b.do(valReq)
	var valResp map[string]any
	_ = json.NewDecoder(valRR.Body).Decode(&valResp) //nolint:errcheck // test setup
	claims := valResp["claims"].(map[string]any)
	jti := claims["jti"].(string)

	// Revoke the token by JTI.
	revokeBody := jsonBody(t, map[string]string{"level": "token", "target": jti})
	revokeReq := httptest.NewRequest("POST", "/v1/revoke", revokeBody)
	revokeReq.Header.Set("Authorization", "Bearer "+adminToken)
	revokeRR := b.do(revokeReq)
	if revokeRR.Code != http.StatusOK {
		t.Fatalf("revoke failed: %d %s", revokeRR.Code, revokeRR.Body.String())
	}

	// Revoked token should be denied on protected endpoint (renew).
	renewReq := httptest.NewRequest("POST", "/v1/token/renew", nil)
	renewReq.Header.Set("Authorization", "Bearer "+agentToken)
	renewRR := b.do(renewReq)
	if renewRR.Code != http.StatusForbidden {
		t.Fatalf("expected 403 after token revocation, got %d", renewRR.Code)
	}

	// Validate endpoint should also report revoked token as invalid.
	valBody2 := jsonBody(t, map[string]string{"token": agentToken})
	valReq2 := httptest.NewRequest("POST", "/v1/token/validate", valBody2)
	valRR2 := b.do(valReq2)
	var valResp2 map[string]any
	_ = json.NewDecoder(valRR2.Body).Decode(&valResp2) //nolint:errcheck // test assertion
	if valResp2["valid"] != false {
		t.Errorf("expected valid=false after revocation, got %v", valResp2["valid"])
	}
}

func TestRevocation_AgentLevel_DeniesAccess(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)

	agentToken, agentID := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	// Revoke at agent level.
	revokeBody := jsonBody(t, map[string]string{"level": "agent", "target": agentID})
	revokeReq := httptest.NewRequest("POST", "/v1/revoke", revokeBody)
	revokeReq.Header.Set("Authorization", "Bearer "+adminToken)
	b.do(revokeReq)

	// Agent's token should now be denied.
	renewReq := httptest.NewRequest("POST", "/v1/token/renew", nil)
	renewReq.Header.Set("Authorization", "Bearer "+agentToken)
	renewRR := b.do(renewReq)
	if renewRR.Code != http.StatusForbidden {
		t.Fatalf("expected 403 after agent-level revocation, got %d", renewRR.Code)
	}
}

func TestRevocation_TaskLevel_DeniesAccess(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)

	agentToken, _ := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	// Get the task_id from the token claims.
	valBody := jsonBody(t, map[string]string{"token": agentToken})
	valReq := httptest.NewRequest("POST", "/v1/token/validate", valBody)
	valRR := b.do(valReq)
	var valResp map[string]any
	_ = json.NewDecoder(valRR.Body).Decode(&valResp) //nolint:errcheck // test setup
	claims := valResp["claims"].(map[string]any)
	taskID := claims["task_id"].(string)

	// Revoke at task level.
	revokeBody := jsonBody(t, map[string]string{"level": "task", "target": taskID})
	revokeReq := httptest.NewRequest("POST", "/v1/revoke", revokeBody)
	revokeReq.Header.Set("Authorization", "Bearer "+adminToken)
	b.do(revokeReq)

	renewReq := httptest.NewRequest("POST", "/v1/token/renew", nil)
	renewReq.Header.Set("Authorization", "Bearer "+agentToken)
	renewRR := b.do(renewReq)
	if renewRR.Code != http.StatusForbidden {
		t.Fatalf("expected 403 after task-level revocation, got %d", renewRR.Code)
	}
}

func TestRevocation_ChainLevel_DeniesAccess(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)

	// Register two agents, delegate from agent1 to agent2.
	agent1Token, agent1ID := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})
	_, agent2ID := registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	delegBody := jsonBody(t, map[string]any{
		"delegate_to": agent2ID,
		"scope":       []string{"read:data:*"},
		"ttl":         60,
	})
	delegReq := httptest.NewRequest("POST", "/v1/delegate", delegBody)
	delegReq.Header.Set("Authorization", "Bearer "+agent1Token)
	delegRR := b.do(delegReq)
	if delegRR.Code != http.StatusOK {
		t.Fatalf("delegate: %d %s", delegRR.Code, delegRR.Body.String())
	}
	var delegResp map[string]any
	_ = json.NewDecoder(delegRR.Body).Decode(&delegResp) //nolint:errcheck // test setup
	delegatedToken := delegResp["access_token"].(string)

	// Revoke the chain by root delegator (agent1ID).
	revokeBody := jsonBody(t, map[string]string{"level": "chain", "target": agent1ID})
	revokeReq := httptest.NewRequest("POST", "/v1/revoke", revokeBody)
	revokeReq.Header.Set("Authorization", "Bearer "+adminToken)
	b.do(revokeReq)

	// The delegated token should be denied.
	renewReq := httptest.NewRequest("POST", "/v1/token/renew", nil)
	renewReq.Header.Set("Authorization", "Bearer "+delegatedToken)
	renewRR := b.do(renewReq)
	if renewRR.Code != http.StatusForbidden {
		t.Fatalf("expected 403 after chain-level revocation, got %d", renewRR.Code)
	}
}

// --- Layer 2: Metrics endpoint exposes counters after flows ---

func TestMetrics_AfterExercisedFlows(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)

	// Exercise: registration flow.
	registerAgentHTTP(t, b, adminToken, []string{"read:data:*"})

	// Exercise: revocation.
	revokeBody := jsonBody(t, map[string]string{"level": "token", "target": "test-jti-metrics"})
	revokeReq := httptest.NewRequest("POST", "/v1/revoke", revokeBody)
	revokeReq.Header.Set("Authorization", "Bearer "+adminToken)
	b.do(revokeReq)

	// Fetch metrics.
	metricsReq := httptest.NewRequest("GET", "/v1/metrics", nil)
	metricsRR := b.do(metricsReq)
	if metricsRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", metricsRR.Code)
	}

	body := metricsRR.Body.String()

	// Check for expected Prometheus metric names.
	expectedMetrics := []string{
		"agentauth_tokens_issued_total",
		"agentauth_tokens_revoked_total",
		"agentauth_registrations_total",
		"agentauth_admin_auth_total",
	}
	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("expected metric %q in metrics output", metric)
		}
	}
}

// --- Layer 2: Scope escalation rejected at registration HTTP level ---

func TestRegister_ScopeEscalation(t *testing.T) {
	b := newTestBroker(t)
	adminToken := getAdminToken(t, b)

	// Create a launch token that only allows read:data:*
	body := jsonBody(t, map[string]any{
		"agent_name":    "limited-agent",
		"allowed_scope": []string{"read:data:*"},
		"max_ttl":       300,
		"ttl":           60,
	})
	req := httptest.NewRequest("POST", "/v1/admin/launch-tokens", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := b.do(req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create launch token: %d %s", rr.Code, rr.Body.String())
	}
	var ltResp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&ltResp) //nolint:errcheck // test setup
	lt := ltResp["launch_token"].(string)

	// Try to register requesting write:data:* (escalation).
	nonce := getNonce(t, b)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	nonceBytes, _ := hex.DecodeString(nonce)
	sig := ed25519.Sign(priv, nonceBytes)

	regBody := jsonBody(t, map[string]any{
		"launch_token":    lt,
		"nonce":           nonce,
		"public_key":      base64.StdEncoding.EncodeToString(pub),
		"signature":       base64.StdEncoding.EncodeToString(sig),
		"orch_id":         "orch-1",
		"task_id":         "task-1",
		"requested_scope": []string{"write:data:*"},
	})
	regReq := httptest.NewRequest("POST", "/v1/register", regBody)
	regRR := b.do(regReq)

	if regRR.Code == http.StatusOK {
		t.Fatal("expected non-200 for scope escalation, but got 200")
	}
}

// --- Helper: register an agent through the full HTTP flow ---

func registerAgentHTTP(t *testing.T, b *testBroker, adminToken string, scope []string) (agentToken, agentID string) {
	t.Helper()

	lt := createLaunchToken(t, b, adminToken)
	nonce := getNonce(t, b)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	nonceBytes, _ := hex.DecodeString(nonce)
	sig := ed25519.Sign(priv, nonceBytes)

	regBody := jsonBody(t, map[string]any{
		"launch_token":    lt,
		"nonce":           nonce,
		"public_key":      base64.StdEncoding.EncodeToString(pub),
		"signature":       base64.StdEncoding.EncodeToString(sig),
		"orch_id":         "orch-1",
		"task_id":         "task-1",
		"requested_scope": scope,
	})
	regReq := httptest.NewRequest("POST", "/v1/register", regBody)
	regRR := b.do(regReq)
	if regRR.Code != http.StatusOK {
		t.Fatalf("register agent: %d %s", regRR.Code, regRR.Body.String())
	}
	var regResp map[string]any
	_ = json.NewDecoder(regRR.Body).Decode(&regResp) //nolint:errcheck // test helper
	return regResp["access_token"].(string), regResp["agent_id"].(string)
}

// --- Middleware integration ---

func TestRequestIDPropagation(t *testing.T) {
	b := newTestBroker(t)

	t.Run("auto-generated when absent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/health", nil)
		rr := b.do(req)
		if rr.Code != http.StatusOK {
			t.Fatalf("health: %d", rr.Code)
		}
		rid := rr.Header().Get("X-Request-ID")
		if rid == "" {
			t.Fatal("expected X-Request-ID header in response")
		}
	})

	t.Run("echoed when provided", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/health", nil)
		req.Header.Set("X-Request-ID", "client-trace-42")
		rr := b.do(req)
		if rr.Code != http.StatusOK {
			t.Fatalf("health: %d", rr.Code)
		}
		if got := rr.Header().Get("X-Request-ID"); got != "client-trace-42" {
			t.Fatalf("X-Request-ID = %q, want %q", got, "client-trace-42")
		}
	})

	t.Run("present in RFC7807 error response", func(t *testing.T) {
		// Use /v1/token/validate (no auth required) with invalid body to trigger 400.
		req := httptest.NewRequest("POST", "/v1/token/validate", strings.NewReader("not-json"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "err-trace-99")
		rr := b.do(req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		var prob map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&prob); err != nil {
			t.Fatalf("decode problem: %v", err)
		}
		if rid, ok := prob["request_id"].(string); !ok || rid != "err-trace-99" {
			t.Fatalf("problem request_id = %v, want %q", prob["request_id"], "err-trace-99")
		}
	})
}

func TestMethodRestriction(t *testing.T) {
	b := newTestBroker(t)

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/token/exchange"},
		{"PUT", "/v1/token/exchange"},
		{"DELETE", "/v1/register"},
		{"PATCH", "/v1/token/renew"},
		{"GET", "/v1/revoke"},
		{"GET", "/v1/sidecar/activate"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := b.do(req)
			if rr.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s: got %d, want 405", tc.method, tc.path, rr.Code)
			}
		})
	}
}
