# Documentation Audit Report — agentauth-core

> **Date:** March 29, 2026 | **Branch:** `fix/docs-overhaul`
>
> **Scope:** All 12 documentation files in `docs/` compared end-to-end against the actual codebase.
>
> **Purpose:** Identify contamination, errors, and gaps before open-source release. **No fixes applied — report only.**

---

## Methodology

Every file in `docs/` was read end-to-end (not grepped). The following code files were then read end-to-end to verify doc claims against actual implementation:

- `cmd/broker/main.go` — version constant, keystore usage, route registration
- `internal/admin/admin_hdl.go` — admin auth handler, rate limiter, app endpoint scopes, request/response structs
- `internal/admin/admin_svc.go` — bcrypt comparison, launch token generation, admin scopes, default TTLs
- `internal/app/app_hdl.go` — app auth rate limiter, app endpoint scopes, request/response structs
- `internal/identity/id_svc.go` — registration flow, required fields, Ed25519 verification, response format
- `internal/token/tkn_svc.go` — JTI generation, token issuance, renewal (predecessor revocation), verify
- `internal/keystore/keystore.go` — LoadOrGenerate persistence behavior
- `internal/handler/health_hdl.go` — health response struct, status value, uptime type
- `internal/handler/reg_hdl.go` — registration HTTP status code (200, not 201)
- `internal/handler/audit_hdl.go` — default audit limit (100, not 50)
- `internal/handler/challenge_hdl.go` — nonce response format

---

## Summary of Findings

| Category | Count | Severity |
|----------|-------|----------|
| Contamination (enterprise in core) | 1 | Medium |
| Critical errors (wrong auth mechanism, wrong API contract) | 8 | Critical |
| Factual errors (wrong values, wrong field names, wrong status codes) | 25+ | High |
| Cross-doc inconsistencies (version numbers, env var names) | 6 | Medium |
| OpenAPI spec vs code mismatches | 7 | High |

---

## 1. Contamination

### C-1: "Enterprise" label in common-tasks.md

**File:** `docs/common-tasks.md`, line 7
**Issue:** Version header reads `"2.0 (Enterprise)"`. The word "Enterprise" should not appear in the open-core repo. This is an artifact from the legacy agentauth repo.
**Risk:** Confuses open-source users about which product they're using.

No other HITL contamination was found. The previous cleanup removed HITL endpoints, `hitl_scopes`, `AA_HITL_APPROVAL_TTL`, `/v1/app/approvals`, `approval_token`, `original_principal`, and `ApprovalSvc` references.

---

## 2. Critical Errors

### CRIT-1: Registration examples use HMAC-SHA256 instead of Ed25519

**Files:** `docs/common-tasks.md` (lines 48–111), `docs/integration-patterns.md` (lines 69, 138–143)
**Code truth:** `internal/identity/id_svc.go` — Agent must submit base64-encoded Ed25519 signature of hex-decoded nonce bytes, plus base64-encoded Ed25519 public key.
**Doc error:** Examples show `hmac.new(launch_token.encode(), nonce.encode(), hashlib.sha256).hexdigest()` — a completely different cryptographic operation. This will cause every developer who copies these examples to fail registration with a signature verification error.
**Scope:** Affects Pattern 1 and Pattern 2 in integration-patterns.md, and the main registration walkthrough in common-tasks.md.

### CRIT-2: api.md app management section is entirely wrong

**File:** `docs/api.md`, lines 650–855
**Code truth:** `internal/app/app_hdl.go` lines 41–90

The entire App Management Endpoints section in api.md describes an API that does not exist. Nearly every field, scope, and status code is wrong:

| Aspect | api.md says | Code actually does |
|--------|-------------|-------------------|
| POST scope | `admin:apps:create` | `admin:launch-tokens:*` |
| GET scope | `admin:apps:read` | `admin:launch-tokens:*` |
| PUT scope | `admin:apps:update` | `admin:launch-tokens:*` |
| DELETE scope | `admin:apps:delete` | `admin:launch-tokens:*` |
| POST request fields | `name`, `client_id` (required), `description` | `name`, `scopes`, `token_ttl` (client_id is auto-generated) |
| POST response ID field | `id` | `app_id` |
| POST response fields | `id`, `name`, `client_id`, `client_secret`, `created_at` | `app_id`, `name`, `client_id`, `client_secret`, `scopes`, `token_ttl`, `status`, `created_at`, `updated_at` |
| PUT request fields | `name`, `description` | `scopes`, `token_ttl` |
| DELETE response | 204 No Content | 200 with JSON body `{app_id, status, deregistered_at}` |

