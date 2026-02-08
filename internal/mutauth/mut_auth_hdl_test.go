package mutauth

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

// testSetup creates the shared infrastructure for handshake tests:
// a store with two registered agents, a TknSvc, and valid tokens for each.
func testSetup(t *testing.T) (
	*MutAuthHdl,
	*store.SqlStore,
	string, // agentA token
	string, // agentB token
	ed25519.PrivateKey, // agentA private key
	ed25519.PrivateKey, // agentB private key
	string, // agentA ID
	string, // agentB ID
	*token.TknSvc,
) {
	t.Helper()

	pubBroker, privBroker, _ := ed25519.GenerateKey(nil)
	tknSvc := token.NewTknSvc(privBroker, pubBroker, cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300})

	st := store.NewSqlStore()

	pubA, privA, _ := ed25519.GenerateKey(nil)
	pubB, privB, _ := ed25519.GenerateKey(nil)

	agentAID := "spiffe://agentauth.local/agent/orch-1/task-1/inst-a"
	agentBID := "spiffe://agentauth.local/agent/orch-1/task-2/inst-b"

	if err := st.SaveAgent(store.AgentRecord{
		AgentID:   agentAID,
		OrchId:    "orch-1",
		TaskId:    "task-1",
		Scope:     []string{"read:Data:*"},
		CreatedAt: time.Now().UTC(),
		PublicKey: pubA,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveAgent(store.AgentRecord{
		AgentID:   agentBID,
		OrchId:    "orch-1",
		TaskId:    "task-2",
		Scope:     []string{"write:Data:*"},
		CreatedAt: time.Now().UTC(),
		PublicKey: pubB,
	}); err != nil {
		t.Fatal(err)
	}

	tokA, err := tknSvc.Issue(token.IssueReq{
		AgentID: agentAID, OrchID: "orch-1", TaskID: "task-1", Scope: []string{"read:Data:*"},
	})
	if err != nil {
		t.Fatal(err)
	}
	tokB, err := tknSvc.Issue(token.IssueReq{
		AgentID: agentBID, OrchID: "orch-1", TaskID: "task-2", Scope: []string{"write:Data:*"},
	})
	if err != nil {
		t.Fatal(err)
	}

	hdl := NewMutAuthHdl(tknSvc, st, nil)
	return hdl, st, tokA.AccessToken, tokB.AccessToken, privA, privB, agentAID, agentBID, tknSvc
}

func TestHandshakeSuccess(t *testing.T) {
	hdl, _, tokA, tokB, _, privB, _, agentBID, _ := testSetup(t)

	req, err := hdl.InitiateHandshake(tokA, agentBID)
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if req.Nonce == "" {
		t.Fatal("expected non-empty nonce")
	}

	resp, err := hdl.RespondToHandshake(req, tokB, privB)
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	if resp.ResponderID != agentBID {
		t.Fatalf("responder ID mismatch: got %s, want %s", resp.ResponderID, agentBID)
	}

	ok, err := hdl.CompleteHandshake(resp, req.Nonce)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if !ok {
		t.Fatal("handshake should have succeeded")
	}
}

func TestHandshakeInvalidInitiatorToken(t *testing.T) {
	hdl, _, _, _, _, _, _, agentBID, _ := testSetup(t)

	_, err := hdl.InitiateHandshake("invalid.token.here", agentBID)
	if !errors.Is(err, ErrHandshakeInvalidToken) {
		t.Fatalf("expected ErrHandshakeInvalidToken, got %v", err)
	}
}

func TestHandshakeUnknownTargetAgent(t *testing.T) {
	hdl, _, tokA, _, _, _, _, _, _ := testSetup(t)

	_, err := hdl.InitiateHandshake(tokA, "spiffe://agentauth.local/agent/orch-1/task-99/unknown")
	if !errors.Is(err, ErrHandshakeUnknownAgent) {
		t.Fatalf("expected ErrHandshakeUnknownAgent, got %v", err)
	}
}

func TestHandshakeUnregisteredInitiator(t *testing.T) {
	// Create an agent with a valid token but NOT registered in the store.
	pubBroker, privBroker, _ := ed25519.GenerateKey(nil)
	tknSvc := token.NewTknSvc(privBroker, pubBroker, cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300})
	st := store.NewSqlStore()

	// Register only the target, not the initiator.
	targetID := "spiffe://agentauth.local/agent/orch-1/task-2/inst-b"
	pubB, _, _ := ed25519.GenerateKey(nil)
	_ = st.SaveAgent(store.AgentRecord{
		AgentID: targetID, OrchId: "orch-1", TaskId: "task-2",
		Scope: []string{"write:Data:*"}, CreatedAt: time.Now().UTC(), PublicKey: pubB,
	})

	ghostID := "spiffe://agentauth.local/agent/orch-1/task-1/ghost"
	tok, _ := tknSvc.Issue(token.IssueReq{
		AgentID: ghostID, OrchID: "orch-1", TaskID: "task-1", Scope: []string{"read:Data:*"},
	})

	hdl := NewMutAuthHdl(tknSvc, st, nil)
	_, err := hdl.InitiateHandshake(tok.AccessToken, targetID)
	if !errors.Is(err, ErrHandshakeUnknownAgent) {
		t.Fatalf("expected ErrHandshakeUnknownAgent for unregistered initiator, got %v", err)
	}
}

