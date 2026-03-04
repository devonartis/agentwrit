package app

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

	"github.com/divineartis/agentauth/internal/admin"
	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

const testAdminSecret = "test-admin-secret-32bytes-long!!"

// newTestAppMux sets up a mux with both AdminHdl (for auth) and AppHdl.
func newTestAppMux(t *testing.T) (*http.ServeMux, *AppSvc) {
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
	al := audit.NewAuditLog(nil)

	adminSvc := admin.NewAdminSvc(testAdminSecret, tknSvc, st, al, "")
	valMw := authz.NewValMw(tknSvc, nil, nil, "")

	appSvc := NewAppSvc(st, tknSvc, al, "")
	appHdl := NewAppHdl(appSvc, valMw)
	adminHdl := admin.NewAdminHdl(adminSvc, valMw, al, nil, st)

	mux := http.NewServeMux()
	adminHdl.RegisterRoutes(mux)
	appHdl.RegisterRoutes(mux)

	return mux, appSvc
}

// getAdminToken obtains a valid admin Bearer token from the mux.
func getAdminToken(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"secret": testAdminSecret,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin auth failed: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode admin token: %v", err)
	}
	return resp.AccessToken
}

// authReq fires an authenticated request and returns the recorder.
func authReq(t *testing.T, mux *http.ServeMux, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// POST /v1/admin/apps — register
// ---------------------------------------------------------------------------

func TestHandleRegisterApp_Success(t *testing.T) {
	mux, _ := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	rec := authReq(t, mux, http.MethodPost, "/v1/admin/apps",
		map[string]any{"name": "weather-bot", "scopes": []string{"read:weather:*"}},
		tok)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		AppID        string   `json:"app_id"`
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		Scopes       []string `json:"scopes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AppID == "" {
		t.Error("expected non-empty app_id")
	}
	if resp.ClientID == "" {
		t.Error("expected non-empty client_id")
	}
	if resp.ClientSecret == "" {
		t.Error("expected non-empty client_secret")
	}
	if len(resp.Scopes) != 1 {
		t.Errorf("expected 1 scope, got %d", len(resp.Scopes))
	}
}

func TestHandleRegisterApp_Unauthenticated(t *testing.T) {
	mux, _ := newTestAppMux(t)

	rec := authReq(t, mux, http.MethodPost, "/v1/admin/apps",
		map[string]any{"name": "weather-bot", "scopes": []string{"read:weather:*"}},
		"") // no token

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleRegisterApp_InvalidName(t *testing.T) {
	mux, _ := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	rec := authReq(t, mux, http.MethodPost, "/v1/admin/apps",
		map[string]any{"name": "Bad Name!", "scopes": []string{"read:data:*"}},
		tok)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleRegisterApp_MissingName(t *testing.T) {
	mux, _ := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	rec := authReq(t, mux, http.MethodPost, "/v1/admin/apps",
		map[string]any{"scopes": []string{"read:data:*"}},
		tok)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GET /v1/admin/apps — list
// ---------------------------------------------------------------------------

func TestHandleListApps_Empty(t *testing.T) {
	mux, _ := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	rec := authReq(t, mux, http.MethodGet, "/v1/admin/apps", nil, tok)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Apps  []any `json:"apps"`
		Total int   `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 0 {
		t.Errorf("expected total=0, got %d", resp.Total)
	}
}

func TestHandleListApps_AfterRegister(t *testing.T) {
	mux, appSvc := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	if _, err := appSvc.RegisterApp("app-one", []string{"read:data:*"}, "admin"); err != nil {
		t.Fatalf("seed app: %v", err)
	}

	rec := authReq(t, mux, http.MethodGet, "/v1/admin/apps", nil, tok)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp struct {
		Apps  []any `json:"apps"`
		Total int   `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected total=1, got %d", resp.Total)
	}
}

func TestHandleListApps_NoSecretHashInResponse(t *testing.T) {
	mux, appSvc := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	if _, err := appSvc.RegisterApp("secret-app", []string{"read:data:*"}, "admin"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := authReq(t, mux, http.MethodGet, "/v1/admin/apps", nil, tok)

	if strings.Contains(rec.Body.String(), "client_secret_hash") {
		t.Error("response must not contain client_secret_hash")
	}
}

// ---------------------------------------------------------------------------
// GET /v1/admin/apps/{id} — get
// ---------------------------------------------------------------------------

func TestHandleGetApp_Success(t *testing.T) {
	mux, appSvc := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	reg, err := appSvc.RegisterApp("my-app", []string{"read:data:*"}, "admin")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := authReq(t, mux, http.MethodGet, "/v1/admin/apps/"+reg.AppID, nil, tok)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AppID  string `json:"app_id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.AppID != reg.AppID {
		t.Errorf("app_id: want %q, got %q", reg.AppID, resp.AppID)
	}
	if resp.Status != "active" {
		t.Errorf("status: want %q, got %q", "active", resp.Status)
	}
}

func TestHandleGetApp_NotFound(t *testing.T) {
	mux, _ := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	rec := authReq(t, mux, http.MethodGet, "/v1/admin/apps/app-nonexistent-000000", nil, tok)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// PUT /v1/admin/apps/{id} — update
// ---------------------------------------------------------------------------

func TestHandleUpdateApp_Success(t *testing.T) {
	mux, appSvc := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	reg, err := appSvc.RegisterApp("my-app", []string{"read:data:*"}, "admin")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := authReq(t, mux, http.MethodPut, "/v1/admin/apps/"+reg.AppID,
		map[string]any{"scopes": []string{"read:data:*", "write:data:*"}},
		tok)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AppID     string   `json:"app_id"`
		Scopes    []string `json:"scopes"`
		UpdatedAt string   `json:"updated_at"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(resp.Scopes))
	}
	if resp.UpdatedAt == "" {
		t.Error("expected non-empty updated_at")
	}
}

func TestHandleUpdateApp_NotFound(t *testing.T) {
	mux, _ := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	rec := authReq(t, mux, http.MethodPut, "/v1/admin/apps/app-nonexistent-000000",
		map[string]any{"scopes": []string{"read:data:*"}},
		tok)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// DELETE /v1/admin/apps/{id} — deregister
// ---------------------------------------------------------------------------

func TestHandleDeregisterApp_Success(t *testing.T) {
	mux, appSvc := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	reg, err := appSvc.RegisterApp("my-app", []string{"read:data:*"}, "admin")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := authReq(t, mux, http.MethodDelete, "/v1/admin/apps/"+reg.AppID, nil, tok)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AppID          string `json:"app_id"`
		Status         string `json:"status"`
		DeregisteredAt string `json:"deregistered_at"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "inactive" {
		t.Errorf("status: want %q, got %q", "inactive", resp.Status)
	}
	if resp.DeregisteredAt == "" {
		t.Error("expected non-empty deregistered_at")
	}
}

func TestHandleDeregisterApp_NotFound(t *testing.T) {
	mux, _ := newTestAppMux(t)
	tok := getAdminToken(t, mux)

	rec := authReq(t, mux, http.MethodDelete, "/v1/admin/apps/app-nonexistent-000000", nil, tok)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// POST /v1/app/auth — app authenticates (no Bearer required)
// ---------------------------------------------------------------------------

func TestHandleAppAuth_Success(t *testing.T) {
	mux, appSvc := newTestAppMux(t)

	reg, err := appSvc.RegisterApp("my-app", []string{"read:data:*"}, "admin")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := authReq(t, mux, http.MethodPost, "/v1/app/auth",
		map[string]string{"client_id": reg.ClientID, "client_secret": reg.ClientSecret},
		"") // no Bearer token — this IS the auth endpoint

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if resp.ExpiresIn != appTokenTTL {
		t.Errorf("expires_in: want %d, got %d", appTokenTTL, resp.ExpiresIn)
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("token_type: want %q, got %q", "Bearer", resp.TokenType)
	}
}

func TestHandleAppAuth_WrongSecret(t *testing.T) {
	mux, appSvc := newTestAppMux(t)

	reg, err := appSvc.RegisterApp("my-app", []string{"read:data:*"}, "admin")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := authReq(t, mux, http.MethodPost, "/v1/app/auth",
		map[string]string{"client_id": reg.ClientID, "client_secret": "wrong"},
		"")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleAppAuth_UnknownClientID(t *testing.T) {
	mux, _ := newTestAppMux(t)

	rec := authReq(t, mux, http.MethodPost, "/v1/app/auth",
		map[string]string{"client_id": "nonexistent", "client_secret": "any"},
		"")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleAppAuth_DeregisteredApp(t *testing.T) {
	mux, appSvc := newTestAppMux(t)

	reg, err := appSvc.RegisterApp("my-app", []string{"read:data:*"}, "admin")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := appSvc.DeregisterApp(reg.AppID, "admin"); err != nil {
		t.Fatalf("deregister: %v", err)
	}

	rec := authReq(t, mux, http.MethodPost, "/v1/app/auth",
		map[string]string{"client_id": reg.ClientID, "client_secret": reg.ClientSecret},
		"")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after deregister, got %d", rec.Code)
	}
}
