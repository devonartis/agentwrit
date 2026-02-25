package admin

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

func newTestHandler(t *testing.T) (*AdminHdl, *AdminSvc, *token.TknSvc) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tknSvc := token.NewTknSvc(priv, pub, cfg.Cfg{DefaultTTL: 300})
	st := store.NewSqlStore()
	adminSvc := NewAdminSvc(testSecret, tknSvc, st, nil)
	valMw := authz.NewValMw(tknSvc, nil, nil)
	hdl := NewAdminHdl(adminSvc, valMw, nil, nil)
	return hdl, adminSvc, tknSvc
}

func newTestMux(t *testing.T) (*http.ServeMux, *AdminSvc, *token.TknSvc) {
	t.Helper()
	hdl, svc, tknSvc := newTestHandler(t)
	mux := http.NewServeMux()
	hdl.RegisterRoutes(mux)
	return mux, svc, tknSvc
}

// --- POST /v1/admin/auth ---

func TestHandleAuth_Success(t *testing.T) {
	mux, _, _ := newTestMux(t)

	body, _ := json.Marshal(authReq{
		ClientID:     "admin-client",
		ClientSecret: testSecret,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp authResp
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("expected token_type=Bearer, got %s", resp.TokenType)
	}
	if resp.ExpiresIn != adminTTL {
		t.Errorf("expected expires_in=%d, got %d", adminTTL, resp.ExpiresIn)
	}
}

func TestHandleAuth_WrongSecret(t *testing.T) {
	mux, _, _ := newTestMux(t)

	body, _ := json.Marshal(authReq{
		ClientID:     "admin-client",
		ClientSecret: "wrong",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("expected problem+json content type, got %s", ct)
	}
}

func TestHandleAuth_MissingFields(t *testing.T) {
	mux, _, _ := newTestMux(t)

	body, _ := json.Marshal(map[string]string{"client_id": "admin-client"})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleAuth_MalformedJSON(t *testing.T) {
	mux, _, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// --- POST /v1/admin/launch-tokens ---

func getAdminToken(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	body, _ := json.Marshal(authReq{
		ClientID:     "admin-client",
		ClientSecret: testSecret,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp authResp
	_ = json.NewDecoder(rec.Body).Decode(&resp) //nolint:errcheck // test helper
	return resp.AccessToken
}

func TestHandleCreateLaunchToken_Success(t *testing.T) {
	mux, _, _ := newTestMux(t)
	adminToken := getAdminToken(t, mux)

	body, _ := json.Marshal(CreateLaunchTokenReq{
		AgentName:    "data-reader",
		AllowedScope: []string{"read:Customers:*"},
		MaxTTL:       300,
		TTL:          30,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/launch-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp CreateLaunchTokenResp
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.LaunchToken == "" {
		t.Error("expected non-empty launch_token")
	}
	if resp.ExpiresAt == "" {
		t.Error("expected non-empty expires_at")
	}
	if len(resp.Policy.AllowedScope) != 1 {
		t.Errorf("expected 1 scope in policy, got %d", len(resp.Policy.AllowedScope))
	}
}

func TestHandleCreateLaunchToken_NoAuth(t *testing.T) {
	mux, _, _ := newTestMux(t)

	body, _ := json.Marshal(CreateLaunchTokenReq{
		AgentName:    "data-reader",
		AllowedScope: []string{"read:Customers:*"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/launch-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateLaunchToken_WrongScope(t *testing.T) {
	mux, _, tknSvc := newTestMux(t)

	// Issue a token with agent-level scope (not admin).
	agentResp, err := tknSvc.Issue(token.IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch/task/inst",
		Scope: []string{"read:Customers:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue agent token: %v", err)
	}

	body, _ := json.Marshal(CreateLaunchTokenReq{
		AgentName:    "data-reader",
		AllowedScope: []string{"read:Customers:*"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/launch-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+agentResp.AccessToken)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateLaunchToken_MissingAgentName(t *testing.T) {
	mux, _, _ := newTestMux(t)
	adminToken := getAdminToken(t, mux)

	body, _ := json.Marshal(CreateLaunchTokenReq{
		AllowedScope: []string{"read:Customers:*"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/launch-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateLaunchToken_EmptyScope(t *testing.T) {
	mux, _, _ := newTestMux(t)
	adminToken := getAdminToken(t, mux)

	body, _ := json.Marshal(CreateLaunchTokenReq{
		AgentName:    "agent-x",
		AllowedScope: []string{},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/launch-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateSidecarActivation_Success(t *testing.T) {
	mux, _, _ := newTestMux(t)
	adminToken := getAdminToken(t, mux)

	body, _ := json.Marshal(CreateSidecarActivationReq{
		AllowedScopes: []string{"read:Customers"},
		TTL:           120,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/sidecar-activations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp CreateSidecarActivationResp
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ActivationToken == "" {
		t.Fatal("expected activation_token")
	}
}

func TestHandleActivateSidecar_SuccessAndReplay(t *testing.T) {
	mux, _, _ := newTestMux(t)
	adminToken := getAdminToken(t, mux)

	createBody, _ := json.Marshal(CreateSidecarActivationReq{
		AllowedScopes: []string{"read:Customers"},
		TTL:           120,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/v1/admin/sidecar-activations", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}

	var createResp CreateSidecarActivationResp
	if err := json.NewDecoder(createRec.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	activateBody, _ := json.Marshal(ActivateSidecarReq{
		SidecarActivationToken: createResp.ActivationToken,
	})
	activateReq := httptest.NewRequest(http.MethodPost, "/v1/sidecar/activate", bytes.NewReader(activateBody))
	activateReq.Header.Set("Content-Type", "application/json")
	activateRec := httptest.NewRecorder()
	mux.ServeHTTP(activateRec, activateReq)
	if activateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", activateRec.Code, activateRec.Body.String())
	}

	var activateResp ActivateSidecarResp
	if err := json.NewDecoder(activateRec.Body).Decode(&activateResp); err != nil {
		t.Fatalf("decode activate response: %v", err)
	}
	if activateResp.AccessToken == "" || activateResp.SidecarID == "" {
		t.Fatalf("expected sidecar token response")
	}

	// Replay should be rejected.
	replayReq := httptest.NewRequest(http.MethodPost, "/v1/sidecar/activate", bytes.NewReader(activateBody))
	replayReq.Header.Set("Content-Type", "application/json")
	replayRec := httptest.NewRecorder()
	mux.ServeHTTP(replayRec, replayReq)
	if replayRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", replayRec.Code, replayRec.Body.String())
	}
	var problem map[string]any
	if err := json.NewDecoder(replayRec.Body).Decode(&problem); err != nil {
		t.Fatalf("decode replay problem: %v", err)
	}
	if problem["error_code"] != "activation_token_replayed" {
		t.Fatalf("expected error_code=activation_token_replayed, got %v", problem["error_code"])
	}
}

func TestHandleActivateSidecar_InvalidToken(t *testing.T) {
	mux, _, _ := newTestMux(t)
	body, _ := json.Marshal(ActivateSidecarReq{
		SidecarActivationToken: "bad-token",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sidecar/activate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- GET /v1/admin/sidecars/{id}/ceiling ---

func TestHandleGetCeiling_Success(t *testing.T) {
	mux, svc, _ := newTestMux(t)
	adminToken := getAdminToken(t, mux)

	// Seed a ceiling in the store.
	_ = svc.store.SaveCeiling("sc-test-1", []string{"read:Customers:*"})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/sidecars/sc-test-1/ceiling", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ceilingResp
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.SidecarID != "sc-test-1" {
		t.Fatalf("expected sidecar_id=sc-test-1, got %s", resp.SidecarID)
	}
	if len(resp.ScopeCeiling) != 1 || resp.ScopeCeiling[0] != "read:Customers:*" {
		t.Fatalf("unexpected ceiling: %v", resp.ScopeCeiling)
	}
}

func TestHandleGetCeiling_NotFound(t *testing.T) {
	mux, _, _ := newTestMux(t)
	adminToken := getAdminToken(t, mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/sidecars/nonexistent/ceiling", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetCeiling_NoAuth(t *testing.T) {
	mux, _, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/sidecars/sc-1/ceiling", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- PUT /v1/admin/sidecars/{id}/ceiling ---

func TestHandleUpdateCeiling_Success(t *testing.T) {
	mux, svc, _ := newTestMux(t)
	adminToken := getAdminToken(t, mux)

	_ = svc.store.SaveCeiling("sc-upd-1", []string{"read:Customers:*", "write:Orders:*"})

	body, _ := json.Marshal(updateCeilingReq{
		ScopeCeiling: []string{"read:Customers:*"},
	})
	req := httptest.NewRequest(http.MethodPut, "/v1/admin/sidecars/sc-upd-1/ceiling", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result CeilingUpdateResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.OldCeiling) != 2 {
		t.Fatalf("expected 2 old scopes, got %d", len(result.OldCeiling))
	}
	if !result.Narrowed {
		t.Fatal("expected narrowed=true")
	}
}

func TestHandleUpdateCeiling_EmptyScope(t *testing.T) {
	mux, _, _ := newTestMux(t)
	adminToken := getAdminToken(t, mux)

	body, _ := json.Marshal(updateCeilingReq{
		ScopeCeiling: []string{},
	})
	req := httptest.NewRequest(http.MethodPut, "/v1/admin/sidecars/sc-1/ceiling", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateCeiling_InvalidScopeFormat(t *testing.T) {
	mux, svc, _ := newTestMux(t)
	adminToken := getAdminToken(t, mux)

	_ = svc.store.SaveCeiling("sc-bad", []string{"read:Customers:*"})

	body, _ := json.Marshal(updateCeilingReq{
		ScopeCeiling: []string{"bad-scope"},
	})
	req := httptest.NewRequest(http.MethodPut, "/v1/admin/sidecars/sc-bad/ceiling", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateCeiling_NoAuth(t *testing.T) {
	mux, _, _ := newTestMux(t)

	body, _ := json.Marshal(updateCeilingReq{
		ScopeCeiling: []string{"read:Customers:*"},
	})
	req := httptest.NewRequest(http.MethodPut, "/v1/admin/sidecars/sc-1/ceiling", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateCeiling_NarrowingTriggersRevocation(t *testing.T) {
	// Build handler with a real RevSvc so we can verify revocation.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tknSvc := token.NewTknSvc(priv, pub, cfg.Cfg{DefaultTTL: 300})
	st := store.NewSqlStore()
	adminSvc := NewAdminSvc(testSecret, tknSvc, st, nil)
	valMw := authz.NewValMw(tknSvc, nil, nil)
	revSvc := revoke.NewRevSvc(nil)
	hdl := NewAdminHdl(adminSvc, valMw, nil, revSvc)

	mux := http.NewServeMux()
	hdl.RegisterRoutes(mux)
	adminToken := getAdminToken(t, mux)

	// Seed a ceiling.
	_ = st.SaveCeiling("sc-rev-1", []string{"read:Customers:*", "write:Orders:*"})

	// Narrow the ceiling.
	body, _ := json.Marshal(updateCeilingReq{
		ScopeCeiling: []string{"read:Customers:*"},
	})
	req := httptest.NewRequest(http.MethodPut, "/v1/admin/sidecars/sc-rev-1/ceiling", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result CeilingUpdateResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !result.Narrowed {
		t.Fatal("expected narrowed=true")
	}
	if !result.Revoked {
		t.Fatal("expected revoked=true after narrowing")
	}
	if result.RevokedCount != 1 {
		t.Fatalf("expected revoked_count=1, got %d", result.RevokedCount)
	}

	// Verify the sidecar token is actually revoked.
	sidecarClaims := &token.TknClaims{Sub: "sidecar:sc-rev-1"}
	if !revSvc.IsRevoked(sidecarClaims) {
		t.Fatal("expected sidecar token to be revoked")
	}
}

func TestHandleUpdateCeiling_WideningNoRevocation(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tknSvc := token.NewTknSvc(priv, pub, cfg.Cfg{DefaultTTL: 300})
	st := store.NewSqlStore()
	adminSvc := NewAdminSvc(testSecret, tknSvc, st, nil)
	valMw := authz.NewValMw(tknSvc, nil, nil)
	revSvc := revoke.NewRevSvc(nil)
	hdl := NewAdminHdl(adminSvc, valMw, nil, revSvc)

	mux := http.NewServeMux()
	hdl.RegisterRoutes(mux)
	adminToken := getAdminToken(t, mux)

	// Seed a ceiling with one scope.
	_ = st.SaveCeiling("sc-wide-1", []string{"read:Customers:*"})

	// Widen the ceiling (add a scope).
	body, _ := json.Marshal(updateCeilingReq{
		ScopeCeiling: []string{"read:Customers:*", "write:Orders:*"},
	})
	req := httptest.NewRequest(http.MethodPut, "/v1/admin/sidecars/sc-wide-1/ceiling", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result CeilingUpdateResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Narrowed {
		t.Fatal("expected narrowed=false for widening")
	}
	if result.Revoked {
		t.Fatal("expected revoked=false when not narrowed")
	}

	// Verify the sidecar token is NOT revoked.
	sidecarClaims := &token.TknClaims{Sub: "sidecar:sc-wide-1"}
	if revSvc.IsRevoked(sidecarClaims) {
		t.Fatal("sidecar token should not be revoked on widening")
	}
}

// --- GET /v1/admin/sidecars (integration) ---

// TestListSidecars_Integration exercises the full list-sidecars flow
// through HTTP: admin auth -> create activation token -> activate sidecar
// -> list sidecars. It uses a real SQLite database to verify the entire
// persistence chain end-to-end.
func TestListSidecars_Integration(t *testing.T) {
	// Setup: SQLite-backed store, Ed25519 key pair, all real services.
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st := store.NewSqlStore()
	if err := st.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer st.Close()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tknSvc := token.NewTknSvc(priv, pub, cfg.Cfg{DefaultTTL: 300, TrustDomain: "test.local"})
	auditLog := audit.NewAuditLog(st)
	revSvc := revoke.NewRevSvc(nil)
	adminSvc := NewAdminSvc(testSecret, tknSvc, st, auditLog)
	valMw := authz.NewValMw(tknSvc, revSvc, auditLog)
	hdl := NewAdminHdl(adminSvc, valMw, auditLog, revSvc)

	mux := http.NewServeMux()
	hdl.RegisterRoutes(mux)

	// 1. Authenticate as admin.
	adminToken := getAdminToken(t, mux)
	if adminToken == "" {
		t.Fatal("expected non-empty admin token")
	}

	// 2. Create sidecar activation token.
	actBody, _ := json.Marshal(CreateSidecarActivationReq{
		AllowedScopes: []string{"read:customer:*", "write:customer:*"},
		TTL:           120,
	})
	actReq := httptest.NewRequest(http.MethodPost, "/v1/admin/sidecar-activations", bytes.NewReader(actBody))
	actReq.Header.Set("Content-Type", "application/json")
	actReq.Header.Set("Authorization", "Bearer "+adminToken)
	actRec := httptest.NewRecorder()
	mux.ServeHTTP(actRec, actReq)
	if actRec.Code != http.StatusCreated {
		t.Fatalf("sidecar activation: expected 201, got %d: %s", actRec.Code, actRec.Body.String())
	}

	var actResp CreateSidecarActivationResp
	if err := json.NewDecoder(actRec.Body).Decode(&actResp); err != nil {
		t.Fatalf("decode activation response: %v", err)
	}
	if actResp.ActivationToken == "" {
		t.Fatal("expected non-empty activation_token")
	}

	// 3. Exchange activation token for a sidecar.
	exchBody, _ := json.Marshal(ActivateSidecarReq{
		SidecarActivationToken: actResp.ActivationToken,
	})
	exchReq := httptest.NewRequest(http.MethodPost, "/v1/sidecar/activate", bytes.NewReader(exchBody))
	exchReq.Header.Set("Content-Type", "application/json")
	exchRec := httptest.NewRecorder()
	mux.ServeHTTP(exchRec, exchReq)
	if exchRec.Code != http.StatusOK {
		t.Fatalf("sidecar activate: expected 200, got %d: %s", exchRec.Code, exchRec.Body.String())
	}

	// 4. List sidecars.
	listReq := httptest.NewRequest(http.MethodGet, "/v1/admin/sidecars", nil)
	listReq.Header.Set("Authorization", "Bearer "+adminToken)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list sidecars: expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var listResp listSidecarsResp
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}

	// 5. Verify the response.
	if listResp.Total != 1 {
		t.Fatalf("expected total=1, got %d", listResp.Total)
	}
	sc := listResp.Sidecars[0]
	if sc.SidecarID == "" {
		t.Error("expected non-empty sidecar_id")
	}
	if sc.Status != "active" {
		t.Errorf("expected status=active, got %s", sc.Status)
	}
	if len(sc.ScopeCeiling) != 2 {
		t.Errorf("expected 2 scope ceiling entries, got %d", len(sc.ScopeCeiling))
	}
	if sc.CreatedAt == "" {
		t.Error("expected non-empty created_at")
	}
	if sc.UpdatedAt == "" {
		t.Error("expected non-empty updated_at")
	}
}
