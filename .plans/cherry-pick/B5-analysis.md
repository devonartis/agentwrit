# B5 (SEC-L2b) Cherry-Pick Analysis

**Date:** 2026-03-30  
**Branch:** agentauth-core (target)  
**Source:** agentauth legacy repo B5 branch  
**Scope:** 5 security commits (daf2995, e592acc, 2857b3a, 247727c, c5da6c4)

---

## Commit Summary

| Commit | Title | Files Changed | LOC |
|--------|-------|---------------|-----|
| daf2995 | SecurityHeaders middleware + HSTS | security_hdl.go (NEW) | +88 |
| e592acc | Global MaxBytesBody + wire SecurityHeaders | main.go, handler_test.go, problemdetails.go | +74 / -13 |
| 2857b3a | Sanitize val_hdl error response | val_hdl.go, handler_test.go | +20 / -2 |
| 247727c | Sanitize renew_hdl error response | renew_hdl.go, handler_test.go | +49 / -1 |
| c5da6c4 | Sanitize ValMw error response | val_mw.go + evidence file | +21 / -16 |

---

## Detailed Analysis by Commit

### 1. daf2995 — SecurityHeaders Middleware (H1)

**What it changes:**
- **NEW FILE:** `internal/handler/security_hdl.go` — Adds SecurityHeaders middleware factory
  - Sets base security headers on all responses: `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `X-Frame-Options: DENY`
  - Conditionally adds HSTS (`Strict-Transport-Security`) when `tlsMode == "tls"` or `"mtls"`
  - Allows handlers to override Cache-Control (last-writer-wins)
- **NEW FILE:** `internal/handler/security_hdl_test.go` — 3 test cases
  - BaseHeaders, HSTSWhenTLS, HandlerCanOverrideCacheControl

**Touches add-on code?**
- **NO** — Pure core security middleware, no references to approval/, oidc/, cloud/, hitl, federation, sidecar

**Conflict risk:**
- **LOW** — New file, no dependencies on existing handler structure
- Target repo has no existing `security_hdl.go`; can apply cleanly
- Tests follow existing patterns (`newTestBroker`, `httptest.NewRequest`)

**Cherry-pick readiness:** ✓ READY

---

### 2. e592acc — Global MaxBytesBody + Wire SecurityHeaders (H1, H7)

**What it changes:**
- **cmd/broker/main.go:**
  - Adds `SecurityHeaders(c.TLSMode)` to global middleware stack
  - Adds `MaxBytesBody` to global middleware stack
  - **Removes all per-route `problemdetails.MaxBytesBody()` wrappers** from 6 handler routes:
    - `/v1/cloud/credentials`, `/v1/token/validate`, `/v1/register`, `/v1/token/renew`, `/v1/delegate`, `/v1/token/release`, `/v1/revoke`
  - Reorders middleware: global → SecurityHeaders → MaxBytesBody → LoggingMiddleware → RequestIDMiddleware

- **internal/handler/handler_test.go:**
  - Updates `newTestBroker()` to mirror global middleware stack (SecurityHeaders + MaxBytesBody)
  - Removes per-route MaxBytesBody wrappers in test setup
  - Adds `TestSecurityHeaders_PresentOnAllResponses`, `TestGlobalBodyLimit_OversizedPayload`

- **internal/problemdetails/problemdetails.go:**
  - Fixes MaxBytesBody to eagerly buffer body (prevents streaming JSON decoders from bypassing 413)

**Touches add-on code?**
- **MINOR TOUCH:** Line references `/v1/cloud/credentials` endpoint
  - However, **NO CODE CHANGES TO CLOUD HANDLER LOGIC**
  - Only removes MaxBytesBody wrapper (refactoring for global stack)
  - Condition `if cloudCredHdl != nil` remains unchanged
  - No inspection of cloud-specific handler implementation

**Conflict risk:**
- **MEDIUM** — Depends on:
  1. `c.TLSMode` config field existing in target repo
  2. Global middleware initialization order in `main.go`
  3. `newTestBroker()` existing and matching route set
  4. Handler instantiation variables: `cloudCredHdl`, `valMw`, `valHdl`, `regHdl`, `renewHdl`, `delegHdl`, `releaseHdl`, `revokeHdl`, `auditHdl`

**Potential issues in agentauth-core:**
- Need to verify target `main.go` has same handler variables and middleware pattern
- If target already has SecurityHeaders or modified MaxBytesBody, conflict risk rises to HIGH

**Cherry-pick readiness:** ✓ READY (pending main.go structure verification)

---

### 3. 2857b3a — Sanitize val_hdl Error Response (H3)

**What it changes:**
- **internal/handler/val_hdl.go:**
  - In token verification error path (line ~66): Replace `"Error": err.Error()` with `"Error": "token is invalid or expired"`
  - In revocation check error path (line ~79): Replace `"Error": "token has been revoked"` with same generic message
  - **Adds observability:** Log full error with request_id before responding with generic message

- **internal/handler/handler_test.go:**
  - Adds `TestValidate_ErrorMessageIsGeneric` — verifies error response uses generic text, not raw error

**Touches add-on code?**
- **NO** — Only modifies error response text in core validation handler
- No federation/oidc/approval/cloud logic touched
- Revocation check uses `h.revSvc` (generic revocation service, not add-on-specific)

**Conflict risk:**
- **LOW** — Minimal changes, isolated to error handling
- Target repo must have `val_hdl.go` with same error paths
- Logging addition uses existing `obs.Warn()` pattern

**Cherry-pick readiness:** ✓ READY

---

### 4. 247727c — Sanitize renew_hdl Error Response (H4)

**What it changes:**
- **internal/handler/renew_hdl.go:**
  - Line 54: Replace `"token renewal failed: "+err.Error()` with `"token renewal failed"`
  - Full error still logged to audit trail before response

- **internal/handler/handler_test.go:**
  - Adds `TestRenew_ErrorMessageIsGeneric` — tampered token, verifies no "signature"/"segment" leak
  - Adds `TestRenew_DirectErrorMessageIsGeneric` — valid token renewal, verifies generic detail

**Touches add-on code?**
- **NO** — Token renewal is core functionality
- Tests use `getAdminToken()`, `registerAgentHTTP()` (no add-on involvement)

**Conflict risk:**
- **LOW** — Single-line change in error path
- Target repo must have `renew_hdl.go` with WriteProblem call

**Cherry-pick readiness:** ✓ READY

---

### 5. c5da6c4 — Sanitize ValMw Error Response (H4)

**What it changes:**
- **internal/authz/val_mw.go:**
  - Line 96: Replace `"token verification failed: "+err.Error()` with `"token verification failed"`
  - Full error still logged to audit (line 93)

- **tests/fix-sec-l2b/evidence/S3-renew-tampered-generic.md:**
  - Documentation update (test evidence file)

**Touches add-on code?**
- **NO** — ValMw is core token validation middleware
- No federation/oidc/approval references
- Audit log record uses `audit.EventTokenAuthFailed` (standard event)

**Conflict risk:**
- **LOW** — Single-line change, foundational middleware
- Target repo must have `internal/authz/val_mw.go` with WriteProblem call
- No structural changes

**Cherry-pick readiness:** ✓ READY

---

## Cross-Commit Contamination Check

**Search terms:** approval, oidc, cloud, federation, hitl, sidecar, issuer, thumbprint, jwk

**Findings:**
- ✓ **NO CONTAMINATION DETECTED**
- Only `cloud` reference is `/v1/cloud/credentials` endpoint route (e592acc)
  - This is a **cosmetic route change**, not cloud handler logic
  - No cloud-specific error handling, config, or features touched
- All error sanitization is **core-only** (val_hdl, renew_hdl, ValMw)
- SecurityHeaders is pure middleware, agnostic to handler types

---

## Target Environment Verification

**agentauth-core current state:**

```
/internal/handler/:
- audit_hdl.go
- challenge_hdl.go
- deleg_hdl.go
- doc.go (NEW in core)
- handler_test.go
- health_hdl.go
- logging.go / logging_test.go
- metrics_hdl.go
- reg_hdl.go
- release_hdl.go / release_hdl_test.go
- renew_hdl.go
- request_id_test.go
- revoke_hdl.go
- val_hdl.go
[NO security_hdl.go yet]

