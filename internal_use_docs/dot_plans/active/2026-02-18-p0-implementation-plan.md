# P0 Implementation Plan: Audit Persistence, Sidecar ID, Observability

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add SQLite-backed audit persistence, expose sidecar_id in health endpoint, and wire observability into all new code paths.

**Architecture:** Add `modernc.org/sqlite` as pure-Go SQLite driver. `SqlStore` gets a `*sql.DB` field and audit table methods. `AuditLog` accepts an `AuditStore` interface for write-through persistence. On startup, broker loads events from SQLite to rebuild the hash chain.

**Tech Stack:** Go 1.24+, `modernc.org/sqlite`, `database/sql`, existing `obs` package for logging/metrics, existing `promauto` for Prometheus.

---

### Task 1: Add SQLite dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add modernc.org/sqlite**

Run:
```bash
go get modernc.org/sqlite
```

**Step 2: Verify it resolves**

Run:
```bash
go mod tidy && go build ./...
```
Expected: Clean build, no errors.

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add modernc.org/sqlite pure-Go SQLite driver"
```

---

### Task 2: Add AA_DB_PATH to config

**Files:**
- Modify: `internal/cfg/cfg.go`

**Step 1: Write the failing test**

Create `internal/cfg/cfg_test.go`:

```go
package cfg

import (
	"os"
	"testing"
)

func TestLoad_DBPathDefault(t *testing.T) {
	os.Unsetenv("AA_DB_PATH")
	c := Load()
	if c.DBPath != "./agentauth.db" {
		t.Fatalf("expected default ./agentauth.db, got %q", c.DBPath)
	}
}

