package deleg

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/token"
)

// buildSignedRecord creates a DelegRecord with a valid Ed25519 signature.
func buildSignedRecord(agent string, scope []string, key ed25519.PrivateKey) token.DelegRecord {
	rec := token.DelegRecord{
		Agent:       agent,
		Scope:       scope,
		DelegatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(rec)
	sig := ed25519.Sign(key, data)
	rec.Signature = hex.EncodeToString(sig)
	return rec
}

func TestVerifyChain_EmptyChain(t *testing.T) {
	ok, cerr := VerifyChain(nil, nil, nil, nil)
	if !ok || cerr != nil {
		t.Fatal("empty chain should pass")
	}
}

func TestVerifyChain_ValidOneHop(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	rec := buildSignedRecord("agentA", []string{"read:Customers:12345"}, priv)
	chain := []token.DelegRecord{rec}

	ok, cerr := VerifyChain(chain, []string{"read:Customers:12345"}, nil, pub)
	if !ok {
		t.Fatalf("valid 1-hop chain should pass: %v", cerr)
	}
}

func TestVerifyChain_ValidThreeHop(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	chain := []token.DelegRecord{
		buildSignedRecord("agentA", []string{"read:Customers:*"}, priv),
		buildSignedRecord("agentB", []string{"read:Customers:12345"}, priv),
		buildSignedRecord("agentC", []string{"read:Customers:12345"}, priv),
	}

	ok, cerr := VerifyChain(chain, []string{"read:Customers:12345"}, nil, pub)
	if !ok {
		t.Fatalf("valid 3-hop chain should pass: %v", cerr)
	}
}

func TestVerifyChain_BrokenSignatureAtHop2(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	chain := []token.DelegRecord{
		buildSignedRecord("agentA", []string{"read:Customers:*"}, priv),
		buildSignedRecord("agentB", []string{"read:Customers:12345"}, priv),
	}
	// Tamper the signature of hop 1 (0-indexed).
	chain[1].Signature = "deadbeef" + chain[1].Signature[8:]

	ok, cerr := VerifyChain(chain, []string{"read:Customers:12345"}, nil, pub)
	if ok {
		t.Fatal("broken signature should fail")
	}
	if cerr.Hop != 1 {
		t.Errorf("expected hop=1, got hop=%d", cerr.Hop)
	}
	if cerr.Reason != "invalid_signature" {
		t.Errorf("expected reason=invalid_signature, got %s", cerr.Reason)
	}
}

func TestVerifyChain_ScopeEscalationAtHop(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	chain := []token.DelegRecord{
		buildSignedRecord("agentA", []string{"read:Customers:12345"}, priv),
		// Hop 1 tries to escalate to wildcard — scope doesn't subset hop 0.
		buildSignedRecord("agentB", []string{"read:Customers:*"}, priv),
	}

	ok, cerr := VerifyChain(chain, []string{"read:Customers:*"}, nil, pub)
	if ok {
		t.Fatal("scope escalation should fail")
	}
	if cerr.Hop != 1 {
		t.Errorf("expected hop=1, got hop=%d", cerr.Hop)
	}
	if cerr.Reason != "scope_escalation" {
		t.Errorf("expected reason=scope_escalation, got %s", cerr.Reason)
	}
}

func TestVerifyChain_RevokedAgentAtHop0(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	revSvc := revoke.NewRevSvc()
	_ = revSvc.RevokeAgent("agentA", "compromised")

	chain := []token.DelegRecord{
		buildSignedRecord("agentA", []string{"read:Customers:*"}, priv),
	}

	ok, cerr := VerifyChain(chain, []string{"read:Customers:*"}, revSvc, pub)
	if ok {
		t.Fatal("revoked agent should fail")
	}
	if cerr.Hop != 0 {
		t.Errorf("expected hop=0, got hop=%d", cerr.Hop)
	}
	if cerr.Reason != "agent_revoked" {
		t.Errorf("expected reason=agent_revoked, got %s", cerr.Reason)
	}
}

func TestVerifyChain_RevokedAgentInMiddle(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	revSvc := revoke.NewRevSvc()
	_ = revSvc.RevokeAgent("agentB", "compromised")

	chain := []token.DelegRecord{
		buildSignedRecord("agentA", []string{"read:Customers:*"}, priv),
		buildSignedRecord("agentB", []string{"read:Customers:12345"}, priv),
		buildSignedRecord("agentC", []string{"read:Customers:12345"}, priv),
	}

	ok, cerr := VerifyChain(chain, []string{"read:Customers:12345"}, revSvc, pub)
	if ok {
		t.Fatal("revoked agent in middle should fail")
	}
	if cerr.Hop != 1 {
		t.Errorf("expected hop=1, got hop=%d", cerr.Hop)
	}
}

func TestVerifyChain_FinalScopeMismatch(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	chain := []token.DelegRecord{
		buildSignedRecord("agentA", []string{"read:Customers:12345"}, priv),
	}

	// Final scope asks for something the chain doesn't grant.
	ok, cerr := VerifyChain(chain, []string{"write:Customers:12345"}, nil, pub)
	if ok {
		t.Fatal("final scope mismatch should fail")
	}
	if cerr.Reason != "final_scope_mismatch" {
		t.Errorf("expected reason=final_scope_mismatch, got %s", cerr.Reason)
	}
}

func TestVerifyChain_WrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)

	chain := []token.DelegRecord{
		buildSignedRecord("agentA", []string{"read:Customers:*"}, priv),
	}

	ok, cerr := VerifyChain(chain, []string{"read:Customers:*"}, nil, otherPub)
	if ok {
		t.Fatal("wrong key should fail")
	}
	if cerr.Reason != "invalid_signature" {
		t.Errorf("expected reason=invalid_signature, got %s", cerr.Reason)
	}
}

func TestVerifyChainHash_Valid(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	chain := []token.DelegRecord{
		buildSignedRecord("agentA", []string{"read:Customers:*"}, priv),
	}

	hash, err := computeChainHash(chain)
	if err != nil {
		t.Fatal(err)
	}

	ok, cerr := VerifyChainHash(chain, hash)
	if !ok {
		t.Fatalf("valid hash should pass: %v", cerr)
	}
}

func TestVerifyChainHash_Mismatch(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	chain := []token.DelegRecord{
		buildSignedRecord("agentA", []string{"read:Customers:*"}, priv),
	}

	ok, cerr := VerifyChainHash(chain, "badhash")
	if ok {
		t.Fatal("mismatched hash should fail")
	}
	if cerr.Reason != "chain_hash_mismatch" {
		t.Errorf("expected reason=chain_hash_mismatch, got %s", cerr.Reason)
	}
}

func TestVerifyChain_NilRevSvc(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	chain := []token.DelegRecord{
		buildSignedRecord("agentA", []string{"read:Customers:*"}, priv),
	}

	// nil revSvc should be safe — skips revocation check.
	ok, cerr := VerifyChain(chain, []string{"read:Customers:*"}, nil, pub)
	if !ok {
		t.Fatalf("nil revSvc should pass: %v", cerr)
	}
}
