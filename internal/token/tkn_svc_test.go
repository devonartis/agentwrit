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
	// kid mismatch is now caught before signature verification (M1 hardening),
	// so we accept either ErrInvalidToken (kid check) or ErrSignatureInvalid.
	if err != ErrInvalidToken && err != ErrSignatureInvalid {
		t.Errorf("expected ErrInvalidToken or ErrSignatureInvalid, got %v", err)
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

	testSid := "session-123"
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


func TestRenew_RevokesPredecessor(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp1, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	mock := &mockRevoker{}
	svc.SetRevoker(mock)

	resp2, err := svc.Renew(resp1.AccessToken)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if resp2.AccessToken == "" {
		t.Fatal("renewed token is empty")
	}

	if len(mock.revoked) != 1 {
		t.Fatalf("expected 1 revocation call, got %d", len(mock.revoked))
	}
	if mock.revoked[0] != resp1.Claims.Jti {
		t.Errorf("revoked JTI = %q, want %q", mock.revoked[0], resp1.Claims.Jti)
	}
}

func TestRenew_RevokeFailureDoesNotBlockRenewal(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp1, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	mock := &mockRevoker{err: errors.New("revocation storage down")}
	svc.SetRevoker(mock)

	resp2, err := svc.Renew(resp1.AccessToken)
	if err != nil {
		t.Fatalf("Renew should succeed even when revocation fails: %v", err)
	}
	if resp2.AccessToken == "" {
		t.Fatal("renewed token is empty")
	}
}

func TestKidInJWTHeader(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	kid := svc.Kid()
	if kid == "" {
		t.Fatal("Kid() returned empty string")
	}

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/test-agent",
		Scope: []string{"read:data:*"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Decode the JWT header (first segment)
	parts := strings.SplitN(resp.AccessToken, ".", 3)
	if len(parts) != 3 {
		t.Fatal("token does not have 3 parts")
	}
	hdrJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var hdr struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(hdrJSON, &hdr); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if hdr.Kid == "" {
		t.Fatal("kid missing from JWT header")
	}
	if hdr.Kid != kid {
		t.Errorf("header kid=%q, Kid()=%q — mismatch", hdr.Kid, kid)
	}
}

func TestKidIsRFC7638Thumbprint(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	kid := svc.Kid()

	// Manually compute expected thumbprint: {"crv":"Ed25519","kty":"OKP","x":"<b64url>"}
	xB64 := base64.RawURLEncoding.EncodeToString(pub)
	canonical := `{"crv":"Ed25519","kty":"OKP","x":"` + xB64 + `"}`

	// SHA-256 → base64url
	h := sha256.Sum256([]byte(canonical))
	expected := base64.RawURLEncoding.EncodeToString(h[:])

	if kid != expected {
		t.Errorf("Kid()=%q, expected RFC7638=%q", kid, expected)
	}
}

func TestKidStableAcrossInstances(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc1 := NewTknSvc(priv, pub, testCfg())
	svc2 := NewTknSvc(priv, pub, testCfg())

	if svc1.Kid() != svc2.Kid() {
		t.Errorf("kid not stable: %q vs %q", svc1.Kid(), svc2.Kid())
	}
}

func TestIssClaimMatchesConfig(t *testing.T) {
	pub, priv := testKeyPair(t)
	c := testCfg()
	c.IssuerURL = "https://broker.example.com"
	svc := NewTknSvc(priv, pub, c)

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

	if claims.Iss != "https://broker.example.com" {
		t.Errorf("iss = %q, want %q", claims.Iss, "https://broker.example.com")
	}
}

func TestVerifyRejectsWrongIssuer(t *testing.T) {
	pub, priv := testKeyPair(t)

	// Issue with one issuer
	c1 := testCfg()
	c1.IssuerURL = "https://broker-a.example.com"
	svc1 := NewTknSvc(priv, pub, c1)

	resp, err := svc1.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
		Scope: []string{"read:Customers:*"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Verify with different issuer expectation (same key, different config)
	c2 := testCfg()
	c2.IssuerURL = "https://broker-b.example.com"
	svc2 := NewTknSvc(priv, pub, c2)

	_, err = svc2.Verify(resp.AccessToken)
	if err != ErrInvalidIssuer {
		t.Errorf("expected ErrInvalidIssuer, got %v", err)
	}
}

func TestRenew_PreservesSid(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	const testSid = "session-renew-001"
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

func TestIssue_MaxTTL_Clamps(t *testing.T) {
	pub, priv := testKeyPair(t)
	c := testCfg()
	c.MaxTTL = 3600 // 1 hour max
	svc := NewTknSvc(priv, pub, c)

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/test/task/abc",
		Scope: []string{"read:data:*"},
		TTL:   7200, // request 2 hours
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if resp.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d, want 3600 (clamped)", resp.ExpiresIn)
	}
}

func TestIssue_MaxTTL_Zero_NoLimit(t *testing.T) {
	pub, priv := testKeyPair(t)
	c := testCfg()
	c.MaxTTL = 0 // no limit
	svc := NewTknSvc(priv, pub, c)

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/test/task/abc",
		Scope: []string{"read:data:*"},
		TTL:   86400,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if resp.ExpiresIn != 86400 {
		t.Errorf("ExpiresIn = %d, want 86400 (no limit)", resp.ExpiresIn)
	}
}

func TestIssue_MaxTTL_UnderLimit_Unchanged(t *testing.T) {
	pub, priv := testKeyPair(t)
	c := testCfg()
	c.MaxTTL = 3600
	svc := NewTknSvc(priv, pub, c)

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/test/task/abc",
		Scope: []string{"read:data:*"},
		TTL:   1800,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if resp.ExpiresIn != 1800 {
		t.Errorf("ExpiresIn = %d, want 1800 (under limit)", resp.ExpiresIn)
	}
}

func TestVerify_RejectsWrongAlg(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/test/task/abc",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Tamper: replace header alg with "HS256"
	parts := strings.SplitN(resp.AccessToken, ".", 3)
	hdrJSON, _ := base64.RawURLEncoding.DecodeString(parts[0])
	tampered := strings.Replace(string(hdrJSON), `"EdDSA"`, `"HS256"`, 1)
	parts[0] = base64.RawURLEncoding.EncodeToString([]byte(tampered))
	// Re-sign so signature is valid for the tampered header+payload
	signingInput := parts[0] + "." + parts[1]
	sig := ed25519.Sign(priv, []byte(signingInput))
	parts[2] = base64.RawURLEncoding.EncodeToString(sig)
	badToken := strings.Join(parts, ".")

	_, err = svc.Verify(badToken)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Verify(alg=HS256) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_RejectsWrongKid(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/test/task/abc",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Tamper: replace kid with a wrong value
	parts := strings.SplitN(resp.AccessToken, ".", 3)
	hdr := jwtHeader{Alg: "EdDSA", Typ: "JWT", Kid: "wrong-kid-value"}
	hdrBytes, _ := json.Marshal(hdr)
	parts[0] = base64.RawURLEncoding.EncodeToString(hdrBytes)
	signingInput := parts[0] + "." + parts[1]
	sig := ed25519.Sign(priv, []byte(signingInput))
	parts[2] = base64.RawURLEncoding.EncodeToString(sig)
	badToken := strings.Join(parts, ".")

	_, err = svc.Verify(badToken)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Verify(wrong kid) = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_AcceptsEmptyKid(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/test/task/abc",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Tamper: remove kid from header (backward compat for old tokens)
	parts := strings.SplitN(resp.AccessToken, ".", 3)
	hdr := jwtHeader{Alg: "EdDSA", Typ: "JWT", Kid: ""}
	hdrBytes, _ := json.Marshal(hdr)
	parts[0] = base64.RawURLEncoding.EncodeToString(hdrBytes)
	signingInput := parts[0] + "." + parts[1]
	sig := ed25519.Sign(priv, []byte(signingInput))
	parts[2] = base64.RawURLEncoding.EncodeToString(sig)
	token := strings.Join(parts, ".")

	_, err = svc.Verify(token)
	if err != nil {
		t.Errorf("Verify(empty kid) should succeed, got %v", err)
	}
}

func TestVerify_RejectsRevokedToken(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())

	resp, err := svc.Issue(IssueReq{
		Sub:   "spiffe://agentauth.local/agent/test/task/abc",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	mock := &mockRevoker{isRevoked: true}
	svc.SetRevoker(mock)

	_, err = svc.Verify(resp.AccessToken)
	if !errors.Is(err, ErrTokenRevoked) {
		t.Errorf("Verify(revoked) = %v, want ErrTokenRevoked", err)
	}
}

func TestVerify_RejectsZeroExpiry(t *testing.T) {
	pub, priv := testKeyPair(t)
	svc := NewTknSvc(priv, pub, testCfg())
	_ = svc

	now := time.Now().Unix()
	claims := &TknClaims{
		Iss:   testCfg().IssuerURL,
		Sub:   "spiffe://test.local/agent/test/task/abc",
		Jti:   "test-jti-zero-exp",
		Iat:   now,
		Nbf:   now,
		Exp:   0,
		Scope: []string{"read:data:*"},
	}
	err := claims.Validate(testCfg().IssuerURL)
	if !errors.Is(err, ErrNoExpiry) {
		t.Errorf("Validate(exp=0) = %v, want ErrNoExpiry", err)
	}
}
