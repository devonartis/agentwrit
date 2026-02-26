# Fix Ordering Analysis: First Principles Derivation

**Date:** 2026-02-25
**Context:** Session 8 revealed the original "independently implementable" claim was wrong. This document derives the correct fix ordering from first principles using code-level evidence.

---

## The Problem

The original `design-solution.md` and `implementation-plan.md` (Session 7) grouped fixes into three phases by theme:

- Phase 1 (Security): Fix 1 + Fix 2
- Phase 2 (Compliance): Fix 3 + Fix 4
- Phase 3 (Operations): Fix 5 + Fix 6

Session 8 Docker testing revealed Fix 1 was incomplete — the sidecar's `brokerClient` in `cmd/sidecar/broker_client.go` uses plain `http.Client` with no TLS config. mTLS between sidecar and broker is unrunnable. The "independently implementable" claim collapsed because Fix 1 and Fix 5 both modify sidecar code.

This analysis replaces thematic grouping with **dependency-driven ordering** derived from actual code relationships.

---

## First Principles

### Principle 1: Close real security gaps before compliance gaps

A security gap means an attacker can exploit the system today. A compliance gap means the system doesn't satisfy a specification checkbox. When both exist, fix the exploitable gap first.

**Application:** Fix 2 (revocations lost on restart) is a real security gap — a revoked compromised token becomes valid again after broker restart. Fix 1 (TLS transport) is a compliance gap — traffic is unencrypted, but the system already runs behind proxies in production. Fix 2 before Fix 1.

### Principle 2: Build foundations before dependents

If Fix B calls a function modified by Fix A, Fix A must be stable before Fix B is built on it. Otherwise Fix B is built on shifting ground.

**Application:** Fix 4 (token release) calls `revSvc.Revoke()`. If Fix 2 (revocation persistence) isn't done, released tokens vanish on restart. The release endpoint would be a compliance checkbox that doesn't actually work. Fix 2 before Fix 4.

### Principle 3: Minimize blast radius of the widest change

The fix that touches the most files should go last. Every fix merged before it is one more file it needs to account for. Every fix merged after it risks merge conflicts with the wide change.

**Application:** Fix 6 (structured audit) touches ~9 caller files plus `audit_log.go` and `sql_store.go`. If Fix 4 is done first, Fix 6 picks up the new `token_release_hdl.go` caller in its single pass. If Fix 6 goes first, Fix 4 must wire into the new structured API — adding coupling. Fix 6 last.

### Principle 4: Group changes to the same binary

Fixes that modify the same binary (broker or sidecar) should be adjacent in the schedule. This reduces context-switching cost and merge conflict windows.

**Application:** Fix 1 (sidecar TLS client) and Fix 5 (sidecar UDS listener) both modify `cmd/sidecar/config.go` and `cmd/sidecar/main.go`. Different code paths (outbound client vs. inbound listener) but same files. Do them back-to-back.

### Principle 5: Prefer small, fast wins early

Small fixes merged early build momentum and reduce the total surface area of uncommitted work.

**Application:** Fix 3 (audience validation, ~50 lines), Fix 4 (token release, ~60 lines), and Fix 2 (~120 lines) are all smaller than Fix 6 (~200 lines). Front-load them.

---

## Code-Level Dependency Evidence

### Hard Dependencies (must-order)

| Constraint | Evidence | Files |
|-----------|----------|-------|
| Fix 2 before Fix 4 | Fix 4's release handler calls `revSvc.Revoke(jti)`. Without Fix 2, this writes to in-memory map only. Broker restart = released tokens revalidate. | `rev_svc.go:Revoke()` called by new `token_release_hdl.go` |

### Soft Conflicts (same file, different functions)

| Pair | Shared File | What Each Touches | Risk |
|------|------------|-------------------|------|
| Fix 1 ↔ Fix 5 | `cmd/sidecar/config.go` | Fix 1: adds TLS client fields. Fix 5: adds `ListenerType`, `SocketPath` | Low — different struct fields, different `loadConfig()` blocks |
| Fix 1 ↔ Fix 5 | `cmd/sidecar/main.go` | Fix 1: modifies `newBrokerClient()`. Fix 5: modifies HTTP server listener | Low — different functions entirely |
| Fix 3 ↔ Fix 4 | `internal/identity/id_svc.go` | Fix 3: adds `Aud` to `IssueReq` in `Register()`. Fix 4: doesn't touch `Register()` | Negligible |
| Fix 4 ↔ Fix 6 | `internal/audit/audit_log.go` | Fix 4: adds `EventTokenReleased` const. Fix 6: restructures `AuditEvent` struct + `computeHash()` | Low — const addition vs. struct modification |

