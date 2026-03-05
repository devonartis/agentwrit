package app

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

func newTestAppSvc(t *testing.T) *AppSvc {
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
	return NewAppSvc(st, tknSvc, nil, "test-audience", 1800)
}

// TestRegisterApp_Success verifies an app can be registered and credentials returned.
func TestRegisterApp_Success(t *testing.T) {
	svc := newTestAppSvc(t)

	resp, err := svc.RegisterApp("weather-bot", []string{"read:weather:*", "write:logs:*"}, "admin", 0)
	if err != nil {
		t.Fatalf("RegisterApp failed: %v", err)
	}

	if resp.AppID == "" {
		t.Error("expected non-empty AppID")
	}
	if resp.ClientID == "" {
		t.Error("expected non-empty ClientID")
	}
	if resp.ClientSecret == "" {
		t.Error("expected non-empty ClientSecret (only returned once)")
	}
	if len(resp.ScopeCeiling) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(resp.ScopeCeiling))
	}
	if !strings.HasPrefix(resp.AppID, "app-weather-bot-") {
		t.Errorf("AppID format wrong: %q", resp.AppID)
	}
}

// TestRegisterApp_DuplicateName verifies duplicate app name is rejected.
func TestRegisterApp_DuplicateName(t *testing.T) {
	svc := newTestAppSvc(t)

	if _, err := svc.RegisterApp("my-app", []string{"read:data:*"}, "admin", 0); err != nil {
		t.Fatalf("first RegisterApp failed: %v", err)
	}
	if _, err := svc.RegisterApp("my-app", []string{"read:data:*"}, "admin", 0); err == nil {
		t.Error("expected error on duplicate name, got nil")
	}
}

// TestRegisterApp_InvalidName verifies names that fail format validation are rejected.
func TestRegisterApp_InvalidName(t *testing.T) {
	svc := newTestAppSvc(t)

	cases := []string{
		"",            // empty
		"My App",      // spaces
		"my_app",      // underscores
		"-my-app",     // starts with hyphen
		"my--app",     // consecutive hyphens
		"1myapp",      // starts with digit
	}
	for _, name := range cases {
		if _, err := svc.RegisterApp(name, []string{"read:data:*"}, "admin", 0); err == nil {
			t.Errorf("expected error for invalid name %q, got nil", name)
		}
	}
}

// TestRegisterApp_InvalidScope verifies scope strings not in action:resource:identifier format are rejected.
func TestRegisterApp_InvalidScope(t *testing.T) {
	svc := newTestAppSvc(t)

	cases := [][]string{
		{"read"},
		{"read:weather"},
		{":weather:*"},
		{"read::*"},
	}
	for _, scopes := range cases {
		if _, err := svc.RegisterApp("valid-app", scopes, "admin", 0); err == nil {
			t.Errorf("expected error for invalid scopes %v, got nil", scopes)
		}
	}
}

// TestAuthenticateApp_Success verifies correct credentials return a JWT with app: scopes.
func TestAuthenticateApp_Success(t *testing.T) {
	svc := newTestAppSvc(t)

	reg, err := svc.RegisterApp("my-app", []string{"read:data:*"}, "admin", 0)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}

	resp, err := svc.AuthenticateApp(reg.ClientID, reg.ClientSecret)
	if err != nil {
		t.Fatalf("AuthenticateApp failed: %v", err)
	}

	if resp.AccessToken == "" {
		t.Fatal("expected non-empty access_token")
	}
	if resp.ExpiresIn != 1800 {
		t.Errorf("ExpiresIn: want 1800, got %d", resp.ExpiresIn)
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("TokenType: want %q, got %q", "Bearer", resp.TokenType)
	}
}

// TestAuthenticateApp_JWTClaims verifies the issued JWT carries the correct sub and scopes.
func TestAuthenticateApp_JWTClaims(t *testing.T) {
	svc := newTestAppSvc(t)

	reg, err := svc.RegisterApp("my-app", []string{"read:data:*"}, "admin", 0)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}

	resp, err := svc.AuthenticateApp(reg.ClientID, reg.ClientSecret)
	if err != nil {
		t.Fatalf("AuthenticateApp: %v", err)
	}

	claims := resp.Claims
	if claims == nil {
		t.Fatal("expected non-nil claims")
	}
	wantSub := "app:" + reg.AppID
	if claims.Sub != wantSub {
		t.Errorf("sub: want %q, got %q", wantSub, claims.Sub)
	}

	wantScopes := []string{"app:launch-tokens:*", "app:agents:*", "app:audit:read"}
	if len(claims.Scope) != len(wantScopes) {
		t.Fatalf("scope count: want %d, got %d", len(wantScopes), len(claims.Scope))
	}
	for i, s := range wantScopes {
		if claims.Scope[i] != s {
			t.Errorf("scope[%d]: want %q, got %q", i, s, claims.Scope[i])
		}
	}
}

