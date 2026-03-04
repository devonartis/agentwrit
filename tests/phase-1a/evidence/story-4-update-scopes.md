# Story 4 — Operator updates an app's scope ceiling

## What we did

Updated weather-bot's scopes to add `read:alerts:*`. Verified with `app get` that scopes changed and `updated_at` advanced.

## Commands and output

### Update scopes

```
$ ./bin/aactl app update --id app-weather-bot-b4065c --scopes "read:weather:*,write:logs:*,read:alerts:*"

APP_ID                  SCOPES                                       UPDATED_AT
app-weather-bot-b4065c  read:weather:*, write:logs:*, read:alerts:*  2026-03-04T03:08:10Z
```

### Verify with get

```
$ ./bin/aactl app get app-weather-bot-b4065c

FIELD      VALUE
APP_ID     app-weather-bot-b4065c
NAME       weather-bot
CLIENT_ID  wb-0753894ae326
STATUS     active
SCOPES     read:weather:*, write:logs:*, read:alerts:*
CREATED    2026-03-04T03:06:07Z
UPDATED    2026-03-04T03:08:10Z
```

`updated_at` changed from `03:06:07Z` to `03:08:10Z`. Scopes now include `read:alerts:*`.

## Verdict

PASS — Scopes updated. Timestamp advanced. Get confirms new scope list.