func TestLoad_DBPathCustom(t *testing.T) {
	os.Setenv("AA_DB_PATH", "/tmp/test.db")
	defer os.Unsetenv("AA_DB_PATH")
	c := Load()
	if c.DBPath != "/tmp/test.db" {
		t.Fatalf("expected /tmp/test.db, got %q", c.DBPath)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cfg/ -run TestLoad_DBPath -v`
Expected: FAIL — `c.DBPath` doesn't exist yet.

**Step 3: Add DBPath to Cfg struct and Load()**

In `internal/cfg/cfg.go`, add `DBPath string` to the `Cfg` struct (after `SeedTokens`), and add `DBPath: envOr("AA_DB_PATH", "./agentauth.db"),` to the `Load()` function.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cfg/ -run TestLoad_DBPath -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cfg/cfg.go internal/cfg/cfg_test.go
git commit -m "feat(cfg): add AA_DB_PATH env var for SQLite database location"
```

---

### Task 3: Add Prometheus metrics for audit and DB operations

**Files:**
- Modify: `internal/obs/obs.go`

**Step 1: Add new metrics to obs.go**

Add after the existing metric declarations (after `ClockSkewTotal`):

```go
// AuditEventsTotal counts audit events recorded, partitioned by event type.
var AuditEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_audit_events_total",
	Help: "Total number of audit events recorded",
}, []string{"event_type"})

// AuditWriteDuration observes the time to persist an audit event to SQLite.
var AuditWriteDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "agentauth_audit_write_duration_seconds",
	Help:    "Time to write an audit event to SQLite",
	Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
})

// DBErrorsTotal counts database operation errors, partitioned by operation.
var DBErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_db_errors_total",
	Help: "Total number of database errors",
}, []string{"operation"})

// AuditEventsLoaded is set once at startup with the count of events loaded
// from SQLite to rebuild the hash chain.
var AuditEventsLoaded = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "agentauth_audit_events_loaded",
	Help: "Number of audit events loaded from SQLite at startup",
})
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Clean build.

**Step 3: Commit**

```bash
git add internal/obs/obs.go
git commit -m "feat(obs): add Prometheus metrics for audit persistence and DB operations"
```

---

### Task 4: Add SQLite initialization and audit table to SqlStore

**Files:**
- Modify: `internal/store/sql_store.go`
- Modify: `internal/store/sql_store_test.go`

**Step 1: Write the failing test**

Add to `internal/store/sql_store_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -run "TestInitDB|TestQueryAuditEvents" -v`
Expected: FAIL — `InitDB`, `SaveAuditEvent`, `LoadAllAuditEvents`, `QueryAuditEvents`, `Close` don't exist.

**Step 3: Implement SQLite methods in SqlStore**

Add to `internal/store/sql_store.go`:

- `db *sql.DB` field on `SqlStore` struct
- `InitDB(path string) error` — opens SQLite, runs CREATE TABLE + indexes
- `SaveAuditEvent(evt audit.AuditEvent) error` — INSERT with `obs.AuditWriteDuration` timing and `obs.DBErrorsTotal` on error
- `LoadAllAuditEvents() ([]audit.AuditEvent, error)` — SELECT * ORDER BY id ASC
- `QueryAuditEvents(filters audit.QueryFilters) ([]audit.AuditEvent, int, error)` — SELECT with WHERE clauses, COUNT for total, LIMIT/OFFSET
- `Close() error` — close the `*sql.DB`

Add imports: `"database/sql"`, `"time"`, `"github.com/divineartis/agentauth/internal/audit"`, `"github.com/divineartis/agentauth/internal/obs"`, and `_ "modernc.org/sqlite"`.

All DB operations log via `obs.Ok/Fail("store", "sqlite", ...)`.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run "TestInitDB|TestQueryAuditEvents" -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test ./... -short`
Expected: All pass.

**Step 6: Commit**

```bash
git add internal/store/sql_store.go internal/store/sql_store_test.go
git commit -m "feat(store): add SQLite-backed audit event persistence"
```

---

### Task 5: Add AuditStore interface and write-through to AuditLog

**Files:**
- Modify: `internal/audit/audit_log.go`
- Modify: `internal/audit/audit_log_test.go`

**Step 1: Write the failing test**

Add to `internal/audit/audit_log_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/audit/ -run "TestRecord_WritesToStore|TestNewAuditLogWithEvents|TestNewAuditLog_NilStore" -v`
Expected: FAIL — `NewAuditLog` doesn't accept an `AuditStore`, `NewAuditLogWithEvents` doesn't exist.

**Step 3: Implement AuditStore interface and update AuditLog**

In `internal/audit/audit_log.go`:

1. Add `AuditStore` interface:
```go
// AuditStore is the persistence interface for audit events. Implementations
// must be safe for concurrent use.
type AuditStore interface {
	SaveAuditEvent(AuditEvent) error
}
```

2. Add `store AuditStore` field to `AuditLog` struct.

3. Change `NewAuditLog(store AuditStore) *AuditLog` — accepts store (can be nil for memory-only).

4. Add `NewAuditLogWithEvents(store AuditStore, events []AuditEvent) *AuditLog` — sets events slice, counter, and prevHash from last event.

5. In `Record()`, after appending to memory, if `a.store != nil`, call `a.store.SaveAuditEvent(evt)`. On error, log via `obs.Fail("audit", "persist", ...)` but don't block.

6. In `Record()`, increment `obs.AuditEventsTotal.WithLabelValues(eventType).Inc()`.

**Step 4: Fix existing callers of NewAuditLog**

All callers currently call `audit.NewAuditLog()` with no args. Update them to `audit.NewAuditLog(nil)` temporarily:

- `cmd/broker/main.go:68`
- `internal/handler/handler_test.go` (search for `audit.NewAuditLog()`)
- Any other test files that call it

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/audit/ -v`
Expected: All PASS (new + existing tests).

Run: `go test ./... -short`
Expected: All PASS.

**Step 6: Commit**

```bash
git add internal/audit/audit_log.go internal/audit/audit_log_test.go cmd/broker/main.go internal/handler/handler_test.go
git commit -m "feat(audit): add AuditStore interface with write-through persistence"
```

---

### Task 6: Wire SQLite into broker startup

**Files:**
- Modify: `cmd/broker/main.go`

**Step 1: Update main() to init DB and load events**

Replace the `auditLog` initialization section (around line 67-68):

```go
// Initialize SQLite
sqlStore := store.NewSqlStore()
if err := sqlStore.InitDB(c.DBPath); err != nil {
	obs.Fail("BROKER", "main", "database init failed", "error="+err.Error())
	fmt.Fprintf(os.Stderr, "FATAL: init database: %v\n", err)
	os.Exit(1)
}
obs.Ok("BROKER", "main", "database initialized", "path="+c.DBPath)

// Load existing audit events from SQLite
existingEvents, err := sqlStore.LoadAllAuditEvents()
if err != nil {
	obs.Fail("BROKER", "main", "audit event load failed", "error="+err.Error())
	fmt.Fprintf(os.Stderr, "FATAL: load audit events: %v\n", err)
	os.Exit(1)
}
obs.Ok("BROKER", "main", "audit events loaded", fmt.Sprintf("count=%d", len(existingEvents)))
obs.AuditEventsLoaded.Set(float64(len(existingEvents)))

// Initialize audit log with persistence
var auditLog *audit.AuditLog
if len(existingEvents) > 0 {
	auditLog = audit.NewAuditLogWithEvents(sqlStore, existingEvents)
} else {
	auditLog = audit.NewAuditLog(sqlStore)
}
```

**Step 2: Verify build**

Run: `go build ./cmd/broker/`
Expected: Clean build.

**Step 3: Commit**

```bash
git add cmd/broker/main.go
git commit -m "feat(broker): wire SQLite audit persistence into startup"
```

---

### Task 7: Add sidecar_id to health endpoint

**Files:**
- Modify: `cmd/sidecar/handler.go`

**Step 1: Write the failing test**

Add to `cmd/sidecar/handler_test.go`:

```go
func TestHealthHandler_IncludesSidecarID(t *testing.T) {
	state := newSidecarState()
	state.sidecarID = "sc-test-123"
	state.setHealthy(true)
	state.setToken("some-token")
	ceiling := newCeilingCache([]string{"read:customer:*"})
	registry := newAgentRegistry()

	h := newHealthHandler(state, ceiling, registry)
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	sid, ok := resp["sidecar_id"]
	if !ok {
		t.Fatal("expected sidecar_id in health response")
	}
	if sid != "sc-test-123" {
		t.Fatalf("expected sc-test-123, got %v", sid)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/sidecar/ -run TestHealthHandler_IncludesSidecarID -v`
Expected: FAIL — `sidecar_id` not in response.

**Step 3: Add sidecar_id to health response**

In `cmd/sidecar/handler.go`, inside the `if h.state != nil` block (around line 329), add:

```go
resp["sidecar_id"] = h.state.sidecarID
```

Add trace log:

```go
obs.Trace("sidecar", "health", "health check served", "sidecar_id="+h.state.sidecarID)
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/sidecar/ -run TestHealthHandler_IncludesSidecarID -v`
Expected: PASS

**Step 5: Run full sidecar tests**

Run: `go test ./cmd/sidecar/ -v`
Expected: All PASS.

**Step 6: Commit**

```bash
git add cmd/sidecar/handler.go cmd/sidecar/handler_test.go
git commit -m "feat(sidecar): expose sidecar_id in health endpoint response"
```

---

### Task 8: Add db_connected and audit_events_count to broker health endpoint

**Files:**
- Modify: `internal/handler/health_hdl.go`
- Modify: `internal/handler/handler_test.go`

**Step 1: Review current HealthHdl**

Read `internal/handler/health_hdl.go` to understand current implementation.

**Step 2: Add audit log reference to HealthHdl**

Update `HealthHdl` to accept an `*audit.AuditLog` and expose `db_connected` (store has DB) and `audit_events_count` in the response.

Note: This requires passing the audit log reference. If HealthHdl doesn't currently hold one, add it as a field and update `NewHealthHdl` and the caller in `main.go`.

**Step 3: Add test for new health fields**

**Step 4: Verify all tests pass**

Run: `go test ./... -short`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/handler/health_hdl.go internal/handler/handler_test.go cmd/broker/main.go
git commit -m "feat(handler): add db_connected and audit_events_count to broker health"
```

---

### Task 9: Live test — audit persistence survives restart

**Files:**
- None (test against running broker)

**Step 1: Start broker**

```bash
AA_ADMIN_SECRET=test123 AA_DB_PATH=/tmp/aa-live-test.db AA_SEED_TOKENS=true go run ./cmd/broker &
```

**Step 2: Generate audit events**

Use the seed admin token to trigger operations (register an agent, exchange a token, etc.) that produce audit events.

**Step 3: Query audit events**

```bash
curl -s -H "Authorization: Bearer $SEED_ADMIN_TOKEN" http://localhost:8080/v1/audit/events | python3 -m json.tool
```

Verify events are returned.

**Step 4: Stop broker**

```bash
kill %1
```

**Step 5: Verify SQLite file has data**

```bash
sqlite3 /tmp/aa-live-test.db "SELECT count(*) FROM audit_events;"
```

Expected: Non-zero count.

**Step 6: Restart broker and verify events persist**

```bash
AA_ADMIN_SECRET=test123 AA_DB_PATH=/tmp/aa-live-test.db go run ./cmd/broker &
```

Query audit events again — should return the same events from before restart.

**Step 7: Verify hash chain integrity**

The first event loaded should have PrevHash matching the genesis hash, and each subsequent event's PrevHash should match the previous event's Hash.

**Step 8: Check Prometheus metrics**

```bash
curl -s http://localhost:8080/v1/metrics | grep agentauth_audit
curl -s http://localhost:8080/v1/metrics | grep agentauth_db
```

Expected: `agentauth_audit_events_total`, `agentauth_audit_write_duration_seconds`, `agentauth_audit_events_loaded` all present with values.

**Step 9: Cleanup**

```bash
kill %1
rm /tmp/aa-live-test.db
```

---

### Task 10: Update operator docs for runtime ceiling management

**Files:**
- Modify: `docs/getting-started-operator.md`

**Step 1: Add "Runtime Ceiling Management" section**

After the existing env var section, add:
- Explanation that `AA_SIDECAR_SCOPE_CEILING` is the bootstrap seed only
- Runtime update via broker admin API (`PUT /v1/admin/sidecars/{id}/ceiling`)
- Sidecar picks up changes on renewal cycle (4-12 minutes)
- CLI examples (if available) or curl examples
- Emergency narrowing triggers immediate revocation of tokens exceeding new ceiling
- New `AA_DB_PATH` env var for audit persistence

**Step 2: Commit**

```bash
git add docs/getting-started-operator.md
git commit -m "docs(operator): add runtime ceiling management and AA_DB_PATH documentation"
```

---

### Task 11: Run gates

**Step 1: Run task-level gates**

```bash
./scripts/gates.sh task
```

Expected: All gates pass.

**Step 2: Fix any issues found by gates**

**Step 3: Final commit if needed**

---

## Task Summary

| Task | Description | Type |
|------|------------|------|
| 1 | Add modernc.org/sqlite dependency | Setup |
| 2 | Add AA_DB_PATH config | TDD |
| 3 | Add Prometheus metrics for audit/DB | Code |
| 4 | SQLite init + audit table in SqlStore | TDD |
| 5 | AuditStore interface + write-through | TDD |
| 6 | Wire SQLite into broker startup | Integration |
| 7 | Sidecar health returns sidecar_id | TDD |
| 8 | Broker health shows db_connected + audit count | TDD |
| 9 | Live test — audit survives restart | Live test |
| 10 | Operator docs for ceiling management | Docs |
| 11 | Run gates | Verification |
