package token

import (
	"strings"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/identity"
)

func newSvc(t *testing.T) *TknSvc {
	t.Helper()
	pub, priv, err := identity.GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	return NewTknSvc(priv, pub, cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300})
}

func TestIssueVerifyRoundTrip(t *testing.T) {
	svc := newSvc(t)
	issued, err := svc.Issue(IssueReq{
		AgentID: "spiffe://agentauth.local/agent/orch/task/inst",
		OrchID:  "orch-1",
		TaskID:  "task-1",
		Scope:   []string{"read:Customers:12345"},
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	claims, err := svc.Verify(issued.AccessToken)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.Sub == "" || claims.TaskId != "task-1" || len(claims.Scope) == 0 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifyBadSignature(t *testing.T) {
	svc := newSvc(t)
	issued, err := svc.Issue(IssueReq{
		AgentID: "spiffe://agentauth.local/agent/orch/task/inst",
		OrchID:  "orch-1",
		TaskID:  "task-1",
		Scope:   []string{"read:Customers:12345"},
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	tampered := issued.AccessToken[:len(issued.AccessToken)-1] + "A"
	if _, err := svc.Verify(tampered); err == nil {
		t.Fatalf("expected signature verification failure")
	}
}

func TestExpiredToken(t *testing.T) {
	svc := newSvc(t)
	claims := TknClaims{
		Iss:    "agentauth://agentauth.local",
		Sub:    "spiffe://agentauth.local/agent/orch/task/inst",
		Scope:  []string{"read:Customers:12345"},
		TaskId: "task-1",
		OrchId: "orch-1",
		Exp:    time.Now().UTC().Add(-2 * time.Minute).Unix(),
	}
	token, err := svc.signClaims(claims)
	if err != nil {
		t.Fatalf("sign claims: %v", err)
	}
	if _, err := svc.Verify(token); err == nil {
		t.Fatalf("expected expired token failure")
	}
}

func TestMissingClaims(t *testing.T) {
	svc := newSvc(t)
	claims := TknClaims{
		Iss: "agentauth://agentauth.local",
		Exp: time.Now().UTC().Add(5 * time.Minute).Unix(),
	}
	token, err := svc.signClaims(claims)
	if err != nil {
		t.Fatalf("sign claims: %v", err)
	}
	if _, err := svc.Verify(token); err == nil {
		t.Fatalf("expected missing claims error")
	}
}

func TestRenew(t *testing.T) {
	svc := newSvc(t)
	issued, err := svc.Issue(IssueReq{
		AgentID: "spiffe://agentauth.local/agent/orch/task/inst",
		OrchID:  "orch-1",
		TaskID:  "task-1",
		Scope:   []string{"read:Customers:12345"},
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	renewed, err := svc.Renew(issued.AccessToken)
	if err != nil {
		t.Fatalf("renew token: %v", err)
	}
	if renewed.AccessToken == issued.AccessToken {
		t.Fatalf("renew should issue a new token")
	}
	if strings.TrimSpace(renewed.AccessToken) == "" {
		t.Fatalf("renewed token must not be empty")
	}
}

