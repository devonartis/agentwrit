# B6 Analysis — SEC-A1 + Gates (2 commits)

**Date:** 2026-03-30
**Batch:** B6 (last batch)
**Branch:** `fix/sec-a1` off `develop`

## Commit 1: `9422e7c` — carry forward original TTL on renewal

**What it changes:** The `Renew()` method in `tkn_svc.go` was calling `Issue()` without setting TTL, causing renewal to fall back to `cfg.DefaultTTL` instead of preserving the original token's TTL.

**Fix:** Computes `originalTTL = claims.Exp - claims.Iat`, guards against `<= 0`, passes `TTL: originalTTL` to `IssueReq`. The MaxTTL clamp in `Issue()` still applies as a global ceiling.

**Security impact:** Closes TTL ceiling bypass — agents launched with TTL=120 can no longer escalate to 300+ seconds through renewal cycles.

**Files:** `internal/token/tkn_svc.go` (+19/-11)
**Add-on code:** None
**Conflict risk:** LOW — surgical change to Renew method. `IssueReq.TTL` field already exists in core.

## Commit 2: `e395a15` — gates.sh regression subcommand

**What it changes:** Adds a `regression` mode to `scripts/gates.sh` that discovers and runs all `tests/*/regression.sh` scripts, reports per-phase pass/fail.

**Files:** `scripts/gates.sh` (+39/-3)
**Add-on code:** None
**Conflict risk:** LOW — additive change, new mode string + isolated block at end.

## Overall Risk: LOW

Both commits are self-contained, no overlaps, no add-on contamination risk. Ready to cherry-pick.
