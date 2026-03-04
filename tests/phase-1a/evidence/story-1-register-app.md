# Story 1 — Operator registers a new app and receives credentials

## What we did

Registered a new app called `weather-bot` with scopes `read:weather:*,write:logs:*`.

## Command and output

```
$ source ./tests/phase-1a-env.sh && ./bin/aactl app register --name weather-bot --scopes "read:weather:*,write:logs:*"

FIELD          VALUE
APP_ID         app-weather-bot-b4065c
CLIENT_ID      wb-0753894ae326
CLIENT_SECRET  fb73ece075af892fe758e1dc03a56a41ae67eb01a40011eb31b38616253671e0
SCOPES         read:weather:*, write:logs:*

WARNING: Save the client_secret — it cannot be retrieved again.
```

## Credentials (saved for use in later stories)

- **app_id:** `app-weather-bot-b4065c`
- **client_id:** `wb-0753894ae326`
- **client_secret:** `fb73ece075af892fe758e1dc03a56a41ae67eb01a40011eb31b38616253671e0`

These are what the operator hands to the 3rd party developer.

## Acceptance criteria check

| Criteria | Result |
|----------|--------|
| Response includes app_id, client_id, client_secret, scopes | PASS |
| app_id format `app-weather-bot-{6hex}` | PASS — `app-weather-bot-b4065c` |
| client_id format `{2-3 abbrev}-{12hex}` | PASS — `wb-0753894ae326` |
| client_secret is 64-char hex string | PASS — 64 hex chars |
| `sk_live_` prefix on client_secret | FAIL — no prefix, plain hex |
| CLI warns about saving secret | PASS |
| client_secret_hash not in response | PASS |
| Audit event app_registered recorded | TO VERIFY in Story 10 |

## Verdict

PARTIAL PASS — All criteria met except `sk_live_` prefix on client_secret. Open decision: fix implementation or update criteria.
