# Phase 1B: App-Scoped Launch Tokens — Docker Live Test Evidence

**Date:** 2026-03-04
**Branch:** `feature/phase-1b-launch-tokens` (commit `142948b`)
**Stack:** Docker Compose — broker only (no sidecar)
**Broker version:** 2.0.0
**Tester:** Claude Opus 4.6 + operator review

## Test Environment

- Docker image built from current branch
- Broker running on `http://127.0.0.1:8080`
- Admin secret: `change-me-in-production` (default from docker-compose.yml)
- Fresh database (no prior state)

## Credentials Used

| Entity | ID | Notes |
|--------|----|-------|
| Admin | shared secret via `AACTL_ADMIN_SECRET` | Operator persona |
| App: weather-bot | `app-weather-bot-*` | Registered with ceiling `["read:weather:*"]` |

## Story Results

| Story | Description | Persona | Verdict |
|-------|-------------|---------|---------|
| R1 | App authentication (Phase 1a regression) | Developer | |
| R2 | App JWT blocked from admin endpoints (Phase 1a regression) | Developer | |
| R3 | Admin auth and audit unchanged (Phase 1a regression) | Operator | |
| 1 | Developer creates launch token using app credentials | Developer | |
| 2 | Developer rejected when requesting scopes outside ceiling | Developer | |
| 3 | Developer registers agent linked to app | Developer | |
| 4 | Operator traces launch token back to app | Operator | |
| 5 | Operator confirms ceiling enforcement works | Operator | |
| 6 | Admin launch tokens still work (backward compatible) | Operator | |
| 7 | Scope attenuation at launch token level | Security | |
| 8 | Agent traceability to originating app | Security | |
| 9 | Agent JWT cannot create launch tokens (wrong scope) | Developer | |
| 10 | Deregistered app's JWT rejected at launch token creation | Operator | |

## Open Issues

(filled after all stories complete)

## How to Read This Evidence

Each story file contains:
1. **Purpose** — what the story tests and why it matters
2. **Preconditions** — what must be true before running
3. **Steps** — exact commands to reproduce, with explanation of each step
4. **Expected vs Actual** — what we expected and what happened
5. **Acceptance Criteria** — pass/fail table against each criterion from the user story
6. **Verdict** — PASS, FAIL, or PARTIAL with explanation