### CRIT-3: App auth rate limit wrong in api.md

**File:** `docs/api.md`, line 816
**Code truth:** `internal/app/app_hdl.go` line 41: `authz.NewRateLimiter(10.0/60.0, 3)` — 10 requests/minute per client_id, burst 3.
**Doc error:** States "5 req/s, burst 10" — off by a factor of 30x.

### CRIT-4: App auth response wrong in api.md

**File:** `docs/api.md`, lines 829–853
**Code truth:** `internal/app/app_hdl.go` — App auth uses the app's configured `token_ttl` (default 1800s/30min from `AA_APP_TOKEN_TTL`). Response scopes are fixed operational scopes: `["app:launch-tokens:*", "app:agents:*", "app:audit:read"]`.
**Doc errors:**
- Says `expires_in` is "Always 300" — wrong, it's the app's `token_ttl` (default 1800)
- Shows `scopes` as the app's data scopes (e.g., `["read:data:*"]`) — wrong, returns fixed operational scopes

### CRIT-5: Token renewal behavior wrong in api.md

**File:** `docs/api.md`, line 326
**Code truth:** `internal/token/tkn_svc.go` lines 224–232: `Renew()` performs **mandatory predecessor revocation** before issuing the replacement token. Comment says: "Predecessor is revoked BEFORE issuing the new token so the old JTI is invalidated."
**Doc error:** States "The original token remains valid until its own expiry." This is the opposite of what happens.

### CRIT-6: common-tasks.md acquire_token() skips entire registration flow

**File:** `docs/common-tasks.md`, lines 877–888
**Code truth:** Registration requires launch_token, nonce from GET /v1/challenge, Ed25519 keypair, signature of hex-decoded nonce, orch_id, task_id, requested_scope.
**Doc error:** The `acquire_token()` helper function calls `POST /v1/register` with just `agent_name`, `scope`, `ttl` — skipping the entire challenge-response flow, Ed25519 signing, and launch token.

### CRIT-7: common-tasks.md references nonexistent endpoint

**File:** `docs/common-tasks.md`, line 625
**Doc error:** Says "Re-acquire via POST /v1/token" — this endpoint does not exist. The actual renewal endpoint is `POST /v1/token/renew`.

### CRIT-8: Signing key persistence claimed as ephemeral in multiple docs

**Files:** `docs/architecture.md` (line 399), `docs/getting-started-operator.md` (lines 139, 263)
**Code truth:** `internal/keystore/keystore.go` — `LoadOrGenerate(path)` reads key from disk if file exists, only generates new key if file is missing. Keys are persisted to `AA_SIGNING_KEY_PATH` (default `./signing.key`).
**Doc error:** Multiple docs state "generates a fresh Ed25519 signing key pair on every startup" or "Fresh Ed25519 keys every startup." This is factually wrong and has security implications — operators who believe keys are ephemeral may not implement proper key management.

---

## 3. Factual Errors by File

### docs/api.md

| Line | Error | Should be |
|------|-------|-----------|
| 658 | Scope `admin:apps:create` | `admin:launch-tokens:*` |
| 665 | `client_id` required in POST request | Not accepted; auto-generated |
| 666 | `description` field in POST request | Does not exist; use `scopes` and `token_ttl` |
| 668 | Response 201 for POST /v1/admin/apps | Needs verification (launch-tokens returns 201, app may differ) |
| 672 | Response field `id` | `app_id` |
| 701 | Scope `admin:apps:read` | `admin:launch-tokens:*` |
| 735 | Scope `admin:apps:read` | `admin:launch-tokens:*` |
| 758 | Scope `admin:apps:update` | `admin:launch-tokens:*` |
| 764–765 | PUT accepts `name`, `description` | Accepts `scopes`, `token_ttl` |
| 792 | Scope `admin:apps:delete` | `admin:launch-tokens:*` |
| 794 | DELETE returns 204 No Content | Returns 200 with JSON `{app_id, status, deregistered_at}` |
| 816 | App auth rate "5 req/s, burst 10" | 10 req/min (0.167 req/s), burst 3 |
| 829–830 | App auth `expires_in: 300` always | Uses app's `token_ttl` (default 1800) |
| 832, 853 | App auth scopes = data scopes | Fixed: `[app:launch-tokens:*, app:agents:*, app:audit:read]` |
| 326 | Renewal: "original token remains valid" | Original token is revoked before new token is issued |

### docs/common-tasks.md

