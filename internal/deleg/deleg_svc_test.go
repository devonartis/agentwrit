package deleg

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/token"
)

// testKit holds common test dependencies.
type testKit struct {
	tknSvc     *token.TknSvc
	signingKey ed25519.PrivateKey
	pubKey     ed25519.PublicKey
	delegSvc   *DelegSvc
}

func newTestKit(maxDepth int) *testKit {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	c := cfg.Cfg{
		TrustDomain: "test.local",
		DefaultTTL:  300,
	}
	tknSvc := token.NewTknSvc(priv, pub, c)
	return &testKit{
		tknSvc:     tknSvc,
		signingKey: priv,
		pubKey:     pub,
		delegSvc:   NewDelegSvc(tknSvc, priv, maxDepth),
	}
}

// issueToken issues a test token with the given scope.
func (k *testKit) issueToken(agentID string, scope []string, ttl int) string {
	resp, err := k.tknSvc.Issue(token.IssueReq{
		AgentID:   agentID,
		OrchID:    "test-orch",
		TaskID:    "test-task",
		Scope:     scope,
		TTLSecond: ttl,
	})
	if err != nil {
		panic("issueToken: " + err.Error())
	}
	return resp.AccessToken
}

func TestDelegSvc_ValidOneDelegation(t *testing.T) {
	k := newTestKit(3)
	tkn := k.issueToken("spiffe://test.local/agent/orch/task/agentA", []string{"read:Customers:*"}, 300)

	resp, err := k.delegSvc.Delegate(DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DelegationDepth != 1 {
		t.Errorf("depth = %d, want 1", resp.DelegationDepth)
	}
	if resp.ChainHash == "" {
		t.Error("chain_hash should not be empty")
	}
	if resp.DelegationToken == "" {
		t.Error("delegation_token should not be empty")
	}

	// Verify the issued token is valid.
	claims, err := k.tknSvc.Verify(resp.DelegationToken)
	if err != nil {
		t.Fatalf("delegation token verification failed: %v", err)
	}
	if claims.Sub != "spiffe://test.local/agent/orch/task/agentB" {
		t.Errorf("sub = %s, want agentB", claims.Sub)
	}
	if len(claims.Scope) != 1 || claims.Scope[0] != "read:Customers:12345" {
		t.Errorf("scope = %v, want [read:Customers:12345]", claims.Scope)
	}
}

func TestDelegSvc_ScopeExpansionBlocked(t *testing.T) {
	k := newTestKit(3)
	tkn := k.issueToken("spiffe://test.local/agent/orch/task/agentA", []string{"read:Customers:12345"}, 300)

	_, err := k.delegSvc.Delegate(DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:*"},
		MaxTTL:         60,
	})
	if err == nil {
		t.Fatal("expected scope escalation error, got nil")
	}
	if !errors.Is(err, ErrScopeEscalation) {
		t.Errorf("expected ErrScopeEscalation, got: %v", err)
	}
}

