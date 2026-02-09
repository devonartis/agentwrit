package audit

import (
	"fmt"
	"strings"
	"testing"
)

func TestSanitizeEmail(t *testing.T) {
	evt := &AuditEvt{
		Resource: "user@example.com requested customers/12345",
		Action:   "read by admin@corp.io",
	}
	out := Sanitize(evt)
	if strings.Contains(out.Resource, "user@example.com") {
		t.Fatal("expected email redacted in Resource")
	}
	if !strings.Contains(out.Resource, "[REDACTED_EMAIL]") {
		t.Fatalf("expected [REDACTED_EMAIL] in Resource, got %s", out.Resource)
	}
	if strings.Contains(out.Action, "admin@corp.io") {
		t.Fatal("expected email redacted in Action")
	}
}

func TestSanitizeConsistentHash(t *testing.T) {
	evt1 := &AuditEvt{Resource: "customer 1234567 accessed"}
	evt2 := &AuditEvt{Resource: "customer 1234567 logged in"}
	out1 := Sanitize(evt1)
	out2 := Sanitize(evt2)

	// Both should contain the same CID hash for "1234567".
	if !strings.Contains(out1.Resource, "[CID:") {
		t.Fatalf("expected hashed customer ID in out1, got %s", out1.Resource)
	}
	// Extract the CID hashes and compare.
	cid1 := extractCID(out1.Resource)
	cid2 := extractCID(out2.Resource)
	if cid1 != cid2 {
		t.Fatalf("expected consistent hashes, got %s vs %s", cid1, cid2)
	}
}

func extractCID(s string) string {
	start := strings.Index(s, "[CID:")
	if start == -1 {
		return ""
	}
	end := strings.Index(s[start:], "]")
	if end == -1 {
		return ""
	}
	return s[start : start+end+1]
}

func TestAggregateReads(t *testing.T) {
	events := make([]AuditEvt, 100)
	for i := range events {
		events[i] = AuditEvt{
			EventId:         fmt.Sprintf("evt-%03d", i),
			EventType:       EvtAccessGranted,
			AgentInstanceId: "agent-1",
			Resource:        "customers/12345",
			Action:          "read",
			Outcome:         "granted",
			Timestamp:       fmt.Sprintf("2026-02-07T12:%02d:00Z", i%60),
		}
	}
	result := AggregateReads(events)
	if len(result) != 1 {
		t.Fatalf("expected 1 summary event, got %d", len(result))
	}
	if !strings.Contains(result[0].Action, "x100") {
		t.Fatalf("expected read count in action, got %s", result[0].Action)
	}
}

func TestAggregateSkipsWrites(t *testing.T) {
	events := []AuditEvt{
		{EventId: "1", EventType: EvtAccessGranted, AgentInstanceId: "a", Resource: "r", Action: "read", Outcome: "granted"},
		{EventId: "2", EventType: EvtAccessGranted, AgentInstanceId: "a", Resource: "r", Action: "read", Outcome: "granted"},
		{EventId: "3", EventType: EvtTokenRevoked, AgentInstanceId: "a", Resource: "r", Action: "write", Outcome: "success"},
		{EventId: "4", EventType: EvtAccessGranted, AgentInstanceId: "a", Resource: "r", Action: "read", Outcome: "granted"},
	}
	result := AggregateReads(events)
	if len(result) != 3 {
		t.Fatalf("expected 3 events (aggregated reads + write + single read), got %d", len(result))
	}
	if result[1].EventType != EvtTokenRevoked {
		t.Fatalf("expected write event preserved, got %s", result[1].EventType)
	}
}