### No Conflicts

| Pair | Reason |
|------|--------|
| Fix 2 ↔ Fix 1, 3, 5, 6 | Fix 2 only touches `rev_svc.go`, `sql_store.go`, broker `main.go` (initialization). No other fix modifies revocation code. |
| Fix 3 ↔ Fix 1, 2, 5 | Fix 3 only touches token validation and issuance code. No overlap with TLS, revocation, or sidecar transport. |

---

## Derived Ordering

Applying all five principles:

```
Step 1: Fix 2 (Revocation Persistence)     — P0, security gap, foundation for Fix 4
Step 2: Fix 3 (Audience Validation)         — P1, independent, pure broker, small
Step 3: Fix 4 (Token Release)               — P1, depends on Fix 2, small
Step 4: Fix 1 (Complete Sidecar TLS Client) — P0, sidecar binary
Step 5: Fix 5 (Sidecar UDS Listen Mode)     — P1, sidecar binary, back-to-back with Fix 1
Step 6: Fix 6 (Structured Audit)            — P2, widest change, picks up all new callers
```

### Why this order and not another

**Why Fix 2 first, not Fix 1?**
Fix 1's broker-side TLS is already merged to develop. What remains is the sidecar client side. The sidecar currently talks to the broker over HTTP — this works (it's how the system runs today). Fix 2 addresses a gap where a revoked compromised token *silently becomes valid again* after restart. That's worse than unencrypted transport behind a proxy. Principle 1.

**Why Fix 3 before Fix 4?**
No hard dependency, but Fix 3 adds `Aud` propagation to `Register()` and `Renew()`. Fix 4 adds a new handler that uses the validation middleware. If audience validation is already in the middleware when Fix 4 is built, the release endpoint automatically benefits from audience checking. Principle 2.

**Why Fix 4 before Fix 1?**
Fix 4 is small (~60 lines), broker-only, and depends on Fix 2 which is already done at this point. Merging it now means one less open branch. Principle 5. Fix 1 requires sidecar work, which we group with Fix 5. Principle 4.

**Why Fix 1 then Fix 5 back-to-back?**
Both modify `cmd/sidecar/config.go` and `cmd/sidecar/main.go`. Doing them sequentially with no intervening fixes means the second fix builds directly on the first's changes with no stale merge context. Principle 4.

**Why Fix 6 last?**
It touches 9+ caller files. Every fix that adds audit callers before Fix 6 means Fix 6 handles them all in one pass. If Fix 6 went earlier, subsequent fixes would need to use the new structured audit API (adding coupling) or use the old API (creating inconsistency). Principle 3.

### Alternative orderings considered and rejected

**Fix 1 first (complete TLS before anything else):**
Rejected. The broker already serves TLS — that's merged. The remaining sidecar work is important but doesn't block any other fix. Meanwhile Fix 2's security gap is actively exploitable on every broker restart.

**Fix 6 before Fix 4 (structured audit first so Fix 4 uses new API):**
Rejected. Fix 6 is the largest change (~200 lines, 9+ files). Doing it early delays all subsequent fixes and creates a wide merge target. Fix 4 can use the existing `Record()` API; Fix 6 will update it along with all other callers.

**Parallel execution (Fix 2 + Fix 3 simultaneously):**
Viable but rejected for this project. These fixes touch completely different files and could theoretically be parallel branches. However, the merge-to-develop cadence means each fix needs a Docker live test before merge. Running parallel branches with sequential merge gates doesn't save wall-clock time and adds merge coordination overhead.

---

## Summary

The ordering is: **2 → 3 → 4 → 1 → 5 → 6**

Each step is justified by a specific principle applied to specific code evidence. No fix is blocked by a later fix. The widest change goes last. Security gaps close before compliance gaps. Same-binary changes are adjacent.
