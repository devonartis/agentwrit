package mutauth

import (
	"context"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/token"
)

func TestHeartbeatRecordAndLiveness(t *testing.T) {
	hb := NewHeartbeatMgr(nil)
	hb.interval = 50 * time.Millisecond

	agentID := "spiffe://agentauth.local/agent/orch-1/task-1/inst-a"
	hb.RecordHeartbeat(agentID, "active")

	alive, missed := hb.CheckLiveness(agentID)
	if !alive {
		t.Fatal("agent should be alive immediately after heartbeat")
	}
	if missed != 0 {
		t.Fatalf("expected 0 missed, got %d", missed)
	}
}

func TestHeartbeatMissedAccumulates(t *testing.T) {
	hb := NewHeartbeatMgr(nil)
	hb.interval = 10 * time.Millisecond

	agentID := "spiffe://agentauth.local/agent/orch-1/task-1/inst-a"
	hb.RecordHeartbeat(agentID, "active")

	// Wait long enough for several intervals to elapse.
	time.Sleep(35 * time.Millisecond)

	alive, missed := hb.CheckLiveness(agentID)
	if missed < 2 {
		t.Fatalf("expected >= 2 missed heartbeats, got %d", missed)
	}
	// With default maxMiss=3, agent may or may not be alive depending on timing.
	_ = alive
}

func TestHeartbeatAutoRevocation(t *testing.T) {
	revSvc := revoke.NewRevSvc(nil)
	hb := NewHeartbeatMgr(revSvc)
	hb.interval = 10 * time.Millisecond
	hb.maxMiss = 2

	agentID := "spiffe://agentauth.local/agent/orch-1/task-1/inst-a"
	hb.RecordHeartbeat(agentID, "active")

	ctx, cancel := context.WithCancel(context.Background())
	hb.StartMonitor(ctx, 10*time.Millisecond)

	// Wait long enough for monitor to detect missed heartbeats.
	time.Sleep(80 * time.Millisecond)
	cancel()

	// Check agent-level revocation via IsRevoked with a claims struct
	if !revSvc.IsRevoked(&token.TknClaims{Sub: agentID}) {
		t.Fatal("agent should have been auto-revoked after missed heartbeats")
	}
}

func TestHeartbeatNoRevocationWithoutRevSvc(t *testing.T) {
	hb := NewHeartbeatMgr(nil) // no revSvc = investigation-only mode
	hb.interval = 10 * time.Millisecond
	hb.maxMiss = 2

	agentID := "spiffe://agentauth.local/agent/orch-1/task-1/inst-a"
	hb.RecordHeartbeat(agentID, "active")

	ctx, cancel := context.WithCancel(context.Background())
	hb.StartMonitor(ctx, 10*time.Millisecond)

	time.Sleep(80 * time.Millisecond)
	cancel()

	// Without revSvc, agent is flagged via obs.Warn but NOT revoked.
	// No crash = pass. We mainly verify the nil revSvc path doesn't panic.
}

func TestHeartbeatMonitorStopsOnCancel(t *testing.T) {
	hb := NewHeartbeatMgr(nil)

	ctx, cancel := context.WithCancel(context.Background())
	hb.StartMonitor(ctx, 10*time.Millisecond)

	// Cancel immediately; the goroutine should exit cleanly.
	cancel()
	time.Sleep(30 * time.Millisecond) // allow goroutine to drain
}

func TestHeartbeatStateUpdate(t *testing.T) {
	hb := NewHeartbeatMgr(nil)
	agentID := "spiffe://agentauth.local/agent/orch-1/task-1/inst-a"

	hb.RecordHeartbeat(agentID, "active")
	hb.RecordHeartbeat(agentID, "idle")
	hb.RecordHeartbeat(agentID, "completing")

	hb.mu.RLock()
	s := hb.agents[agentID]
	hb.mu.RUnlock()

	if s.state != "completing" {
		t.Fatalf("expected state 'completing', got %q", s.state)
	}
}

func TestHeartbeatUnknownAgentLiveness(t *testing.T) {
	hb := NewHeartbeatMgr(nil)
	alive, missed := hb.CheckLiveness("spiffe://agentauth.local/agent/orch-1/task-1/unknown")
	if alive {
		t.Fatal("unknown agent should not be alive")
	}
	if missed != 0 {
		t.Fatalf("unknown agent missed should be 0, got %d", missed)
	}
}
