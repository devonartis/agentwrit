# TD-006 — Per-App JWT TTL: Live Test Evidence

**Date:** 2026-03-05
**Branch:** `feature/td-006-app-jwt-ttl`
**Stack:** Broker only (Docker)
**Broker version:** v2.0.0

## Story Results

| Story | Description | Persona | Tool | Verdict |
|-------|------------|---------|------|---------|
| TD006-S1 | Register app with default TTL | Operator | aactl | PASS |
| TD006-S2 | Register app with custom TTL | Operator + Developer | aactl + curl | PASS |
| TD006-S3 | Update existing app's TTL | Operator + Developer | aactl + curl | PASS |
| TD006-S4 | Reject out-of-bounds TTL | Operator | aactl | PASS |
| TD006-S5 | Developer token reflects configured TTL | Developer | curl | PASS |
| TD006-S6 | TTL bounds prevent misconfiguration | Security | aactl | PASS (after fix) |
| TD006-S7 | TTL changes are audited | Security | aactl | PASS |

## Issues Found and Fixed

### FIXED: TTL 0 and -1 silently accepted (S6)

**Found:** First run of S6. `--token-ttl 0` and `--token-ttl -1` created apps with the default TTL instead of being rejected.

**Root cause:** CLI used `if tokenTTL > 0` to decide whether to send the value — 0 and negatives were treated as "not provided." Handler used `int` (not `*int`) for token_ttl, so JSON couldn't distinguish absent from 0.

**Fix:** Handler changed `registerAppReq.TokenTTL` from `int` to `*int`. Handler validates `<= 0` before calling service. CLI uses `cmd.Flags().Changed("token-ttl")` instead of `> 0`. Two new unit tests added.

**Verified:** S6 re-run — all five boundary cases now correct.

## Open Issues

### BUG: Duplicate app name returns 500 instead of 409 (found during testing)

Registering an app with a name that already exists returns HTTP 500 (internal_error) instead of HTTP 409 (conflict). The SQLite UNIQUE constraint error bubbles up as an unhandled internal error. The broker should catch this and return a proper client error.

**Severity:** Low — functional but poor error reporting. Deferred to future work.
