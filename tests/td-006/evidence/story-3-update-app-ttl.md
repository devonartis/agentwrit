# TD006-S3 — Operator Updates Existing App's TTL

Who: The operator.

What: The operator changes the TTL of an existing app without re-registering
it. The app ttl-custom-s2 was registered with a 3600-second TTL. The operator
now updates it to 7200 seconds (2 hours) using aactl app update. After the
update, the operator checks the app details to confirm the new value. Then
a developer authenticates to prove the next token uses the new TTL. Finally,
the operator checks the audit trail for an app_updated event that records
the change.

Why: Operators need to change TTLs on the fly — for example, extending
token lifetime during a long deployment window, or shortening it after a
security incident. If update doesn't work, the only option is to delete
and re-register the app, which means new credentials and downtime.

How to run: Source the environment file. Run aactl app update with the
app_id from S2 and --token-ttl 7200. Then run aactl app get to verify.
Then authenticate as a developer with the same credentials. Then check
audit events for app_updated.

Expected: The update succeeds. App get shows token_ttl: 7200. Developer
auth returns expires_in: 7200. Audit trail has an app_updated event.

## Test Output — Update

{
  "app_id": "app-ttl-custom-s2-5a8397",
  "name": "ttl-custom-s2",
  "client_id": "tcs-c67f93ed098b",
  "scopes": [
    "read:data:*"
  ],
  "token_ttl": 7200,
  "status": "active",
  "created_at": "2026-03-05T17:37:20Z",
  "updated_at": "2026-03-05T17:37:50Z"
}


## Test Output — App Get

{
  "app_id": "app-ttl-custom-s2-5a8397",
  "name": "ttl-custom-s2",
  "client_id": "tcs-c67f93ed098b",
  "scopes": [
    "read:data:*"
  ],
  "token_ttl": 7200,
  "status": "active",
  "created_at": "2026-03-05T17:37:20Z",
  "updated_at": "2026-03-05T17:37:50Z"
}


## Test Output — Developer Auth

{
    "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhcHA6YXBwLXR0bC1jdXN0b20tczItNWE4Mzk3IiwiZXhwIjoxNzcyNzM5NDcwLCJuYmYiOjE3NzI3MzIyNzAsImlhdCI6MTc3MjczMjI3MCwianRpIjoiYzY2MDQ4MDYxOTlmZWRmNjRlNzcxZTFjYTY4ODAxZmMiLCJzY29wZSI6WyJhcHA6bGF1bmNoLXRva2VuczoqIiwiYXBwOmFnZW50czoqIiwiYXBwOmF1ZGl0OnJlYWQiXX0.YcQTjMffGQg_gvNRotKnk5cZIgsdJOP0nq9PrdeYIZGoqX2rdVi9AB7XQzcMqK0jDFWV4HXjIKkh67_Y8rZuDg",
    "expires_in": 7200,
    "token_type": "Bearer",
    "scopes": [
        "app:launch-tokens:*",
        "app:agents:*",
        "app:audit:read"
    ]
}

## Test Output — Audit Trail

ID          TIMESTAMP                       EVENT TYPE   AGENT ID  OUTCOME  DETAIL
evt-000013  2026-03-05T17:37:50.058182335Z  app_updated            success  app_id=app-ttl-custom-s2-5a8397 token_ttl=3600->7200 upda...
Showing 1 of 1 events (offset=0, limit=100)

## Verdict

PASS — The operator updated the app's TTL from 3600 to 7200. App get confirms token_ttl: 7200. Developer auth returns expires_in: 7200, proving the new TTL flows through to issued tokens immediately. The audit trail recorded the change as app_updated with the old and new values (token_ttl=3600->7200).
