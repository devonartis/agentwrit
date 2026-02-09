package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/cfg"
)

func testKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	return pub, priv
}

func testCfg() cfg.Cfg {
	return cfg.Cfg{
		Port:        "8080",
		LogLevel:    "quiet",
		TrustDomain: "agentauth.local",
		DefaultTTL:  300,
	}
}

func TestIssueAndVerify(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp, err := svc.Issue(IssueReq{
		Sub:    "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope:  []string{"read:Customers:*"},
		TaskId: "task-1",
		OrchId: "orch-1",
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if resp.TokenType != "Bearer" {
		t.Errorf("token type = %q, want Bearer", resp.TokenType)
	}
	if resp.ExpiresIn != 300 {
		t.Errorf("expires_in = %d, want 300", resp.ExpiresIn)
	}

	// Verify the token
	claims, err := svc.Verify(resp.AccessToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if claims.Sub != "spiffe://agentauth.local/agent/orch-1/task-1/abc123" {
		t.Errorf("sub = %q", claims.Sub)
	}
	if claims.Iss != "agentauth" {
		t.Errorf("iss = %q", claims.Iss)
	}
	if len(claims.Scope) != 1 || claims.Scope[0] != "read:Customers:*" {
		t.Errorf("scope = %v", claims.Scope)
	}
	if claims.TaskId != "task-1" {
		t.Errorf("task_id = %q", claims.TaskId)
	}
	if claims.OrchId != "orch-1" {
		t.Errorf("orch_id = %q", claims.OrchId)
	}
	if claims.Jti == "" {
		t.Error("jti is empty")
	}
}

func TestVerifyTamperedSignature(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope: []string{"read:Customers:*"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Tamper with the signature by decoding, flipping a byte, re-encoding
	parts := strings.SplitN(resp.AccessToken, ".", 3)
	sigBytes, _ := base64.RawURLEncoding.DecodeString(parts[2])
	sigBytes[0] ^= 0xFF
	tampered := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(sigBytes)

	_, err = svc.Verify(tampered)
	if err != ErrSignatureInvalid {
		t.Errorf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestVerifyTamperedPayload(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope: []string{"read:Customers:*"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Tamper with the payload by decoding, flipping a byte, re-encoding
	parts := strings.SplitN(resp.AccessToken, ".", 3)
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
	payloadBytes[0] ^= 0xFF
	tampered := parts[0] + "." + base64.RawURLEncoding.EncodeToString(payloadBytes) + "." + parts[2]

	_, err = svc.Verify(tampered)
	if err != ErrSignatureInvalid {
		t.Errorf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestVerifyWrongKey(t *testing.T) {
	_, priv1 := testKeyPair(t)
	pub2, _ := testKeyPair(t)
	svc1 := NewTknSvc(priv1, priv1.Public().(ed25519.PublicKey), testCfg())
	svc2 := NewTknSvc(nil, pub2, testCfg()) // different public key for verification

	resp, err := svc1.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope: []string{"read:Customers:*"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	_, err = svc2.Verify(resp.AccessToken)
	if err != ErrSignatureInvalid {
		t.Errorf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	// Issue a token with very short TTL
	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope: []string{"read:Customers:*"},
		TTL:   1, // 1 second
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	_, err = svc.Verify(resp.AccessToken)
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerifyNotYetValid(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	// Manually craft a token with nbf in the future by issuing, then
	// manipulating claims and re-signing. Instead, we directly construct
	// claims with a future nbf.
	now := time.Now().Unix()
	claims := &TknClaims{
		Iss:   "agentauth",
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/nbf-test",
		Exp:   now + 3600,
		Nbf:   now + 600, // 10 minutes in the future
		Iat:   now,
		Jti:   "nbf-test-jti",
		Scope: []string{"read:Customers:*"},
	}

	tokenStr, err := svc.sign(claims)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	_, err = svc.Verify(tokenStr)
	if err != ErrTokenNotYetValid {
		t.Errorf("expected ErrTokenNotYetValid, got %v", err)
	}
}

func TestVerifyInvalidFormat(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	_, err := svc.Verify("not.a.valid.token")
	if err == nil {
		t.Error("expected error for invalid format")
	}

	_, err = svc.Verify("single-part")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestRenew(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp1, err := svc.Issue(IssueReq{
		Sub:    "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope:  []string{"read:Customers:*"},
		TaskId: "task-1",
		OrchId: "orch-1",
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	resp2, err := svc.Renew(resp1.AccessToken)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}

	// New token should be different
	if resp2.AccessToken == resp1.AccessToken {
		t.Error("renewed token is identical to original")
	}

	// But claims should have same sub/scope
	claims2, err := svc.Verify(resp2.AccessToken)
	if err != nil {
		t.Fatalf("verify renewed: %v", err)
	}
	if claims2.Sub != "spiffe://agentauth.local/agent/orch-1/task-1/abc123" {
		t.Errorf("renewed sub = %q", claims2.Sub)
	}
	if claims2.TaskId != "task-1" {
		t.Errorf("renewed task_id = %q", claims2.TaskId)
	}
	if claims2.Jti == resp1.Claims.Jti {
		t.Error("renewed JTI should be different")
	}
}

func TestRenew_PreservesChainHash(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	chainHash := "abc123def456"
	chain := []DelegRecord{
		{Agent: "spiffe://test/agent/o/t/a1", Scope: []string{"read:data:*"}, DelegatedAt: time.Now(), Signature: "sig1"},
	}

	resp1, err := svc.Issue(IssueReq{
		Sub:        "spiffe://agentauth.local/agent/orch-1/task-1/delegated",
		Scope:      []string{"read:data:*"},
		TaskId:     "task-1",
		OrchId:     "orch-1",
		DelegChain: chain,
		ChainHash:  chainHash,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	resp2, err := svc.Renew(resp1.AccessToken)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}

	claims2, err := svc.Verify(resp2.AccessToken)
	if err != nil {
		t.Fatalf("Verify renewed: %v", err)
	}

	if claims2.ChainHash != chainHash {
		t.Errorf("renewed ChainHash = %q, want %q", claims2.ChainHash, chainHash)
	}
	if len(claims2.DelegChain) != 1 {
		t.Errorf("renewed DelegChain length = %d, want 1", len(claims2.DelegChain))
	}
}

func TestPublicKey(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())
	if got := svc.PublicKey(); !got.Equal(pub) {
		t.Error("PublicKey() does not match")
	}
}

func TestCustomTTL(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope: []string{"read:Customers:*"},
		TTL:   60,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if resp.ExpiresIn != 60 {
		t.Errorf("expires_in = %d, want 60", resp.ExpiresIn)
	}
}

func TestSidClaim(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	testSid := "sidecar-123"
	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope: []string{"read:Customers:*"},
		Sid:   testSid,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims, err := svc.Verify(resp.AccessToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if claims.Sid != testSid {
		t.Errorf("sid = %q, want %q", claims.Sid, testSid)
	}
}

func TestTokenExchange(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	// 1. Issue a sidecar token
	sidecarSid := "sidecar-abc"
	sidecarToken, _ := svc.Issue(IssueReq{
		Sub:   "sidecar:" + sidecarSid,
		Scope: []string{"sidecar:manage", "sidecar:scope:read:data:*"},
		Sid:   sidecarSid,
	})

	// 2. Exchange for an agent token
	agentID := "spiffe://test/agent/foo"
	requestedScope := []string{"read:data:profile"}
	
	// Red Phase: This will fail because Exchange is missing
	resp, err := svc.Exchange(sidecarToken.AccessToken, agentID, requestedScope, 60)
	if err != nil {
		t.Fatalf("Exchange failed: %v", err)
	}

	claims, _ := svc.Verify(resp.AccessToken)
	if claims.Sub != agentID {
		t.Errorf("sub = %q, want %q", claims.Sub, agentID)
	}
	if claims.Sid != "" {
		t.Error("agent token should not have Sid claim (that's for sidecar tokens)")
	}
	// We'll need a way to check for the injected sidecar_id.
	// Our spec says "injected into the issued agent token".
	// Let's assume we add a SidecarID field to TknClaims.
}

func TestIssueWithoutSid_DefaultsEmpty(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope: []string{"read:Customers:*"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims, err := svc.Verify(resp.AccessToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if claims.Sid != "" {
		t.Errorf("sid = %q, want empty", claims.Sid)
	}
}

func TestRenew_PreservesSid(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	const testSid = "sidecar-renew-001"
	resp1, err := svc.Issue(IssueReq{
		Sub:    "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope:  []string{"read:Customers:*"},
		TaskId: "task-1",
		OrchId: "orch-1",
		Sid:    testSid,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	resp2, err := svc.Renew(resp1.AccessToken)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}

	claims2, err := svc.Verify(resp2.AccessToken)
	if err != nil {
		t.Fatalf("verify renewed: %v", err)
	}

	if claims2.Sid != testSid {
		t.Errorf("renewed sid = %q, want %q", claims2.Sid, testSid)
	}
}