/internal/authz/:
- (minimal, assumed to have val_mw.go)
```

**Key observations:**
1. Target repo **already has all handler files** that will be modified (val_hdl, renew_hdl, handler_test)
2. **security_hdl.go does NOT exist** — clean add for daf2995
3. Structure suggests agentauth-core is a parallel/forked codebase from legacy, **NOT a merge**

---

## Conflict Risk Summary

| Commit | Risk | Blocker | Notes |
|--------|------|---------|-------|
| daf2995 | **LOW** | None | New file, no deps |
| e592acc | **MEDIUM** | Check main.go structure | Depends on handler vars, TLSMode config |
| 2857b3a | **LOW** | None | Error path only, isolated |
| 247727c | **LOW** | None | Single-line change |
| c5da6c4 | **LOW** | None | Single-line change |

---

## Recommended Cherry-Pick Order

1. **daf2995** — Add SecurityHeaders middleware (NEW, no deps)
2. **2857b3a** — Sanitize val_hdl (isolated, no middleware deps)
3. **247727c** — Sanitize renew_hdl (isolated, no middleware deps)
4. **c5da6c4** — Sanitize ValMw (isolated, core authz only)
5. **e592acc** — Wire global middleware (depends on #1, tested after core changes)

---

## Approval Status

- ✓ **CLEAN FOR CHERRY-PICK** — No add-on contamination detected
- ✓ **NO HIDDEN DEPENDENCIES** — Core-only changes
- ⚠️ **VERIFY BEFORE APPLY:**
  - Target `cmd/broker/main.go` has same handler variable names and middleware pattern
  - Target `internal/authz/val_mw.go` exists with same WriteProblem call (line ~96)
  - Target supports `c.TLSMode` config field

