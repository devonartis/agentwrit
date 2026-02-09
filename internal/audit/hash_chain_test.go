package audit

import (
	"fmt"
	"testing"
)

func TestHashEventDeterministic(t *testing.T) {
	evt := &AuditEvt{
		EventId:         "evt-001",
		EventType:       EvtAccessGranted,
		Timestamp:       "2026-02-07T12:00:00Z",
		AgentInstanceId: "spiffe://agentauth.local/agent/orch/task/inst",
		TaskId:          "task-1",
		OrchId:          "orch-1",
		Resource:        "customers/12345",
		Action:          "read",
		Outcome:         "granted",
	}
	h1 := HashEvent(evt, "")
	h2 := HashEvent(evt, "")
	if h1 != h2 {
		t.Fatalf("expected deterministic hash, got %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("expected 64-char hex hash, got %d chars", len(h1))
	}
}

func TestVerifyChainValid(t *testing.T) {
	events := buildChain(10)
	ok, idx := VerifyChain(events)
	if !ok {
		t.Fatalf("expected valid chain, first invalid at %d", idx)
	}
}

func TestVerifyChainTampered(t *testing.T) {
	events := buildChain(10)
	events[5].Action = "tampered"
	ok, idx := VerifyChain(events)
	if ok {
		t.Fatal("expected invalid chain after tampering")
	}
	if idx != 5 {
		t.Fatalf("expected first invalid at 5, got %d", idx)
	}
}

func TestVerifyChainEmpty(t *testing.T) {
	ok, idx := VerifyChain(nil)
	if !ok {
		t.Fatal("expected empty chain to be valid")
	}
	if idx != -1 {
		t.Fatalf("expected index -1 for empty chain, got %d", idx)
	}
}

func buildChain(n int) []AuditEvt {
	events := make([]AuditEvt, 0, n)
	prevHash := ""
	for i := 0; i < n; i++ {
		evt := AuditEvt{
			EventId:         fmt.Sprintf("evt-%03d", i),
			EventType:       EvtAccessGranted,
			Timestamp:       fmt.Sprintf("2026-02-07T12:%02d:00Z", i),
			AgentInstanceId: "spiffe://agentauth.local/agent/orch/task/inst",
			TaskId:          "task-1",
			OrchId:          "orch-1",
			Resource:        "customers/12345",
			Action:          "read",
			Outcome:         "granted",
			PrevHash:        prevHash,
		}
		evt.EventHash = HashEvent(&evt, prevHash)
		prevHash = evt.EventHash
		events = append(events, evt)
	}
	return events
}
