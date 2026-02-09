package audit

import (
	"fmt"
	"testing"
)

func TestLogAndQueryRoundTrip(t *testing.T) {
	al := NewAuditLog()
	evt := &AuditEvt{
		EventType:       EvtAccessGranted,
		AgentInstanceId: "agent-1",
		TaskId:          "task-1",
		OrchId:          "orch-1",
		Resource:        "customers/12345",
		Action:          "read",
		Outcome:         "granted",
	}
	if err := al.LogEvent(evt); err != nil {
		t.Fatalf("LogEvent: %v", err)
	}

	events, total, err := al.QueryEvents(AuditFilter{})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != EvtAccessGranted {
		t.Fatalf("expected event_type=%s, got %s", EvtAccessGranted, events[0].EventType)
	}
	if events[0].EventId == "" {
		t.Fatal("expected non-empty event_id")
	}
	if events[0].Timestamp == "" {
		t.Fatal("expected non-empty timestamp")
	}
	if events[0].EventHash == "" {
		t.Fatal("expected non-empty event_hash")
	}
}

func TestChainIntegrityAfterMultipleEvents(t *testing.T) {
	al := NewAuditLog()
	for i := 0; i < 5; i++ {
		if err := al.LogEvent(&AuditEvt{
			EventType:       EvtAccessGranted,
			AgentInstanceId: fmt.Sprintf("agent-%d", i),
			TaskId:          "task-1",
			OrchId:          "orch-1",
			Resource:        "res",
			Action:          "read",
			Outcome:         "granted",
		}); err != nil {
			t.Fatalf("LogEvent %d: %v", i, err)
		}
	}

	events, _, _ := al.QueryEvents(AuditFilter{})
	ok, idx := VerifyChain(events)
	if !ok {
		t.Fatalf("chain invalid at index %d", idx)
	}
}

func TestQueryFilterByType(t *testing.T) {
	al := NewAuditLog()
	_ = al.LogEvent(&AuditEvt{EventType: EvtAccessGranted, Outcome: "granted"})
	_ = al.LogEvent(&AuditEvt{EventType: EvtAccessDenied, Outcome: "denied"})
	_ = al.LogEvent(&AuditEvt{EventType: EvtAccessGranted, Outcome: "granted"})

	events, total, _ := al.QueryEvents(AuditFilter{EventType: EvtAccessDenied})
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != EvtAccessDenied {
		t.Fatalf("expected %s, got %s", EvtAccessDenied, events[0].EventType)
	}
}

func TestQueryFilterByAgent(t *testing.T) {
	al := NewAuditLog()
	_ = al.LogEvent(&AuditEvt{EventType: EvtAccessGranted, AgentInstanceId: "agent-a", Outcome: "granted"})
	_ = al.LogEvent(&AuditEvt{EventType: EvtAccessGranted, AgentInstanceId: "agent-b", Outcome: "granted"})
	_ = al.LogEvent(&AuditEvt{EventType: EvtAccessGranted, AgentInstanceId: "agent-a", Outcome: "granted"})

	events, total, _ := al.QueryEvents(AuditFilter{AgentId: "agent-a"})
	if total != 2 {
		t.Fatalf("expected total=2, got %d", total)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestQueryPagination(t *testing.T) {
	al := NewAuditLog()
	for i := 0; i < 25; i++ {
		_ = al.LogEvent(&AuditEvt{
			EventType:       EvtAccessGranted,
			AgentInstanceId: "agent-1",
			Outcome:         "granted",
		})
	}

	// First page.
	events, total, _ := al.QueryEvents(AuditFilter{Limit: 10, Offset: 0})
	if total != 25 {
		t.Fatalf("expected total=25, got %d", total)
	}
	if len(events) != 10 {
		t.Fatalf("expected 10 events on page 1, got %d", len(events))
	}

	// Second page.
	events2, total2, _ := al.QueryEvents(AuditFilter{Limit: 10, Offset: 10})
	if total2 != 25 {
		t.Fatalf("expected total=25, got %d", total2)
	}
	if len(events2) != 10 {
		t.Fatalf("expected 10 events on page 2, got %d", len(events2))
	}

	// Third page (partial).
	events3, _, _ := al.QueryEvents(AuditFilter{Limit: 10, Offset: 20})
	if len(events3) != 5 {
		t.Fatalf("expected 5 events on page 3, got %d", len(events3))
	}

	// Beyond range.
	events4, _, _ := al.QueryEvents(AuditFilter{Limit: 10, Offset: 30})
	if len(events4) != 0 {
		t.Fatalf("expected 0 events beyond range, got %d", len(events4))
	}
}
