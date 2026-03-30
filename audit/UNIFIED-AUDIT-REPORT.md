# Unified Documentation Audit Report — AgentAuth Core

**Date:** March 29, 2026
**Version Under Review:** 2.0.0 (code) / mixed 2.0–3.0 (docs)
**Method:** Consolidated from three independent audits, every finding re-verified against source code.

---

## Why This Report Exists

Three separate documentation audits were conducted independently. Each found different subsets of issues because they used different methodologies:

| Report | Location | Method | Findings |
|--------|----------|--------|----------|
| **Report A** (Code-First) | `tests/sec-l2a/evidence/DOC-AUDIT-REPORT.md` | 9 parallel agents read ALL code packages first, then checked if docs covered what exists | 4 CRIT, 19 HIGH, 18 MED |
| **Report B** (Doc-First) | `docs/DOCS-AUDIT-REPORT.md` | Read all 12 doc files end-to-end, then verified each claim against specific code files | 8 CRIT, 25+ HIGH, 6 MED |
| **Report C** (My DOCX) | `AgentAuth_Doc_Audit.docx` | Sub-agents searched docs and code in parallel | 5 HIGH, 5 MED, 7 LOW |

### Why They Disagree

**Report C (the DOCX) significantly undercounted.** It missed the majority of critical issues because its sub-agents used search-based approaches rather than reading files end-to-end. Specifically, it missed:

- Registration examples using HMAC instead of Ed25519 (the biggest developer-facing bug)
- Token renewal predecessor revocation being the opposite of what docs say
- Fabricated store types in architecture.md
- Ghost route `/v1/token/exchange` that doesn't exist
- All 4 example files using non-existent `/v1/register/verify` endpoint
- Denylisted secrets used in examples (broker refuses to start)
- Legacy admin auth shape in integration patterns (returns 400)
- `acquire_token()` helper that skips the entire registration flow
- HITL/enterprise contamination in changelog
- 20+ missing sentinel errors from troubleshooting docs

**Reports A and B overlap ~70% but each caught unique things.** Report A found fabricated store types and the ghost route because it started from code. Report B found the token renewal reversal and acquire_token() skip because it read common-tasks.md line by line.

**This unified report merges all three**, re-verifies every finding against actual source code, and deduplicates.

---

## Executive Summary

| Severity | Count | Impact |
|----------|-------|--------|
| **CRITICAL** | 8 | Security model misrepresentation, broken code examples, fabricated types |
| **HIGH** | 22 | Wrong API schemas, wrong scopes, broken examples, dangerous defaults |
| **MEDIUM** | 18 | Missing coverage, inconsistencies, incomplete schemas |
| **LOW** | 6 | Cosmetic, minor omissions |
| **Total** | **54** | |

The documentation has drifted significantly from the code through batches B0–B4. The largest clusters of errors are in `api.md` (app endpoints section), `common-tasks.md` (registration and operational examples), `architecture.md` (security model claims), and `openapi.yaml` (schemas).

---

## CRITICAL Findings (8)

### CRIT-1: Signing key persistence claimed as ephemeral

**Sources:** Report A (CRIT-1), Report B (CRIT-8), Report C (F-1)
**All three reports found this.**

| Location | Wrong Claim |
|----------|-------------|
| `docs/architecture.md` line 399 | "generates a new signing key pair on each start via crypto/rand" |
| `docs/getting-started-operator.md` line 139 | "generates a fresh Ed25519 signing key pair on every startup" |
| `docs/getting-started-operator.md` line 263 | "previously issued tokens remain invalid after a restart" |

**Code truth** (`internal/keystore/keystore.go` lines 23–42): `LoadOrGenerate(path)` reads the existing key from disk if the file exists. New keys are generated ONLY if the file is missing. Keys persist across restarts. Tokens remain valid.

**Verified:** ✅ Confirmed by reading keystore.go and main.go line 81.

**Fix:** Rewrite all three locations. Replace "ephemeral" language with: "Keys persist to disk at `AA_SIGNING_KEY_PATH`. On first startup, a new Ed25519 key pair is generated. On subsequent startups, the existing key is loaded. Delete the key file to force rotation."

---