| Line | Error | Should be |
|------|-------|-----------|
| 7 | Version "2.0 (Enterprise)" | "3.0" (or "2.0.0" to match code); remove "Enterprise" |
| 48–111 | Registration uses HMAC-SHA256 | Ed25519 signing of hex-decoded nonce bytes |
| 113 | Registration returns "201 Created" | Returns 200 OK (`reg_hdl.go` line 54: `StatusOK`) |
| 329 | JTI "32 hex chars" | 32 hex chars is correct (16 random bytes); however inconsistent with other docs |
| 625 | "POST /v1/token" endpoint | Does not exist; use POST /v1/token/renew |
| 877–888 | `acquire_token()` skips challenge-response | Missing launch_token, nonce, keypair, signature, orch_id, task_id |
| 1065–1068 | aactl env vars `AA_ADMIN_SECRET` | aactl uses `AACTL_ADMIN_SECRET` and `AACTL_BROKER_URL` |
| 1152 | Admin auth rate "5 requests/second, burst 10" | This is actually correct per code (admin_hdl.go line 47) |
| 1856 | Launch token TTL 1800s in example | Default launch token TTL is 30s (`defaultTokenTTL = 30`); 1800 is unreasonably long |
| 1865 | App auth `expires_in: 300` | Uses app's `token_ttl` (default 1800) |
| 1866 | App auth scopes = data scopes | Fixed: `[app:launch-tokens:*, app:agents:*, app:audit:read]` |

### docs/architecture.md

| Line | Error | Should be |
|------|-------|-----------|
| 387–389 | App auth rate "5/s, burst 10" | 10 req/min (0.167 req/s), burst 3. Admin auth rate is correctly shown as 5/s, burst 10 |
| 399 | "Fresh Ed25519 keys every startup" | Keys persist via `keystore.LoadOrGenerate` |

### docs/getting-started-operator.md

| Line | Error | Should be |
|------|-------|-----------|
| 79 | aactl env vars `AGENTAUTH_BROKER_URL`, `AGENTAUTH_ADMIN_SECRET` | `AACTL_BROKER_URL`, `AACTL_ADMIN_SECRET` |
| 139 | "generates a fresh Ed25519 signing key pair on every startup" | Loads from disk if exists; only generates if missing |
| 263 | Same signing key claim repeated | Same fix needed |

### docs/troubleshooting.md

| Line | Error | Should be |
|------|-------|-----------|
| 1 | Version "2.0" | Inconsistent with other docs saying "3.0" |
| 402 | "uses bcrypt comparison to prevent timing attacks" | Code DOES use `bcrypt.CompareHashAndPassword` (admin_svc.go line 120) — this is actually **CORRECT**. Earlier session notes incorrectly flagged this. |
| 457 | Admin auth rate "5 requests per second (burst 10)" | This is **CORRECT** per admin_hdl.go line 47 |

### docs/integration-patterns.md

| Line | Error | Should be |
|------|-------|-----------|
| 1 | Version "2.0" | Inconsistent with other docs |
| 69, 138–143 | Registration uses HMAC-SHA256 | Ed25519 signing of hex-decoded nonce bytes |
| 684 | Duplicate `BROKER = os.environ.get(...)` | Remove duplicate |
| 976 | Duplicate `BROKER = os.environ.get(...)` | Remove duplicate |

### docs/concepts.md

| Line | Error | Should be |
|------|-------|-----------|
| 1 | Version "2.0" | Inconsistent with other docs saying "3.0" |

*Note: The registration flow diagram in concepts.md correctly shows Ed25519, unlike common-tasks.md and integration-patterns.md.*

---

## 4. Cross-Document Inconsistencies

### Version Number Chaos

| Source | Version shown |
|--------|--------------|
| `cmd/broker/main.go` (line 58) | `"2.0.0"` (what health endpoint actually returns) |
| `docs/api.md` | "3.0" |
| `docs/getting-started-developer.md` | "3.0" |
| `docs/getting-started-operator.md` | "3.0" |
| `docs/getting-started-user.md` | "3.0" |
| `docs/api/openapi.yaml` | "3.0.0" |
| `docs/common-tasks.md` | "2.0 (Enterprise)" |
| `docs/concepts.md` | "2.0" |
| `docs/integration-patterns.md` | "2.0" |
| `docs/troubleshooting.md` | "2.0" |
| `docs/architecture.md` | Not stated |

**Decision needed:** Either update `main.go` to `"3.0.0"` or update all docs to `"2.0.0"`. Currently the health endpoint returns `"2.0.0"` which will confuse anyone reading docs that say "3.0".

### aactl Environment Variable Names

