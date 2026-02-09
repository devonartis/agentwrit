package admin

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

const testSecret = "test-admin-secret-32bytes-long!"

func newTestAdminSvc(t *testing.T) *AdminSvc {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tknSvc := token.NewTknSvc(priv, pub, cfg.Cfg{DefaultTTL: 300})
	st := store.NewSqlStore()
	return NewAdminSvc(testSecret, tknSvc, st, nil)
}

// --- Authenticate ---

func TestAuthenticate_Success(t *testing.T) {
	svc := newTestAdminSvc(t)

	resp, err := svc.Authenticate("admin-client", testSecret)
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.AccessToken == "" {
		t.Fatal("expected non-empty access_token")
	}
	if resp.ExpiresIn != adminTTL {
		t.Errorf("expected expires_in=%d, got %d", adminTTL, resp.ExpiresIn)
	}

	// Verify the issued token has correct claims.
	claims, err := svc.tknSvc.Verify(resp.AccessToken)
	if err != nil {
		t.Fatalf("issued token should be valid: %v", err)
	}
	if claims.Sub != adminSub {
		t.Errorf("expected sub=%q, got %q", adminSub, claims.Sub)
	}
	if len(claims.Scope) != 3 {
		t.Errorf("expected 3 scopes, got %d", len(claims.Scope))
	}
}

func TestAuthenticate_WrongSecret(t *testing.T) {
	svc := newTestAdminSvc(t)

	_, err := svc.Authenticate("admin-client", "wrong-secret")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
	if err != ErrInvalidSecret {
		t.Errorf("expected ErrInvalidSecret, got: %v", err)
	}
}

func TestAuthenticate_EmptySecret(t *testing.T) {
	svc := newTestAdminSvc(t)

	_, err := svc.Authenticate("admin-client", "")
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
	if err != ErrInvalidSecret {
		t.Errorf("expected ErrInvalidSecret, got: %v", err)
	}
}

func TestAuthenticate_DifferentLengthSecret(t *testing.T) {
	svc := newTestAdminSvc(t)

	// Different-length secret should also fail (constant-time compare handles this).
	_, err := svc.Authenticate("admin-client", "short")
	if err != ErrInvalidSecret {
		t.Errorf("expected ErrInvalidSecret for different-length secret, got: %v", err)
	}

	_, err = svc.Authenticate("admin-client", testSecret+"extra-long-suffix-that-should-fail")
	if err != ErrInvalidSecret {
		t.Errorf("expected ErrInvalidSecret for longer secret, got: %v", err)
	}
}

// --- CreateLaunchToken ---

func TestCreateLaunchToken_Success(t *testing.T) {
	svc := newTestAdminSvc(t)

	singleUse := true
	req := CreateLaunchTokenReq{
		AgentName:    "data-reader",
		AllowedScope: []string{"read:Customers:*"},
		MaxTTL:       300,
		SingleUse:    &singleUse,
		TTL:          30,
	}

	resp, err := svc.CreateLaunchToken(req, adminSub)
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if resp.LaunchToken == "" {
		t.Fatal("expected non-empty launch_token")
	}
	// Token should be 64 hex chars (32 bytes).
	if len(resp.LaunchToken) != 64 {
		t.Errorf("expected 64-char hex token, got %d chars", len(resp.LaunchToken))
	}
	if resp.ExpiresAt == "" {
		t.Fatal("expected non-empty expires_at")
	}
	if len(resp.Policy.AllowedScope) != 1 || resp.Policy.AllowedScope[0] != "read:Customers:*" {
		t.Errorf("unexpected policy scope: %v", resp.Policy.AllowedScope)
	}
	if resp.Policy.MaxTTL != 300 {
		t.Errorf("expected max_ttl=300, got %d", resp.Policy.MaxTTL)
	}
}

func TestCreateLaunchToken_Defaults(t *testing.T) {
	svc := newTestAdminSvc(t)

	// No MaxTTL, no TTL, no SingleUse — should get defaults.
	req := CreateLaunchTokenReq{
		AgentName:    "agent-x",
		AllowedScope: []string{"read:Orders:*"},
	}

	resp, err := svc.CreateLaunchToken(req, adminSub)
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if resp.Policy.MaxTTL != defaultMaxTTL {
		t.Errorf("expected default max_ttl=%d, got %d", defaultMaxTTL, resp.Policy.MaxTTL)
	}

	// Verify the token record has SingleUse=true by default.
	rec, err := svc.ValidateLaunchToken(resp.LaunchToken)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !rec.SingleUse {
		t.Error("expected SingleUse=true by default")
	}
}

func TestCreateLaunchToken_MissingAgentName(t *testing.T) {
	svc := newTestAdminSvc(t)

	req := CreateLaunchTokenReq{
		AllowedScope: []string{"read:Customers:*"},
	}

	_, err := svc.CreateLaunchToken(req, adminSub)
	if err != ErrAgentNameEmpty {
		t.Errorf("expected ErrAgentNameEmpty, got: %v", err)
	}
}

func TestCreateLaunchToken_EmptyScope(t *testing.T) {
	svc := newTestAdminSvc(t)

	req := CreateLaunchTokenReq{
		AgentName:    "agent-x",
		AllowedScope: []string{},
	}

	_, err := svc.CreateLaunchToken(req, adminSub)
	if err != ErrScopeEmpty {
		t.Errorf("expected ErrScopeEmpty, got: %v", err)
	}
}

// --- ValidateLaunchToken & ConsumeLaunchToken ---