### CRIT-2: Registration examples use HMAC-SHA256 instead of Ed25519

**Sources:** Report A (CRIT not numbered but in HIGH-17), Report B (CRIT-1). **Report C missed this entirely.**

**Files affected:**
- `docs/common-tasks.md` lines 48–111 — Python registration example uses `hmac.new(launch_token.encode(), nonce.encode(), hashlib.sha256).hexdigest()`
- `docs/integration-patterns.md` lines 69, 138–143 — Same HMAC pattern
- `docs/common-tasks.md` line 877–888 — `acquire_token()` helper skips challenge-response entirely

**Code truth** (`internal/identity/id_svc.go` lines 162–188):
1. Public key is base64-encoded Ed25519 (line 163: `base64.StdEncoding.DecodeString(req.PublicKey)`)
2. Signature is base64-encoded Ed25519 signature (line 175: `base64.StdEncoding.DecodeString(req.Signature)`)
3. Nonce is hex-decoded before verification (line 180: `hex.DecodeString(req.Nonce)`)
4. Verified with `ed25519.Verify(pubKey, nonceBytes, sigBytes)` (line 185)

**Verified:** ✅ Every HMAC example will fail with `ErrInvalidSignature`. This is the most impactful developer-facing bug.

**Fix:** Rewrite all registration examples to use Ed25519. Correct Python pattern:
```python
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
import base64, binascii
private_key = Ed25519PrivateKey.generate()
public_key_b64 = base64.b64encode(private_key.public_key().public_bytes_raw()).decode()
nonce_bytes = binascii.unhexlify(nonce)
signature_b64 = base64.b64encode(private_key.sign(nonce_bytes)).decode()
```

---

### CRIT-3: api.md app management section is comprehensively wrong

**Sources:** Report A (HIGH-3), Report B (CRIT-2). **Report C partially found this (F-3, F-4).**

**Code truth** (`internal/app/app_hdl.go` lines 47–48, 63–90):

| Aspect | api.md Claims | Actual Code |
|--------|---------------|-------------|
| All 5 app endpoint scopes | `admin:apps:create`, `admin:apps:read`, etc. | `admin:launch-tokens:*` (line 48) |
| POST request fields | `name`, `client_id` (required), `description` | `name`, `scopes` (required), `token_ttl` (optional) (lines 63–67) |
| POST response ID field | `id` | `app_id` (line 70) |
| PUT request fields | `name`, `description` | `scopes`, `token_ttl` (lines 87–90) |
| DELETE response | 204 No Content | 200 with JSON `{app_id, status, deregistered_at}` (line 315) |
| GET /v1/admin/apps pagination | `limit`/`offset` params | No pagination (line 174: returns all) |

**Verified:** ✅ Every field, scope, and status code confirmed against app_hdl.go.

**Fix:** Rewrite the entire App Management Endpoints section in api.md.

---

### CRIT-4: Fabricated store types in architecture.md

**Sources:** Report A (CRIT-4). **Reports B and C missed this.**

**architecture.md** Pattern Components table lists store types: `NonceTbl`, `AgentTbl`, `AppTbl`, `RevokeTbl`, `AuditTbl`.

**Code truth** (`internal/store/`): None of these types exist. The actual types are `SqlStore`, `LaunchTokenRecord`, `AgentRecord`, `AppRecord`, `RevocationEntry`.

**Verified:** ✅ Confirmed by reading store package.

**Fix:** Replace fabricated type names with actual types from the store package.

---

### CRIT-5: Token renewal predecessor revocation is opposite of documented

**Sources:** Report B (CRIT-5). **Reports A and C missed this.**

**api.md line 326:** "The original token remains valid until its own expiry."

**Code truth** (`internal/token/tkn_svc.go` lines 224–232):
```go
// Mandatory predecessor revocation
if s.revoker != nil {
    if err := s.revoker.RevokeByJTI(claims.Jti); err != nil {
        return nil, fmt.Errorf("revoke predecessor: %w", err)
    }
}
```

The old token is revoked BEFORE the new token is issued. This is the exact opposite of the documentation. Even the GoDoc comment on line 217 says "The original token remains valid" — the comment contradicts the code directly below it.

