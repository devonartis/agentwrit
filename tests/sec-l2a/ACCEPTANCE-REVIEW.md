# SEC-L2a Token Hardening — Acceptance Testing Review

**Branch:** `fix/sec-l2a`
**Date:** 2026-03-29
**Reviewer:** Cowork

---

## Results: 11 PASS / 2 FAIL

| Story | Tracker | Description | Persona | Mode | Verdict |
|-------|---------|-------------|---------|------|---------|
| S1 | b4-s1 | Auth and list apps after hardening | Operator | VPS | **PASS** |
| S2 | b4-s2 | MaxTTL=60 caps token lifetime | Operator | VPS | **PASS** |
| S3 | b4-s3 | MaxTTL=0 disables ceiling | Operator | VPS | **PASS** |
| S4 | b4-s4 | Revoked token rejected everywhere | Security Reviewer | VPS | **FAIL** |
| S5 | b4-s5 | Token renewal kills old token | Operator | VPS | **FAIL** |
| S6 | b4-s6 | Broker warns DefaultTTL > MaxTTL | Operator | VPS | **PASS** |
| S7 | b4-s7 | Empty kid accepted (backward compat) | Operator | VPS | **PASS** |
| N1 | b4-n1 | Tampered alg=HS256 rejected | Security Reviewer | VPS | **PASS** |
| N2 | b4-n2 | Wrong kid rejected | Security Reviewer | VPS | **PASS** |
| N3 | b4-n3 | No-expiry tokens rejected | Security Reviewer | Unit | **PASS** |
| N4 | b4-n4 | Wrong admin secret rejected | Security Reviewer | VPS | **PASS** |
| N5 | b4-n5 | Renewal fails on revoke error | Security Reviewer | Unit | **PASS** |
| SEC1 | b4-sec1 | Verify() check order review | Security Reviewer | Code | **PASS** |

---

## Failure Analysis: S4 and S5

Both failures have the **same root cause** identified in the code review (see CODE-REVIEW.md, finding C1):

**The `TknSvc.revoker` field is nil at runtime.** The broker's `main.go` never calls `tknSvc.SetRevoker(revSvc)`, so the revocation checks inside `Verify()` and `Renew()` are silently skipped (guard: `if s.revoker != nil`).

### S4: Revoked Token Is Rejected Everywhere
- **Expected:** After renewal, old token returns 401
- **Observed:** Old token still returns 200
- **Root cause:** Code bug — revoker not wired in main.go

### S5: Token Renewal Issues New Token and Kills Old One
- **Expected:** After renewal, old token returns 401, JTIs differ
- **Observed:** Old token still returns 200
- **Root cause:** Same as S4

### Why Unit Tests Pass But Live Tests Fail
- N5 (`TestRenew_RevokeFailureBlocksRenewal`) passes because the **unit test injects a mockRevoker** via `SetRevoker()`. The mock correctly simulates revocation behavior.
- At runtime, `SetRevoker()` is never called, so revoker is nil and revocation is silently skipped.
- This is a classic mock/integration gap: mocks prove the logic works, but the wiring is never tested end-to-end.

---

## Container Mode Testing

**Status: NOT RUN**

Stories S1, S4, S5, S7, N1, N2, N4 require VPS + Container mode per the spec. Only VPS mode was tested. Container mode is currently **blocked by the S4/S5 failures** — the application logic must be fixed first, since Container mode would show the same failures.

**After fixing C1, Container mode tests must be run for all 7 VPS+Container stories.**

---

## Evidence Quality Assessment

All 13 evidence files are present in `tests/sec-l2a/evidence/` with:
- Proper banners (who/what/why/how/expected)
- Test output captured in the evidence file
- Verdicts written based on observed output

Minor cosmetic issues: macOS base64url decode errors in some files (doesn't affect verdicts — HTTP status codes and response bodies were used instead).

Evidence README.md and FAILURE-REVIEW.md were created by Claude Code.

---

## What Needs to Happen Next

### Must Fix Before Merge (P0)

1. **Wire revoker in main.go** — Add `tknSvc.SetRevoker(revSvc)` in `cmd/broker/main.go` after both services are constructed. This is a one-line fix.

2. **Re-run S4 and S5** — After the fix, re-test both stories in VPS mode. Evidence files should be overwritten with passing results.

3. **Fix error info leakage in renew_hdl.go** — Return generic error to HTTP client, log full error internally. (See CODE-REVIEW.md, finding H1.)

### Must Do Before Merge (P1)

4. **Run Container mode tests** — After S4/S5 pass in VPS mode, run Container mode for: S1, S4, S5, S7, N1, N2, N4. Record evidence with `[Container]` sections in the existing evidence files.

5. **Update evidence README.md** — Reflect final PASS/FAIL status after fixes.

6. **Update tracker.jsonl** — Mark b4-s4 and b4-s5 as `done` after they pass.

### Should Do Before Merge (P2)

7. **Update Verify() docstring** — Add ErrTokenRevoked to return values. (CODE-REVIEW.md, M1)

8. **Add comment in Renew()** — Explain revoke-before-issue design choice. (CODE-REVIEW.md, M3)

### Can Do After Merge (P3)

9. **Add ErrKidMismatch sentinel** — Or document that kid mismatch returns ErrInvalidToken. (CODE-REVIEW.md, M2)

10. **Add WARN to envIntOr()** — Log when AA_MAX_TTL has an invalid value. (CODE-REVIEW.md, L3)

11. **Add RFC 7638 reference to computeKid()** — Docstring improvement. (CODE-REVIEW.md, L2)

---

## Sequence for Claude Code

The fix and retest sequence:

1. Read CODE-REVIEW.md and this file
2. Fix C1: Add `tknSvc.SetRevoker(revSvc)` in `cmd/broker/main.go`
3. Fix H1: Generic error response in `renew_hdl.go`
4. Fix M1: Update Verify() docstring
5. Fix M3: Add Renew() comment
6. Run gates (gates.sh)
7. Re-run S4 acceptance test (VPS)
8. Re-run S5 acceptance test (VPS)
9. Run Container mode for S1, S4, S5, S7, N1, N2, N4
10. Update evidence README.md
11. Update tracker.jsonl — all b4-* entries done
12. STOP for Cowork merge review