func TestValidateLaunchToken_Success(t *testing.T) {
	svc := newTestAdminSvc(t)

	resp, err := svc.CreateLaunchToken(CreateLaunchTokenReq{
		AgentName:    "agent-a",
		AllowedScope: []string{"read:Customers:*"},
		TTL:          60,
	}, adminSub)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	rec, err := svc.ValidateLaunchToken(resp.LaunchToken)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if rec.AgentName != "agent-a" {
		t.Errorf("expected agent_name=agent-a, got %s", rec.AgentName)
	}
}

func TestValidateLaunchToken_NotFound(t *testing.T) {
	svc := newTestAdminSvc(t)

	_, err := svc.ValidateLaunchToken("nonexistent-token")
	if err != store.ErrTokenNotFound {
		t.Errorf("expected store.ErrTokenNotFound, got: %v", err)
	}
}

func TestValidateLaunchToken_Expired(t *testing.T) {
	svc := newTestAdminSvc(t)

	// Create a token, then overwrite it in the store with a backdated expiry.
	resp, err := svc.CreateLaunchToken(CreateLaunchTokenReq{
		AgentName:    "agent-exp",
		AllowedScope: []string{"read:Customers:*"},
		TTL:          1,
	}, adminSub)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Overwrite in the store with an already-expired record.
	past := time.Now().UTC().Add(-1 * time.Second)
	err = svc.store.SaveLaunchToken(store.LaunchTokenRecord{
		Token:        resp.LaunchToken,
		AgentName:    "agent-exp",
		AllowedScope: []string{"read:Customers:*"},
		MaxTTL:       defaultMaxTTL,
		SingleUse:    true,
		CreatedAt:    past.Add(-30 * time.Second),
		ExpiresAt:    past,
		CreatedBy:    adminSub,
	})
	if err != nil {
		t.Fatalf("save backdated token: %v", err)
	}

	_, err = svc.ValidateLaunchToken(resp.LaunchToken)
	if err != store.ErrTokenExpired {
		t.Errorf("expected store.ErrTokenExpired, got: %v", err)
	}
}

func TestConsumeLaunchToken_SingleUse(t *testing.T) {
	svc := newTestAdminSvc(t)

	singleUse := true
	resp, err := svc.CreateLaunchToken(CreateLaunchTokenReq{
		AgentName:    "agent-single",
		AllowedScope: []string{"read:Customers:*"},
		SingleUse:    &singleUse,
		TTL:          60,
	}, adminSub)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// First validation should succeed.
	_, err = svc.ValidateLaunchToken(resp.LaunchToken)
	if err != nil {
		t.Fatalf("first validate: %v", err)
	}

	// Consume the token.
	if err := svc.ConsumeLaunchToken(resp.LaunchToken); err != nil {
		t.Fatalf("consume: %v", err)
	}

	// Second validation should fail — token consumed.
	_, err = svc.ValidateLaunchToken(resp.LaunchToken)
	if err != store.ErrTokenConsumed {
		t.Errorf("expected store.ErrTokenConsumed after consumption, got: %v", err)
	}
}

func TestConsumeLaunchToken_MultiUse(t *testing.T) {
	svc := newTestAdminSvc(t)

	multiUse := false
	resp, err := svc.CreateLaunchToken(CreateLaunchTokenReq{
		AgentName:    "agent-multi",
		AllowedScope: []string{"read:Customers:*"},
		SingleUse:    &multiUse,
		TTL:          60,
	}, adminSub)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Consume should be a no-op for multi-use tokens.
	if err := svc.ConsumeLaunchToken(resp.LaunchToken); err != nil {
		t.Fatalf("consume: %v", err)
	}

	// Should still validate successfully.
	_, err = svc.ValidateLaunchToken(resp.LaunchToken)
	if err != nil {
		t.Errorf("multi-use token should remain valid after consume, got: %v", err)
	}
}

func TestConsumeLaunchToken_NotFound(t *testing.T) {
	svc := newTestAdminSvc(t)

	err := svc.ConsumeLaunchToken("nonexistent")
	if err != store.ErrTokenNotFound {
		t.Errorf("expected store.ErrTokenNotFound, got: %v", err)
	}
}

// --- Token uniqueness ---

func TestCreateLaunchToken_UniqueTokens(t *testing.T) {
	svc := newTestAdminSvc(t)

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		resp, err := svc.CreateLaunchToken(CreateLaunchTokenReq{
			AgentName:    "agent",
			AllowedScope: []string{"read:Customers:*"},
		}, adminSub)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[resp.LaunchToken] {
			t.Fatalf("duplicate token at iteration %d", i)
		}
		seen[resp.LaunchToken] = true
	}
}

// --- Token format ---

func TestCreateLaunchToken_HexFormat(t *testing.T) {
	svc := newTestAdminSvc(t)

	resp, err := svc.CreateLaunchToken(CreateLaunchTokenReq{
		AgentName:    "agent",
		AllowedScope: []string{"read:Customers:*"},
	}, adminSub)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Should be lowercase hex.
	if strings.ToLower(resp.LaunchToken) != resp.LaunchToken {
		t.Error("expected lowercase hex token")
	}
	for _, c := range resp.LaunchToken {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character in token: %c", c)
			break
		}
	}
}

// Compile-time check: LaunchTokenRecord fields match spec.
func TestLaunchTokenRecord_SpecCompliance(t *testing.T) {
	rec := store.LaunchTokenRecord{
		Token:        "abc",
		AgentName:    "agent",
		AllowedScope: []string{"read:Customers:*"},
		MaxTTL:       300,
		SingleUse:    true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now(),
		CreatedBy:    adminSub,
	}
	// ConsumedAt is a pointer — nil means not consumed.
	if rec.ConsumedAt != nil {
		t.Error("new record should have nil ConsumedAt")
	}
}
