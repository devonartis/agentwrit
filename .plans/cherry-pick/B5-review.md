# B5 (SEC-L2b) Code Review

**Branch:** `fix/sec-l2b` (7 commits over `develop`)
**Reviewer:** Senior Code Reviewer (automated)
**Date:** 2026-03-30
**Verdict:** PASS — ready to merge, one Important note, two Suggestions

---

## 1. Plan Alignment

| Planned Item | Status | Notes |
|---|---|---|
| SecurityHeaders middleware (nosniff, no-store, DENY, HSTS) | DONE | `internal/handler/security_hdl.go` — all 4 headers correct |
| Global MaxBytesBody (1MB, moved from per-route) | DONE | `problemdetails.go` upgraded with eager buffering; per-route wrappers removed from `main.go` |
| Sanitized errors in val_hdl | DONE | Both parse-error and revoked-token paths return `"token is invalid or expired"` |
| Sanitized errors in renew_hdl | ALREADY DONE | `renew_hdl.go` is **identical** on branch and develop — it already used `"token renewal failed"`. No diff needed. Correctly noted in cherry-pick log as conflict resolution keeping existing code. |
| Sanitized errors in ValMw | DONE | Changed from `"token verification failed: "+err.Error()` to `"token verification failed"` |

**Extra changes (justified):**
- Docs updates (CHANGELOG, FLOW, MEMORY, architecture, api, concepts, implementation-map, getting-started-operator) — required by standing rule.
- Integration test script (`tests/sec-l2b/integration.sh`) and user stories — good addition.
- `handler_test.go` — 6 new test functions covering all B5 behaviors.

**Nothing extra or missing. Plan fully satisfied.**

---

## 2. Contamination Check

**CLEAN.** The grep for `oidc|hitl|sidecar|federation|cloud|openid` across the diff returns only documentation references (descriptions of what enterprise modules are, conflict resolution notes). Zero contamination in `internal/` or `cmd/` Go code.

---

## 3. Error Sanitization Audit

| Handler | Client-facing message | Internal details logged? | Verdict |
|---|---|---|---|
| val_hdl (parse error) | `"token is invalid or expired"` | Yes — `obs.Warn` with `err.Error()` + `request_id` | GOOD |
| val_hdl (revoked) | `"token is invalid or expired"` | Yes — `obs.Warn` with `sub` + `request_id` | GOOD |
| renew_hdl | `"token renewal failed"` | Yes — `obs.Warn` with `err.Error()`, audit log with agent + error | GOOD |
| ValMw | `"token verification failed"` | Yes — audit log with `err.Error()` + path | GOOD |

No internal error details leak to clients. All errors are logged server-side with enough context for debugging.

---

## 4. Middleware Ordering

In `main.go`:
```
mux -> SecurityHeaders -> MaxBytesBody -> LoggingMiddleware -> RequestIDMiddleware
```

Execution order (outermost to innermost, i.e., request flows right-to-left):
1. **RequestIDMiddleware** — assigns request ID (innermost wrap = first to execute)
2. **LoggingMiddleware** — logs request with ID
3. **MaxBytesBody** — rejects oversized bodies before handler processing
4. **SecurityHeaders** — sets response headers before handler runs

This is correct. RequestID must run first so all downstream middleware/handlers can reference it. SecurityHeaders sets response headers before `next.ServeHTTP` so handlers can override `Cache-Control` (last-writer-wins). MaxBytesBody eagerly buffers before handlers parse JSON.

**Test broker (`handler_test.go`) mirrors this ordering exactly** — good.

---

## 5. Test Coverage

| Behavior | Unit Test | Integration Test |
|---|---|---|
| Security headers on all endpoints | `TestSecurityHeaders_PresentOnAllResponses`, `TestSecurityHeaders_BaseHeaders` | S4 |
| HSTS on TLS | `TestSecurityHeaders_HSTSWhenTLS` | S5 (skipped, needs TLS cert) |
| Cache-Control override | `TestSecurityHeaders_HandlerCanOverrideCacheControl` | — |
| Global body limit (413) | `TestGlobalBodyLimit_OversizedPayload` | S6 |
| val_hdl generic error | `TestValidate_ErrorMessageIsGeneric` | S1 |
| val_hdl revoked generic error | (covered by existing revoke tests) | S2 |
| renew_hdl generic (via ValMw) | `TestRenew_ErrorMessageIsGeneric` | S3 |
| renew_hdl direct error | `TestRenew_DirectErrorMessageIsGeneric` | — |

Coverage is thorough. Both the ValMw-rejection path and the direct RenewHdl error path are tested.

---

## 6. Issues

### Important

**I-1: Duplicate gate log entries in TESTING.md.** The diff shows G1-G3 for B5 recorded twice (once from initial cherry-pick, once after Docker gates). This is cosmetic but makes the audit log confusing. Consider deduplicating before merge.

### Suggestions

**S-1: MaxBytesBody eager buffering holds full body in memory.** The new implementation reads the entire body (up to 1MB) into a `[]byte` buffer, then wraps it in a `bytes.Reader`. This is fine for 1MB but worth noting in the doc comment that the memory cost is O(min(body_size, 1MB)) per concurrent request. Not a bug — just a documentation opportunity.

**S-2: `renew_hdl.go` audit log still includes `err.Error()` in the audit record detail string** (`fmt.Sprintf("token renewal failed for agent=%s: %s", claims.Sub, err.Error())`). This is server-side only (audit log, not client response) so it is not a leak. However, if audit logs are ever exposed via the `/v1/audit/events` endpoint to non-admin users in the future, this could become one. Current access control (`admin:audit:*` scope required) makes this safe today.

---

## 7. Summary

The B5 batch is clean, well-tested, and faithfully implements all five planned items. The renew_hdl sanitization was already in place from a prior batch, which was correctly handled during conflict resolution. Middleware ordering is correct. No contamination. No security issues.

**Recommendation: Merge after deduplicating TESTING.md gate entries (I-1).**
