package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

func setupIdSvc(t *testing.T) (*IdSvc, *store.SqlStore, *audit.AuditLog) {
	t.Helper()

	sqlStore := store.NewSqlStore()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	c := cfg.Cfg{
		Port:        "8080",
		LogLevel:    "quiet",
		TrustDomain: "agentauth.local",
		DefaultTTL:  300,
	}
	tknSvc := token.NewTknSvc(priv, pub, c)
	auditLog := audit.NewAuditLog(nil)
	idSvc := NewIdSvc(sqlStore, tknSvc, "agentauth.local", auditLog, "")

	return idSvc, sqlStore, auditLog
}

func createLaunchToken(t *testing.T, s *store.SqlStore, allowedScope []string) string {
	t.Helper()

	tokenVal := make([]byte, 16)
	if _, err := rand.Read(tokenVal); err != nil {
		t.Fatal(err)
	}
	tok := hex.EncodeToString(tokenVal)

	err := s.SaveLaunchToken(store.LaunchTokenRecord{
		Token:        tok,
		AgentName:    "test-agent",
		AllowedScope: allowedScope,
		MaxTTL:       300,
		SingleUse:    true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Second),
		CreatedBy:    "admin:orchestrator",
	})
	if err != nil {
		t.Fatalf("save launch token: %v", err)
	}

	return tok
}

func signNonce(t *testing.T, nonce string, privKey ed25519.PrivateKey) (pubB64, sigB64 string) {
	t.Helper()
	nonceBytes, err := hex.DecodeString(nonce)
	if err != nil {
		nonceBytes = []byte(nonce)
	}
	sig := ed25519.Sign(privKey, nonceBytes)
	pubB64 = base64.StdEncoding.EncodeToString(privKey.Public().(ed25519.PublicKey))
	sigB64 = base64.StdEncoding.EncodeToString(sig)
	return
}

