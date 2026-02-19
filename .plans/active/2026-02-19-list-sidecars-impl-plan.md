# List Sidecars Endpoint — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Persist sidecar records to SQLite and expose `GET /v1/admin/sidecars` so operators can enumerate all registered sidecars.

**Architecture:** Dual-write pattern (same as audit persistence). In-memory ceiling map stays for fast reads; SQLite writes happen alongside for durability. On startup, load sidecars from SQLite to populate the ceiling map. New list endpoint reads from SQLite.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), stdlib `net/http`, Prometheus (promauto)

---

### Task 1: Add `SidecarRecord` struct and `sidecars` table DDL

**Files:**
- Modify: `internal/store/sql_store.go`

**Step 1: Add the SidecarRecord struct after AgentRecord (line ~68)**

```go
// SidecarRecord stores the persistent state of an activated sidecar.
type SidecarRecord struct {
	ID        string
	Ceiling   []string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

**Step 2: Add the createSidecarsTable DDL constant after createAuditTable**

```go
const createSidecarsTable = `
CREATE TABLE IF NOT EXISTS sidecars (
	id         TEXT PRIMARY KEY,
	ceiling    TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`
```

**Step 3: Execute the new DDL in InitDB, after the audit table creation**

Add `createSidecarsTable` exec right after the audit table exec block in `InitDB()`.

**Step 4: Run tests to verify nothing broke**

Run: `cd /Users/divineartis/proj/agentAuth && go test ./internal/store/ -v -run TestInitDB`
Expected: PASS (existing InitDB tests still work, new table created silently)

**Step 5: Commit**

```bash
git add internal/store/sql_store.go
git commit -m "feat(store): add SidecarRecord struct and sidecars table DDL"
```

---

### Task 2: Implement SaveSidecar store method + test

**Files:**
- Modify: `internal/store/sql_store.go`
- Modify: `internal/store/sql_store_test.go`

**Step 1: Write the failing test**

```go
func TestSaveSidecar_AndList(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer s.Close()

	err := s.SaveSidecar("sc-001", []string{"read:customer:*", "write:customer:*"})
	if err != nil {
		t.Fatalf("SaveSidecar: %v", err)
	}

	sidecars, err := s.ListSidecars()
	if err != nil {
		t.Fatalf("ListSidecars: %v", err)
	}
	if len(sidecars) != 1 {
		t.Fatalf("expected 1 sidecar, got %d", len(sidecars))
	}
	if sidecars[0].ID != "sc-001" {
		t.Errorf("expected id=sc-001, got %s", sidecars[0].ID)
	}
	if sidecars[0].Status != "active" {
		t.Errorf("expected status=active, got %s", sidecars[0].Status)
	}
	if len(sidecars[0].Ceiling) != 2 || sidecars[0].Ceiling[0] != "read:customer:*" {
		t.Errorf("unexpected ceiling: %v", sidecars[0].Ceiling)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/divineartis/proj/agentAuth && go test ./internal/store/ -v -run TestSaveSidecar`
Expected: FAIL — `SaveSidecar` and `ListSidecars` undefined

**Step 3: Implement SaveSidecar and ListSidecars**

```go
// SaveSidecar persists a new sidecar record to SQLite. The ceiling is
// stored as a JSON array string.
func (s *SqlStore) SaveSidecar(id string, ceiling []string) error {
	if s.db == nil {
		return errors.New("database not initialized: call InitDB first")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	ceilingJSON := "[" + joinQuoted(ceiling) + "]"

	const q = `INSERT INTO sidecars (id, ceiling, status, created_at, updated_at)
		VALUES (?, ?, 'active', ?, ?)
		ON CONFLICT(id) DO UPDATE SET ceiling=excluded.ceiling, updated_at=excluded.updated_at`

	if _, err := s.db.Exec(q, id, ceilingJSON, now, now); err != nil {
		obs.Fail("store", "sqlite", "failed to save sidecar", "id="+id, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("save_sidecar").Inc()
		return fmt.Errorf("save sidecar %s: %w", id, err)
	}
	obs.Ok("store", "sqlite", "sidecar persisted", "id="+id)
	return nil
}

// ListSidecars returns all sidecar records from SQLite ordered by created_at.
func (s *SqlStore) ListSidecars() ([]SidecarRecord, error) {
	if s.db == nil {
		return nil, errors.New("database not initialized: call InitDB first")
	}

	const q = `SELECT id, ceiling, status, created_at, updated_at
		FROM sidecars ORDER BY created_at ASC`

	rows, err := s.db.Query(q)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to list sidecars", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("list_sidecars").Inc()
		return nil, fmt.Errorf("list sidecars: %w", err)
	}
	defer rows.Close()

	var sidecars []SidecarRecord
	for rows.Next() {
		var rec SidecarRecord
		var ceilingJSON, createdStr, updatedStr string
		if err := rows.Scan(&rec.ID, &ceilingJSON, &rec.Status, &createdStr, &updatedStr); err != nil {
			obs.Fail("store", "sqlite", "failed to scan sidecar row", "error="+err.Error())
			obs.DBErrorsTotal.WithLabelValues("scan_sidecar").Inc()
			return nil, fmt.Errorf("scan sidecar: %w", err)
		}
		rec.Ceiling = parseJSONStringArray(ceilingJSON)
		if ts, err := time.Parse(time.RFC3339, createdStr); err == nil {
			rec.CreatedAt = ts
		}
		if ts, err := time.Parse(time.RFC3339, updatedStr); err == nil {
			rec.UpdatedAt = ts
		}
		sidecars = append(sidecars, rec)
	}
	if err := rows.Err(); err != nil {
		obs.Fail("store", "sqlite", "row iteration error on sidecars", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("iterate_sidecars").Inc()
		return nil, fmt.Errorf("iterate sidecars: %w", err)
	}
	return sidecars, nil
}

// joinQuoted produces `"a","b","c"` from a string slice.
func joinQuoted(ss []string) string {
	parts := make([]string, len(ss))
	for i, s := range ss {
		parts[i] = `"` + s + `"`
	}
	return strings.Join(parts, ",")
}

// parseJSONStringArray parses `["a","b"]` into []string.
// Uses encoding/json for correctness.
func parseJSONStringArray(s string) []string {
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}
```

Note: add `"encoding/json"` to the imports.

**Step 4: Run test to verify it passes**

Run: `cd /Users/divineartis/proj/agentAuth && go test ./internal/store/ -v -run TestSaveSidecar`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/sql_store.go internal/store/sql_store_test.go
git commit -m "feat(store): add SaveSidecar and ListSidecars with SQLite persistence"
```

---

### Task 3: Implement UpdateSidecarCeiling and UpdateSidecarStatus store methods + tests

**Files:**
- Modify: `internal/store/sql_store.go`
- Modify: `internal/store/sql_store_test.go`

**Step 1: Write failing tests**

```go
func TestUpdateSidecarCeiling(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer s.Close()

	_ = s.SaveSidecar("sc-001", []string{"read:customer:*"})

	err := s.UpdateSidecarCeiling("sc-001", []string{"read:customer:*", "write:customer:*"})
	if err != nil {
		t.Fatalf("UpdateSidecarCeiling: %v", err)
	}

	sidecars, _ := s.ListSidecars()
	if len(sidecars[0].Ceiling) != 2 {
		t.Errorf("expected 2 ceiling scopes after update, got %d", len(sidecars[0].Ceiling))
	}
}

func TestUpdateSidecarStatus(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer s.Close()

	_ = s.SaveSidecar("sc-001", []string{"read:customer:*"})

	err := s.UpdateSidecarStatus("sc-001", "revoked")
	if err != nil {
		t.Fatalf("UpdateSidecarStatus: %v", err)
	}

	sidecars, _ := s.ListSidecars()
	if sidecars[0].Status != "revoked" {
		t.Errorf("expected status=revoked, got %s", sidecars[0].Status)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/divineartis/proj/agentAuth && go test ./internal/store/ -v -run "TestUpdateSidecar"`
Expected: FAIL — methods undefined

**Step 3: Implement both methods**

```go
// UpdateSidecarCeiling updates the ceiling and updated_at for an existing sidecar.
func (s *SqlStore) UpdateSidecarCeiling(id string, ceiling []string) error {
	if s.db == nil {
		return errors.New("database not initialized: call InitDB first")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	ceilingJSON := "[" + joinQuoted(ceiling) + "]"

	const q = `UPDATE sidecars SET ceiling = ?, updated_at = ? WHERE id = ?`
	res, err := s.db.Exec(q, ceilingJSON, now, id)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to update sidecar ceiling", "id="+id, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("update_sidecar_ceiling").Inc()
		return fmt.Errorf("update sidecar ceiling %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrCeilingNotFound
	}
	obs.Ok("store", "sqlite", "sidecar ceiling updated", "id="+id)
	return nil
}

// UpdateSidecarStatus updates the status and updated_at for an existing sidecar.
func (s *SqlStore) UpdateSidecarStatus(id string, status string) error {
	if s.db == nil {
		return errors.New("database not initialized: call InitDB first")
	}
	now := time.Now().UTC().Format(time.RFC3339)

	const q = `UPDATE sidecars SET status = ?, updated_at = ? WHERE id = ?`
	res, err := s.db.Exec(q, status, now, id)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to update sidecar status", "id="+id, "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("update_sidecar_status").Inc()
		return fmt.Errorf("update sidecar status %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrCeilingNotFound
	}
	obs.Ok("store", "sqlite", "sidecar status updated", "id="+id, "status="+status)
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/divineartis/proj/agentAuth && go test ./internal/store/ -v -run "TestUpdateSidecar"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/sql_store.go internal/store/sql_store_test.go
git commit -m "feat(store): add UpdateSidecarCeiling and UpdateSidecarStatus"
```

---

### Task 4: Implement LoadAllSidecars for startup + test

**Files:**
- Modify: `internal/store/sql_store.go`
- Modify: `internal/store/sql_store_test.go`

**Step 1: Write failing test**

```go
func TestLoadAllSidecars(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer s.Close()

	_ = s.SaveSidecar("sc-001", []string{"read:customer:*"})
	_ = s.SaveSidecar("sc-002", []string{"write:customer:*"})
	_ = s.UpdateSidecarStatus("sc-002", "revoked")

	ceilings, err := s.LoadAllSidecars()
	if err != nil {
		t.Fatalf("LoadAllSidecars: %v", err)
	}
	// Only active sidecars should be loaded
	if len(ceilings) != 1 {
		t.Fatalf("expected 1 active sidecar, got %d", len(ceilings))
	}
	if _, ok := ceilings["sc-001"]; !ok {
		t.Error("expected sc-001 in loaded ceilings")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/divineartis/proj/agentAuth && go test ./internal/store/ -v -run TestLoadAllSidecars`
Expected: FAIL

**Step 3: Implement LoadAllSidecars**

```go
// LoadAllSidecars returns active sidecars as a map of ID→ceiling for
// populating the in-memory ceiling map at startup.
func (s *SqlStore) LoadAllSidecars() (map[string][]string, error) {
	if s.db == nil {
		return nil, errors.New("database not initialized: call InitDB first")
	}

	const q = `SELECT id, ceiling FROM sidecars WHERE status = 'active'`
	rows, err := s.db.Query(q)
	if err != nil {
		obs.Fail("store", "sqlite", "failed to load sidecars", "error="+err.Error())
		obs.DBErrorsTotal.WithLabelValues("load_sidecars").Inc()
		return nil, fmt.Errorf("load sidecars: %w", err)
	}
	defer rows.Close()

	ceilings := make(map[string][]string)
	for rows.Next() {
		var id, ceilingJSON string
		if err := rows.Scan(&id, &ceilingJSON); err != nil {
			obs.Fail("store", "sqlite", "failed to scan sidecar", "error="+err.Error())
			obs.DBErrorsTotal.WithLabelValues("scan_sidecar").Inc()
			return nil, fmt.Errorf("scan sidecar: %w", err)
		}
		ceilings[id] = parseJSONStringArray(ceilingJSON)
	}
	if err := rows.Err(); err != nil {
		obs.Fail("store", "sqlite", "row iteration error on load sidecars", "error="+err.Error())
		return nil, fmt.Errorf("iterate sidecars: %w", err)
	}
	obs.Ok("store", "sqlite", "sidecars loaded", fmt.Sprintf("count=%d", len(ceilings)))
	return ceilings, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/divineartis/proj/agentAuth && go test ./internal/store/ -v -run TestLoadAllSidecars`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/sql_store.go internal/store/sql_store_test.go
git commit -m "feat(store): add LoadAllSidecars for startup ceiling map population"
```

---

### Task 5: Add Prometheus metrics for sidecars

**Files:**
- Modify: `internal/obs/obs.go`

**Step 1: Add metrics after existing metrics (no test needed — declarative)**

```go
// SidecarsTotal tracks the number of registered sidecars by status.
var SidecarsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "agentauth_sidecars_total",
	Help: "Number of registered sidecars",
}, []string{"status"})

// SidecarListDuration observes the time to serve the list sidecars endpoint.
var SidecarListDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "agentauth_sidecar_list_duration_seconds",
	Help:    "Time to serve the list sidecars endpoint",
	Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
})
```

**Step 2: Verify compilation**

Run: `cd /Users/divineartis/proj/agentAuth && go build ./...`
Expected: success

**Step 3: Commit**

```bash
git add internal/obs/obs.go
git commit -m "feat(obs): add sidecars_total gauge and sidecar_list_duration histogram"
```

---

### Task 6: Wire SaveSidecar into ActivateSidecar + add ListSidecars to admin service

**Files:**
- Modify: `internal/admin/admin_svc.go`

**Step 1: In ActivateSidecar(), add SaveSidecar call right after SaveCeiling (line ~362)**

After the existing `s.store.SaveCeiling(sidecarID, scopePrefixes)` call, add:

```go
	if s.store.HasDB() {
		if err := s.store.SaveSidecar(sidecarID, scopePrefixes); err != nil {
			obs.Fail(mod, cmp, "failed to persist sidecar record", "err="+err.Error())
		} else {
			obs.SidecarsTotal.WithLabelValues("active").Inc()
		}
	}
