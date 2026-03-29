# SEC-L2a Acceptance Test Results

**Branch:** `fix/sec-l2a`
**Date:** 2026-03-29

## Summary: 13/13 PASS

| Story | Tracker | Description | VPS | Container |
|-------|---------|-------------|-----|-----------|
| S1 | b4-s1 | Auth and list apps after hardening | PASS | PASS |
| S2 | b4-s2 | MaxTTL=60 caps token lifetime | PASS | VPS only |
| S3 | b4-s3 | MaxTTL=0 disables ceiling | PASS | VPS only |
| S4 | b4-s4 | Revoked token rejected everywhere | PASS | PASS |
| S5 | b4-s5 | Token renewal kills old token | PASS | PASS |
| S6 | b4-s6 | Broker warns DefaultTTL > MaxTTL | PASS | VPS only |
| S7 | b4-s7 | Empty kid accepted (backward compat) | PASS | PASS |
| N1 | b4-n1 | Tampered alg=HS256 rejected | PASS | PASS |
| N2 | b4-n2 | Wrong kid rejected | PASS | PASS |
| N3 | b4-n3 | No-expiry tokens rejected | PASS (unit) | N/A |
| N4 | b4-n4 | Wrong admin secret rejected | PASS | PASS |
| N5 | b4-n5 | Renewal fails on revoke error | PASS (unit) | N/A |
| SEC1 | b4-sec1 | Verify() check order review | PASS (code) | N/A |

## Fixes Applied During Testing

S4 and S5 initially FAILED because `TknSvc.revoker` was nil at runtime
(see CODE-REVIEW.md finding C1). Four fixes were applied:

1. **C1** — `tknSvc.SetRevoker(revSvc)` added in `cmd/broker/main.go`
2. **C1** — `RevokeByJTI()` method added to `RevSvc` to implement `token.Revoker`
3. **H1** — Error info leakage sanitized in `internal/handler/renew_hdl.go`
4. **M1** — `ErrTokenRevoked` added to `Verify()` docstring
5. **M3** — Revoke-before-issue design comment added to `Renew()`

All gates passed after fixes (build, lint, unit tests).
S4 and S5 re-tested and PASS in both VPS and Container modes.

## Test Tooling Note

macOS `base64 -d` does not handle base64url encoding (no padding, `-_` chars).
JWT header/payload decodes produce jq parse errors in some evidence files. This
is cosmetic — broker behavior was verified through HTTP status codes and response
bodies.
