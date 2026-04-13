// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

package store

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/devonartis/agentwrit/internal/audit"
)

// Keep errors import for upcoming cherry-pick tests (agent expiry sentinel checks).
var _ = errors.New

// --- Nonce lifecycle ---

func TestCreateNonce_Returns64HexChars(t *testing.T) {
	st := NewSqlStore()
	nonce := st.CreateNonce()
	if len(nonce) != 64 {
		t.Errorf("expected 64-char hex nonce, got %d chars", len(nonce))
	}
}

func TestConsumeNonce_Success(t *testing.T) {
	st := NewSqlStore()
	nonce := st.CreateNonce()

	if err := st.ConsumeNonce(nonce); err != nil {
		t.Fatalf("unexpected error consuming nonce: %v", err)
	}
}

func TestConsumeNonce_DoubleConsume(t *testing.T) {
	st := NewSqlStore()
	nonce := st.CreateNonce()

	if err := st.ConsumeNonce(nonce); err != nil {
		t.Fatalf("first consume: %v", err)
	}

	err := st.ConsumeNonce(nonce)
	if err != ErrNonceConsumed {
		t.Errorf("expected ErrNonceConsumed on double consume, got: %v", err)
	}
}

func TestConsumeNonce_NotFound(t *testing.T) {
	st := NewSqlStore()

	err := st.ConsumeNonce("nonexistent-nonce")
	if err != ErrNonceNotFound {
		t.Errorf("expected ErrNonceNotFound, got: %v", err)
	}
}

func TestConsumeNonce_Expired(t *testing.T) {
	st := NewSqlStore()
	nonce := st.CreateNonce()

	// Manually backdate the expiry.
	st.mu.Lock()
	st.nonces[nonce].expiresAt = time.Now().Add(-1 * time.Second)
	st.mu.Unlock()

	err := st.ConsumeNonce(nonce)
	if err != ErrNonceNotFound {
		t.Errorf("expected ErrNonceNotFound for expired nonce, got: %v", err)
	}
}

func TestCreateNonce_Uniqueness(t *testing.T) {
	st := NewSqlStore()
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		nonce := st.CreateNonce()
		if seen[nonce] {
			t.Fatalf("duplicate nonce at iteration %d", i)
		}
		seen[nonce] = true
	}
}

// --- Launch Token lifecycle ---

func TestSaveLaunchToken_AndGet(t *testing.T) {
	st := NewSqlStore()

	rec := LaunchTokenRecord{
		Token:        "test-token-abc",
		AgentName:    "data-reader",
		AllowedScope: []string{"read:Customers:*"},
		MaxTTL:       300,
		SingleUse:    true,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(60 * time.Second),
		CreatedBy:    "admin",
	}

	if err := st.SaveLaunchToken(rec); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := st.GetLaunchToken("test-token-abc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AgentName != "data-reader" {
		t.Errorf("expected agent_name=data-reader, got %s", got.AgentName)
	}
	if len(got.AllowedScope) != 1 || got.AllowedScope[0] != "read:Customers:*" {
		t.Errorf("unexpected scope: %v", got.AllowedScope)
	}
}

func TestGetLaunchToken_NotFound(t *testing.T) {
	st := NewSqlStore()

	_, err := st.GetLaunchToken("nonexistent")
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound, got: %v", err)
	}
}

func TestGetLaunchToken_Expired(t *testing.T) {
	st := NewSqlStore()

	rec := LaunchTokenRecord{
		Token:     "expired-token",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Second),
	}
	_ = st.SaveLaunchToken(rec) //nolint:errcheck // test setup

	_, err := st.GetLaunchToken("expired-token")
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got: %v", err)
	}
}

func TestConsumeLaunchToken_Success(t *testing.T) {
	st := NewSqlStore()

	rec := LaunchTokenRecord{
		Token:     "consume-me",
		ExpiresAt: time.Now().UTC().Add(60 * time.Second),
	}
	_ = st.SaveLaunchToken(rec) //nolint:errcheck // test setup

	if err := st.ConsumeLaunchToken("consume-me"); err != nil {
		t.Fatalf("consume: %v", err)
	}

	// After consumption, GetLaunchToken should return ErrTokenConsumed.
	_, err := st.GetLaunchToken("consume-me")
	if err != ErrTokenConsumed {
		t.Errorf("expected ErrTokenConsumed after consumption, got: %v", err)
	}
}