// TestAuthenticateApp_WrongSecret verifies a bad secret returns an error.
func TestAuthenticateApp_WrongSecret(t *testing.T) {
	svc := newTestAppSvc(t)

	reg, err := svc.RegisterApp("my-app", []string{"read:data:*"}, "admin", 0)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}

	_, err = svc.AuthenticateApp(reg.ClientID, "wrong-secret")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

// TestAuthenticateApp_UnknownClientID verifies an unknown client_id returns ErrInvalidCredentials.
func TestAuthenticateApp_UnknownClientID(t *testing.T) {
	svc := newTestAppSvc(t)

	_, err := svc.AuthenticateApp("nonexistent-client-id", "any-secret")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

// TestAuthenticateApp_InactiveApp verifies a deregistered app cannot authenticate.
func TestAuthenticateApp_InactiveApp(t *testing.T) {
	svc := newTestAppSvc(t)

	reg, err := svc.RegisterApp("my-app", []string{"read:data:*"}, "admin", 0)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}

	if err := svc.DeregisterApp(reg.AppID, "admin"); err != nil {
		t.Fatalf("DeregisterApp: %v", err)
	}

	_, err = svc.AuthenticateApp(reg.ClientID, reg.ClientSecret)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials after deregister, got %v", err)
	}
}

// TestDeregisterApp_SoftDelete verifies deregister sets status inactive without deleting.
func TestDeregisterApp_SoftDelete(t *testing.T) {
	svc := newTestAppSvc(t)

	reg, err := svc.RegisterApp("my-app", []string{"read:data:*"}, "admin", 0)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}

	if err := svc.DeregisterApp(reg.AppID, "admin"); err != nil {
		t.Fatalf("DeregisterApp: %v", err)
	}

	// Record still accessible (soft delete).
	got, err := svc.GetApp(reg.AppID)
	if err != nil {
		t.Fatalf("GetApp after deregister: %v", err)
	}
	if got.Status != "inactive" {
		t.Errorf("status: want %q, got %q", "inactive", got.Status)
	}
}

// TestDeregisterApp_NotFound verifies deregistering a nonexistent app returns an error.
func TestDeregisterApp_NotFound(t *testing.T) {
	svc := newTestAppSvc(t)

	err := svc.DeregisterApp("app-nonexistent-000000", "admin")
	if !errors.Is(err, store.ErrAppNotFound) {
		t.Errorf("expected ErrAppNotFound, got %v", err)
	}
}

// TestListApps verifies all registered apps are returned.
func TestListApps(t *testing.T) {
	svc := newTestAppSvc(t)

	if _, err := svc.RegisterApp("app-one", []string{"read:data:*"}, "admin", 0); err != nil {
		t.Fatalf("RegisterApp app-one: %v", err)
	}
	if _, err := svc.RegisterApp("app-two", []string{"write:data:*"}, "admin", 0); err != nil {
		t.Fatalf("RegisterApp app-two: %v", err)
	}

	apps, err := svc.ListApps()
	if err != nil {
		t.Fatalf("ListApps: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

// TestGetApp_NotFound verifies ErrAppNotFound for unknown app.
func TestGetApp_NotFound(t *testing.T) {
	svc := newTestAppSvc(t)

	_, err := svc.GetApp("app-nonexistent-000000")
	if !errors.Is(err, store.ErrAppNotFound) {
		t.Errorf("expected ErrAppNotFound, got %v", err)
	}
}

// TestUpdateApp_Success verifies scope ceiling can be updated.
func TestUpdateApp_Success(t *testing.T) {
	svc := newTestAppSvc(t)

	reg, err := svc.RegisterApp("my-app", []string{"read:data:*"}, "admin", 0)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}

	newScopes := []string{"read:data:*", "write:data:*", "read:alerts:*"}
	if err := svc.UpdateApp(reg.AppID, newScopes, "admin"); err != nil {
		t.Fatalf("UpdateApp: %v", err)
	}

	got, err := svc.GetApp(reg.AppID)
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}
	if len(got.ScopeCeiling) != 3 {
		t.Errorf("scope count: want 3, got %d", len(got.ScopeCeiling))
	}
}

// TestRegisterApp_DefaultTTL verifies 0 means use default (1800).
func TestRegisterApp_DefaultTTL(t *testing.T) {
	svc := newTestAppSvc(t)
	resp, err := svc.RegisterApp("default-ttl", []string{"read:data:*"}, "admin", 0)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}
	rec, _ := svc.GetApp(resp.AppID)
	if rec.TokenTTL != 1800 {
		t.Fatalf("expected default TTL 1800, got %d", rec.TokenTTL)
	}
	if resp.TokenTTL != 1800 {
		t.Fatalf("expected resp TTL 1800, got %d", resp.TokenTTL)
	}
}

