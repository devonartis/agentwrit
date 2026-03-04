# Phase 1a Live Test Evidence

**Date:** 2026-03-03
**Branch:** `feature/phase-1a-app-registration`
**Stack:** Broker only (sidecar removed — no defined use case)
**Admin secret:** Default from docker-compose.yml

## Story Results

| Story | Description | Persona | Verdict |
|-------|------------|---------|---------|
| 0 | Broker starts clean, no apps | Operator | PASS |
| 1 | Register app, receive credentials | Operator | PARTIAL — `sk_live_` prefix missing |
| 2 | App list grows: 1 → 2 → 3 apps, distinct creds | Operator | PASS |
| 3 | Get app details by app_id | Operator | PASS |
| 4 | Update scope ceiling | Operator | PASS |
| 5 | Deregister app, credentials rejected | Operator + Developer | PASS |
| 6 | Developer authenticates with client_id + client_secret | Developer | PASS |
| 7 | Bad credentials return generic 401 | Developer | PASS |
| 8 | App JWT cannot access admin endpoints | Security | PASS |
| 9 | Per-client_id rate limiting | Security | PASS |
| 10 | Audit trail completeness | Security | PARTIAL — `app_rate_limited` event missing |
| 11 | Regression: admin/audit flows still work | Operator | PASS |

## Open Issues

1. **`sk_live_` prefix missing from client_secret** (Story 1) — Implementation generates plain 64-char hex. Acceptance criteria requires `sk_live_` prefix. Decision needed: fix implementation or update criteria.

2. **`app_rate_limited` audit event not emitted** (Story 10) — Rate limiter returns 429 but does not log an audit event. The rate limiting middleware fires before the handler, so the audit call is never reached. Needs a fix in the rate limiter or handler.

## Credentials Used

Saved in individual story files. Three apps registered with distinct credentials:

| App | client_id | Saved in |
|-----|-----------|----------|
| weather-bot | wb-0753894ae326 | story-1-register-app.md |
| log-agent | la-b728a7a04770 | story-2-app-list-progression.md |
| alert-service | as-b188a0881d44 | story-2-app-list-progression.md |
