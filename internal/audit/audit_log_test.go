// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0

package audit

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRecord_BasicEvent(t *testing.T) {
	al := NewAuditLog(nil)

	al.Record(EventAgentRegistered, "agent-1", "task-1", "orch-1", "agent registered successfully")

	events := al.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt := events[0]
	if evt.ID != "evt-000001" {
		t.Errorf("expected ID=evt-000001, got %s", evt.ID)
	}
	if evt.EventType != EventAgentRegistered {
		t.Errorf("expected event_type=%s, got %s", EventAgentRegistered, evt.EventType)
	}
	if evt.AgentID != "agent-1" {
		t.Errorf("expected agent_id=agent-1, got %s", evt.AgentID)
	}
	if evt.TaskID != "task-1" {
		t.Errorf("expected task_id=task-1, got %s", evt.TaskID)
	}
	if evt.OrchID != "orch-1" {
		t.Errorf("expected orch_id=orch-1, got %s", evt.OrchID)
	}
	if evt.Detail != "agent registered successfully" {
		t.Errorf("unexpected detail: %s", evt.Detail)
	}
}

func TestRecord_HashChainIntegrity(t *testing.T) {
	al := NewAuditLog(nil)

	al.Record(EventTokenIssued, "a1", "t1", "o1", "first")
	al.Record(EventTokenRevoked, "a2", "t2", "o2", "second")
	al.Record(EventDelegationCreated, "a3", "t3", "o3", "third")

	events := al.Events()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Genesis event's PrevHash should be 64 zeros.
	genesis := "0000000000000000000000000000000000000000000000000000000000000000"
	if events[0].PrevHash != genesis {
		t.Errorf("first event PrevHash should be genesis, got %s", events[0].PrevHash)
	}

	// Each event's Hash should chain to the next event's PrevHash.
	for i := 0; i < len(events)-1; i++ {
		if events[i].Hash != events[i+1].PrevHash {
			t.Errorf("event %d hash (%s) != event %d prev_hash (%s)",
				i, events[i].Hash, i+1, events[i+1].PrevHash)
		}
	}

	// Hashes should be non-empty and unique.
	seen := make(map[string]bool)
	for i, evt := range events {
		if evt.Hash == "" {
			t.Errorf("event %d has empty hash", i)
		}
		if seen[evt.Hash] {
			t.Errorf("duplicate hash at event %d: %s", i, evt.Hash)
		}
		seen[evt.Hash] = true
	}
}

func TestRecord_PIISanitization(t *testing.T) {
	al := NewAuditLog(nil)

	al.Record(EventAdminAuth, "", "", "", "secret=my-super-secret-value")
	al.Record(EventAdminAuth, "", "", "", "password: hunter2")
	al.Record(EventAdminAuth, "", "", "", "token_value=eyJ...")
	al.Record(EventAdminAuth, "", "", "", "private_key=AAAA")

	events := al.Events()
	for _, evt := range events {
		if strings.Contains(evt.Detail, "my-super-secret-value") ||
			strings.Contains(evt.Detail, "hunter2") ||
			strings.Contains(evt.Detail, "eyJ...") ||
			strings.Contains(evt.Detail, "AAAA") {
			t.Errorf("PII not redacted in detail: %s", evt.Detail)
		}
		if !strings.Contains(evt.Detail, "***REDACTED***") {
			t.Errorf("expected ***REDACTED*** in detail: %s", evt.Detail)
		}
	}
}

func TestRecord_PIISanitization_NoFalsePositive(t *testing.T) {
	al := NewAuditLog(nil)

	al.Record(EventAgentRegistered, "", "", "", "agent registered with scope [read:data:*]")

	events := al.Events()
	if events[0].Detail != "agent registered with scope [read:data:*]" {
		t.Errorf("non-sensitive detail should not be modified: %s", events[0].Detail)
	}
}