func TestConsumeLaunchToken_DoubleConsume(t *testing.T) {
	st := NewSqlStore()

	rec := LaunchTokenRecord{
		Token:     "double-consume",
		ExpiresAt: time.Now().UTC().Add(60 * time.Second),
	}
	_ = st.SaveLaunchToken(rec) //nolint:errcheck // test setup

	_ = st.ConsumeLaunchToken("double-consume") //nolint:errcheck // test setup
	err := st.ConsumeLaunchToken("double-consume")
	if err != ErrTokenConsumed {
		t.Errorf("expected ErrTokenConsumed on double consume, got: %v", err)
	}
}

func TestConsumeLaunchToken_NotFound(t *testing.T) {
	st := NewSqlStore()

	err := st.ConsumeLaunchToken("nonexistent")
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound, got: %v", err)
	}
}

func TestSaveLaunchToken_PreservesAppID(t *testing.T) {
	st := NewSqlStore()

	rec := LaunchTokenRecord{
		Token:        "app-token-123",
		AgentName:    "weather-agent",
		AllowedScope: []string{"read:weather:*"},
		MaxTTL:       600,
		SingleUse:    true,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(60 * time.Second),
		CreatedBy:    "app-weather-bot-abc123",
		AppID:        "app-weather-bot-abc123",
	}

	if err := st.SaveLaunchToken(rec); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := st.GetLaunchToken("app-token-123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AppID != "app-weather-bot-abc123" {
		t.Errorf("expected AppID=app-weather-bot-abc123, got %q", got.AppID)
	}
}

func TestSaveLaunchToken_AdminHasEmptyAppID(t *testing.T) {
	st := NewSqlStore()

	rec := LaunchTokenRecord{
		Token:        "admin-token-456",
		AgentName:    "data-reader",
		AllowedScope: []string{"read:data:*"},
		MaxTTL:       300,
		SingleUse:    true,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(60 * time.Second),
		CreatedBy:    "admin",
	}

	if err := st.SaveLaunchToken(rec); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := st.GetLaunchToken("admin-token-456")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AppID != "" {
		t.Errorf("expected empty AppID for admin-created token, got %q", got.AppID)
	}
}

// --- Agent CRUD ---

func TestSaveAgent_AndGet(t *testing.T) {
	st := NewSqlStore()

	rec := AgentRecord{
		AgentID:      "spiffe://test/agent/o/t/i",
		PublicKey:    []byte("fake-key-bytes"),
		OrchID:       "orch-1",
		TaskID:       "task-1",
		Scope:        []string{"read:data:*"},
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
	}

	if err := st.SaveAgent(rec); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := st.GetAgent("spiffe://test/agent/o/t/i")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.OrchID != "orch-1" {
		t.Errorf("expected orch_id=orch-1, got %s", got.OrchID)
	}
	if got.TaskID != "task-1" {
		t.Errorf("expected task_id=task-1, got %s", got.TaskID)
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	st := NewSqlStore()

	_, err := st.GetAgent("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got: %v", err)
	}
}

func TestUpdateAgentLastSeen(t *testing.T) {
	st := NewSqlStore()

	past := time.Now().Add(-1 * time.Hour)
	rec := AgentRecord{
		AgentID:  "spiffe://test/agent/o/t/ls",
		LastSeen: past,
	}
	_ = st.SaveAgent(rec) //nolint:errcheck // test setup

	if err := st.UpdateAgentLastSeen("spiffe://test/agent/o/t/ls"); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := st.GetAgent("spiffe://test/agent/o/t/ls")
	if !got.LastSeen.After(past) {
		t.Error("expected LastSeen to be updated to a more recent time")
	}
}

func TestUpdateAgentLastSeen_NotFound(t *testing.T) {
	st := NewSqlStore()

	err := st.UpdateAgentLastSeen("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got: %v", err)
	}
}

func TestSaveAgent_Overwrite(t *testing.T) {
	st := NewSqlStore()

	rec1 := AgentRecord{
		AgentID: "spiffe://test/agent/o/t/ow",
		Scope:   []string{"read:data:*"},
	}
	_ = st.SaveAgent(rec1) //nolint:errcheck // test setup

	rec2 := AgentRecord{
		AgentID: "spiffe://test/agent/o/t/ow",
		Scope:   []string{"write:data:*"},
	}
	_ = st.SaveAgent(rec2) //nolint:errcheck // test setup

	got, _ := st.GetAgent("spiffe://test/agent/o/t/ow")
	if len(got.Scope) != 1 || got.Scope[0] != "write:data:*" {
		t.Errorf("expected overwritten scope [write:data:*], got %v", got.Scope)
	}
}

// --- Agent AppID ---

