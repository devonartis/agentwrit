# P1B-S4 — Operator Traces Launch Token Back to App

Who: The operator.

What: The operator checks the audit trail to see which app created a launch token.
In Phase 1B, developers can create their own launch tokens using app credentials.
The operator needs to verify that when a developer creates a launch token, the audit
event records which app did it. The operator also checks that admin-created launch
tokens do NOT have an app_id — because the admin isn't an app.

Why: If the operator can't trace a launch token back to an app, they lose visibility
into who is provisioning agents. This is a compliance and security requirement —
the operator must know which app is responsible for each agent in their system.

How to run: Pull audit events filtered by launch_token_issued via the API. Check
that app-created tokens have app_id and created_by=app:<app_id>. Then create an
admin launch token via the API and verify it has created_by=admin with no app_id.

Expected: App-created launch tokens show app_id and created_by=app:<app_id>. Admin-
created launch tokens show created_by=admin with no app_id.

## Test Output — Step 1: App-Created Launch Token Audit Events

{
  "id": "evt-000010",
  "timestamp": "2026-03-05T02:26:43.713494013Z",
  "event_type": "launch_token_issued",
  "detail": "launch token issued for agent=fetcher scope=[read:weather:current] max_ttl=300 created_by=app:app-weather-bot-fffad0 app_id=app-weather-bot-fffad0",
  "outcome": "success",
  "hash": "cc1c962aa7b4022062d44d3fecd37b4b34da6224775f31eafcf42b1ea121a3c6",
  "prev_hash": "a2f326015ab9ca6cdef81641b08caa8ea2a8ded9968965ede14ab3b0ca1ae4d4"
}

## Test Output — Step 2: Admin-Created Launch Token Audit Event

{
  "id": "evt-000033",
  "timestamp": "2026-03-05T02:36:57.446324546Z",
  "event_type": "launch_token_issued",
  "detail": "launch token issued for agent=admin-agent scope=[admin:*] max_ttl=300 created_by=admin",
  "outcome": "success",
  "hash": "aa3846b6f7b99e75b831428da7fc30fbdf368bc796a8a3e091ace2441bc92f08",
  "prev_hash": "cde7c844a408cb1fefef8022926c52e1435c8719cd93cd9de576396932ec17d9"
}


## Verdict

PASS — App-created launch token (evt-000010) shows "created_by=app:app-weather-bot-fffad0 app_id=app-weather-bot-fffad0" — full traceability to the originating app. Admin-created launch token (evt-000033) shows "created_by=admin" with no app_id — correctly distinguishes admin vs app origin.
