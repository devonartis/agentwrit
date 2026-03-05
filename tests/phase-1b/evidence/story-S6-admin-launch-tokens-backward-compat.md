# P1B-S6 — Admin Launch Tokens Still Work (Backward Compatible)

Who: The operator.

What: The operator creates a launch token using their admin credentials — the same
way it worked before Phase 1B. Before this phase, only the admin could create launch
tokens. Now that developers can also create them through the app model, the operator
needs to confirm that the old admin flow still works exactly the same. The operator
creates an admin launch token, uses it to register an agent, and checks that no
app_id appears in the audit trail (because the admin is not an app).

Why: If admin-created launch tokens break after Phase 1B, all existing operator
workflows stop working. Backward compatibility is mandatory.

How to run: Source the environment file. Use aactl to authenticate as admin, then
use curl to create a launch token with admin Bearer JWT. Use that launch token to
register an agent (challenge-response with Ed25519). Then check the audit trail to
confirm no app_id appears on the admin-created events.

Expected: Admin launch token creation returns 201. Agent registration with that
token returns 200. Audit events for the admin token and agent show no app_id.

## Test Output — Step 1: Admin Creates Launch Token

{"launch_token":"232335e792b8754d6b3f6ee4990c821236f3df3ba3eae4e57003d7b1afa0efb7","expires_at":"2026-03-05T02:42:49Z","policy":{"allowed_scope":["admin:reports:read"],"max_ttl":300}}

HTTP 201

## Test Output — Step 2: Register Agent with Admin Launch Token

{"agent_id":"spiffe://agentauth.local/agent/admin-orch/admin-task-1/5eec57c0e1fe9b0c","access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJzcGlmZmU6Ly9hZ2VudGF1dGgubG9jYWwvYWdlbnQvYWRtaW4tb3JjaC9hZG1pbi10YXNrLTEvNWVlYzU3YzBlMWZlOWIwYyIsImV4cCI6MTc3MjY3ODg0MCwibmJmIjoxNzcyNjc4NTQwLCJpYXQiOjE3NzI2Nzg1NDAsImp0aSI6IjEyZmUzOWNjNzhlNGI5MThkMGY2MmY4YTI1ZTNhZDRmIiwic2NvcGUiOlsiYWRtaW46cmVwb3J0czpyZWFkIl0sInRhc2tfaWQiOiJhZG1pbi10YXNrLTEiLCJvcmNoX2lkIjoiYWRtaW4tb3JjaCJ9.u2juYT0m9j17b-z-B-5Nfm1XA3cUKfs1LdvA4PFwI2F2I9Ak3hAwReFWIqSspquI-9frpivPBD1mZu7cFYLUBg","expires_in":300}

HTTP 200


## Test Output — Step 3: Audit Trail (admin-created events have no app_id)

evt-000023: Agent registered with scope [read:weather:current] app_id=app-weather-bot-fffad0
evt-000054: Agent registered with scope [admin:reports:read]

evt-000024: Token issued, jti=6e9062b60c4c1157255615a10ddb0955, ttl=300 app_id=app-weather-bot-fffad0
evt-000055: Token issued, jti=12fe39cc78e4b918d0f62f8a25e3ad4f, ttl=300


## Verdict

PASS — Admin launch token creation returned 201. Agent registration with the admin token returned 200 with a SPIFFE agent_id and access_token. Audit trail confirms the distinction: the app-created agent (evt-000023) has "app_id=app-weather-bot-fffad0" in its detail, while the admin-created agent (evt-000054) has no app_id. Same pattern for token_issued — evt-000024 has app_id, evt-000055 does not. Admin flow is fully backward compatible and correctly distinguished from app flow.