func TestRegisterSuccess(t *testing.T) {
	idSvc, sqlStore, auditLog := setupIdSvc(t)

	// Create launch token and nonce
	lt := createLaunchToken(t, sqlStore, []string{"read:Customers:*"})
	nonce := sqlStore.CreateNonce()

	// Generate agent key pair
	_, agentPriv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64, sigB64 := signNonce(t, nonce, agentPriv)

	resp, err := idSvc.Register(RegisterReq{
		LaunchToken:    lt,
		Nonce:          nonce,
		PublicKey:      pubB64,
		Signature:      sigB64,
		OrchID:         "orch-1",
		TaskID:         "task-1",
		RequestedScope: []string{"read:Customers:12345"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if resp.AgentID == "" {
		t.Error("agent_id is empty")
	}
	if resp.AccessToken == "" {
		t.Error("access_token is empty")
	}
	if resp.ExpiresIn != 300 {
		t.Errorf("expires_in = %d, want 300", resp.ExpiresIn)
	}

	// Verify agent ID is a valid SPIFFE ID
	orchID, taskID, _, err := ParseSpiffeId(resp.AgentID)
	if err != nil {
		t.Fatalf("parse SPIFFE ID: %v", err)
	}
	if orchID != "orch-1" {
		t.Errorf("orchID = %q, want orch-1", orchID)
	}
	if taskID != "task-1" {
		t.Errorf("taskID = %q, want task-1", taskID)
	}

	// Verify audit events were recorded
	events := auditLog.Events()
	if len(events) < 2 {
		t.Fatalf("expected at least 2 audit events, got %d", len(events))
	}

	foundRegistered := false
	foundTokenIssued := false
	for _, e := range events {
		if e.EventType == "agent_registered" {
			foundRegistered = true
		}
		if e.EventType == "token_issued" {
			foundTokenIssued = true
		}
	}
	if !foundRegistered {
		t.Error("missing agent_registered audit event")
	}
	if !foundTokenIssued {
		t.Error("missing token_issued audit event")
	}
}

func TestRegisterScopeViolation(t *testing.T) {
	idSvc, sqlStore, _ := setupIdSvc(t)

	lt := createLaunchToken(t, sqlStore, []string{"read:Customers:12345"})
	nonce := sqlStore.CreateNonce()

	_, agentPriv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64, sigB64 := signNonce(t, nonce, agentPriv)

	// Request broader scope than allowed (wildcard vs specific)
	_, err := idSvc.Register(RegisterReq{
		LaunchToken:    lt,
		Nonce:          nonce,
		PublicKey:      pubB64,
		Signature:      sigB64,
		OrchID:         "orch-1",
		TaskID:         "task-1",
		RequestedScope: []string{"read:Customers:*"},
	})
	if err != ErrScopeViolation {
		t.Errorf("expected ErrScopeViolation, got %v", err)
	}

	// Verify launch token was NOT consumed
	_, err = sqlStore.GetLaunchToken(lt)
	if err != nil {
		t.Error("launch token should NOT be consumed after scope violation")
	}
}

func TestRegisterDifferentActionViolation(t *testing.T) {
	idSvc, sqlStore, _ := setupIdSvc(t)

	lt := createLaunchToken(t, sqlStore, []string{"read:Customers:*"})
	nonce := sqlStore.CreateNonce()

	_, agentPriv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64, sigB64 := signNonce(t, nonce, agentPriv)

	_, err := idSvc.Register(RegisterReq{
		LaunchToken:    lt,
		Nonce:          nonce,
		PublicKey:      pubB64,
		Signature:      sigB64,
		OrchID:         "orch-1",
		TaskID:         "task-1",
		RequestedScope: []string{"write:Customers:*"},
	})
	if err != ErrScopeViolation {
		t.Errorf("expected ErrScopeViolation, got %v", err)
	}
}

func TestRegisterInvalidSignature(t *testing.T) {
	idSvc, sqlStore, _ := setupIdSvc(t)

	lt := createLaunchToken(t, sqlStore, []string{"read:Customers:*"})
	nonce := sqlStore.CreateNonce()

	_, agentPriv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64, _ := signNonce(t, nonce, agentPriv)

	// Use a different key to sign (wrong signature)
	_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)
	_, wrongSigB64 := signNonce(t, nonce, wrongPriv)

	_, err := idSvc.Register(RegisterReq{
		LaunchToken:    lt,
		Nonce:          nonce,
		PublicKey:      pubB64,
		Signature:      wrongSigB64,
		OrchID:         "orch-1",
		TaskID:         "task-1",
		RequestedScope: []string{"read:Customers:12345"},
	})
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestRegisterMissingFields(t *testing.T) {
	idSvc, _, _ := setupIdSvc(t)

	_, err := idSvc.Register(RegisterReq{})
	if err == nil {
		t.Error("expected error for empty request")
	}
}

func TestRegister_AgentInheritsAppIDFromLaunchToken(t *testing.T) {
	idSvc, sqlStore, _ := setupIdSvc(t)

	// Create a launch token WITH an AppID (app-created)
	tokenVal := make([]byte, 16)
	if _, err := rand.Read(tokenVal); err != nil {
		t.Fatal(err)
	}
	tok := hex.EncodeToString(tokenVal)

	err := sqlStore.SaveLaunchToken(store.LaunchTokenRecord{
		Token:        tok,
		AgentName:    "weather-agent",
		AllowedScope: []string{"read:Weather:*"},
		MaxTTL:       300,
		SingleUse:    true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Second),
		CreatedBy:    "app:app-weather-bot-abc123",
		AppID:        "app-weather-bot-abc123",
	})
	if err != nil {
		t.Fatalf("save launch token: %v", err)
	}

	nonce := sqlStore.CreateNonce()
	_, agentPriv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64, sigB64 := signNonce(t, nonce, agentPriv)

	resp, err := idSvc.Register(RegisterReq{
		LaunchToken:    tok,
		Nonce:          nonce,
		PublicKey:      pubB64,
		Signature:      sigB64,
		OrchID:         "orch-weather",
		TaskID:         "task-forecast",
		RequestedScope: []string{"read:Weather:12345"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Verify the agent record inherited the AppID
	agent, err := sqlStore.GetAgent(resp.AgentID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.AppID != "app-weather-bot-abc123" {
		t.Errorf("agent.AppID = %q, want %q", agent.AppID, "app-weather-bot-abc123")
	}
}

func TestRegister_AdminLaunchTokenAgentHasNoAppID(t *testing.T) {
	idSvc, sqlStore, _ := setupIdSvc(t)

	// Create a launch token WITHOUT an AppID (admin-created)
	lt := createLaunchToken(t, sqlStore, []string{"read:Customers:*"})

	nonce := sqlStore.CreateNonce()
	_, agentPriv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64, sigB64 := signNonce(t, nonce, agentPriv)

	resp, err := idSvc.Register(RegisterReq{
		LaunchToken:    lt,
		Nonce:          nonce,
		PublicKey:      pubB64,
		Signature:      sigB64,
		OrchID:         "orch-admin",
		TaskID:         "task-admin",
		RequestedScope: []string{"read:Customers:12345"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Verify the agent record has empty AppID
	agent, err := sqlStore.GetAgent(resp.AgentID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.AppID != "" {
		t.Errorf("agent.AppID = %q, want empty string", agent.AppID)
	}
}

func TestRegisterLaunchTokenConsumed(t *testing.T) {
	idSvc, sqlStore, _ := setupIdSvc(t)

	lt := createLaunchToken(t, sqlStore, []string{"read:Customers:*"})

	// Register first time
	nonce1 := sqlStore.CreateNonce()
	_, agentPriv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64, sigB64 := signNonce(t, nonce1, agentPriv)

	_, err := idSvc.Register(RegisterReq{
		LaunchToken:    lt,
		Nonce:          nonce1,
		PublicKey:      pubB64,
		Signature:      sigB64,
		OrchID:         "orch-1",
		TaskID:         "task-1",
		RequestedScope: []string{"read:Customers:12345"},
	})
	if err != nil {
		t.Fatalf("first Register: %v", err)
	}

	// Try to register again with same launch token
	nonce2 := sqlStore.CreateNonce()
	pubB64_2, sigB64_2 := signNonce(t, nonce2, agentPriv)

	_, err = idSvc.Register(RegisterReq{
		LaunchToken:    lt,
		Nonce:          nonce2,
		PublicKey:      pubB64_2,
		Signature:      sigB64_2,
		OrchID:         "orch-1",
		TaskID:         "task-2",
		RequestedScope: []string{"read:Customers:12345"},
	})
	if err == nil {
		t.Error("expected error for consumed launch token")
	}
}

// --- Audit: app_id in registration events ---

func TestRegister_AuditIncludesAppID(t *testing.T) {
	idSvc, sqlStore, auditLog := setupIdSvc(t)

	// Create a launch token WITH an AppID (app-created)
	tokenVal := make([]byte, 16)
	if _, err := rand.Read(tokenVal); err != nil {
		t.Fatal(err)
	}
	tok := hex.EncodeToString(tokenVal)

	err := sqlStore.SaveLaunchToken(store.LaunchTokenRecord{
		Token:        tok,
		AgentName:    "audit-agent",
		AllowedScope: []string{"read:Customers:*"},
		MaxTTL:       300,
		SingleUse:    true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Second),
		CreatedBy:    "app:app-audit-test",
		AppID:        "app-audit-test",
	})
	if err != nil {
		t.Fatalf("save launch token: %v", err)
	}

	nonce := sqlStore.CreateNonce()
	_, agentPriv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64, sigB64 := signNonce(t, nonce, agentPriv)

	_, err = idSvc.Register(RegisterReq{
		LaunchToken:    tok,
		Nonce:          nonce,
		PublicKey:      pubB64,
		Signature:      sigB64,
		OrchID:         "orch-audit",
		TaskID:         "task-audit",
		RequestedScope: []string{"read:Customers:12345"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	events := auditLog.Events()
	foundRegistered := false
	foundTokenIssued := false
	for _, e := range events {
		if e.EventType == "agent_registered" {
			foundRegistered = true
			if !strings.Contains(e.Detail, "app_id=app-audit-test") {
				t.Errorf("agent_registered detail should contain app_id=app-audit-test, got: %s", e.Detail)
			}
		}
		if e.EventType == "token_issued" {
			foundTokenIssued = true
			if !strings.Contains(e.Detail, "app_id=app-audit-test") {
				t.Errorf("token_issued detail should contain app_id=app-audit-test, got: %s", e.Detail)
			}
		}
	}
	if !foundRegistered {
		t.Fatal("missing agent_registered audit event")
	}
	if !foundTokenIssued {
		t.Fatal("missing token_issued audit event")
	}
}

func TestRegister_AuditNoAppIDForAdminToken(t *testing.T) {
	idSvc, sqlStore, auditLog := setupIdSvc(t)

	// Create a launch token WITHOUT an AppID (admin-created)
	lt := createLaunchToken(t, sqlStore, []string{"read:Customers:*"})

	nonce := sqlStore.CreateNonce()
	_, agentPriv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64, sigB64 := signNonce(t, nonce, agentPriv)

	_, err := idSvc.Register(RegisterReq{
		LaunchToken:    lt,
		Nonce:          nonce,
		PublicKey:      pubB64,
		Signature:      sigB64,
		OrchID:         "orch-noid",
		TaskID:         "task-noid",
		RequestedScope: []string{"read:Customers:12345"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	events := auditLog.Events()
	for _, e := range events {
		if e.EventType == "agent_registered" || e.EventType == "token_issued" {
			if strings.Contains(e.Detail, "app_id=") {
				t.Errorf("admin-created token audit event should NOT contain app_id=, got: %s", e.Detail)
			}
		}
	}
}
