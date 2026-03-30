# SEC-A1 — TTL Carry-Forward on Renewal: Live Test Evidence

**Date:** 2026-03-30
**Branch:** `fix/sec-a1`
**Stack:** Broker VPS mode (compiled binary)

## Story Results

| Story | Description | Persona | Mode | Verdict |
|-------|------------|---------|------|---------|
| A1-S1 | Admin flow — renewal preserves TTL | Security Reviewer | VPS | PASS |
| A1-S2 | App flow — renewal preserves TTL (production path) | Security Reviewer | VPS | PASS |
| A1-S3 | App cannot use admin launch-token endpoint | Security Reviewer | VPS | PASS |
| A1-R1 | Regression — full issue-validate-renew lifecycle | App | VPS | PASS |

## Open Issues

- TD-012: Missing role model documentation (`docs/roles.md`) — CRITICAL
- TD-013: `POST /v1/admin/launch-tokens` lets admin create agents without scope ceiling — needs design decision
- TD-014: Code comments audit needed across all `internal/` packages
