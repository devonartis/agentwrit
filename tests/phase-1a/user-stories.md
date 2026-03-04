# User Stories — Phase 1a: App Registration & Authentication

**Feature branch:** `feature/phase-1a-app-registration`
**Priority:** P0 — unblocks all subsequent phases
**Date:** 2026-03-03
**Spec:** `.plans/phase-1a/Phase-1a-App-Registration-Auth.md`
**PRD:** `.plans/PRD.md`

---

## No Cutting Corners — How to Run This Test

These stories must be run exactly as the real user would use the system. No shortcuts.

### Two personas, two tools

| Persona | Tool | Stories | What they have |
|---------|------|---------|----------------|
| **Operator** | `aactl` (CLI binary) | 0–5, 11 | Admin secret, full control |
| **Developer** | `curl` / HTTP client (no SDK yet — SDK is Phase 3) | 6–7 | Only `client_id` + `client_secret` from operator |
| **Security reviewer** | Both | 8–10 | Verifying boundaries between the two |

The operator uses `aactl`. The developer uses the REST API directly — they have no CLI tool and no admin access. Each story must be tested with the tool that persona would actually use.

### Before running operator stories (0–5, 11):
1. Build the `aactl` binary: `go build -o ./bin/aactl ./cmd/aactl`
2. Source the test environment: `source ./tests/phase-1a-env.sh`
   - This sets `AACTL_BROKER_URL` and `AACTL_ADMIN_SECRET` once — not inlined on every command
3. Bring up the broker stack: `./scripts/stack_up.sh`
4. Run stories using the binary: `./bin/aactl app list` — not `go run`, not `/tmp/aactl`

### Before running developer stories (6–7):
1. The operator has already registered the app and given you `client_id` + `client_secret`
2. You know the broker URL (e.g. `http://127.0.0.1:8080`)
3. You call `POST /v1/app/auth` with your credentials — this is `curl` or your app's HTTP client
4. You do NOT have `aactl`, the admin secret, or any operator tooling

### What counts as cutting corners:
- Building aactl to `/tmp/` instead of `./bin/`
- Inlining `AACTL_BROKER_URL=... AACTL_ADMIN_SECRET=...` on every command instead of sourcing env once
- Using `go run ./cmd/aactl` instead of the built binary
- Testing developer stories with `aactl` instead of the REST API
- Skipping a story because it "looks fine"
- Claiming pass without showing the actual output

**Evidence must be saved to `tests/phase-1a-evidence/`** — one file per story, plain English, raw output included.

---

## Credential Flow

```
Operator (aactl)          Broker                    3rd Party Developer
     |                      |                               |
     |-- app register ----→ |                               |
     |←- client_id,         |                               |
     |   client_secret ---- |                               |
     |                      |                               |
     |-- hands client_id + client_secret to developer ---→  |
     |                      |                               |
     |                      |←- POST /v1/app/auth --------- |
     |                      |   (client_id + client_secret) |
     |                      |-- scoped JWT ---------------→ |
```

The **operator** owns `aactl` and the master admin key — they register apps and set scope ceilings.
The **3rd party developer** receives only `client_id` + `client_secret` from the operator — they never see the admin key.
The `client_secret` is shown exactly once at registration. The broker stores only a bcrypt hash.

---

## Story 0 — Broker starts clean with no apps

> As an **operator**, I want the broker to start with an empty app registry so that I can register apps incrementally without any pre-seeded data.

**Acceptance criteria:**
- `aactl app list` immediately after stack start returns an empty table
- `aactl app list --json` returns `{"apps": [], "total": 0}`
- Broker health endpoint returns `200 ok`

**Covered by:** `live_test.sh --phase1a` → Story 0

---

## Story 1 — Operator registers a new app and receives credentials to hand to the developer

> As an **operator**, I want to register a new app with a name and scope ceiling so that I can hand the generated `client_id` and `client_secret` to the 3rd party developer — they use these to authenticate their app directly with the broker, without ever needing the master admin key.

**Credential handoff:** The operator runs `aactl app register`, receives `client_id` + `client_secret` once, and securely delivers them to the developer. The broker stores only a bcrypt hash — the plaintext secret is never recoverable after this point.