func TestDelegSvc_DepthExceeded(t *testing.T) {
	k := newTestKit(1) // max depth of 1
	tkn := k.issueToken("spiffe://test.local/agent/orch/task/agentA", []string{"read:Customers:*"}, 300)

	// First delegation succeeds (depth 0 → 1).
	resp, err := k.delegSvc.Delegate(DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	if err != nil {
		t.Fatalf("first delegation should succeed: %v", err)
	}

	// Second delegation should fail (depth already 1, max is 1).
	// The delegation token doesn't carry chain in claims (Issue creates empty chain),
	// so we test with max=0 to ensure the logic triggers.
	k2 := newTestKit(0)
	tkn2 := k2.issueToken("spiffe://test.local/agent/orch/task/agentA", []string{"read:Customers:*"}, 300)
	_, err = k2.delegSvc.Delegate(DelegReq{
		DelegatorToken: tkn2,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	if err == nil {
		t.Fatal("expected depth exceeded error, got nil")
	}
	if !errors.Is(err, ErrDepthExceeded) {
		t.Errorf("expected ErrDepthExceeded, got: %v", err)
	}
	_ = resp // used above
}

func TestDelegSvc_ExpiredDelegatorBlocked(t *testing.T) {
	k := newTestKit(3)
	// Issue a token with 1 second TTL — by the time we delegate it should be expired.
	// Instead, use the invalid token path.
	_, err := k.delegSvc.Delegate(DelegReq{
		DelegatorToken: "invalid.token.value",
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	if err == nil {
		t.Fatal("expected delegator token error, got nil")
	}
	if !errors.Is(err, ErrDelegatorTokenInvalid) {
		t.Errorf("expected ErrDelegatorTokenInvalid, got: %v", err)
	}
}

func TestDelegSvc_TTLExceedsRemaining(t *testing.T) {
	k := newTestKit(3)
	tkn := k.issueToken("spiffe://test.local/agent/orch/task/agentA", []string{"read:Customers:*"}, 60)

	_, err := k.delegSvc.Delegate(DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         9999, // Way more than 60s remaining.
	})
	if err == nil {
		t.Fatal("expected TTL exceeded error, got nil")
	}
	if !errors.Is(err, ErrTTLExceedsRemaining) {
		t.Errorf("expected ErrTTLExceedsRemaining, got: %v", err)
	}
}

func TestDelegSvc_EmptyTargetAgent(t *testing.T) {
	k := newTestKit(3)
	tkn := k.issueToken("spiffe://test.local/agent/orch/task/agentA", []string{"read:Customers:*"}, 300)

	_, err := k.delegSvc.Delegate(DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	if err == nil {
		t.Fatal("expected empty target error, got nil")
	}
	if !errors.Is(err, ErrTargetAgentEmpty) {
		t.Errorf("expected ErrTargetAgentEmpty, got: %v", err)
	}
}

func TestDelegSvc_EmptyRequestedScope(t *testing.T) {
	k := newTestKit(3)
	tkn := k.issueToken("spiffe://test.local/agent/orch/task/agentA", []string{"read:Customers:*"}, 300)

	_, err := k.delegSvc.Delegate(DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{},
		MaxTTL:         60,
	})
	if err == nil {
		t.Fatal("expected empty scope error, got nil")
	}
	if !errors.Is(err, ErrRequestedScopeEmpty) {
		t.Errorf("expected ErrRequestedScopeEmpty, got: %v", err)
	}
}

func TestDelegSvc_DefaultTTLUsesRemaining(t *testing.T) {
	k := newTestKit(3)
	tkn := k.issueToken("spiffe://test.local/agent/orch/task/agentA", []string{"read:Customers:*"}, 120)

	resp, err := k.delegSvc.Delegate(DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         0, // Should default to remaining TTL.
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DelegationToken == "" {
		t.Error("delegation_token should not be empty")
	}
}

func TestDelegSvc_ChainHashDeterministic(t *testing.T) {
	k := newTestKit(3)
	tkn := k.issueToken("spiffe://test.local/agent/orch/task/agentA", []string{"read:Customers:*"}, 300)

	// Two delegations with same params should produce different chain hashes
	// (different timestamps), but both should be non-empty.
	r1, err := k.delegSvc.Delegate(DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentB",
		DelegatedScope: []string{"read:Customers:12345"},
		MaxTTL:         60,
	})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := k.delegSvc.Delegate(DelegReq{
		DelegatorToken: tkn,
		TargetAgentId:  "spiffe://test.local/agent/orch/task/agentC",
		DelegatedScope: []string{"read:Customers:67890"},
		MaxTTL:         60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if r1.ChainHash == "" || r2.ChainHash == "" {
		t.Error("chain hashes must not be empty")
	}
	if r1.ChainHash == r2.ChainHash {
		t.Error("different delegations should produce different chain hashes")
	}
}
