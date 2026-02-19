# Design: List Sidecars Endpoint with SQLite Persistence

**Date:** 2026-02-19
**Branch:** TBD (feature branch from develop)
**Status:** Approved
**Backlog Ref:** #5 (List active sidecars)
**Roadmap Ref:** 5.3

---

## Problem

Operators have no way to enumerate registered sidecars. The ceiling map is in-memory only — sidecars are lost on broker restart. Without visibility, operators can't manage what they can't see.

## Decision

**Approach B — Dual-write (in-memory + SQLite).**

Same pattern as audit persistence (`4c2733d`). Write-through to SQLite, in-memory map for fast reads, load from SQLite on startup. Existing `SaveCeiling`/`GetCeiling` callers don't change.

---

## Endpoint

```
GET /v1/admin/sidecars
Authorization: Bearer <admin-token>
Required scope: admin:launch-tokens:*
```

**Response:**
```json
{
  "sidecars": [
    {
      "sidecar_id": "sc-abc123",
      "scope_ceiling": ["read:customer:*", "write:customer:*"],
      "status": "active",
      "created_at": "2026-02-18T18:57:58Z",
      "updated_at": "2026-02-18T19:30:00Z"
    }
  ],
  "total": 1
}
```

No filtering, no pagination. Returns all sidecars.

---

## Storage

### SQLite Table

```sql
CREATE TABLE IF NOT EXISTS sidecars (
    id         TEXT PRIMARY KEY,
    ceiling    TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

`ceiling` stored as JSON array string (e.g. `'["read:customer:*","write:customer:*"]'`).

### New SqlStore Methods

- `SaveSidecar(id string, ceiling []string) error` — INSERT on activation
- `UpdateSidecarCeiling(id string, ceiling []string) error` — UPDATE ceiling + updated_at
- `UpdateSidecarStatus(id string, status string) error` — UPDATE status (for revocation)
- `ListSidecars() ([]SidecarRecord, error)` — SELECT all
- `LoadAllSidecars() (map[string][]string, error)` — startup: load into in-memory ceiling map

### Write-Through

Existing in-memory ceiling map stays for fast reads. SQLite writes happen alongside:
- `ActivateSidecar()` → `store.SaveCeiling()` + `store.SaveSidecar()`
- `UpdateSidecarCeiling()` → `store.SaveCeiling()` (memory) + `store.UpdateSidecarCeiling()` (SQLite)
- Revocation → `store.UpdateSidecarStatus(id, "revoked")`

### Startup

`LoadAllSidecars()` populates the in-memory ceiling map before the broker accepts traffic.

---

## Code Changes

| File | Change |
|------|--------|
| `internal/store/sql_store.go` | Add `sidecars` table, `SidecarRecord` struct, `SaveSidecar`, `UpdateSidecarCeiling`, `UpdateSidecarStatus`, `ListSidecars`, `LoadAllSidecars` |
| `internal/store/sql_store_test.go` | Tests for all new store methods |
| `internal/admin/admin_svc.go` | In `ActivateSidecar()` call `store.SaveSidecar()`. Add `ListSidecars()` service method |
| `internal/admin/admin_hdl.go` | Add `handleListSidecars` handler, register `GET /v1/admin/sidecars` route |
| `internal/handler/handler_test.go` | Integration test for the new endpoint |
| `internal/obs/obs.go` | Add `SidecarsTotal` gauge, `SidecarListDuration` histogram |
| `cmd/broker/main.go` | On startup, call `LoadAllSidecars()` to populate ceiling map from SQLite |

---

## Observability

### Prometheus Metrics

- `agentauth_sidecars_total` — gauge, set on startup and updated on activation/revocation
- `agentauth_sidecar_list_duration_seconds` — histogram, time to serve list endpoint

### Structured Logging

| Path | Log |
|------|-----|
| Startup load success | `obs.Ok("store", "sqlite", "sidecars loaded", "count=N")` |
| Startup load failure | `obs.Fail("store", "sqlite", "sidecar load failed", "error=...")` |
| SaveSidecar success | `obs.Ok("store", "sqlite", "sidecar persisted", "id=...", "ceiling=...")` |
| SaveSidecar failure | `obs.Fail("store", "sqlite", "sidecar persist failed", "error=...")` |
| UpdateSidecarCeiling success | `obs.Ok("store", "sqlite", "sidecar ceiling updated", "id=...")` |
| UpdateSidecarCeiling failure | `obs.Fail("store", "sqlite", "sidecar ceiling update failed", "error=...")` |
| UpdateSidecarStatus success | `obs.Ok("store", "sqlite", "sidecar status updated", "id=...", "status=...")` |
| UpdateSidecarStatus failure | `obs.Fail("store", "sqlite", "sidecar status update failed", "error=...")` |
| ListSidecars success | `obs.Ok("admin", "list-sidecars", "listed sidecars", "count=N")` |
| ListSidecars DB failure | `obs.Fail("admin", "list-sidecars", "list failed", "error=...")` |

### DB Error Counters

`obs.DBErrorsTotal.WithLabelValues("save_sidecar")`, `"update_sidecar_ceiling"`, `"update_sidecar_status"`, `"list_sidecars"` on any DB error.

---

## Auth

Same as existing ceiling endpoints: `admin:launch-tokens:*` scope, wrapped with `valMw`. Follows the route registration pattern in `RegisterRoutes()`.

---

## Testing Strategy

| Component | Test Type | What |
|-----------|-----------|------|
| SaveSidecar | Unit | Insert and verify fields |
| UpdateSidecarCeiling | Unit | Update ceiling, verify updated_at changes |
| UpdateSidecarStatus | Unit | Change status to revoked |
| ListSidecars | Unit | Insert multiple, list all, verify order |
| LoadAllSidecars | Unit | Insert sidecars, load into map, verify |
| List endpoint | Integration | Admin auth → list → verify JSON response |
| Startup load | Integration | Insert sidecars in DB, restart, verify ceiling map populated |
| Observability | Integration | Verify Prometheus metrics present after operations |

---

## Non-Goals

- Filtering or pagination on the list endpoint
- Sidecar health/connectivity status (sidecar reports its own health)
- Agent count per sidecar (separate feature, backlog #6)
- Ceiling persistence migration for existing in-memory sidecars (they re-register on restart anyway)
