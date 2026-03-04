# Story 5 — Operator deregisters an app

## What we did

Operator deregistered alert-service. Verified it shows as `inactive` in the list (soft delete). Then, as the developer, tried to authenticate with the deregistered app's credentials.

## Commands and output

### Operator deregisters the app

```
$ ./bin/aactl app remove --id app-alert-service-1c27fc

APP_ID                    STATUS    DEREGISTERED_AT
app-alert-service-1c27fc  inactive  2026-03-04T03:11:12Z
App deregistered. The record is retained; credentials are revoked.
```

### Operator checks list — app still visible as inactive

```
$ ./bin/aactl app list

NAME           APP_ID                    CLIENT_ID        STATUS    SCOPES                                     CREATED
alert-service  app-alert-service-1c27fc  as-b188a0881d44  inactive  read:alerts:*,write:logs:*                 2026-03-04T03:07:16Z
log-agent      app-log-agent-21dd57      la-b728a7a04770  active    write:logs:*                               2026-03-04T03:06:33Z
weather-bot    app-weather-bot-b4065c    wb-0753894ae326  active    read:weather:*,write:logs:*,read:alerts:*  2026-03-04T03:06:07Z
Total: 3
```

### Developer tries to authenticate with deregistered credentials — rejected

```
$ curl -s -X POST http://127.0.0.1:8080/v1/app/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id": "as-b188a0881d44", "client_secret": "5766bc3c..."}'

{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,
  "detail":"Authentication failed","instance":"/v1/app/auth"}
```

## Verdict

PASS — App deregistered, shows `inactive` in list (soft delete), credentials immediately rejected with 401.