**Acceptance criteria:**
- `aactl app register --name weather-bot --scopes "read:weather:*,write:logs:*"` succeeds
- Response includes `app_id`, `client_id`, `client_secret`, and `scopes`
- `app_id` format is `app-weather-bot-{6hexchars}`
- `client_id` format is `{2-3 char abbrev}-{12hexchars}`
- `client_secret` is a random 64-char hex string
- CLI warns: "Save the client_secret — it cannot be retrieved again"
- `client_secret_hash` is **not** present in the response
- Audit event `app_registered` is recorded in the trail

**Covered by:** `live_test.sh --phase1a` → Story 1

---

## Story 2 — App list grows as operator registers more apps

> As an **operator**, I want the app list to reflect every registration in real time so that I always have an accurate picture of what's connected.

**Acceptance criteria:**
- Register `weather-bot` → `aactl app list` shows 1 app, `total: 1`
- Register `log-agent` → `aactl app list` shows 2 apps, `total: 2`
- Register `alert-service` → `aactl app list` shows 3 apps, `total: 3`
- Table columns: NAME | APP_ID | CLIENT_ID | STATUS | SCOPES | CREATED
- All three apps have `status: active`
- `aactl app list --json` returns valid JSON with `apps` array and correct `total` count
- `client_secret_hash` is **not** present in any list output

**Covered by:** `live_test.sh --phase1a` → Story 2

---

## Story 3 — Operator views details of a specific app

> As an **operator**, I want to view details of a specific app (name, scopes, status, created date) so that I can verify its configuration.

**Acceptance criteria:**
- `aactl app get <app_id>` returns full app details
- Response includes: `app_id`, `name`, `client_id`, `scopes`, `status`, `created_at`, `updated_at`
- Timestamps are in RFC3339 format
- `client_secret_hash` is **not** present in the response
- Returns a clear error for an unknown app_id

**Covered by:** `live_test.sh --phase1a` → Story 3

---

## Story 4 — Operator updates an app's scope ceiling

> As an **operator**, I want to update an app's scope ceiling so that I can adjust permissions without re-registering.

**Acceptance criteria:**
- `aactl app update --id <app_id> --scopes "read:weather:*,write:logs:*,read:alerts:*"` succeeds
- Response confirms the new scope list and updated `updated_at` timestamp
- `aactl app get <app_id>` reflects the new scopes
- Audit event `app_updated` is recorded with the new scope list

**Covered by:** `live_test.sh --phase1a` → Story 4

---

## Story 5 — Operator deregisters an app

> As an **operator**, I want to deregister an app so that its credentials stop working immediately.

**Acceptance criteria:**
- `aactl app remove --id <app_id>` succeeds and confirms deregistration
- Response includes `status: inactive` and `deregistered_at` timestamp
- After removal, `POST /v1/app/auth` with the old credentials returns 401
- `aactl app list` still shows the app with `status: inactive` (soft delete — row not removed)
- Audit event `app_deregistered` is recorded

**Covered by:** `live_test.sh --phase1a` → Story 5

---

## Story 6 — Developer authenticates app using credentials received from the operator

> As a **developer**, I want to authenticate my app using the `client_id` + `client_secret` that the operator gave me so that I can get a scoped JWT to interact with the broker — without needing the admin key.

**Acceptance criteria:**
- `POST /v1/app/auth` with valid `client_id` + `client_secret` returns 200
- Response includes: `access_token`, `expires_in: 300`, `token_type: "Bearer"`, `scopes`
- JWT carries exactly these 3 scopes: `app:launch-tokens:*`, `app:agents:*`, `app:audit:read`
- JWT `sub` claim is `app:{app_id}`
- JWT `exp` is approximately 5 minutes from issue time
- Audit event `app_authenticated` is recorded

**Covered by:** `live_test.sh --phase1a` → Story 6

---

## Story 7 — Developer receives clear error on bad credentials

> As a **developer**, I want a clear error message when my credentials are wrong or my app has been deregistered so that I can diagnose auth failures.