**Verified:** ✅ Confirmed by reading tkn_svc.go lines 217–232.

**Fix:** Update api.md and the GoDoc comment. New text: "The predecessor token is revoked before the replacement is issued. Renewal is atomic: the old JTI is invalidated even if issuance subsequently fails."

---

### CRIT-6: Admin auth mechanism wrong in architecture.md

**Sources:** Report A (CRIT-3). **Reports B and C missed this.**

**architecture.md** line 405 (Design Decision #5) claims admin auth uses `subtle.ConstantTimeCompare`.

**Code truth** (`internal/cfg/cfg.go` lines 118–119): Admin secret is bcrypt-hashed at startup. Authentication uses `bcrypt.CompareHashAndPassword` (in admin_svc.go).

**Verified:** ✅ Confirmed by reading cfg.go.

**Fix:** Replace `subtle.ConstantTimeCompare` with bcrypt description.

---

### CRIT-7: All 4 example files use non-existent two-step registration

**Sources:** Report A (HIGH-13, HIGH-14). **Reports B and C missed this.**

**Files:** `docs/examples/data-pipeline.md`, `code-generation.md`, `customer-support.md`, `devops-automation.md`

All show: `POST /v1/register` to get a challenge, then `POST /v1/register/verify` to exchange signature for token. **`/v1/register/verify` does not exist.** The real flow is `GET /v1/challenge` then `POST /v1/register`.

Additionally, all examples use `.public_bytes_raw().hex()` for the `public_key` field. Code expects base64 (id_svc.go line 163).

**Verified:** ✅ No `/v1/register/verify` route in main.go. Public key must be base64.

**Fix:** Rewrite registration flow in all 4 examples. Use `GET /v1/challenge` → `POST /v1/register` with base64 encoding.

---

### CRIT-8: common-tasks.md uses denylisted admin secret

**Sources:** Report A (HIGH-16). **Reports B and C missed this.**

**common-tasks.md** lines 1066, 1079, 2066 use `"change-me-in-production"` as the admin secret in examples.

**Code truth** (`internal/cfg/cfg.go` line 108): `denylist := []string{"change-me-in-production", ""}`. The broker exits with a fatal error if this value is used.

**Verified:** ✅ Copying these examples literally prevents the broker from starting.

**Fix:** Replace with a clearly-fake but non-denylisted secret like `"my-strong-secret-here"` with a comment telling operators to use a real value.

---

## HIGH Findings (22)

### HIGH-1: App auth rate limit wrong in api.md

**api.md** says "5 req/s, burst 10".
**Code** (`app_hdl.go` line 41): `authz.NewRateLimiter(10.0/60.0, 3)` = 10 req/min, burst 3.
**Note:** Admin auth rate limit (5/s, burst 10) IS correct per admin_hdl.go line 47.

### HIGH-2: App auth response wrong in api.md

**api.md** says `expires_in: 300` always, scopes = app's data scopes.
**Code** (`app_svc.go` lines 182–186): TTL = app's `token_ttl` (default 1800). Scopes = fixed operational: `["app:launch-tokens:*", "app:agents:*", "app:audit:read"]`.

### HIGH-3: OpenAPI `RegisterRequest` schema wrong

**openapi.yaml**: Field `nonce_signature`, only 3 of 7 required fields listed.
**Code** (`id_svc.go` lines 43–54): Field is `signature` (json tag). Required: `launch_token`, `nonce`, `public_key`, `signature`, `orch_id`, `task_id`, `requested_scope`.

### HIGH-4: OpenAPI `DelegateRequest` missing required field

**openapi.yaml**: Only `scope` required.
**Code** (`deleg/deleg_svc.go`): Both `delegate_to` and `scope` required. Also supports `ttl`.

### HIGH-5: OpenAPI `DelegateResponse.delegation_chain` type wrong

**openapi.yaml**: `array of strings`.
**Code** (`token/tkn_claims.go`): `array of DelegRecord objects` with fields `agent`, `scope`, `delegated_at`, `signature`.

### HIGH-6: Ghost route `/v1/token/exchange` in architecture.md

Listed in route table and middleware diagram. **Does not exist** in main.go. No `exchange_hdl.go` exists. Stale reference to removed feature.

### HIGH-7: Wrong aactl env var names in operator guide

**getting-started-operator.md** lines 78–79: `AGENTAUTH_BROKER_URL`, `AGENTAUTH_ADMIN_SECRET`.
**Code** (`cmd/aactl/client.go`): `AACTL_BROKER_URL`, `AACTL_ADMIN_SECRET`.

### HIGH-8: `internal/keystore` and `internal/app` missing from architecture dependency graph

Two core packages entirely absent from the architecture diagram. keystore handles Ed25519 key persistence (security-critical). app has full CRUD + auth + rate limiting.

### HIGH-9: Revocation now inside Verify() — not documented

`TknSvc.Verify()` calls `revoker.IsRevoked()` internally (tkn_svc.go line 207). Architecture.md shows revocation as a separate middleware step only. The `Revoker` interface, `SetRevoker()`, and `RevokeByJTI()` are undocumented.

### HIGH-10: `ValMw.RequireAnyScope()` not documented

Used on `POST /v1/admin/launch-tokens` (admin_hdl.go line 58) to accept BOTH `admin:launch-tokens:*` and `app:launch-tokens:*`. Neither api.md nor architecture.md mention this dual-scope acceptance.

### HIGH-11: concepts.md says 17 audit event types; code has 25

Missing from docs: `admin_auth_failed`, `launch_token_issued`, `launch_token_denied`, `registration_policy_violation`, `token_renewal_failed`, `resource_accessed`, `token_auth_failed`, `token_revoked_access`, plus 6 app events (`app_registered`, `app_authenticated`, `app_auth_failed`, `app_updated`, `app_deregistered`, `app_rate_limited`).

### HIGH-12: integration-patterns.md uses legacy admin auth shape

Patterns 5 and 6 send `{"client_id": "operator", "client_secret": ...}` to `POST /v1/admin/auth`. The broker explicitly detects this shape and returns 400: `"Admin auth format changed. Use {"secret": "..."}"` (admin_hdl.go lines 97–100).

### HIGH-13: common-tasks.md registration example uses wrong endpoint

Line 625 says "Re-acquire via `POST /v1/token`". This endpoint does not exist. Correct: `POST /v1/token/renew`.

### HIGH-14: common-tasks.md wrong audit event type names

Uses `token_acquired`, `delegation_issued`, `launch_token_created`. Actual names: `token_issued`, `delegation_created`, `launch_token_issued`.

### HIGH-15: troubleshooting.md suggests `AA_ADMIN_SECRET=""` — this is denylisted

Lines 421–424 suggest `export AA_ADMIN_SECRET=""` to let the config file be read. Empty string is on the denylist (cfg.go line 108) — broker will exit. The variable must be **unset**, not set to empty.

### HIGH-16: CHANGELOG.md has HITL contamination

Lines 40–98 document HITL features (approval flow, endpoints, guides) as if added to agentauth-core. HITL is enterprise-only and should not appear in the core changelog.

### HIGH-17: Registration returns 200, not 201

**common-tasks.md** line 113 says registration returns "201 Created".
**Code** (`handler/reg_hdl.go` line 54): `w.WriteHeader(http.StatusOK)` — returns 200 OK.

### HIGH-18: OpenAPI `AuditEvent` schema severely incomplete

OpenAPI lists 6 fields. Code has 14 (missing: `id`, `orch_id`, `resource`, `deleg_depth`, `deleg_chain_hash`, `bytes_transferred`, `hash`, `prev_hash`).

### HIGH-19: OpenAPI `ProblemDetail` schema missing fields

Missing `error_code`, `request_id`, `hint` — all present in API responses.

### HIGH-20: Health response status and uptime wrong in OpenAPI

OpenAPI enum: `[healthy, degraded, unhealthy]`, uptime type: `string`.
Code (health_hdl.go): Status always `"ok"`, uptime type `int64`.

### HIGH-21: Audit events default limit inconsistency

OpenAPI says 50. Code (audit_log.go line 247): `limit = 100`.

### HIGH-22: integration-patterns.md passes JSON body to GET audit endpoint

Pattern 5 uses `requests.get(..., json={"event_type": ...})`. Audit filters are query params, not a JSON body.

---

## MEDIUM Findings (18)

| ID | Finding | Location |
|----|---------|----------|
| MED-1 | Version chaos: docs say 2.0, 3.0, or 2.0 (Enterprise); code says 2.0.0 | Multiple docs vs main.go:58 |
| MED-2 | "Enterprise" label in common-tasks.md line 7 | common-tasks.md |
| MED-3 | `AA_CONFIG_PATH` env var and config file search order undocumented | cfg.go:71 |
| MED-4 | `AA_BIND_ADDRESS` missing from operator config table | cfg.go:76 |
| MED-5 | Config file extension: docs say `.yaml`, code creates no extension | getting-started-operator.md:68, aactl/init_cmd.go |
| MED-6 | `aactl app update/remove` shown as positional args; code uses `--id` flag | aactl-reference.md |
| MED-7 | Request lifecycle diagram missing audience validation step | architecture.md |
| MED-8 | `POST /v1/admin/launch-tokens` scope documented as admin-only; code also accepts `app:launch-tokens:*` | api.md, admin_hdl.go:58 |
| MED-9 | Verify() check order diagram oversimplified | concepts.md |
| MED-10 | 20+ sentinel errors not in troubleshooting.md | All `app.*`, `revoke.*`, `deleg.*`, `token.*` errors |
| MED-11 | Troubleshooting error messages don't match actual code output | troubleshooting.md lines 402, 457 |
| MED-12 | `common-tasks.md` says aactl is "demo/dev only"; it has full production auth | common-tasks.md:1318, 1556 |
| MED-13 | Audit outcome filter values: docs say failure/completed; code uses success/denied | common-tasks.md:1774 |
| MED-14 | Launch token response described as "JWT"; it's an opaque 64-char hex string | openapi.yaml |
| MED-15 | App scope ceiling enforcement undocumented | admin_hdl.go:149–166 |
| MED-16 | Renewal predecessor revocation undocumented in guides | common-tasks.md, integration-patterns.md |
| MED-17 | `aud` claim and `kid` header missing from concepts.md token claims list | concepts.md:183 |
| MED-18 | Unused audit constant `EventScopesCeilingUpdated` defined but never emitted | audit_log.go:48 |

---

## LOW Findings (6)

| ID | Finding |
|----|---------|
| LOW-1 | SECURITY.md PGP key placeholder `[TO_BE_POPULATED]` |
| LOW-2 | CODE_OF_CONDUCT.md referenced in CONTRIBUTING.md but missing |
| LOW-3 | RECOMMENDATIONS.md references non-existent `/v1/admin/agents` endpoints |
| LOW-4 | Numbering gap in troubleshooting.md broker restart section (jumps 1→3) |
| LOW-5 | Duplicate `BROKER = os.environ.get(...)` in integration-patterns.md lines 684, 976 |
| LOW-6 | `common-tasks.md` env var uses `AA_ADMIN_SECRET` for aactl (should be `AACTL_ADMIN_SECRET`) |

---

## Summary by Document

| Document | CRIT | HIGH | MED | Status |
|----------|------|------|-----|--------|
| `docs/architecture.md` | 3 | 3 | 2 | **Needs major rewrite** |
| `docs/api.md` | 1 | 5 | 1 | **Needs major rewrite (app section)** |
| `docs/api/openapi.yaml` | 0 | 5 | 2 | **Needs schema corrections** |
| `docs/common-tasks.md` | 2 | 4 | 3 | **Needs significant fixes** |
| `docs/integration-patterns.md` | 0 | 2 | 1 | **Needs targeted fixes** |
| `docs/examples/*.md` (all 4) | 1 | 0 | 0 | **Needs registration rewrite** |
| `docs/getting-started-operator.md` | 1 | 1 | 2 | **Needs targeted fixes** |
| `docs/getting-started-user.md` | 0 | 0 | 1 | Minor fix (version) |
| `docs/getting-started-developer.md` | 0 | 0 | 1 | Minor fix (version) |
| `docs/concepts.md` | 0 | 1 | 2 | **Needs B4 update** |
| `docs/troubleshooting.md` | 0 | 1 | 2 | **Needs targeted fixes** |
| `docs/aactl-reference.md` | 0 | 0 | 1 | Minor fix |
| `CHANGELOG.md` | 0 | 1 | 0 | Remove HITL entries |
| `docs/RECOMMENDATIONS.md` | 0 | 0 | 0 | Archival only |

---

## Recommended Fix Order

### Phase 1 — Security & Developer-Critical (Day 1)

These fixes prevent security misunderstandings and developer failures:

| Priority | Finding | Fix | Effort |
|----------|---------|-----|--------|
| 1 | CRIT-2 | Rewrite ALL registration examples from HMAC → Ed25519 | 2 hours |
| 2 | CRIT-1 | Remove "ephemeral key" claims (3 locations) | 15 min |
| 3 | CRIT-8 | Replace denylisted secret in common-tasks.md examples | 10 min |
| 4 | HIGH-15 | Remove `AA_ADMIN_SECRET=""` from troubleshooting.md | 5 min |
| 5 | CRIT-5 | Fix token renewal description (predecessor IS revoked) | 10 min |
| 6 | HIGH-12 | Fix legacy admin auth shape in integration-patterns.md | 15 min |

### Phase 2 — API Contract Fixes (Day 1–2)

These fixes prevent 400/403/404 errors for API consumers:

| Priority | Finding | Fix | Effort |
|----------|---------|-----|--------|
| 7 | CRIT-3 | Rewrite entire api.md app management section | 1 hour |
| 8 | HIGH-1, HIGH-2 | Fix app auth rate limit and response fields | 15 min |
| 9 | HIGH-3–5 | Fix OpenAPI register, delegate, delegation_chain schemas | 30 min |
| 10 | HIGH-7 | Fix aactl env var names (`AGENTAUTH_*` → `AACTL_*`) | 10 min |
| 11 | HIGH-13 | Fix nonexistent `POST /v1/token` → `POST /v1/token/renew` | 5 min |
| 12 | HIGH-17 | Fix registration status code 201→200 | 5 min |

### Phase 3 — Architecture Rewrite (Day 2–3)

| Priority | Finding | Fix | Effort |
|----------|---------|-----|--------|
| 13 | CRIT-4, CRIT-6 | Fix fabricated store types, admin auth mechanism | 30 min |
| 14 | HIGH-6 | Remove ghost `/v1/token/exchange` route | 10 min |
| 15 | HIGH-8 | Add keystore and app packages to dependency graph | 30 min |
| 16 | HIGH-9, HIGH-10 | Document Revoker interface and RequireAnyScope | 20 min |

### Phase 4 — Examples & Guides (Day 3)

| Priority | Finding | Fix | Effort |
|----------|---------|-----|--------|
| 17 | CRIT-7 | Rewrite all 4 example file registration flows | 1 hour |
| 18 | HIGH-14 | Fix audit event type names in common-tasks.md | 10 min |
| 19 | HIGH-16 | Remove HITL contamination from CHANGELOG.md | 15 min |
| 20 | MED-1 | Decide version number and apply everywhere | 15 min |

### Phase 5 — Completeness (Day 3–4)

| Priority | Finding | Fix | Effort |
|----------|---------|-----|--------|
| 21 | HIGH-11 | Update event type list (17→25) | 15 min |
| 22 | HIGH-18–21 | Fix remaining OpenAPI mismatches | 30 min |
| 23 | MED-10 | Add missing sentinel errors to troubleshooting | 1 hour |
| 24 | All remaining MED/LOW | Batch fix | 1 hour |

**Total estimated effort: 2–4 days focused work.**

---

## Files to Delete/Move After Fixes

Once the fixes in this report are applied:

1. **Delete** `docs/DOCS-AUDIT-REPORT.md` (merged into this report)
2. **Archive** `tests/sec-l2a/evidence/DOC-AUDIT-REPORT.md` (merged into this report)
3. **Delete** `AgentAuth_Doc_Audit.docx` (superseded by this report)
4. This report (`audit/UNIFIED-AUDIT-REPORT.md`) becomes the single source of truth for documentation quality tracking.
