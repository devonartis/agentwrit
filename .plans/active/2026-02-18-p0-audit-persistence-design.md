# P0 Design: Audit Persistence, Sidecar ID, CLI Auto-Discovery

**Date:** 2026-02-18
**Branch:** coWork/p0-audit-persistence-and-fixes
**Status:** Approved

---

## Scope

Four P0 backlog items plus mandatory observability for all changes:

| # | Item | Repo | Type |
|---|------|------|------|
| 0 | Audit log persistence to SQLite | Go broker | Feature |
| 1 | Sidecar health returns sidecar_id | Go broker | Fix |
| 2 | CLI auto-discover sidecar ID | Python app | Fix |
| 3 | Operator docs for runtime ceiling mgmt | Docs | Docs |

---

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| SQLite driver | `modernc.org/sqlite` (pure Go) | Zero CGo, cross-compiles, no C compiler needed. Performance is sufficient for audit volumes. |
| DB file location | `AA_DB_PATH` env var, default `./agentauth.db` | Configurable, consistent with existing `AA_*` pattern. |
| Persistence model | Write-through (memory + SQLite) | Fast in-memory queries, durable on-disk storage. Startup loads from SQLite to rebuild hash chain. |
| Observability | Mandatory for all code paths | Every new operation gets `obs.*` logging + Prometheus metrics. This is a permanent project rule. |

---

## P0-0: Audit Persistence to SQLite

### Schema

```sql
CREATE TABLE IF NOT EXISTS audit_events (
    id         TEXT PRIMARY KEY,
    timestamp  TEXT NOT NULL,
    event_type TEXT NOT NULL,
    agent_id   TEXT DEFAULT '',
    task_id    TEXT DEFAULT '',
    orch_id    TEXT DEFAULT '',
    detail     TEXT DEFAULT '',
    hash       TEXT NOT NULL,
    prev_hash  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audit_events(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_agent ON audit_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp);
```

### Code Changes

**`internal/cfg/cfg.go`** — Add:
- `AA_DB_PATH` (default: `./agentauth.db`)

**`internal/store/sql_store.go`** — Add:
- `InitDB(path string) error` — open SQLite, run migrations, return error on failure
- `SaveAuditEvent(evt audit.AuditEvent) error` — INSERT into audit_events
- `QueryAuditEvents(filters audit.QueryFilters) ([]audit.AuditEvent, int, error)` — SELECT with filters, pagination, total count
- `LoadAllAuditEvents() ([]audit.AuditEvent, error)` — SELECT * ORDER BY id ASC (for startup chain rebuild)
- `Close() error` — close DB connection

**`internal/audit/audit_log.go`** — Change:
- Define `AuditStore` interface: `SaveAuditEvent(AuditEvent) error`
- `NewAuditLog(store AuditStore) *AuditLog` — accepts optional store; nil = memory-only (backwards compatible for tests)
- `NewAuditLogWithEvents(store AuditStore, events []AuditEvent) *AuditLog` — rebuild chain state from preloaded events
- `Record()` — after appending to memory, call `store.SaveAuditEvent()` if store is non-nil. Log error via obs.Fail but don't block (audit should not crash the broker).

**`cmd/broker/main.go`** — Change:
- Call `store.InitDB(cfg.DBPath)` at startup
- Call `store.LoadAllAuditEvents()` to get existing events
- Pass events + store to `audit.NewAuditLogWithEvents()`
- Call `store.Close()` on shutdown

### Observability

**Prometheus metrics (new in `internal/obs/obs.go`):**
- `agentauth_audit_events_total{event_type}` — counter, incremented on every `Record()` call
- `agentauth_audit_write_duration_seconds` — histogram, time to write to SQLite
- `agentauth_db_errors_total{operation}` — counter, incremented on any DB error (write, read, init)
- `agentauth_audit_events_loaded` — gauge, set once at startup with count of events loaded from SQLite

**Structured logging:**
- `obs.Ok("store", "sqlite", "database initialized", "path", dbPath)` — on successful init
- `obs.Ok("store", "sqlite", "audit events loaded", "count", N)` — on startup load
- `obs.Ok("audit", "record", "event persisted", "id", evt.ID, "type", evt.EventType)` — on each write (trace level)
- `obs.Fail("store", "sqlite", "write failed", "error", err.Error())` — on write error
- `obs.Fail("store", "sqlite", "init failed", "error", err.Error())` — on init error

**Health endpoint:**
- Add `db_connected: true/false` and `audit_events_count: N` to broker `GET /v1/health` response

---

## P0-1: Sidecar Health Returns sidecar_id

### Code Changes

**`broker/cmd/sidecar/handler.go`** (~line 329) — Add to health response:
```go
if h.state != nil {
    resp["sidecar_id"] = h.state.sidecarID
}
```

### Observability

- `obs.Trace("sidecar", "health", "sidecar_id included in health response", "id", sidecarID)` — trace level on health check

---

## P0-2: CLI Auto-Discover Sidecar ID

### Code Changes

**`app/cli/operator.py`** — Change `sidecar_update_ceiling()` and `sidecar_get_ceiling()`:
- Make `--sidecar-id` optional (default None)
- If not provided: query sidecar health endpoint, extract `sidecar_id`
- If health endpoint doesn't return `sidecar_id`: error with upgrade message
- Log auto-discovery via typer.echo

### Observability

- CLI echo: `"Auto-discovered sidecar: {sidecar_id}"` on successful discovery
- CLI echo: `"Error: Sidecar health endpoint doesn't return sidecar_id..."` on failure

---

## P0-3: Operator Docs — Runtime Ceiling Management

### Content Changes

**`docs/getting-started-operator.md`** — Add new section "Runtime Ceiling Management" after env var section:
1. Explain that `AA_SIDECAR_SCOPE_CEILING` is the bootstrap seed only
2. CLI examples: `show-ceiling`, `get-ceiling`, `update-ceiling`
3. Explain renewal cycle (ceiling changes take effect within 4-12 minutes)
4. Emergency narrowing and auto-revocation behavior
5. Reference to User Stories doc for full persona walkthroughs

---

## Testing Strategy

**All testing is live — no mocks.** Bring up broker + sidecar, hit real endpoints, verify real behavior.

| Component | Test Type | What |
|-----------|-----------|------|
| SQLite store | Unit | SaveAuditEvent, QueryAuditEvents with filters, LoadAllAuditEvents, InitDB with bad path |
| Audit log | Unit | Record with store, NewAuditLogWithEvents chain rebuild, nil store fallback |
| Audit persistence | Live | Start broker, trigger events (register agent, exchange token), restart broker, verify events survived via `GET /v1/audit/events` |
| Sidecar health | Live | Start sidecar, hit `GET /v1/health`, verify `sidecar_id` in response |
| Metrics | Live | Start broker, trigger operations, hit `GET /v1/metrics`, verify new Prometheus counters/histograms are present and incrementing |
| Hash chain integrity | Live | Start broker, generate events, restart, verify chain rebuilds correctly (no broken hashes) |
| Full stack | Live | `docker compose up`, run smoketest, verify audit events persist across container restart |

---

## Non-Goals

- Full SQL migration of agents/tokens/nonces (future work)
- Audit event streaming/webhooks
- Audit log compaction or rotation
