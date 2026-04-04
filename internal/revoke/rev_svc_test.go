package revoke

import (
	"strings"
	"testing"
	"time"

	"github.com/devonartis/agentauth/internal/token"
)

// mockRevStore implements RevocationStore for testing.
type mockRevStore struct {
	saved []struct{ Level, Target string }
}

func (m *mockRevStore) SaveRevocation(level, target string) error {
	m.saved = append(m.saved, struct{ Level, Target string }{level, target})
	return nil
}

// nopRevStore is a no-op RevocationStore for tests that don't need persistence.
type nopRevStore struct{}

func (nopRevStore) SaveRevocation(_, _ string) error { return nil }

func TestNewRevSvc_NilStorePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("NewRevSvc(nil) should panic")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "must not be nil") {
			t.Errorf("panic = %v, want 'must not be nil'", r)
		}
	}()
	NewRevSvc(nil)
}

func TestRevoke_PersistsToStore(t *testing.T) {
	ms := &mockRevStore{}
	svc := NewRevSvc(ms)

	_, err := svc.Revoke("token", "jti-abc")
	if err != nil {
		t.Fatal(err)
	}

	if len(ms.saved) != 1 {
		t.Fatalf("expected 1 save call, got %d", len(ms.saved))
	}
	if ms.saved[0].Level != "token" || ms.saved[0].Target != "jti-abc" {
		t.Fatalf("unexpected save: %+v", ms.saved[0])
	}
}

func TestRevSvc_LoadFromEntries(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})
	entries := []struct{ Level, Target string }{
		{"token", "jti-1"},
		{"agent", "agent-a"},
		{"task", "task-1"},
		{"chain", "chain-root"},
	}
	svc.LoadFromEntries(entries)

	claims := &token.TknClaims{Jti: "jti-1", Sub: "agent-a"}
	if !svc.IsRevoked(claims) {
		t.Fatal("expected revoked by JTI")
	}
}

func TestRevoke_InvalidLevel(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})
	_, err := svc.Revoke("invalid", "target")
	if err != ErrInvalidLevel {
		t.Fatalf("expected ErrInvalidLevel, got %v", err)
	}
}

func TestRevoke_MissingTarget(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})
	for _, level := range []string{"token", "agent", "task", "chain"} {
		_, err := svc.Revoke(level, "")
		if err != ErrMissingTarget {
			t.Fatalf("level %q: expected ErrMissingTarget, got %v", level, err)
		}
	}
}

func TestRevoke_AllLevels(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})
	for _, level := range []string{"token", "agent", "task", "chain"} {
		n, err := svc.Revoke(level, "target-"+level)
		if err != nil {
			t.Fatalf("level %q: unexpected error: %v", level, err)
		}
		if n != 1 {
			t.Fatalf("level %q: expected count 1, got %d", level, n)
		}
	}
}

func TestIsRevoked_Token(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})
	claims := &token.TknClaims{Jti: "jti-1", Sub: "agent-a"}

	if svc.IsRevoked(claims) {
		t.Fatal("should not be revoked before any revocation")
	}

	_, _ = svc.Revoke("token", "jti-1") //nolint:errcheck // test setup: error checked in dedicated test
	if !svc.IsRevoked(claims) {
		t.Fatal("should be revoked after token-level revocation")
	}
}

func TestIsRevoked_Agent(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})
	claims := &token.TknClaims{Jti: "jti-2", Sub: "spiffe://example/agent/a"}

	_, _ = svc.Revoke("agent", "spiffe://example/agent/a") //nolint:errcheck // test setup
	if !svc.IsRevoked(claims) {
		t.Fatal("should be revoked after agent-level revocation")
	}
}

func TestIsRevoked_Task(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})
	claims := &token.TknClaims{Jti: "jti-3", Sub: "agent-b", TaskId: "task-42"}

	_, _ = svc.Revoke("task", "task-42") //nolint:errcheck // test setup
	if !svc.IsRevoked(claims) {
		t.Fatal("should be revoked after task-level revocation")
	}
}

