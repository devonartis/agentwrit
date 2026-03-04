# Story 10 — All app operations are recorded in the tamper-evident audit trail

## What we did

As operator, ran `aactl audit events` to view the full audit trail after running Stories 0-9. Checked for all 6 expected app event types, verified no `client_secret` appears in any detail, and verified the hash chain is intact.

## Command and output

```
$ ./bin/aactl audit events

ID          TIMESTAMP                       EVENT TYPE         OUTCOME  DETAIL
evt-000007  2026-03-04T03:06:07Z  app_registered     success  app=weather-bot client_id=wb-0753894ae326 scopes=[read:we...
evt-000010  2026-03-04T03:06:33Z  app_registered     success  app=log-agent client_id=la-b728a7a04770 scopes=[write:log...
evt-000013  2026-03-04T03:07:16Z  app_registered     success  app=alert-service client_id=as-b188a0881d44 scopes=[read:...
evt-000018  2026-03-04T03:08:10Z  app_updated        success  app_id=app-weather-bot-b4065c scopes=[read:weather:* writ...
evt-000021  2026-03-04T03:11:12Z  app_deregistered   success  app_id=app-alert-service-1c27fc name=alert-service deregi...
evt-000023  2026-03-04T03:11:25Z  app_auth_failed    denied   client_id=as-b188a0881d44 reason=app_inactive
evt-000024  2026-03-04T03:11:43Z  app_authenticated  success  client_id=wb-0753894ae326 app_id=app-weather-bot-b4065c
evt-000025  2026-03-04T03:12:12Z  app_auth_failed    denied   client_id=wb-0753894ae326 reason=wrong_secret
evt-000026  2026-03-04T03:12:29Z  app_auth_failed    denied   client_id=nonexistent-id-000 reason=unknown_client_id
```

## Event type summary

| Event type | Count | Triggered by |
|-----------|-------|--------------|
| app_registered | 3 | Stories 1, 2 (3 apps) |
| app_updated | 1 | Story 4 |
| app_deregistered | 1 | Story 5 |
| app_authenticated | 9 | Stories 6, 8, 9 |
| app_auth_failed | 3 | Stories 5, 7 (wrong secret, unknown ID, inactive app) |
| app_rate_limited | 0 | **MISSING — Story 9 triggers 429 but no audit event** |

## Hash chain

Verified from JSON output: all 43 events link correctly via `prev_hash`. Chain is intact.

## client_secret in audit trail

Searched all event detail fields — `client_secret` does NOT appear in any audit event. Only `client_id` is logged.

## Acceptance criteria check

| Criteria | Result |
|----------|--------|
| app_registered events recorded | PASS |
| app_authenticated events recorded | PASS |
| app_auth_failed events recorded | PASS |
| app_deregistered events recorded | PASS |
| app_rate_limited events recorded | FAIL — no audit event emitted on rate limit |
| All events include prev_hash (chain intact) | PASS |
| client_secret never in audit detail | PASS |

## Verdict

PARTIAL PASS — 5 of 6 expected event types present. `app_rate_limited` audit event is missing. Hash chain intact. No secrets leaked to audit trail.