| Source | Var names used |
|--------|---------------|
| `docs/aactl-reference.md` | `AACTL_BROKER_URL`, `AACTL_ADMIN_SECRET` (**correct**) |
| `docs/getting-started-operator.md` | `AGENTAUTH_BROKER_URL`, `AGENTAUTH_ADMIN_SECRET` (**wrong**) |
| `docs/common-tasks.md` | `AA_ADMIN_SECRET` for aactl context (**wrong**) |

---

## 5. OpenAPI Spec (docs/api/openapi.yaml) vs Code

The OpenAPI spec is generally better than the prose docs, but has these issues:

| Location | Error | Should be |
|----------|-------|-----------|
| RegisterRequest schema | Field `nonce_signature` | Code uses `signature` (id_svc.go RegisterReq) |
| RegisterRequest schema | Missing fields | Code requires `nonce`, `orch_id`, `task_id`, `requested_scope` |
| Launch token response description | "Single-use JWT launch token" | It's an opaque 64-char hex string, not a JWT |
| HealthResponse status enum | `[healthy, degraded, unhealthy]` | Code returns `"ok"` (health_hdl.go line 65) |
| HealthResponse uptime type | `string` | Code returns `int64` (seconds as integer) |
| Audit events limit default | 50 (in path parameter description) | Code defaults to 100 (audit_hdl.go line 79) |
| AppAuthResponse scopes description | "Scopes granted to this token" (ambiguous) | Should clarify these are fixed operational scopes, not the app's data scopes |

---

## 6. Docs That Are Accurate

For completeness, these docs are well-aligned with code:

- **docs/aactl-reference.md** — Accurate endpoint documentation, correct env var names, correct admin auth format. No errors found.
- **docs/getting-started-developer.md** — Registration examples correctly use Ed25519. Both Python and TypeScript examples are accurate. Version "3.0" is at least internally consistent with other Getting Started docs.
- **docs/getting-started-user.md** — Previously cleaned up (sidecar removed). Registration flow is correct.
- **docs/RECOMMENDATIONS.md** — Meta-document about doc quality. Archival sidecar references are acceptable (struck through).

---

## 7. Gaps (Code Features Not Documented)

### Gap-1: Launch token `single_use` and `ttl` fields

The `CreateLaunchTokenReq` struct in `admin_svc.go` has `SingleUse *bool` and `TTL int` fields. The api.md documents these correctly, but the openapi.yaml `CreateLaunchTokenRequest` schema does not include `single_use`.

### Gap-2: App scope ceiling enforcement

When an app-authenticated caller creates a launch token, the broker enforces that requested scopes are a subset of the app's scope ceiling (`admin_hdl.go` lines 149–166). This scope ceiling enforcement is not documented in any guide.

### Gap-3: Registration requires orch_id and task_id

The code requires `orch_id` and `task_id` as mandatory fields in the registration request (`id_svc.go` line 126). These are used to construct the SPIFFE ID. The api.md documents this correctly, but the openapi.yaml RegisterRequest schema is missing these fields entirely.

### Gap-4: Renewal revokes predecessor

The `Renew()` function in `tkn_svc.go` revokes the predecessor token before issuing the replacement. This is security-critical behavior (prevents token accumulation) but is documented incorrectly (api.md claims original stays valid) or not at all in most docs.

### Gap-5: Admin scopes include admin:audit:*

The admin JWT carries three scopes: `admin:launch-tokens:*`, `admin:revoke:*`, `admin:audit:*`. The api.md line 235 documents this correctly, but other docs that reference admin capabilities don't always mention the audit scope.

---

## Priority Fix Order

1. **CRIT-1 + CRIT-6:** Fix registration examples in common-tasks.md and integration-patterns.md (HMAC → Ed25519). Developers will copy-paste these and fail.
2. **CRIT-2 + CRIT-3 + CRIT-4:** Rewrite the entire App Management section in api.md. Every field, scope, and status code is wrong.
3. **CRIT-8:** Fix signing key persistence claims in architecture.md and getting-started-operator.md. Security-critical misunderstanding.
4. **CRIT-5:** Fix renewal behavior description in api.md.
5. **CRIT-7:** Fix nonexistent endpoint reference in common-tasks.md.
6. **Version chaos:** Decide on version number and apply consistently.
7. **aactl env vars:** Standardize to `AACTL_BROKER_URL` / `AACTL_ADMIN_SECRET`.
8. **OpenAPI fixes:** Update RegisterRequest schema, health response, audit defaults.
9. **C-1:** Remove "Enterprise" label from common-tasks.md.
