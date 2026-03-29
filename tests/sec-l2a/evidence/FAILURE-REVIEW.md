# SEC-L2a Failure Review

**Date:** 2026-03-29
**Failing stories:** S4 (b4-s4), S5 (b4-s5)

## Symptom

After calling `POST /v1/token/renew` with Token A, getting Token B back:
- Token A should return 401 on any authenticated endpoint — **it returns 200**
- Token B works correctly (200)

Both S4 and S5 exhibit the identical symptom.

## Root Cause Hypothesis

The renewal endpoint issues a new token (Token B) but the old token (Token A)
is not being rejected on subsequent requests. Possible causes:

1. **Revocation store not wired into Verify()**: The `revoker` field on `TknSvc`
   may be nil at runtime, even though `NewRevSvc` requires a non-nil store.
   If the broker's main.go doesn't pass the revocation store when constructing
   `TknSvc`, the `s.revoker != nil` check in `Verify()` silently skips
   revocation checks.

2. **Renewal endpoint not calling Revoke()**: The renewal handler may issue a
   new token without actually revoking the old one's JTI in the store.

3. **In-memory vs persistent revocation**: If the revocation store is in-memory
   and the renewal writes to a different instance than the one Verify() reads
   from, revocations would be invisible.

4. **RevocationStore interface mismatch**: The Revoke() call in renewal may
   succeed but IsRevoked() may check a different key or format.

## What To Investigate

1. Check how `TknSvc` is constructed in `cmd/broker/main.go` — is a revoker
   passed in?
2. Check the renewal handler (`internal/handler/renew_hdl.go`) — does it call
   `tknSvc.Renew()` which internally revokes the old JTI?
3. Check `TknSvc.Renew()` — does it call `s.revoker.Revoke(oldJTI)` before
   issuing the new token?
4. Check that the unit test N5 (TestRenew_RevokeFailureBlocksRenewal) passes
   but uses a mock — the mock may behave differently from the real SQLite store.

## Action

Do NOT fix yet. Report back to Cowork for triage.
