# Phase 0: Legacy Cleanup & Documentation Reconciliation

**Phase:** 0 (P-CRITICAL — blocks Phase 1B merge)
**Priority:** Must complete before Phase 1B Docker live test
**Effort:** 0.5 day
**Depends on:** Phase 1a (complete), Phase 1b code (complete)
**Created:** 2026-03-04, Session 25
**Status:** Not started

---

## Why This Phase Exists

Session 25 attempted to run the Phase 1B Docker live test and had to stop. Three problems were discovered:

1. **Sidecar routes exposed on running broker** — 6 endpoints respond to HTTP requests but there is no running sidecar in the Docker stack. Anyone hitting the broker sees endpoints for infrastructure that doesn't exist.
2. **Admin auth endpoint has wrong request shape** — takes `{"client_id": "admin", "client_secret": "..."}` but `client_id` is ignored. Looks like app auth but isn't. Legacy from pre-PRD v2-respec baseline.
3. **Documentation not updated for Phase 1a** — `docs/api.md`, `docs/aactl-reference.md`, getting-started guides all predate the app-centric redesign. Tests were being written against endpoints that shouldn't exist.

**Core lesson (Divine, Session 25):** "we should be updating docs and removing things not leave REST API exposed that is old or outdated" and "we need to stop and review what tech debt is creating."

---

## Task 0.1 — Remove Sidecar Routes from Broker

**File:** `cmd/broker/main.go`

Remove the following route registrations:

| Route | Handler | What to Remove |
|-------|---------|---------------|
| `GET /v1/admin/sidecars` | `adminHdl.ListSidecars` | Route registration + handler method |
| `POST /v1/admin/sidecar-activations` | `adminHdl.CreateSidecarActivation` | Route registration + handler method |
| `POST /v1/sidecar/activate` | `adminHdl.ActivateSidecar` | Route registration + handler method |
| `GET /v1/admin/sidecars/{id}/ceiling` | `adminHdl.GetSidecarCeiling` | Route registration + handler method |
| `PUT /v1/admin/sidecars/{id}/ceiling` | `adminHdl.UpdateSidecarCeiling` | Route registration + handler method |
| `POST /v1/token/exchange` | `hdl.TokenExchange` | Route registration + handler method |

**What to preserve:**
- All sidecar code in `cmd/sidecar/` — untouched
- `internal/store/` sidecar-related methods — keep for Phase 2
- `internal/admin/` sidecar handler methods — can stay in source file, just unwire from mux
- `internal/handler/` token exchange method — can stay in source file, just unwire from mux

**What to remove:**
- Route registrations in `main.go` (the `mux.HandleFunc` or `mux.Handle` lines)
- Any sidecar-specific service initialization in `main.go` (if no other route needs it)
- Update any middleware chains that reference sidecar-specific auth

**Tests to update:**
- Any integration test that calls sidecar routes from the broker should be removed or skipped
- Unit tests for the handler methods themselves can remain (they test the method, not the route)

**Verification:**
- `curl http://127.0.0.1:8080/v1/admin/sidecars` returns 404
- `curl -X POST http://127.0.0.1:8080/v1/admin/sidecar-activations` returns 404
- `curl -X POST http://127.0.0.1:8080/v1/sidecar/activate` returns 404
- `curl http://127.0.0.1:8080/v1/admin/sidecars/test/ceiling` returns 404
- `curl -X POST http://127.0.0.1:8080/v1/token/exchange` returns 404
- All existing non-sidecar routes still work

---

## Task 0.2 — Fix Admin Auth Endpoint Shape (TD-004)

**Files:**
- `internal/admin/admin_hdl.go` — change request struct
- `internal/admin/admin_hdl_test.go` — update tests
- `cmd/aactl/client.go` or `cmd/aactl/admin.go` — update admin auth call
- `tests/phase-1a/env.sh` — update if it contains admin auth examples
- `tests/phase-1b/env.sh` — update if it contains admin auth examples

**Current request shape:**
```json
{"client_id": "admin", "client_secret": "the-admin-secret"}
```

**New request shape:**
```json
{"secret": "the-admin-secret"}
```