func TestHandshakeWrongSigningKey(t *testing.T) {
	hdl, _, tokA, tokB, _, _, _, agentBID, _ := testSetup(t)

	req, err := hdl.InitiateHandshake(tokA, agentBID)
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}

	// Respond using a WRONG private key (not agentB's registered key).
	_, wrongKey, _ := ed25519.GenerateKey(nil)
	resp, err := hdl.RespondToHandshake(req, tokB, wrongKey)
	if err != nil {
		t.Fatalf("respond: %v", err)
	}

	// Completion should fail because the signed nonce won't verify against agentB's registered public key.
	ok, err := hdl.CompleteHandshake(resp, req.Nonce)
	if !errors.Is(err, ErrHandshakeNonceMismatch) {
		t.Fatalf("expected ErrHandshakeNonceMismatch, got ok=%v err=%v", ok, err)
	}
}

func TestHandshakeInvalidResponderToken(t *testing.T) {
	hdl, _, tokA, _, _, privB, _, agentBID, _ := testSetup(t)

	req, err := hdl.InitiateHandshake(tokA, agentBID)
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}

	_, err = hdl.RespondToHandshake(req, "bad.responder.token", privB)
	if !errors.Is(err, ErrHandshakeInvalidToken) {
		t.Fatalf("expected ErrHandshakeInvalidToken, got %v", err)
	}
}

func TestHandshakeInitiatorIDTampering(t *testing.T) {
	hdl, _, tokA, tokB, _, privB, agentAID, agentBID, _ := testSetup(t)

	// A initiates handshake targeting B — produces a valid HandshakeReq.
	req, err := hdl.InitiateHandshake(tokA, agentBID)
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if req.InitiatorID != agentAID {
		t.Fatalf("expected initiator ID %s, got %s", agentAID, req.InitiatorID)
	}

	// Simulate tampering: attacker modifies InitiatorID to claim to be Agent B
	// while carrying Agent A's valid token.
	req.InitiatorID = agentBID

	// B responds — should detect that token subject (A) != declared ID (B).
	_, err = hdl.RespondToHandshake(req, tokB, privB)
	if !errors.Is(err, ErrInitiatorMismatch) {
		t.Fatalf("expected ErrInitiatorMismatch, got %v", err)
	}
}

func TestHandshakePeerMismatch(t *testing.T) {
	hdl, st, tokA, _, _, _, _, agentBID, tknSvc := testSetup(t)

	// Register a third agent (C) — valid but not the intended handshake target.
	pubC, privC, _ := ed25519.GenerateKey(nil)
	agentCID := "spiffe://agentauth.local/agent/orch-1/task-3/inst-c"
	if err := st.SaveAgent(store.AgentRecord{
		AgentID:   agentCID,
		OrchId:    "orch-1",
		TaskId:    "task-3",
		Scope:     []string{"read:Data:*"},
		CreatedAt: time.Now().UTC(),
		PublicKey: pubC,
	}); err != nil {
		t.Fatal(err)
	}
	tokC, err := tknSvc.Issue(token.IssueReq{
		AgentID: agentCID, OrchID: "orch-1", TaskID: "task-3", Scope: []string{"read:Data:*"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// A initiates handshake targeting B.
	req, err := hdl.InitiateHandshake(tokA, agentBID)
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}

	// C (not B) attempts to respond — should be rejected.
	_, err = hdl.RespondToHandshake(req, tokC.AccessToken, privC)
	if !errors.Is(err, ErrPeerMismatch) {
		t.Fatalf("expected ErrPeerMismatch, got %v", err)
	}
}