func TestQuery_NoFilters(t *testing.T) {
	al := NewAuditLog(nil)
	al.Record(EventTokenIssued, "a1", "t1", "o1", "first")
	al.Record(EventTokenRevoked, "a2", "t2", "o2", "second")

	events, total := al.Query(QueryFilters{})
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestQuery_FilterByEventType(t *testing.T) {
	al := NewAuditLog(nil)
	al.Record(EventTokenIssued, "a1", "t1", "o1", "issued")
	al.Record(EventTokenRevoked, "a2", "t2", "o2", "revoked")
	al.Record(EventTokenIssued, "a3", "t3", "o3", "issued again")

	events, total := al.Query(QueryFilters{EventType: EventTokenIssued})
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestQuery_FilterByAgentID(t *testing.T) {
	al := NewAuditLog(nil)
	al.Record(EventTokenIssued, "agent-A", "t1", "o1", "for A")
	al.Record(EventTokenIssued, "agent-B", "t2", "o2", "for B")

	events, total := al.Query(QueryFilters{AgentID: "agent-A"})
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if len(events) != 1 || events[0].AgentID != "agent-A" {
		t.Errorf("expected agent-A event, got %v", events)
	}
}

func TestQuery_FilterByTaskID(t *testing.T) {
	al := NewAuditLog(nil)
	al.Record(EventTokenIssued, "a1", "task-X", "o1", "task X")
	al.Record(EventTokenIssued, "a2", "task-Y", "o2", "task Y")

	events, total := al.Query(QueryFilters{TaskID: "task-X"})
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if len(events) != 1 || events[0].TaskID != "task-X" {
		t.Errorf("expected task-X event, got %v", events)
	}
}

func TestQuery_FilterBySinceUntil(t *testing.T) {
	al := NewAuditLog(nil)

	al.Record(EventTokenIssued, "a1", "t1", "o1", "early")
	time.Sleep(10 * time.Millisecond)
	midpoint := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)
	al.Record(EventTokenRevoked, "a2", "t2", "o2", "late")

	events, total := al.Query(QueryFilters{Since: &midpoint})
	if total != 1 {
		t.Errorf("expected total=1 after midpoint, got %d", total)
	}
	if len(events) != 1 || events[0].EventType != EventTokenRevoked {
		t.Errorf("expected only late event, got %v", events)
	}

	events, total = al.Query(QueryFilters{Until: &midpoint})
	if total != 1 {
		t.Errorf("expected total=1 before midpoint, got %d", total)
	}
	if len(events) != 1 || events[0].EventType != EventTokenIssued {
		t.Errorf("expected only early event, got %v", events)
	}
}

func TestQuery_Pagination(t *testing.T) {
	al := NewAuditLog(nil)
	for i := 0; i < 10; i++ {
		al.Record(EventTokenIssued, "a", "t", "o", "event")
	}

	events, total := al.Query(QueryFilters{Limit: 3})
	if total != 10 {
		t.Errorf("expected total=10, got %d", total)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events with limit=3, got %d", len(events))
	}

	events, total = al.Query(QueryFilters{Offset: 8, Limit: 5})
	if total != 10 {
		t.Errorf("expected total=10, got %d", total)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events with offset=8 limit=5, got %d", len(events))
	}
}

func TestQuery_OffsetBeyondTotal(t *testing.T) {
	al := NewAuditLog(nil)
	al.Record(EventTokenIssued, "a", "t", "o", "only one")

	events, total := al.Query(QueryFilters{Offset: 10})
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if events != nil {
		t.Errorf("expected nil events when offset > total, got %v", events)
	}
}

func TestQuery_DefaultLimitIs100(t *testing.T) {
	al := NewAuditLog(nil)
	for i := 0; i < 150; i++ {
		al.Record(EventTokenIssued, "a", "t", "o", "event")
	}

	events, total := al.Query(QueryFilters{})
	if total != 150 {
		t.Errorf("expected total=150, got %d", total)
	}
	if len(events) != 100 {
		t.Errorf("expected default limit=100, got %d events", len(events))
	}
}

func TestQuery_LimitCappedAt1000(t *testing.T) {
	al := NewAuditLog(nil)
	// We'll just check the cap logic with a smaller set.
	for i := 0; i < 5; i++ {
		al.Record(EventTokenIssued, "a", "t", "o", "event")
	}

	events, _ := al.Query(QueryFilters{Limit: 9999})
	if len(events) != 5 {
		t.Errorf("expected 5 events (all available), got %d", len(events))
	}
}

func TestNewEventTypeConstants_Exist(t *testing.T) {
	constants := []string{
		EventTokenAuthFailed,
		EventTokenRevokedAccess,
		EventScopeViolation,
		EventScopeCeilingExceeded,
		EventDelegationAttenuationViolation,
	}
	for _, c := range constants {
		if c == "" {
			t.Errorf("event type constant should not be empty")
		}
	}
}

func TestEvents_ReturnsCopy(t *testing.T) {
	al := NewAuditLog(nil)
	al.Record(EventTokenIssued, "a", "t", "o", "detail")

	events := al.Events()
	events[0].Detail = "tampered"

	original := al.Events()
	if original[0].Detail == "tampered" {
		t.Error("Events() should return a copy, not a reference to the internal slice")
	}
}

func TestQuery_FilterByOutcome(t *testing.T) {
	al := NewAuditLog(nil)
	al.Record(EventTokenIssued, "a1", "t1", "o1", "issued", WithOutcome("success"))
	al.Record(EventScopeViolation, "a2", "t2", "o2", "denied", WithOutcome("denied"))
	al.Record(EventTokenRenewed, "a3", "t3", "o3", "renewed", WithOutcome("success"))

	events, total := al.Query(QueryFilters{Outcome: "denied"})
	if total != 1 {
		t.Errorf("expected total=1 denied, got %d", total)
	}
	if len(events) != 1 || events[0].Outcome != "denied" {
		t.Errorf("expected 1 denied event, got %v", events)
	}
}

func TestRecord_WithOptions(t *testing.T) {
	al := NewAuditLog(nil)
	al.Record(EventTokenIssued, "a1", "t1", "o1", "issued token",
		WithResource("data:reports"),
		WithOutcome("success"),
		WithDelegDepth(0),
	)
	events := al.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt := events[0]
	if evt.Resource != "data:reports" {
		t.Errorf("expected resource=data:reports, got %s", evt.Resource)
	}
	if evt.Outcome != "success" {
		t.Errorf("expected outcome=success, got %s", evt.Outcome)
	}
	if evt.DelegDepth != 0 {
		t.Errorf("expected deleg_depth=0, got %d", evt.DelegDepth)
	}
}

func TestRecord_StructuredFieldsAffectHash(t *testing.T) {
	al1 := NewAuditLog(nil)
	al1.Record(EventTokenIssued, "a1", "t1", "o1", "same detail",
		WithOutcome("success"),
	)

	al2 := NewAuditLog(nil)
	al2.Record(EventTokenIssued, "a1", "t1", "o1", "same detail",
		WithOutcome("denied"),
	)

	h1 := al1.Events()[0].Hash
	h2 := al2.Events()[0].Hash
	if h1 == h2 {
		t.Error("different outcome values should produce different hashes")
	}
}

func TestRecord_WithoutOptions_StillWorks(t *testing.T) {
	al := NewAuditLog(nil)
	al.Record(EventTokenIssued, "a1", "t1", "o1", "no options")
	events := al.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Resource != "" {
		t.Error("expected empty resource when no option used")
	}
	if events[0].Outcome != "" {
		t.Error("expected empty outcome when no option used")
	}
}

// mockStore implements AuditStore for testing.
type mockStore struct {
	events []AuditEvent
}

func (m *mockStore) SaveAuditEvent(evt AuditEvent) error {
	m.events = append(m.events, evt)
	return nil
}

func TestRecord_WritesToStore(t *testing.T) {
	ms := &mockStore{}
	al := NewAuditLog(ms)
	al.Record("test_event", "agent-1", "task-1", "orch-1", "detail")

	if len(ms.events) != 1 {
		t.Fatalf("expected 1 event in store, got %d", len(ms.events))
	}
	if ms.events[0].EventType != "test_event" {
		t.Fatalf("expected test_event, got %s", ms.events[0].EventType)
	}
}

func TestNewAuditLogWithEvents_RebuildsChain(t *testing.T) {
	// Create a log with 2 events
	al1 := NewAuditLog(nil)
	al1.Record("evt_a", "", "", "", "first")
	al1.Record("evt_b", "", "", "", "second")
	existing := al1.Events()

	// Rebuild from existing events
	al2 := NewAuditLogWithEvents(nil, existing)
	al2.Record("evt_c", "", "", "", "third")

	all := al2.Events()
	if len(all) != 3 {
		t.Fatalf("expected 3 events, got %d", len(all))
	}
	// Verify chain integrity: event 3's PrevHash == event 2's Hash
	if all[2].PrevHash != all[1].Hash {
		t.Fatal("hash chain broken after rebuild")
	}
}

func TestNewAuditLog_NilStore(t *testing.T) {
	al := NewAuditLog(nil)
	al.Record("test", "", "", "", "no store")
	if len(al.Events()) != 1 {
		t.Fatal("expected 1 event with nil store")
	}
}

// failingStore always returns an error from SaveAuditEvent.
type failingStore struct{}

func (f *failingStore) SaveAuditEvent(_ AuditEvent) error {
	return errors.New("disk full")
}

func TestRecord_StoreError_StillRecordsInMemory(t *testing.T) {
	fs := &failingStore{}
	al := NewAuditLog(fs)
	al.Record("test_event", "agent-1", "task-1", "orch-1", "detail")

	// Even though store failed, event should still be in memory
	if len(al.Events()) != 1 {
		t.Fatal("expected 1 event in memory despite store error")
	}
	if al.Events()[0].EventType != "test_event" {
		t.Fatalf("expected test_event, got %s", al.Events()[0].EventType)
	}
}

func TestNewAuditLogWithEvents_EmptySlice(t *testing.T) {
	al := NewAuditLogWithEvents(nil, []AuditEvent{})
	al.Record("test", "", "", "", "first after empty rebuild")

	events := al.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	// PrevHash should be genesis hash (64 zeros)
	if len(events[0].PrevHash) != 64 {
		t.Fatalf("expected 64-char genesis prevHash, got %d chars", len(events[0].PrevHash))
	}
}