**Implementation:**
1. Change the admin auth request struct from `clientID`+`clientSecret` fields to a single `secret` field
2. Update the handler to read `secret` from the new shape
3. If someone sends the old shape, return a clear error: `{"type": "invalid_request", "title": "Admin auth format changed", "detail": "Use {\"secret\": \"...\"} instead of client_id/client_secret"}`
4. Update `aactl` admin auth command to send new shape
5. Update all test files

**Verification:**
- `curl -X POST http://127.0.0.1:8080/v1/admin/auth -d '{"secret":"test"}' -H 'Content-Type: application/json'` works
- `curl -X POST http://127.0.0.1:8080/v1/admin/auth -d '{"client_id":"admin","client_secret":"test"}' -H 'Content-Type: application/json'` returns clear error
- `aactl` admin commands work with new shape
- All unit tests pass

---

## Task 0.3 — Update Codebase-Map.md (TD-006)

**File:** `.plans/Codebase-Map.md`

**Add:**
- `apps` table DDL and schema
- `AppRecord` struct (fields: ID, Name, ClientID, ClientSecretHash, Scopes, Status, CreatedAt, UpdatedAt)
- `app:` JWT scope family: `app:launch-tokens:*`, `app:agents:*`, `app:audit:read`
- `internal/app/` package: AppSvc, AppHdl
- App CRUD endpoints: `POST/GET /v1/admin/apps`, `GET/PUT/DELETE /v1/admin/apps/{id}`
- App auth endpoint: `POST /v1/app/auth`
- Phase 1b additions: `AppID` field on `LaunchTokenRecord` and `AgentRecord`
- `RequireAnyScope` middleware in `internal/authz/val_mw.go`
- Per-client_id rate limiting on app auth

**Update:**
- Route table: remove sidecar routes, add app routes
- Service list: add AppSvc
- Middleware section: add RequireAnyScope

**Mark as deferred (not active):**
- `internal/mutauth/` — Go API complete, HTTP exposure planned, not currently wired
- Sidecar routes — removed in Phase 0, returning in Phase 2

---

## Task 0.4 — Documentation Reconciliation (TD-007)

**Target:** All files in `docs/`

This task updates application documentation to reflect the current state of the system. The principle: if a user reads the docs and tries what they say, it should work against the running broker.

### docs/api.md

**Add:**
- App registration endpoints (POST/GET/PUT/DELETE `/v1/admin/apps`, with request/response examples)
- App auth endpoint (`POST /v1/app/auth`, with request/response example)
- Updated admin auth endpoint (new `{"secret": "..."}` shape)

**~~Strikethrough~~:**
- Sidecar activation endpoints — mark with: "~~Removed in Phase 0 (2026-03-04). Returns in Phase 2 with app-scoped activation tokens.~~"
- Token exchange endpoint — mark with: "~~Removed in Phase 0. Returns in Phase 2.~~"
- Sidecar ceiling endpoints — mark with: "~~Removed in Phase 0. Returns in Phase 2.~~"
- Sidecar list endpoint — mark with: "~~Removed in Phase 0. Returns in Phase 2.~~"
- Old admin auth request shape — mark with: "~~`client_id`/`client_secret` shape deprecated. Use `{"secret": "..."}` instead.~~"

### docs/aactl-reference.md

**Add:**
- `aactl app register --name NAME --scopes SCOPES` — register app, returns client_id + client_secret
- `aactl app list` — list all apps
- `aactl app get --id ID` — get app details
- `aactl app update --id ID --scopes SCOPES` — update app scopes
- `aactl app remove --id ID` — deregister app

### docs/getting-started-developer.md

**Add:**
- App auth flow as primary getting-started path (register app -> authenticate -> create launch token -> register agent)
- Show curl examples for the full app-based flow

**~~Strikethrough~~:**
- Sidecar-based flow as the "only" way — mark with: "~~Sidecar flow shown here requires Phase 2 activation tokens. For now, use app auth flow above.~~"

### docs/getting-started-operator.md

**Add:**
- App registration workflow: `aactl app register`, sharing credentials with developer, setting scope ceiling
- Updated admin auth flow with new `{"secret": "..."}` shape

### docs/architecture.md

