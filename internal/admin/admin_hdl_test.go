package admin

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/audit"
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
	adminSvc := NewAdminSvc(testSecret, tknSvc, st, nil, "")
	valMw := authz.NewValMw(tknSvc, nil, nil, "")
	hdl := NewAdminHdl(adminSvc, valMw, nil, nil, st)
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

	body, _ := json.Marshal(authReq{Secret: testSecret})
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

	body, _ := json.Marshal(authReq{Secret: "wrong"})
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

func TestHandleAuth_MissingSecret(t *testing.T) {
	mux, _, _ := newTestMux(t)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleAuth_LegacyShapeReturnsError(t *testing.T) {
	mux, _, _ := newTestMux(t)

	body, _ := json.Marshal(map[string]string{
		"client_id":     "admin",
		"client_secret": testSecret,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for legacy shape, got %d: %s", rec.Code, rec.Body.String())
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
	body, _ := json.Marshal(authReq{Secret: testSecret})
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

// Sidecar route tests removed in Phase 0 (2026-03-04). Handler methods
// remain in source for Phase 2; route tests will return when routes are
// re-wired with app-scoped activation tokens.

// --- App scope ceiling enforcement on POST /v1/admin/launch-tokens ---

// newAppTestMux builds a mux with a SQLite-backed store so that app records
// can be persisted and looked up by the handler during ceiling enforcement.
func newAppTestMux(t *testing.T) (*http.ServeMux, *AdminSvc, *token.TknSvc, *store.SqlStore, *audit.AuditLog) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st := store.NewSqlStore()
	if err := st.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tknSvc := token.NewTknSvc(priv, pub, cfg.Cfg{DefaultTTL: 300})
	al := audit.NewAuditLog(st)
	adminSvc := NewAdminSvc(testSecret, tknSvc, st, al, "")
	valMw := authz.NewValMw(tknSvc, nil, al, "")
	hdl := NewAdminHdl(adminSvc, valMw, al, nil, st)

	mux := http.NewServeMux()
	hdl.RegisterRoutes(mux)
	return mux, adminSvc, tknSvc, st, al
}

// seedApp inserts an app record into the store and returns an app JWT.
func seedApp(t *testing.T, tknSvc *token.TknSvc, st *store.SqlStore, appID string, ceiling []string) string {
	t.Helper()
	now := time.Now().UTC()
	rec := store.AppRecord{
		AppID:            appID,
		Name:             "test-app-" + appID,
		ClientID:         "cli-" + appID,
		ClientSecretHash: "unused",
		ScopeCeiling:     ceiling,
		Status:           "active",
		CreatedAt:        now,
		UpdatedAt:        now,
		CreatedBy:        "admin",
	}
	if err := st.SaveApp(rec); err != nil {
		t.Fatalf("save app: %v", err)
	}

	resp, err := tknSvc.Issue(token.IssueReq{
		Sub:   "app:" + appID,
		Scope: []string{"app:launch-tokens:*", "app:agents:*", "app:audit:read"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue app token: %v", err)
	}
	return resp.AccessToken
}

func TestCreateLaunchToken_AppCallerWithinCeiling(t *testing.T) {
	mux, _, tknSvc, st, _ := newAppTestMux(t)
	appToken := seedApp(t, tknSvc, st, "app-weather-bot-a1b2c3", []string{"read:weather:*"})

	body, _ := json.Marshal(CreateLaunchTokenReq{
		AgentName:    "weather-agent",
		AllowedScope: []string{"read:weather:current"},
		TTL:          30,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/launch-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+appToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp CreateLaunchTokenResp
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.LaunchToken == "" {
		t.Error("expected non-empty launch_token")
	}
}

func TestCreateLaunchToken_AppCallerExceedsCeiling(t *testing.T) {
	mux, _, tknSvc, st, _ := newAppTestMux(t)
	appToken := seedApp(t, tknSvc, st, "app-weather-bot-c3d4e5", []string{"read:weather:*"})

	body, _ := json.Marshal(CreateLaunchTokenReq{
		AgentName:    "data-writer",
		AllowedScope: []string{"write:data:all"},
		TTL:          30,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/launch-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+appToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the error message mentions the ceiling.
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "ceiling") {
		t.Errorf("expected error message to mention ceiling, got: %s", respBody)
	}
}

func TestCreateLaunchToken_AppCallerTokenCarriesAppID(t *testing.T) {
	mux, adminSvc, tknSvc, st, _ := newAppTestMux(t)
	appID := "app-weather-bot-f6g7h8"
	appToken := seedApp(t, tknSvc, st, appID, []string{"read:weather:*"})

	body, _ := json.Marshal(CreateLaunchTokenReq{
		AgentName:    "weather-agent",
		AllowedScope: []string{"read:weather:current"},
		TTL:          30,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/launch-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+appToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp CreateLaunchTokenResp
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Verify the launch token record carries the app ID.
	tokenRec, err := adminSvc.ValidateLaunchToken(resp.LaunchToken)
	if err != nil {
		t.Fatalf("validate launch token: %v", err)
	}
	if tokenRec.AppID != appID {
		t.Errorf("expected AppID=%q, got %q", appID, tokenRec.AppID)
	}
}

func TestCreateLaunchToken_AdminCallerNoCeilingCheck(t *testing.T) {
	mux, adminSvc, _, _, _ := newAppTestMux(t)
	adminToken := getAdminToken(t, mux)

	body, _ := json.Marshal(CreateLaunchTokenReq{
		AgentName:    "unrestricted-agent",
		AllowedScope: []string{"write:data:all", "read:everything:*"},
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
		t.Fatalf("decode: %v", err)
	}

	// Admin-created tokens should have empty AppID.
	tokenRec, err := adminSvc.ValidateLaunchToken(resp.LaunchToken)
	if err != nil {
		t.Fatalf("validate launch token: %v", err)
	}
	if tokenRec.AppID != "" {
		t.Errorf("expected empty AppID for admin caller, got %q", tokenRec.AppID)
	}
}

func TestCreateLaunchToken_AdminCallerStillWorks(t *testing.T) {
	// Regression: existing admin flow must remain unchanged.
	mux, _, _, _, _ := newAppTestMux(t)
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
		t.Fatalf("decode: %v", err)
	}
	if resp.LaunchToken == "" {
		t.Error("expected non-empty launch_token")
	}
	if resp.ExpiresAt == "" {
		t.Error("expected non-empty expires_at")
	}
	if len(resp.Policy.AllowedScope) != 1 || resp.Policy.AllowedScope[0] != "read:Customers:*" {
		t.Errorf("unexpected policy scope: %v", resp.Policy.AllowedScope)
	}
}

func TestCreateLaunchToken_AppCallerAuditOnCeilingExceeded(t *testing.T) {
	mux, _, tknSvc, st, al := newAppTestMux(t)
	appToken := seedApp(t, tknSvc, st, "app-audit-test-d4e5f6", []string{"read:weather:*"})

	body, _ := json.Marshal(CreateLaunchTokenReq{
		AgentName:    "bad-agent",
		AllowedScope: []string{"write:data:all"},
		TTL:          30,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/launch-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+appToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify audit event was recorded.
	events := al.Events()
	found := false
	for _, e := range events {
		if e.EventType == audit.EventScopeCeilingExceeded {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected audit event EventScopeCeilingExceeded")
	}
}
