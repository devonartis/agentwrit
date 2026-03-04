# Phase 0 — Legacy Cleanup: Live Test Evidence

**Date:** 2026-03-04
**Branch:** `fix/phase-0-legacy-cleanup`
**Stack:** Broker only (no sidecar in docker-compose)
**Broker version:** v2.0.0
**Admin secret:** Default from docker-compose.yml

## Story Results

| Story | Description | Persona | Tool | Verdict |
|-------|------------|---------|------|---------|
| P0-S1 | Sidecar list endpoint is gone | Security | curl | PASS |
| P0-S2 | Sidecar activation create endpoint is gone | Security | curl | PASS |
| P0-S3 | Sidecar activate endpoint is gone | Security | curl | PASS |
| P0-S4 | Token exchange endpoint is gone | Security | curl | PASS |
| P0-S5 | Sidecar ceiling read endpoint is gone | Security | curl | PASS |
| P0-S6 | Sidecar ceiling update endpoint is gone | Security | curl | PASS |
| P0-S7 | Operator logs in with new admin format | Operator | aactl | PASS |
| P0-S8 | Old admin login format rejected | Developer | curl | PASS |
| P0-R1 | Regression: register app | Operator | aactl | PASS |
| P0-R2 | Regression: developer app login | Developer | curl | PASS |
| P0-R3 | Regression: app cannot access admin | Security | curl | PASS |
| P0-R4 | Regression: audit trail | Operator | aactl | PASS |

## Open Issues

None.

## Credentials Used

- **app_id:** app-cleanup-test-c0e7b8
- **client_id:** ct-09ccbf99777a
- **client_secret:** c1878375389c36ec82b0746a6b13bcd61e5196d46f2bb030476a43b787eda145