**Add:**
- App entity in system component diagram
- App -> Launch Token -> Agent traceability chain
- Three paths to broker: SDK (Phase 3), Token Proxy (Phase 2), raw HTTP (now)

**~~Strikethrough~~:**
- "Sidecar is the only path" — mark with: "~~Sidecar was the only connection method. As of Phase 1a, apps authenticate directly.~~"

### docs/sidecar-deployment.md

**Add at top:**
- Status banner: "NOTE: Sidecar routes are currently not wired in the broker (removed in Phase 0, 2026-03-04). Sidecar functionality returns in Phase 2 with app-scoped activation tokens. This document describes the sidecar design and will be updated when Phase 2 ships."

### docs/concepts.md

**Add:**
- App identity concept: what an app is, how it relates to agents, the traceability chain
- App scopes and ceiling concept

---

## Task 0.5 — Docker Live Test for Cleanup

**Prerequisite:** Tasks 0.1 and 0.2 complete, `go test ./...` passes

**Steps:**
1. `./scripts/stack_up.sh` — bring up the stack
2. `curl http://127.0.0.1:8080/v1/health` — verify broker is healthy
3. Verify sidecar routes return 404:
   - `curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8080/v1/admin/sidecars` -> 404
   - `curl -s -o /dev/null -w "%{http_code}" -X POST http://127.0.0.1:8080/v1/admin/sidecar-activations` -> 404
   - `curl -s -o /dev/null -w "%{http_code}" -X POST http://127.0.0.1:8080/v1/sidecar/activate` -> 404
   - `curl -s -o /dev/null -w "%{http_code}" -X POST http://127.0.0.1:8080/v1/token/exchange` -> 404
4. Verify admin auth works with new shape:
   - `curl -X POST http://127.0.0.1:8080/v1/admin/auth -d '{"secret":"..."}' -H 'Content-Type: application/json'` -> 200 + JWT
   - `curl -X POST http://127.0.0.1:8080/v1/admin/auth -d '{"client_id":"admin","client_secret":"..."}' -H 'Content-Type: application/json'` -> 400 + clear error
5. Run Phase 1a regression stories (from `tests/phase-1a/user-stories.md`):
   - Story 0: Admin auth (new shape)
   - Story 6: Developer app auth
   - Story 8: App JWT scope isolation
   - Story 11: Admin auth unchanged (updated for new shape)
6. `docker compose down -v` — tear down

**Evidence:** Save to `tests/phase-0/evidence/`

---

## Acceptance Criteria

- [ ] No sidecar routes respond on the running broker (all return 404)
- [ ] Admin auth accepts `{"secret": "..."}` — old shape returns clear error message
- [ ] `aactl` admin commands work with new auth shape
- [ ] `go test ./...` passes across all packages
- [ ] Codebase-Map.md reflects current state (Phase 1a + 1b additions, sidecar routes removed)
- [ ] `docs/api.md` has app endpoints; sidecar endpoints marked with strikethrough
- [ ] `docs/aactl-reference.md` has `aactl app` commands
- [ ] `docs/getting-started-developer.md` shows app auth flow
- [ ] `docs/sidecar-deployment.md` has status banner about Phase 0 removal
- [ ] All Phase 1a regression stories pass on Docker stack
- [ ] Evidence saved to `tests/phase-0/evidence/`

---

## Doc Impact Assessment

| Document | Action | What Changes |
|----------|--------|-------------|
| `docs/api.md` | Add + ~~strikethrough~~ | Add app endpoints; strikethrough sidecar endpoints; update admin auth shape |
| `docs/aactl-reference.md` | Add | Add `aactl app` commands |
| `docs/getting-started-developer.md` | Add + ~~strikethrough~~ | Add app auth flow; strikethrough sidecar-only language |
| `docs/getting-started-operator.md` | Add | Add app registration workflow |
| `docs/architecture.md` | Add + ~~strikethrough~~ | Add app entity; strikethrough "sidecar is only path" |
| `docs/sidecar-deployment.md` | Add banner | Status note about Phase 0 removal |
| `docs/concepts.md` | Add | App identity concept |
| `.plans/Codebase-Map.md` | Rewrite sections | Phase 1a/1b additions, sidecar routes removed |
