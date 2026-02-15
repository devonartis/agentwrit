package admin

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
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
	hdl := NewAdminHdl(adminSvc, valMw, nil)
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
