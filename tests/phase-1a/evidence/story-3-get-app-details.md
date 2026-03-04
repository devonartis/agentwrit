# Story 3 — Operator views details of a specific app

## What we did

Retrieved details for `app-weather-bot-b4065c` by app_id. Also tested with a nonexistent app_id.

## Commands and output

### Get existing app

```
$ ./bin/aactl app get app-weather-bot-b4065c

FIELD      VALUE
APP_ID     app-weather-bot-b4065c
NAME       weather-bot
CLIENT_ID  wb-0753894ae326
STATUS     active
SCOPES     read:weather:*, write:logs:*
CREATED    2026-03-04T03:06:07Z
UPDATED    2026-03-04T03:06:07Z
```

### Get nonexistent app

```
$ ./bin/aactl app get nonexistent-app-id

Error: HTTP 404: {"type":"urn:agentauth:error:not_found","title":"Not Found","status":404,
  "detail":"app not found","instance":"/v1/admin/apps/nonexistent-app-id"}
```

## Acceptance criteria check

| Criteria | Result |
|----------|--------|
| Response includes app_id, name, client_id, scopes, status, created_at, updated_at | PASS |
| Timestamps in RFC3339 format | PASS — `2026-03-04T03:06:07Z` |
| client_secret_hash not in response | PASS |
| Clear error for unknown app_id | PASS — HTTP 404 with RFC 7807 body |

## Verdict

PASS — Full details returned. Timestamps in RFC3339. No secret hash exposed. Clear 404 for unknown app.