// TestRegisterApp_CustomTTL verifies explicit TTL is stored.
func TestRegisterApp_CustomTTL(t *testing.T) {
	svc := newTestAppSvc(t)
	resp, err := svc.RegisterApp("custom-ttl", []string{"read:data:*"}, "admin", 3600)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}
	rec, _ := svc.GetApp(resp.AppID)
	if rec.TokenTTL != 3600 {
		t.Fatalf("expected TTL 3600, got %d", rec.TokenTTL)
	}
}

// TestRegisterApp_TTLTooLow verifies TTL below minimum is rejected.
func TestRegisterApp_TTLTooLow(t *testing.T) {
	svc := newTestAppSvc(t)
	_, err := svc.RegisterApp("ttl-low", []string{"read:data:*"}, "admin", 30)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("expected ErrInvalidTTL, got %v", err)
	}
}

// TestRegisterApp_TTLTooHigh verifies TTL above maximum is rejected.
func TestRegisterApp_TTLTooHigh(t *testing.T) {
	svc := newTestAppSvc(t)
	_, err := svc.RegisterApp("ttl-high", []string{"read:data:*"}, "admin", 100000)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("expected ErrInvalidTTL, got %v", err)
	}
}

// TestAuthenticateApp_UsesPerAppTTL verifies auth uses the stored per-app TTL.
func TestAuthenticateApp_UsesPerAppTTL(t *testing.T) {
	svc := newTestAppSvc(t)
	resp, _ := svc.RegisterApp("auth-ttl", []string{"read:data:*"}, "admin", 7200)
	authResp, err := svc.AuthenticateApp(resp.ClientID, resp.ClientSecret)
	if err != nil {
		t.Fatalf("AuthenticateApp: %v", err)
	}
	if authResp.ExpiresIn != 7200 {
		t.Fatalf("expected ExpiresIn 7200, got %d", authResp.ExpiresIn)
	}
}

// TestUpdateAppTTL_Service verifies TTL can be updated via the service.
func TestUpdateAppTTL_Service(t *testing.T) {
	svc := newTestAppSvc(t)
	resp, _ := svc.RegisterApp("update-ttl", []string{"read:data:*"}, "admin", 1800)
	if err := svc.UpdateAppTTL(resp.AppID, 3600, "admin"); err != nil {
		t.Fatalf("UpdateAppTTL: %v", err)
	}
	rec, _ := svc.GetApp(resp.AppID)
	if rec.TokenTTL != 3600 {
		t.Fatalf("expected 3600, got %d", rec.TokenTTL)
	}
}

// TestUpdateAppTTL_OutOfBounds verifies bounds are enforced on update.
func TestUpdateAppTTL_OutOfBounds(t *testing.T) {
	svc := newTestAppSvc(t)
	resp, _ := svc.RegisterApp("bounds-ttl", []string{"read:data:*"}, "admin", 1800)
	if err := svc.UpdateAppTTL(resp.AppID, 30, "admin"); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("expected ErrInvalidTTL for too low, got %v", err)
	}
	if err := svc.UpdateAppTTL(resp.AppID, 100000, "admin"); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("expected ErrInvalidTTL for too high, got %v", err)
	}
}

// TestUpdateApp_InvalidScope verifies bad scopes are rejected on update.
func TestUpdateApp_InvalidScope(t *testing.T) {
	svc := newTestAppSvc(t)

	reg, err := svc.RegisterApp("my-app", []string{"read:data:*"}, "admin", 0)
	if err != nil {
		t.Fatalf("RegisterApp: %v", err)
	}

	err = svc.UpdateApp(reg.AppID, []string{"bad-scope"}, "admin")
	if err == nil {
		t.Error("expected error for invalid scope, got nil")
	}
}