```

**Step 2: Add ListSidecars service method**

```go
// ListSidecars returns all sidecar records from persistent storage.
func (s *AdminSvc) ListSidecars() ([]store.SidecarRecord, error) {
	return s.store.ListSidecars()
}
```

**Step 3: Wire UpdateSidecarCeiling to also update SQLite**

In `UpdateSidecarCeiling()`, after `s.store.SaveCeiling(sidecarID, newCeiling)` (line ~413), add:

```go
	if s.store.HasDB() {
		if err := s.store.UpdateSidecarCeiling(sidecarID, newCeiling); err != nil {
			obs.Fail(mod, cmp, "failed to update sidecar ceiling in SQLite", "err="+err.Error())
		}
	}
```

**Step 4: Verify compilation**

Run: `cd /Users/divineartis/proj/agentAuth && go build ./...`
Expected: success

**Step 5: Commit**

```bash
git add internal/admin/admin_svc.go
git commit -m "feat(admin): wire SaveSidecar into activation, add ListSidecars service method"
```

---

### Task 7: Add handleListSidecars HTTP handler + route registration

**Files:**
- Modify: `internal/admin/admin_hdl.go`

**Step 1: Add response types**

```go
// listSidecarsResp is the JSON response for GET /v1/admin/sidecars.
type listSidecarsResp struct {
	Sidecars []sidecarEntry `json:"sidecars"`
	Total    int            `json:"total"`
}

