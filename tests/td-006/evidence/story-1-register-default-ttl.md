# TD006-S1 — Operator Registers App with Default TTL

Who: The operator.

What: The operator registers a new app called ttl-default on the broker
without specifying a token TTL. The broker should assign the global default
TTL of 1800 seconds (30 minutes). This is the baseline behavior — if an
operator doesn't care about TTL, the app gets a safe default.

Why: If the default TTL isn't applied, apps would either get no TTL (tokens
never expire) or an unexpected value. Both would be security problems.

How to run: Source the environment file. Then run aactl app register with
a name and scopes but no --token-ttl flag. Then run aactl app get to confirm
the stored value matches.

Expected: The app is created with token_ttl: 1800. Both the register response
and the app get response show 1800.

## Test Output — Register

{
  "app_id": "app-ttl-default-deb18b",
  "name": "",
  "client_id": "td-5dc9e9cdc775",
  "client_secret": "464a090cf17e53a9478dbc5051b800ee56000c778890ab635bbdb92f2579a056",
  "scopes": [
    "read:data:*"
  ],
  "token_ttl": 1800,
  "status": ""
}


## Test Output — App Get

{
  "app_id": "app-ttl-default-deb18b",
  "name": "ttl-default",
  "client_id": "td-5dc9e9cdc775",
  "scopes": [
    "read:data:*"
  ],
  "token_ttl": 1800,
  "status": "active",
  "created_at": "2026-03-05T17:33:28Z",
  "updated_at": "2026-03-05T17:33:28Z"
}



## Verdict

PASS — The broker assigned token_ttl: 1800 (the global default) to the app. Both the register response and the app get response confirm the value. No --token-ttl flag was provided, so the default was applied correctly.