func TestSaveAgent_PreservesAppID(t *testing.T) {
	st := NewSqlStore()

	rec := AgentRecord{
		AgentID:      "spiffe://test/agent/o/t/app1",
		PublicKey:    []byte("fake-key"),
		OrchID:       "orch-1",
		TaskID:       "task-1",
		Scope:        []string{"read:data:*"},
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
		AppID:        "app-weather-bot-abc123",
	}

	if err := st.SaveAgent(rec); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := st.GetAgent("spiffe://test/agent/o/t/app1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AppID != "app-weather-bot-abc123" {
		t.Errorf("expected AppID=app-weather-bot-abc123, got %q", got.AppID)
	}
}

func TestSaveAgent_NoAppIDByDefault(t *testing.T) {
	st := NewSqlStore()

	rec := AgentRecord{
		AgentID:      "spiffe://test/agent/o/t/noapp",
		PublicKey:    []byte("fake-key"),
		OrchID:       "orch-1",
		TaskID:       "task-1",
		Scope:        []string{"read:data:*"},
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
	}

	if err := st.SaveAgent(rec); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := st.GetAgent("spiffe://test/agent/o/t/noapp")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AppID != "" {
		t.Errorf("expected AppID to be empty, got %q", got.AppID)
	}
}

// --- SQLite audit persistence ---

func TestInitDB_CreatesAuditTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer s.Close()

	// Verify table exists by inserting and querying
	evt := audit.AuditEvent{
		ID: "evt-000001", Timestamp: time.Now().UTC(),
		EventType: "test_event", AgentID: "agent-1",
		TaskID: "task-1", OrchID: "orch-1",
		Detail: "test detail", Hash: "abc123", PrevHash: "000000",
	}
	if err := s.SaveAuditEvent(evt); err != nil {
		t.Fatalf("SaveAuditEvent failed: %v", err)
	}

	events, err := s.LoadAllAuditEvents()
	if err != nil {
		t.Fatalf("LoadAllAuditEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ID != "evt-000001" {
		t.Fatalf("expected evt-000001, got %s", events[0].ID)
	}
}

func TestInitDB_BadPath(t *testing.T) {
	s := NewSqlStore()
	err := s.InitDB("/nonexistent/dir/test.db")
	if err == nil {
		t.Fatal("expected error for bad path")
	}
}