func TestIsRevoked_Chain(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})
	rootAgent := "spiffe://example/agent/root"

	// Simulate a 2-level delegation chain: root → mid → leaf.
	// DelegChain[0].Agent is the root delegator.
	claims := &token.TknClaims{
		Jti: "leaf-jti",
		Sub: "spiffe://example/agent/leaf",
		DelegChain: []token.DelegRecord{
			{Agent: rootAgent, Scope: []string{"read:res:*"}, DelegatedAt: time.Now()},
			{Agent: "spiffe://example/agent/mid", Scope: []string{"read:res:*"}, DelegatedAt: time.Now()},
		},
	}

	if svc.IsRevoked(claims) {
		t.Fatal("should not be revoked before chain revocation")
	}

	// Revoke the chain by the root delegator's agent ID.
	_, _ = svc.Revoke("chain", rootAgent) //nolint:errcheck // test setup

	if !svc.IsRevoked(claims) {
		t.Fatal("delegated token should be revoked after chain-level revocation of root agent")
	}
}

func TestIsRevoked_ChainDoesNotAffectNonDelegated(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})
	rootAgent := "spiffe://example/agent/root"

	// A non-delegated token from the same agent should NOT be caught by chain revocation.
	claims := &token.TknClaims{
		Jti: "direct-jti",
		Sub: rootAgent,
	}

	_, _ = svc.Revoke("chain", rootAgent) //nolint:errcheck // test setup

	if svc.IsRevoked(claims) {
		t.Fatal("non-delegated token should not be affected by chain revocation")
	}
}

func TestIsRevoked_ChainSubDelegation(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})
	rootAgent := "spiffe://example/agent/root"

	// A sub-delegation (3 levels) still has the root as DelegChain[0].Agent.
	claims := &token.TknClaims{
		Jti: "sub-deleg-jti",
		Sub: "spiffe://example/agent/deep",
		DelegChain: []token.DelegRecord{
			{Agent: rootAgent, Scope: []string{"read:res:*"}, DelegatedAt: time.Now()},
			{Agent: "spiffe://example/agent/mid", Scope: []string{"read:res:*"}, DelegatedAt: time.Now()},
			{Agent: "spiffe://example/agent/leaf", Scope: []string{"read:res:*"}, DelegatedAt: time.Now()},
		},
	}

	_, _ = svc.Revoke("chain", rootAgent) //nolint:errcheck // test setup

	if !svc.IsRevoked(claims) {
		t.Fatal("sub-delegated token should be revoked when root is chain-revoked")
	}
}

func TestIsRevoked_ChainWrongRoot(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})

	// Revoke a different root — should not affect this chain.
	claims := &token.TknClaims{
		Jti: "other-jti",
		Sub: "spiffe://example/agent/leaf",
		DelegChain: []token.DelegRecord{
			{Agent: "spiffe://example/agent/root-A", Scope: []string{"read:res:*"}, DelegatedAt: time.Now()},
		},
	}

	_, _ = svc.Revoke("chain", "spiffe://example/agent/root-B") //nolint:errcheck // test setup

	if svc.IsRevoked(claims) {
		t.Fatal("chain revocation of a different root should not affect this token")
	}
}

func TestIsRevoked_EmptyDelegChainSkipsChainCheck(t *testing.T) {
	svc := NewRevSvc(nopRevStore{})

	claims := &token.TknClaims{
		Jti: "no-chain-jti",
		Sub: "spiffe://example/agent/solo",
	}

	// Revoke something at chain level — shouldn't match a non-delegated token.
	_, _ = svc.Revoke("chain", "no-chain-jti") //nolint:errcheck // test setup

	if svc.IsRevoked(claims) {
		t.Fatal("non-delegated token should not match chain revocation even if JTI matches target")
	}
}
