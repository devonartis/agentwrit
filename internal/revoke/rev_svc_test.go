package revoke

import (
	"testing"
)

func TestRevokeAndCheckToken(t *testing.T) {
	svc := NewRevSvc()
	jti := "test-jti-001"
	if svc.IsTokenRevoked(jti) {
		t.Fatal("expected token not revoked before revocation")
	}
	if err := svc.RevokeToken(jti, "compromised"); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if !svc.IsTokenRevoked(jti) {
		t.Fatal("expected token revoked after revocation")
	}
}

func TestRevokeAndCheckAgent(t *testing.T) {
	svc := NewRevSvc()
	agentID := "spiffe://agentauth.local/agent/orch/task/inst"
	if svc.IsAgentRevoked(agentID) {
		t.Fatal("expected agent not revoked before revocation")
	}
	if err := svc.RevokeAgent(agentID, "decommissioned"); err != nil {
		t.Fatalf("RevokeAgent: %v", err)
	}
	if !svc.IsAgentRevoked(agentID) {
		t.Fatal("expected agent revoked after revocation")
	}
}

func TestRevokeAndCheckTask(t *testing.T) {
	svc := NewRevSvc()
	taskID := "task-789"
	if svc.IsTaskRevoked(taskID) {
		t.Fatal("expected task not revoked before revocation")
	}
	if err := svc.RevokeTask(taskID, "task completed"); err != nil {
		t.Fatalf("RevokeTask: %v", err)
	}
	if !svc.IsTaskRevoked(taskID) {
		t.Fatal("expected task revoked after revocation")
	}
}

func TestRevokeAndCheckChain(t *testing.T) {
	svc := NewRevSvc()
	chainHash := "abc123def456"
	if svc.IsChainRevoked(chainHash) {
		t.Fatal("expected chain not revoked before revocation")
	}
	if err := svc.RevokeDelegChain(chainHash, "chain compromised"); err != nil {
		t.Fatalf("RevokeDelegChain: %v", err)
	}
	if !svc.IsChainRevoked(chainHash) {
		t.Fatal("expected chain revoked after revocation")
	}
}

func TestIsRevokedMultiLevel(t *testing.T) {
	svc := NewRevSvc()
	jti := "jti-multi"
	agentID := "agent-multi"
	taskID := "task-multi"
	chainHash := "chain-multi"

	// Nothing revoked.
	revoked, level := svc.IsRevoked(jti, agentID, taskID, chainHash)
	if revoked {
		t.Fatalf("expected not revoked, got level=%s", level)
	}

	// Revoke at token level — should match first.
	_ = svc.RevokeToken(jti, "test")
	revoked, level = svc.IsRevoked(jti, agentID, taskID, chainHash)
	if !revoked || level != "token" {
		t.Fatalf("expected revoked at token level, got revoked=%v level=%s", revoked, level)
	}

	// Revoke at agent level — token check still matches first.
	_ = svc.RevokeAgent(agentID, "test")
	revoked, level = svc.IsRevoked(jti, agentID, taskID, chainHash)
	if !revoked || level != "token" {
		t.Fatalf("expected revoked at token level (priority), got revoked=%v level=%s", revoked, level)
	}

	// With only agent revoked (different jti).
	revoked, level = svc.IsRevoked("other-jti", agentID, taskID, chainHash)
	if !revoked || level != "agent" {
		t.Fatalf("expected revoked at agent level, got revoked=%v level=%s", revoked, level)
	}

	// Revoke task — check with unrevoked jti and agent.
	_ = svc.RevokeTask(taskID, "test")
	revoked, level = svc.IsRevoked("other-jti", "other-agent", taskID, chainHash)
	if !revoked || level != "task" {
		t.Fatalf("expected revoked at task level, got revoked=%v level=%s", revoked, level)
	}

	// Revoke chain — check with everything else unrevoked.
	_ = svc.RevokeDelegChain(chainHash, "test")
	revoked, level = svc.IsRevoked("other-jti", "other-agent", "other-task", chainHash)
	if !revoked || level != "delegation_chain" {
		t.Fatalf("expected revoked at delegation_chain level, got revoked=%v level=%s", revoked, level)
	}
}

func TestNotRevoked(t *testing.T) {
	svc := NewRevSvc()
	if svc.IsTokenRevoked("nonexistent") {
		t.Fatal("expected false for non-revoked token")
	}
	if svc.IsAgentRevoked("nonexistent") {
		t.Fatal("expected false for non-revoked agent")
	}
	if svc.IsTaskRevoked("nonexistent") {
		t.Fatal("expected false for non-revoked task")
	}
	if svc.IsChainRevoked("nonexistent") {
		t.Fatal("expected false for non-revoked chain")
	}
	revoked, level := svc.IsRevoked("a", "b", "c", "d")
	if revoked {
		t.Fatalf("expected not revoked, got level=%s", level)
	}
}

func TestRevokeRecordReason(t *testing.T) {
	svc := NewRevSvc()
	reason := "policy violation detected"
	_ = svc.RevokeToken("jti-reason", reason)
	svc.mu.RLock()
	rec, ok := svc.tokens["jti-reason"]
	svc.mu.RUnlock()
	if !ok {
		t.Fatal("expected revocation record to exist")
	}
	if rec.Reason != reason {
		t.Fatalf("expected reason %q, got %q", reason, rec.Reason)
	}
	if rec.Level != "token" {
		t.Fatalf("expected level 'token', got %q", rec.Level)
	}
	if rec.RevokedAt.IsZero() {
		t.Fatal("expected non-zero RevokedAt")
	}
}

func TestRevCheckerInterface(t *testing.T) {
	svc := NewRevSvc()
	// Compile-time interface check.
	var _ RevChecker = svc
}