func TestQueryAuditEvents_Filters(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer s.Close()

	// Insert 3 events with different types and agents
	now := time.Now().UTC()
	events := []audit.AuditEvent{
		{ID: "evt-000001", Timestamp: now, EventType: "token_issued", AgentID: "agent-1", Hash: "h1", PrevHash: "p0"},
		{ID: "evt-000002", Timestamp: now.Add(time.Second), EventType: "token_revoked", AgentID: "agent-2", Hash: "h2", PrevHash: "h1"},
		{ID: "evt-000003", Timestamp: now.Add(2 * time.Second), EventType: "token_issued", AgentID: "agent-1", Hash: "h3", PrevHash: "h2"},
	}
	for _, e := range events {
		if err := s.SaveAuditEvent(e); err != nil {
			t.Fatalf("SaveAuditEvent failed: %v", err)
		}
	}

	// Filter by event type
	results, total, err := s.QueryAuditEvents(audit.QueryFilters{EventType: "token_issued"})
	if err != nil {
		t.Fatalf("QueryAuditEvents failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Filter by agent
	results, total, err = s.QueryAuditEvents(audit.QueryFilters{AgentID: "agent-2"})
	if err != nil {
		t.Fatalf("QueryAuditEvents failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	_ = results

	// Pagination
	results, total, err = s.QueryAuditEvents(audit.QueryFilters{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("QueryAuditEvents failed: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "evt-000002" {
		t.Fatalf("expected evt-000002, got %s", results[0].ID)
	}
}

func TestHasDB_FalseBeforeInit(t *testing.T) {
	s := NewSqlStore()
	if s.HasDB() {
		t.Fatal("expected HasDB()=false before InitDB")
	}
}

func TestHasDB_TrueAfterInit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer s.Close()
	if !s.HasDB() {
		t.Fatal("expected HasDB()=true after InitDB")
	}
}

func TestClose_ReleasesDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	// After close, SaveAuditEvent should fail
	evt := audit.AuditEvent{
		ID: "evt-000001", Timestamp: time.Now().UTC(),
		EventType: "test", Hash: "h1", PrevHash: "p0",
	}
	if err := s.SaveAuditEvent(evt); err == nil {
		t.Fatal("expected error after Close, got nil")
	}
}

func TestSaveAuditEvent_WithoutInitDB(t *testing.T) {
	s := NewSqlStore()
	evt := audit.AuditEvent{
		ID: "evt-000001", Timestamp: time.Now().UTC(),
		EventType: "test", Hash: "h1", PrevHash: "p0",
	}
	err := s.SaveAuditEvent(evt)
	if err == nil {
		t.Fatal("expected error saving without InitDB")
	}
}

func TestLoadAllAuditEvents_Empty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer s.Close()

	events, err := s.LoadAllAuditEvents()
	if err != nil {
		t.Fatalf("LoadAllAuditEvents failed: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events on fresh DB, got %d", len(events))
	}
}

func TestLoadAllAuditEvents_OrderById(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer s.Close()

	// Insert in reverse order
	for _, id := range []string{"evt-000003", "evt-000001", "evt-000002"} {
		evt := audit.AuditEvent{
			ID: id, Timestamp: time.Now().UTC(),
			EventType: "test", Hash: id, PrevHash: "p",
		}
		if err := s.SaveAuditEvent(evt); err != nil {
			t.Fatalf("SaveAuditEvent(%s) failed: %v", id, err)
		}
	}

	events, err := s.LoadAllAuditEvents()
	if err != nil {
		t.Fatalf("LoadAllAuditEvents failed: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	// Should be ordered by id ASC
	if events[0].ID != "evt-000001" || events[1].ID != "evt-000002" || events[2].ID != "evt-000003" {
		t.Fatalf("expected order evt-000001,2,3, got %s,%s,%s", events[0].ID, events[1].ID, events[2].ID)
	}
}

func TestSQLite_StructuredAuditFields(t *testing.T) {
	s := NewSqlStore()
	defer s.Close()
	if err := s.InitDB(":memory:"); err != nil {
		t.Fatal(err)
	}

	evt := audit.AuditEvent{
		ID: "evt-000001", Timestamp: time.Now().UTC(),
		EventType: "token_issued", AgentID: "a1", TaskID: "t1", OrchID: "o1",
		Detail: "issued", Hash: "abc", PrevHash: "000",
		Resource: "data:reports", Outcome: "success", DelegDepth: 2,
		DelegChainHash: "chainhash", BytesTransferred: 1024,
	}
	if err := s.SaveAuditEvent(evt); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.LoadAllAuditEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded))
	}
	got := loaded[0]
	if got.Resource != "data:reports" {
		t.Errorf("resource mismatch: %s", got.Resource)
	}
	if got.Outcome != "success" {
		t.Errorf("outcome mismatch: %s", got.Outcome)
	}
	if got.DelegDepth != 2 {
		t.Errorf("deleg_depth mismatch: %d", got.DelegDepth)
	}
	if got.DelegChainHash != "chainhash" {
		t.Errorf("deleg_chain_hash mismatch: %s", got.DelegChainHash)
	}
	if got.BytesTransferred != 1024 {
		t.Errorf("bytes_transferred mismatch: %d", got.BytesTransferred)
	}
}

func TestSQLite_QueryByOutcome(t *testing.T) {
	s := NewSqlStore()
	defer s.Close()
	if err := s.InitDB(":memory:"); err != nil {
		t.Fatal(err)
	}

	if err := s.SaveAuditEvent(audit.AuditEvent{ID: "evt-1", Timestamp: time.Now().UTC(),
		EventType: "token_issued", Hash: "h1", PrevHash: "p1", Outcome: "success"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAuditEvent(audit.AuditEvent{ID: "evt-2", Timestamp: time.Now().UTC(),
		EventType: "scope_violation", Hash: "h2", PrevHash: "h1", Outcome: "denied"}); err != nil {
		t.Fatal(err)
	}

	events, total, err := s.QueryAuditEvents(audit.QueryFilters{Outcome: "denied"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("expected 1 denied, got %d", total)
	}
	if len(events) != 1 || events[0].Outcome != "denied" {
		t.Errorf("expected denied event, got %v", events)
	}
}

func TestQueryAuditEvents_TimestampFilters(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer s.Close()

	now := time.Now().UTC()
	for i, ts := range []time.Time{now.Add(-2 * time.Hour), now.Add(-1 * time.Hour), now} {
		evt := audit.AuditEvent{
			ID: fmt.Sprintf("evt-%06d", i+1), Timestamp: ts,
			EventType: "test", Hash: fmt.Sprintf("h%d", i), PrevHash: "p",
		}
		if err := s.SaveAuditEvent(evt); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	// Since filter: only events in last 90 minutes
	since := now.Add(-90 * time.Minute)
	results, total, err := s.QueryAuditEvents(audit.QueryFilters{Since: &since})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 events since 90min ago, got %d", total)
	}
	_ = results

	// Until filter: only events before 90 minutes ago
	until := now.Add(-90 * time.Minute)
	results, total, err = s.QueryAuditEvents(audit.QueryFilters{Until: &until})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 event before 90min ago, got %d", total)
	}
	_ = results
}
