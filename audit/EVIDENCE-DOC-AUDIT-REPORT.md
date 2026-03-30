# Documentation Audit Report — Code-First Analysis

**Date:** 2026-03-29
**Branch:** `fix/docs-overhaul`
**Method:** 9 parallel agents audited all code packages, routes, config, CLI, errors, and audit events, then checked docs coverage.

---

## Executive Summary

The codebase has **significant documentation drift**. The docs were written at an earlier point and have not been updated through B0-B4. Key findings:

- **4 CRITICAL** — factually wrong security claims, fabricated types
- **19 HIGH** — wrong field names, wrong scopes, wrong schemas, broken examples, dangerous defaults
- **18 MEDIUM** — incomplete coverage, missing packages, missing error types
- **10+ LOW** — minor omissions, cosmetic issues

---

## CRITICAL Findings (Must Fix)

### CRIT-1: Signing key persistence claim is wrong (3 locations)

The docs claim the broker generates a **fresh key on every startup** and that there is **no persistent key material**. This is completely false since B1 added `internal/keystore`.

| Location | Wrong Claim |
|----------|-------------|
| `docs/getting-started-operator.md` line 139 | "generates a fresh Ed25519 signing key pair on every startup" |
| `docs/getting-started-operator.md` line 263 | "previously issued tokens remain invalid after a restart" |
| `docs/architecture.md` line 399 (Design Decision #2) | "generates a new signing key pair on each start via crypto/rand" |

**Reality:** `keystore.LoadOrGenerate()` loads the existing key from `AA_SIGNING_KEY_PATH` (default `./signing.key`). Keys persist across restarts. Tokens remain valid.

### CRIT-2: `internal/keystore` package completely absent from architecture.md

Not in the directory layout, not in the package dependency graph, not in any component table. This package handles Ed25519 key persistence — a core security function.

### CRIT-3: Admin auth mechanism is wrong in architecture.md

`docs/architecture.md` line 405 (Design Decision #5) claims admin auth uses `subtle.ConstantTimeCompare`. **Reality:** Admin auth uses `bcrypt.CompareHashAndPassword` (changed in B2). The secret is bcrypt-hashed at startup in `cfg.Load()`.

### CRIT-4: Store types in architecture.md are fabricated

`docs/architecture.md` Pattern Components table lists `NonceTbl`, `AgentTbl`, `AppTbl`, `RevokeTbl`, `AuditTbl` as types in `internal/store`. **None of these types exist.** The actual types are `SqlStore`, `LaunchTokenRecord`, `AgentRecord`, `AppRecord`, `RevocationEntry`.

---

## HIGH Findings

### HIGH-1: CHANGELOG.md has HITL contamination (lines 40-98)

Lines 40, 45, 53-98 document HITL features (approval flow, HITL guide, HITL endpoints) as if added to agentauth-core. HITL is enterprise-only and should not appear in the core changelog.

### HIGH-2: Wrong aactl env var names in operator guide

`docs/getting-started-operator.md` lines 78-79 show `AGENTAUTH_BROKER_URL` and `AGENTAUTH_ADMIN_SECRET`. The actual env vars are `AACTL_BROKER_URL` and `AACTL_ADMIN_SECRET`. Following the docs causes auth failures.

### HIGH-3: api.md app management endpoints are comprehensively wrong

| Endpoint | Issue |
|----------|-------|
| `POST /v1/admin/apps` | Request fields `client_id`, `description` don't exist; code uses `name`, `scopes`, `token_ttl` |
| `POST /v1/admin/apps` | Response field `id` should be `app_id` |
| `DELETE /v1/admin/apps/{id}` | Doc says 204; code returns 200 with JSON body |
| `GET /v1/admin/apps` | Doc says `limit`/`offset` supported; code has no pagination |
| All 5 app routes | Doc says scope is `admin:apps:*`; code uses `admin:launch-tokens:*` |

### HIGH-4: Ghost route `POST /v1/token/exchange` in architecture.md

Listed in the route table and middleware diagram but not registered in `main.go`. No `exchange_hdl.go` exists. Also referenced in `handler/doc.go` line 19. Stale reference to removed/unimplemented feature.

### HIGH-5: openapi.yaml `RegisterRequest` schema is wrong

- Field name `nonce_signature` should be `signature`
- Missing 4 required fields: `nonce`, `orch_id`, `task_id`, `requested_scope`
- Only lists 3 of 7 required fields

### HIGH-6: openapi.yaml `DelegateRequest` missing `delegate_to` (required field)

Schema only has `scope`. Code requires `delegate_to` and supports `ttl`.

### HIGH-7: openapi.yaml `DelegateResponse.delegation_chain` type wrong

Schema says `array of strings`. Code returns `array of DelegRecord objects` with fields `agent`, `scope`, `delegated_at`, `signature`.

### HIGH-8: `POST /v1/app/auth` rate limit wrong in api.md

Doc says "5 req/s, burst 10". Code is 10/min (0.167/s), burst 3 per client_id.

### HIGH-9: Revocation now inside Verify() — not documented

`TknSvc.Verify()` calls `revoker.IsRevoked()` internally (B4 change). Architecture.md shows revocation as a separate middleware step only. The `Revoker` interface, `SetRevoker()`, and `RevokeByJTI()` are undocumented.

### HIGH-10: `internal/app` package absent from architecture.md dependency graph

The package has full CRUD, authentication, rate limiting. Not in the dependency graph at all.

### HIGH-11: `ValMw.RequireAnyScope()` not documented anywhere

Used on `POST /v1/admin/launch-tokens` to accept both `admin:launch-tokens:*` and `app:launch-tokens:*`. Neither api.md nor architecture.md mention this dual-scope acceptance.

### HIGH-12: concepts.md says 17 audit event types; code has 25

Missing from docs: `admin_auth_failed`, `launch_token_issued`, `launch_token_denied`, `registration_policy_violation`, `token_renewal_failed`, `resource_accessed`, `token_auth_failed`, `token_revoked_access`, plus the entire app audit group (6 events: `app_registered`, `app_authenticated`, `app_auth_failed`, `app_updated`, `app_deregistered`, `app_rate_limited`).

### HIGH-13: All 4 example files use a non-existent two-step registration API

`docs/examples/data-pipeline.md`, `code-generation.md`, `customer-support.md`, `devops-automation.md` all show: `POST /v1/register` to get a challenge, then `POST /v1/register/verify` to exchange signature for token. **`/v1/register/verify` does not exist.** The real flow is `GET /v1/challenge` then `POST /v1/register` (single step with all fields).

### HIGH-14: Example public key encoding is wrong (hex instead of base64)

All example files use `.public_bytes_raw().hex()` for the `public_key` field. Code expects base64 raw-encoded bytes (`id_svc.go:44`). Also `public_bytes_raw()` is not a standard Python cryptography method.

### HIGH-15: integration-patterns.md uses legacy admin auth shape (rejected with 400)

Patterns 5 and 6 send `{"client_id": "operator", "client_secret": ...}` to `POST /v1/admin/auth`. The broker explicitly detects this shape and returns 400: `"Admin auth format changed. Use {"secret": "..."}"`. These examples will fail.

### HIGH-16: common-tasks.md uses denylisted secret `change-me-in-production`

Lines 1066, 1079, 2066 use `"change-me-in-production"` as the admin secret in examples. This string is on the broker's denylist (`cfg.go:108`). The broker refuses to start with this value. Following the docs literally crashes the broker.

### HIGH-17: common-tasks.md registration example uses HMAC (wrong algorithm)

The Python registration example uses `hmac.new(launch_token.encode(), nonce.encode(), hashlib.sha256)` as the signature. The actual registration requires Ed25519 signature over the hex-decoded nonce bytes, with the public key submitted. The HMAC approach will fail.

### HIGH-18: troubleshooting.md suggests `AA_ADMIN_SECRET=""` — this is on the denylist

Lines 421-424 suggest `export AA_ADMIN_SECRET=""` to let the config file be read. Empty string is on the denylist — broker will exit. The variable must be **unset**, not set to empty.

### HIGH-19: common-tasks.md and integration-patterns.md have wrong audit event type names

`common-tasks.md` uses `token_acquired`, `delegation_issued`, `launch_token_created`. Actual names: `token_issued`, `delegation_created`, `launch_token_issued`.

---

## MEDIUM Findings

### MED-1: `AA_CONFIG_PATH` env var undocumented

Config file search order (`AA_CONFIG_PATH` > `/etc/agentauth/config` > `~/.agentauth/config`), symlink rejection, and 0600 permission enforcement not described in any doc.

### MED-2: `AA_BIND_ADDRESS` missing from operator config table

Default `127.0.0.1`. Critical for Docker (must be `0.0.0.0`). In the code but not in the config reference table.

### MED-3: Config file path extension inconsistency

`getting-started-operator.md` line 68 says `~/.agentauth/config.yaml`. Code generates `~/.agentauth/config` (no extension).

### MED-4: `aactl app update` and `app remove` shown as positional args

Docs show `aactl app update <app-id>` and `aactl app remove <app-id>`. Code uses `--id` flag for both.

### MED-5: Request lifecycle diagram missing audience validation step

`ValMw.Wrap()` checks audience between revocation and context injection. Not in the diagram.

### MED-6: `POST /v1/admin/launch-tokens` scope documented as admin-only

Code also accepts `app:launch-tokens:*` via `RequireAnyScope`.

### MED-7: openapi.yaml `AuditEvent` schema severely incomplete

Lists 6 fields. Code has 14 (missing: `id`, `orch_id`, `resource`, `deleg_depth`, `deleg_chain_hash`, `bytes_transferred`, `hash`, `prev_hash`).

### MED-8: openapi.yaml `ProblemDetail` schema missing fields

Missing `error_code`, `request_id`, `hint` — all present in API responses.

### MED-9: openapi.yaml audit `limit` default is 50; code defaults to 100

### MED-10: openapi.yaml health status enum wrong

Says `[healthy, degraded, unhealthy]`. Code always returns `"ok"`. Uptime typed as `string`; code returns `int64`.

### MED-11: Health response version in docs is 3.0.0; code is 2.0.0

`getting-started-operator.md` and `getting-started-user.md` examples show version 3.0.0. `cmd/broker/main.go` line 58: `const version = "2.0.0"`.

### MED-12: 20+ sentinel errors not in troubleshooting.md

All `app.*` errors, `revoke.ErrInvalidLevel`, `revoke.ErrMissingTarget`, `deleg.ErrDepthExceeded`, `token.ErrNoExpiry`, `token.ErrTokenNotYetValid`, `token.ErrInvalidIssuer`, `token.ErrMissingJTI`, `token.ErrMissingSubject`, all `mutauth.*` errors.

### MED-13: integration-patterns.md passes JSON body to GET audit endpoint

Pattern 5 uses `requests.get(..., json={"event_type": ...})`. Audit filters are query params, not a JSON body. The `json=` argument is ignored or rejected.

### MED-14: troubleshooting.md error messages don't match code

- Startup FATAL: doc says `"AA_ADMIN_SECRET must be set (non-empty)"`; code says `"No admin secret configured. Run 'aactl init' or set the AA_ADMIN_SECRET environment variable."`
- Weak secret: doc says `"admin secret does not meet security requirements"` at `/v1/admin/auth`; reality: this error fires at startup in `cfg.Load()`, never reaches the auth handler.
- Numbered list in broker restart section jumps from 1 to 3 (item 2 missing).

### MED-15: `common-tasks.md` says aactl is "demo/dev only"

Lines 1318 and 1556 say "aactl is available for demo and development use." The code implements full production auth via `AACTL_ADMIN_SECRET`.

### MED-14: `common-tasks.md` outcome filter values inconsistent

Uses `failure`/`completed` as filter values. `aactl-reference.md` says `success`/`denied`.

### MED-15: Verify() check order diagram in concepts.md is oversimplified

Shows: sig -> expiry -> revocation -> scope. Actual: format -> alg -> kid -> sig -> claims(iss/sub/jti/exp/nbf) -> revocation. Scope is checked by middleware, not Verify().

---

## Summary by Document

| Document | Critical | High | Medium | Status |
|----------|----------|------|--------|--------|
| `docs/architecture.md` | 3 (key persistence, admin auth, store types) | 3 (ghost route, missing app pkg, revocation model) | 2 | **Needs major rewrite** |
| `docs/api.md` | 0 | 4 (app endpoints wrong, rate limit, event counts) | 1 | **Needs targeted fixes** |
| `docs/api/openapi.yaml` | 0 | 3 (register, delegate schemas wrong) | 5 | **Needs schema corrections** |
| `docs/getting-started-operator.md` | 1 (key persistence) | 1 (env var names) | 4 | **Needs targeted fixes** |
| `docs/getting-started-user.md` | 0 | 0 | 1 (version 3.0.0) | Minor fix |
| `docs/concepts.md` | 0 | 1 (event count) | 2 (verify order, B4 features) | **Needs update for B4** |
| `docs/common-tasks.md` | 0 | 3 (denylisted secret, HMAC registration, wrong event names) | 3 | **Needs significant fixes** |
| `docs/integration-patterns.md` | 0 | 2 (legacy auth shape, public key encoding) | 1 (GET with JSON body) | **Needs significant fixes** |
| `docs/examples/*.md` (all 4) | 0 | 2 (non-existent /register/verify, hex public key) | 0 | **Needs rewrite of registration flow** |
| `docs/aactl-reference.md` | 0 | 0 | 1 | Minor fix |
| `docs/troubleshooting.md` | 0 | 1 (AA_ADMIN_SECRET="" denylist) | 2 (wrong error messages, missing errors) | **Needs targeted fixes** |
| `docs/RECOMMENDATIONS.md` | 0 | 0 | 1 (stale sidecar/KI-001 entries) | Minor cleanup |
| `CHANGELOG.md` | 0 | 1 (HITL contamination) | 0 | Remove HITL entries |

---

## Recommended Fix Priority

### Phase 1 — Security-Critical (do first)
1. Fix CRIT-1: Remove "fresh key on every startup" claims (3 locations)
2. Fix HIGH-18: Remove `AA_ADMIN_SECRET=""` suggestion from troubleshooting
3. Fix HIGH-16: Replace denylisted secret in common-tasks.md examples

### Phase 2 — Operational Breakage (do second)
4. Fix HIGH-2: Wrong aactl env var names (`AGENTAUTH_*` -> `AACTL_*`)
5. Fix HIGH-15: Legacy admin auth shape in integration-patterns.md
6. Fix HIGH-3: api.md app management endpoints (fields, scopes, status codes)
7. Fix MED-4: `aactl app update/remove` positional vs `--id` flag

### Phase 3 — Architecture & Schema (do third)
8. Fix CRIT-2, CRIT-3, CRIT-4: Architecture.md major rewrite (keystore, bcrypt, store types)
9. Fix HIGH-4: Remove ghost `/v1/token/exchange` route
10. Fix HIGH-5, HIGH-6, HIGH-7: openapi.yaml schema corrections
11. Fix HIGH-9, HIGH-10, HIGH-11: Architecture revocation model + app package + RequireAnyScope

### Phase 4 — Examples & Guides (do fourth)
12. Fix HIGH-13, HIGH-14: Rewrite all 4 example registration flows
13. Fix HIGH-17: Rewrite common-tasks.md registration example
14. Fix HIGH-19: Correct audit event type names

### Phase 5 — Completeness (do last)
15. Fix HIGH-12: Update event type list (17 -> 25)
16. Fix MED-12: Add missing sentinel errors to troubleshooting
17. Fix remaining MEDIUM items