type sidecarEntry struct {
	SidecarID    string   `json:"sidecar_id"`
	ScopeCeiling []string `json:"scope_ceiling"`
	Status       string   `json:"status"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}
```

**Step 2: Add the handler**

```go
func (h *AdminHdl) handleListSidecars(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	sidecars, err := h.adminSvc.ListSidecars()
	if err != nil {
		obs.Fail(mod, hdlCmp, "list sidecars failed", "err="+err.Error())
		problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to list sidecars", r.URL.Path)
		return
	}

	entries := make([]sidecarEntry, len(sidecars))
	for i, sc := range sidecars {
		entries[i] = sidecarEntry{
			SidecarID:    sc.ID,
			ScopeCeiling: sc.Ceiling,
			Status:       sc.Status,
			CreatedAt:    sc.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    sc.UpdatedAt.Format(time.RFC3339),
		}
	}

	obs.SidecarListDuration.Observe(time.Since(start).Seconds())
	obs.Ok(mod, hdlCmp, "listed sidecars", fmt.Sprintf("count=%d", len(entries)))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(listSidecarsResp{
		Sidecars: entries,
		Total:    len(entries),
	}); err != nil {
		obs.Warn(mod, hdlCmp, "failed to encode list sidecars response", "err="+err.Error())
	}
}
```

Note: add `"time"` to the imports in `admin_hdl.go`.

**Step 3: Register the route in RegisterRoutes()**

Add after the existing `PUT /v1/admin/sidecars/{id}/ceiling` registration:

```go
	mux.Handle("GET /v1/admin/sidecars",
		h.valMw.Wrap(h.valMw.RequireScope("admin:launch-tokens:*",
			http.HandlerFunc(h.handleListSidecars))))
```

**Step 4: Verify compilation**

Run: `cd /Users/divineartis/proj/agentAuth && go build ./...`
Expected: success

**Step 5: Commit**

```bash
git add internal/admin/admin_hdl.go
git commit -m "feat(admin): add GET /v1/admin/sidecars handler and route"
```

---

### Task 8: Wire startup sidecar loading in broker main.go

**Files:**
- Modify: `cmd/broker/main.go`

**Step 1: Add sidecar loading after audit event loading (after line ~83)**

```go
	// Load existing sidecars from SQLite to populate ceiling map
	sidecarCeilings, err := sqlStore.LoadAllSidecars()
	if err != nil {
		obs.Fail("BROKER", "main", "sidecar load failed", "error="+err.Error())
		fmt.Fprintf(os.Stderr, "FATAL: load sidecars: %v\n", err)
		os.Exit(1)
	}
	for id, ceiling := range sidecarCeilings {
		if err := sqlStore.SaveCeiling(id, ceiling); err != nil {
			obs.Fail("BROKER", "main", "failed to restore sidecar ceiling", "id="+id)
		}
	}
	obs.Ok("BROKER", "main", "sidecars loaded", fmt.Sprintf("count=%d", len(sidecarCeilings)))
	obs.SidecarsTotal.WithLabelValues("active").Set(float64(len(sidecarCeilings)))
```

**Step 2: Update the route table comment at top of file**

Add `GET  /v1/admin/sidecars` to the route table comment.

**Step 3: Verify compilation**

Run: `cd /Users/divineartis/proj/agentAuth && go build ./...`
Expected: success

**Step 4: Commit**

```bash
git add cmd/broker/main.go
git commit -m "feat(broker): load sidecars from SQLite on startup, populate ceiling map"
```

---

### Task 9: Integration test — list sidecars endpoint

**Files:**
- Modify: `internal/handler/handler_test.go` (or create `internal/admin/admin_hdl_test.go` if handler_test.go doesn't test admin routes)

**Step 1: Check where existing admin route tests live**

Look at existing test files to determine the right location.

**Step 2: Write integration test**

Test should:
1. Set up a broker with SQLite, admin secret, token service
2. Authenticate as admin
3. Activate a sidecar
4. Call `GET /v1/admin/sidecars` with admin token
5. Verify response has the activated sidecar with correct fields

**Step 3: Run test**

Run: `cd /Users/divineartis/proj/agentAuth && go test ./internal/admin/ -v -run TestListSidecars`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/admin/admin_hdl_test.go
git commit -m "test(admin): add integration test for GET /v1/admin/sidecars"
```

---

### Task 10: Run full test suite + update docs

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `MEMORY.md`

**Step 1: Run the full test suite**

Run: `cd /Users/divineartis/proj/agentAuth && go test ./... -v`
Expected: all PASS

**Step 2: Run gates**

Run: `cd /Users/divineartis/proj/agentAuth && ./scripts/gates.sh task`
Expected: all checks pass

**Step 3: Add CHANGELOG entry**

Add entry under the appropriate section for the list sidecars feature.

**Step 4: Update MEMORY.md (latest entry at TOP)**

Add today's session work at the top of the file, before any existing entries.

**Step 5: Commit**

```bash
git add CHANGELOG.md MEMORY.md
git commit -m "docs: add changelog and memory entries for list sidecars feature"
```

---

## Summary

| Task | What | Key Files |
|------|------|-----------|
| 1 | SidecarRecord struct + DDL | `sql_store.go` |
| 2 | SaveSidecar + ListSidecars | `sql_store.go`, `sql_store_test.go` |
| 3 | UpdateSidecarCeiling + UpdateSidecarStatus | `sql_store.go`, `sql_store_test.go` |
| 4 | LoadAllSidecars (startup) | `sql_store.go`, `sql_store_test.go` |
| 5 | Prometheus metrics | `obs.go` |
| 6 | Wire into admin service | `admin_svc.go` |
| 7 | HTTP handler + route | `admin_hdl.go` |
| 8 | Startup loading in main.go | `main.go` |
| 9 | Integration test | `admin_hdl_test.go` |
| 10 | Full suite + docs | `CHANGELOG.md`, `MEMORY.md` |
