# SEC-L2a Token Hardening — Code Review

**Branch:** `fix/sec-l2a`
**Date:** 2026-03-29
**Reviewer:** Cowork (deep dive)

---

## Summary

11/13 acceptance tests PASS. 2 FAIL (S4, S5 — revocation not enforced at runtime). The code is functionally sound but has one architectural gap that explains both failures: the `TknSvc.revoker` field is never initialized in the broker, so revocation checks in `Verify()` and `Renew()` are silently skipped. Test coverage for M1, M3, M5 is exemplary. No contamination from private-repo features.

---

## CRITICAL Findings

### C1: Revoker Not Initialized at Broker Startup (ROOT CAUSE OF S4/S5 FAILURES)

**File:** `internal/token/tkn_svc.go` (lines ~94-99) + `cmd/broker/main.go` (lines ~127-128)
**Severity:** CRITICAL

The `TknSvc.revoker` field is initialized to nil (zero value) in `NewTknSvc()` and is only set via `SetRevoker()`. The broker's `main()` never calls `SetRevoker()`. The guards at Verify() line ~206 and Renew() line ~224 check `if s.revoker != nil` — since it's nil, revocation checks are **silently skipped at runtime**.

This is why S4 and S5 fail: renewal issues a new token, but the old token is never revoked because the revoker is nil. The `ValMw` middleware performs revocation checking independently, but the stated M4 goal of moving revocation "into Verify()" is defeated.

**Fix:** Call `tknSvc.SetRevoker(revSvc)` in `cmd/broker/main.go` after constructing both services. One line.

### C2: MaxTTL Clamping is Silent (No Per-Issuance Log)

**File:** `internal/token/tkn_svc.go` (lines ~110-112)
**Severity:** MEDIUM (labeled CRITICAL for completeness but functionally acceptable)

When a caller requests TTL=7200 and MaxTTL=3600 is set, Issue() silently clamps to 3600. The config-layer WARN at startup is present (S6 proves this), but there's no per-issuance log. The response `ExpiresIn` field reflects the clamped value, so callers CAN detect it.

**Fix:** Add a debug-level log when clamping occurs. Low priority — the config WARN is the primary safety net.

---

## HIGH Findings

### H1: Renewal Error Exposes Internal Details to HTTP Client

**File:** `internal/handler/renew_hdl.go` (line ~53)
**Severity:** HIGH

When Renew() fails, the error message is passed directly to the HTTP response: `"token renewal failed: "+err.Error()`. If predecessor revocation fails (M5), the client sees `"token renewal failed: revoke predecessor: [underlying error]"` — leaking internal architecture.

**Fix:** Map specific token errors to generic problem detail codes. Log the full error internally, return only `"token renewal failed"` to the client.

### H2: MockRevoker Field Naming Fragility

**File:** `internal/token/tkn_svc_test.go` (lines ~18-39)
**Severity:** HIGH (for future maintenance)

The mockRevoker uses `revokeErr` field. Earlier cherry-pick had `.err` which was fixed in commit 52638f8. The field naming inconsistency could cause future copy-paste errors.

**Fix:** Rename to `revokeByJTIErr` for clarity. Low urgency.

---

## MEDIUM Findings

### M1: ErrTokenRevoked Missing From Verify() Docstring

**File:** `internal/token/tkn_svc.go` (lines ~153-156)

Verify() docstring lists ErrInvalidToken and ErrSignatureInvalid but omits ErrTokenRevoked, which is now returned when revoker is active. Callers don't know to check for it.

**Fix:** Update docstring to include ErrTokenRevoked.

### M2: Kid Mismatch Returns ErrInvalidToken (Same as Alg Mismatch)

**File:** `internal/token/tkn_svc.go` (lines ~172-178 vs ~187-189)

Both kid mismatch and alg mismatch return ErrInvalidToken. API clients can't distinguish "key rotation in progress" from "format error". The WARN log helps operators but not API consumers.

**Fix:** Consider adding ErrKidMismatch sentinel error. Or document that kid mismatch returns ErrInvalidToken.

### M3: Renewal Comment Doesn't Explain Revoke-Before-Issue Design Choice

**File:** `internal/token/tkn_svc.go` (lines ~223-227)

Renew() calls RevokeByJTI() BEFORE Issue(). If Issue() fails, predecessor is revoked but no new token exists. This is the correct design (leaving one unrevoked is worse), but the comment doesn't explain why. A future maintainer might reorder thinking they're fixing a bug.

**Fix:** Add explanatory comment.

### M4: NewRevSvc(nil) Panic Test Only Checks String Match

**File:** `internal/revoke/rev_svc_test.go` (lines ~25-35)

If panic value changes from string to error type, the test would pass incorrectly. Test should assert `ok == true` more explicitly.

**Fix:** Improve panic type assertion.

---

## LOW Findings

### L1: Kid Mismatch Log Uses String Concatenation

**File:** `internal/token/tkn_svc.go` (line ~176)

Stylistic — log output is correct but inconsistent with cfg.go which uses fmt.Sprintf().

### L2: computeKid() Doesn't Reference RFC 7638 in Docstring

**File:** `internal/token/tkn_svc.go` (lines ~58-64)

The function implements RFC 7638 JWK Thumbprint but doesn't mention the RFC. Test code documents it, but implementation should too.

### L3: EnvIntOr() Silent Fallback on Invalid Numeric Values

**File:** `internal/cfg/cfg.go` (lines ~157-167)

If operator sets `AA_MAX_TTL=invalid-string`, envIntOr() silently falls back to default (86400). No log warning.

---

## What's Good

1. **Test coverage for M1, M3, M5 is exemplary.** Algorithm confusion, kid validation, MaxTTL clamping, and transactional renewal all have thorough positive and negative test cases.
2. **Contamination check is clean.** No references to OIDC, HITL, approval, cloud, or sidecar features. Cherry-pick was clean.
3. **Error wrapping uses %w consistently.** All error chains work with errors.Is() and errors.As().
4. **Revoker interface is minimal and well-designed.** Two methods, breaks circular dependency cleanly.
5. **Verify() check order is correct.** format → alg → kid → signature → claims → revocation. Cheap checks before expensive. SEC1 confirmed this.
6. **Kid computation follows RFC 7638.** SHA-256 thumbprint of canonical JWK, base64url-encoded.

---

## Action Items

| Priority | Finding | Fix | Blocks Merge? |
|----------|---------|-----|---------------|
| **P0** | C1: Revoker nil at startup | Call `SetRevoker(revSvc)` in main.go | **YES** — causes S4/S5 failures |
| **P1** | H1: Error info leakage | Generic error to client, full error to log | YES — security |
| **P2** | M1: Docstring gap | Update Verify() docstring | No |
| **P2** | M3: Renewal comment | Add explanatory comment | No |
| **P3** | M2: ErrKidMismatch | Document or add sentinel error | No |
| **P3** | L3: EnvIntOr logging | Add WARN on parse failure | No |
| **P3** | L2: RFC 7638 docstring | Add RFC reference | No |
