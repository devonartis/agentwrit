package deleg

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

func newTestDelegSvc(t *testing.T) (*DelegSvc, *token.TknSvc, *store.SqlStore) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tknSvc := token.NewTknSvc(priv, pub, cfg.Cfg{DefaultTTL: 300})
	st := store.NewSqlStore()
	delegSvc := NewDelegSvc(tknSvc, st, nil)
	return delegSvc, tknSvc, st
}

func registerAgent(t *testing.T, st *store.SqlStore, agentID string) {
	t.Helper()
	err := st.SaveAgent(store.AgentRecord{
		AgentID:      agentID,
		PublicKey:    []byte("fake-key"),
		Scope:        []string{"read:data:*"},
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
	})
	if err != nil {
		t.Fatalf("save agent: %v", err)
	}
}

func TestDelegate_Success(t *testing.T) {
	delegSvc, tknSvc, st := newTestDelegSvc(t)

	delegator := "spiffe://test/agent/o/t/delegator"
	delegate := "spiffe://test/agent/o/t/delegate"
	registerAgent(t, st, delegator)
	registerAgent(t, st, delegate)

	// Issue a token for the delegator.
	issResp, err := tknSvc.Issue(token.IssueReq{
		Sub:   delegator,
		Scope: []string{"read:data:*", "write:data:*"},
		TTL:   300,
	})
	if err != nil {
		t.Fatalf("issue delegator token: %v", err)
	}

	// Delegate with attenuated scope.
	resp, err := delegSvc.Delegate(issResp.Claims, DelegReq{
		DelegateTo: delegate,
		Scope:      []string{"read:data:*"},
		TTL:        60,
	})
	if err != nil {
		t.Fatalf("delegate: %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if resp.ExpiresIn != 60 {
		t.Errorf("expected expires_in=60, got %d", resp.ExpiresIn)
	}
	if len(resp.DelegationChain) != 1 {
		t.Fatalf("expected 1 entry in delegation chain, got %d", len(resp.DelegationChain))
	}
	if resp.DelegationChain[0].Agent != delegator {
		t.Errorf("expected chain[0].Agent=%s, got %s", delegator, resp.DelegationChain[0].Agent)
	}

	// Verify the delegated token is valid.
	claims, err := tknSvc.Verify(resp.AccessToken)
	if err != nil {
		t.Fatalf("delegated token should be valid: %v", err)
	}
	if claims.Sub != delegate {
		t.Errorf("expected sub=%s, got %s", delegate, claims.Sub)
	}
	if len(claims.Scope) != 1 || claims.Scope[0] != "read:data:*" {
		t.Errorf("expected attenuated scope [read:data:*], got %v", claims.Scope)
	}
}

func TestDelegate_ScopeEscalation(t *testing.T) {
	delegSvc, tknSvc, st := newTestDelegSvc(t)

	delegator := "spiffe://test/agent/o/t/d1"
	delegate := "spiffe://test/agent/o/t/d2"
	registerAgent(t, st, delegator)
	registerAgent(t, st, delegate)

	issResp, _ := tknSvc.Issue(token.IssueReq{
		Sub:   delegator,
		Scope: []string{"read:data:*"},
		TTL:   300,
	})

	// Try to escalate scope beyond what delegator has.
	_, err := delegSvc.Delegate(issResp.Claims, DelegReq{
		DelegateTo: delegate,
		Scope:      []string{"read:data:*", "write:data:*"},
	})
	if !errors.Is(err, ErrScopeViolation) {
		t.Errorf("expected ErrScopeViolation, got: %v", err)
	}
}

func TestDelegate_DepthLimit(t *testing.T) {
	delegSvc, tknSvc, st := newTestDelegSvc(t)

	delegate := "spiffe://test/agent/o/t/deep"
	registerAgent(t, st, delegate)

	// Build a chain that is already at maxDelegDepth (5).
	chain := make([]token.DelegRecord, maxDelegDepth)
	for i := range chain {
		chain[i] = token.DelegRecord{
			Agent:       "spiffe://test/agent/o/t/level-" + string(rune('0'+i)),
			Scope:       []string{"read:data:*"},
			DelegatedAt: time.Now(),
		}
	}

	issResp, _ := tknSvc.Issue(token.IssueReq{
		Sub:        "spiffe://test/agent/o/t/at-limit",
		Scope:      []string{"read:data:*"},
		TTL:        300,
		DelegChain: chain,
	})

	_, err := delegSvc.Delegate(issResp.Claims, DelegReq{
		DelegateTo: delegate,
		Scope:      []string{"read:data:*"},
	})
	if !errors.Is(err, ErrDepthExceeded) {
		t.Errorf("expected ErrDepthExceeded, got: %v", err)
	}
}

func TestDelegate_DelegateNotFound(t *testing.T) {
	delegSvc, tknSvc, _ := newTestDelegSvc(t)

	issResp, _ := tknSvc.Issue(token.IssueReq{
		Sub:   "spiffe://test/agent/o/t/d1",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})

	_, err := delegSvc.Delegate(issResp.Claims, DelegReq{
		DelegateTo: "spiffe://test/agent/o/t/nonexistent",
		Scope:      []string{"read:data:*"},
	})
	if !errors.Is(err, ErrDelegateNotFound) {
		t.Errorf("expected ErrDelegateNotFound, got: %v", err)
	}
}

func TestDelegate_MissingDelegateTo(t *testing.T) {
	delegSvc, tknSvc, _ := newTestDelegSvc(t)

	issResp, _ := tknSvc.Issue(token.IssueReq{
		Sub:   "spiffe://test/agent/o/t/d1",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})

	_, err := delegSvc.Delegate(issResp.Claims, DelegReq{
		DelegateTo: "",
		Scope:      []string{"read:data:*"},
	})
	if !errors.Is(err, ErrMissingField) {
		t.Errorf("expected ErrMissingField, got: %v", err)
	}
}

func TestDelegate_MissingScope(t *testing.T) {
	delegSvc, tknSvc, _ := newTestDelegSvc(t)

	issResp, _ := tknSvc.Issue(token.IssueReq{
		Sub:   "spiffe://test/agent/o/t/d1",
		Scope: []string{"read:data:*"},
		TTL:   300,
	})

	_, err := delegSvc.Delegate(issResp.Claims, DelegReq{
		DelegateTo: "spiffe://test/agent/o/t/d2",
		Scope:      nil,
	})
	if !errors.Is(err, ErrMissingField) {
		t.Errorf("expected ErrMissingField, got: %v", err)
	}
}

func TestDelegate_DefaultTTL(t *testing.T) {
	delegSvc, tknSvc, st := newTestDelegSvc(t)

	delegator := "spiffe://test/agent/o/t/dtl1"
	delegate := "spiffe://test/agent/o/t/dtl2"
	registerAgent(t, st, delegator)
	registerAgent(t, st, delegate)

	issResp, _ := tknSvc.Issue(token.IssueReq{
		Sub:   delegator,
		Scope: []string{"read:data:*"},
		TTL:   300,
	})

	// TTL=0 should default to 60.
	resp, err := delegSvc.Delegate(issResp.Claims, DelegReq{
		DelegateTo: delegate,
		Scope:      []string{"read:data:*"},
		TTL:        0,
	})
	if err != nil {
		t.Fatalf("delegate: %v", err)
	}
	if resp.ExpiresIn != 60 {
		t.Errorf("expected default TTL=60, got %d", resp.ExpiresIn)
	}
}

func TestDelegate_ChainGrows(t *testing.T) {
	delegSvc, tknSvc, st := newTestDelegSvc(t)

	a1 := "spiffe://test/agent/o/t/a1"
	a2 := "spiffe://test/agent/o/t/a2"
	a3 := "spiffe://test/agent/o/t/a3"
	registerAgent(t, st, a1)
	registerAgent(t, st, a2)
	registerAgent(t, st, a3)

	// a1 delegates to a2.
	issResp1, _ := tknSvc.Issue(token.IssueReq{
		Sub:   a1,
		Scope: []string{"read:data:*"},
		TTL:   300,
	})
	resp1, err := delegSvc.Delegate(issResp1.Claims, DelegReq{
		DelegateTo: a2,
		Scope:      []string{"read:data:*"},
		TTL:        120,
	})
	if err != nil {
		t.Fatalf("first delegation: %v", err)
	}
	if len(resp1.DelegationChain) != 1 {
		t.Fatalf("expected chain length=1, got %d", len(resp1.DelegationChain))
	}

	// a2 delegates to a3 using the delegated token.
	claims2, err := tknSvc.Verify(resp1.AccessToken)
	if err != nil {
		t.Fatalf("verify delegated token: %v", err)
	}

	resp2, err := delegSvc.Delegate(claims2, DelegReq{
		DelegateTo: a3,
		Scope:      []string{"read:data:*"},
		TTL:        60,
	})
	if err != nil {
		t.Fatalf("second delegation: %v", err)
	}
	if len(resp2.DelegationChain) != 2 {
		t.Fatalf("expected chain length=2, got %d", len(resp2.DelegationChain))
	}
	if resp2.DelegationChain[0].Agent != a1 {
		t.Errorf("expected chain[0].Agent=%s, got %s", a1, resp2.DelegationChain[0].Agent)
	}
	if resp2.DelegationChain[1].Agent != a2 {
		t.Errorf("expected chain[1].Agent=%s, got %s", a2, resp2.DelegationChain[1].Agent)
	}
}