**Acceptance criteria:**
- Wrong `client_secret` → 401 with RFC 7807 body: `"title": "Authentication failed"`, `"detail": "Invalid client credentials"`
- Unknown `client_id` → 401 with same generic message (no enumeration of valid IDs)
- Deregistered app → 401 with same generic message
- Audit event `app_auth_failed` is recorded for every failure
- Response time for failed auth is consistent with successful auth (no timing oracle)

**Covered by:** `live_test.sh --phase1a` → Story 7

---

## Story 8 — App credentials are scoped, not the master key

> As a **security reviewer**, I want each app to have its own scoped credentials (not the master key) so that a compromise of one app doesn't compromise the entire system.

**Acceptance criteria:**
- App JWT carries `app:` scopes only — never `admin:` scopes
- App JWT cannot be used on admin endpoints (returns 403)
- Master key (`AACTL_ADMIN_TOKEN`) is not required to authenticate as an app
- Deregistering one app does not affect any other app's authentication
- `client_secret_hash` is never visible in any API response or audit log

**Covered by:** `live_test.sh --phase1a` → Story 8

---

## Story 9 — Per-app rate limiting on the auth endpoint

> As a **security reviewer**, I want per-app rate limiting on the auth endpoint so that credential-stuffing against one `client_id` doesn't affect other apps.

**Acceptance criteria:**
- Sending 11+ rapid `POST /v1/app/auth` requests for the same `client_id` triggers 429
- 429 response includes `Retry-After` header
- Rate-limited app does NOT affect authentication for a different `client_id`
- Audit event `app_rate_limited` is recorded when the limit triggers
- Rate limit resets after the window expires (10 requests/minute per client_id)

**Covered by:** `live_test.sh --phase1a` → Story 9

---

## Story 10 — All app operations are recorded in the tamper-evident audit trail

> As a **security reviewer**, I want all app registration and authentication events recorded in the tamper-evident audit trail so that I can trace who did what.

**Acceptance criteria:**
- `GET /v1/audit/events?event_type=app_registered` returns the registration event
- `GET /v1/audit/events?event_type=app_authenticated` returns successful auth events
- `GET /v1/audit/events?event_type=app_auth_failed` returns failed auth events
- `GET /v1/audit/events?event_type=app_deregistered` returns deregistration events
- All events include `prev_hash` linking (hash chain intact)
- `client_secret` never appears in any audit event detail field

**Covered by:** `live_test.sh --phase1a` → Story 10

---

## Story 11 — Existing flows are unaffected by Phase 1a changes

> As an **operator**, I want all existing admin, sidecar, and agent flows to continue working after Phase 1a ships, so that the deployment is not disrupted.

**Acceptance criteria:**
- Admin auth (`POST /v1/admin/auth`) still works
- Agent challenge-response and JWT issuance still work
- All existing `aactl` commands (`aactl audit`, `aactl revoke`) still work
- `gates.sh` passes with zero failures

**Covered by:** `live_test.sh --phase1a` → Story 11 (regression gate)

---

## Phase 1a Verification Checklist

Before marking Phase 1a complete, all of the following must pass with Docker running:

- [ ] `gates.sh` passes (lint, vet, unit tests)
- [ ] Docker stack (broker only) starts successfully with new code
- [ ] `aactl app list` returns empty on fresh start (Story 0)
- [ ] `aactl app register` returns credentials (Story 1)
- [ ] `aactl app list` grows correctly: 1 app, then 2, then 3 (Story 2)
- [ ] `aactl app get` returns full details with RFC3339 timestamps (Story 3)
- [ ] `aactl app update` changes scopes and records audit event (Story 4)
- [ ] `aactl app remove` makes subsequent auth fail with 401 (Story 5)
- [ ] `POST /v1/app/auth` returns JWT with `app:` scopes (Story 6)
- [ ] Bad credentials return generic 401, no enumeration (Story 7)
- [ ] App JWT cannot access admin endpoints (Story 8)
- [ ] Rate limiting triggers on 11+ rapid auth attempts for one client_id (Story 9)
- [ ] Rate limiting does NOT affect a second client_id (Story 9)
- [ ] Audit events recorded for all 6 event types (Story 10)
- [ ] `client_secret_hash` never appears in any API response (Stories 1–5)
- [ ] `client_secret` never appears in audit logs (Story 10)
- [ ] Existing admin/agent flows pass regression check (Story 11)
