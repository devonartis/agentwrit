# Phase 1B: App-Scoped Launch Tokens — Docker Live Test Evidence

**Date:** 2026-03-04
**Branch:** `feature/phase-1b-launch-tokens`
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
| App: weather-bot | `app-weather-bot-fffad0` | Registered with ceiling `["read:weather:*"]` |
| App: narrow-bot-2 | `app-narrow-bot-2-357caa` | Registered with ceiling `["read:weather:current"]` (used for S7 case 4) |

## Story Results

| Story | Description | Persona | Tool | Verdict |
|-------|-------------|---------|------|---------|
| S1 | Developer creates launch token using app credentials | Developer | curl | PASS |
| S2 | Developer rejected when requesting scopes outside ceiling | Developer | curl | PASS |
| S3 | Developer registers agent linked to app | Developer | curl + python | PASS |
| S4 | Operator traces launch token back to app | Operator | aactl + curl | PASS |
| S5 | Operator confirms ceiling enforcement works | Operator | curl + aactl | PASS |
| S6 | Admin launch tokens still work (backward compatible) | Operator | curl + python | PASS |
| S7 | Scope attenuation at launch token level | Security | curl + aactl | PASS |
| S8 | Agent traceability to originating app | Security | aactl | PASS |
| R1 | App authentication (Phase 1a regression) | Developer | curl | PASS |
| R2 | App JWT blocked from admin endpoints (Phase 1a regression) | Security | curl | PASS |
| R3 | Admin auth and audit unchanged (Phase 1a regression) | Operator | aactl + curl | PASS |

## Open Issues

None.

## Notes

- Ed25519 key generation and signing done via Python `cryptography` library (macOS LibreSSL does not support Ed25519)
- Launch tokens have a 30-second default TTL — agent registration must happen immediately after token creation
- App JWT TTL is 5 minutes (logged as TD-006 — should be 30 min minimum, configurable per-app)

## How to Read This Evidence

Each story file contains:
1. **Banner** — who is doing this, what they're doing, why it matters, how to run, what's expected
2. **Test Output** — raw output piped directly from the command
3. **Verdict** — PASS or FAIL with explanation based on what the output showed
