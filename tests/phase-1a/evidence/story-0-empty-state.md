# Story 0 — Broker starts clean with no apps

## What we did

Started the broker-only Docker stack (no sidecar). Sourced the test environment. Ran `aactl app list` to check the initial state.

## Commands and output

### List apps (table)

```
$ source ./tests/phase-1a-env.sh && ./bin/aactl app list

NAME  APP_ID  CLIENT_ID  STATUS  SCOPES  CREATED
Total: 0
```

### List apps (JSON)

```
$ source ./tests/phase-1a-env.sh && ./bin/aactl app list --json

{
  "apps": [],
  "total": 0
}
```

## Verdict

PASS — Broker starts with an empty app registry. Table shows no rows. JSON returns empty array with `total: 0`.
