# TD-006 Regression — Phase 1A Key Stories

Who: The operator and the developer.

What: After implementing per-app JWT TTL (TD-006) and fixing the TTL 0/-1
bounds bug, we verify that core Phase 1A functionality still works: app
registration, developer authentication, bad credential handling, and admin
auth. The only expected change from Phase 1A is that expires_in is now
1800 (the new default) instead of 300 (the old hardcoded value).

Why: TD-006 changed how token TTL is resolved — from a hardcoded constant
to a per-app database field with a configurable global default. If anything
in that chain broke, basic app registration or authentication would fail.

How to run: Source the TD-006 environment file. Register an app, authenticate
as a developer, try bad credentials, and verify admin auth still works.

Expected: All operations succeed. Developer auth returns expires_in: 1800
(new default, not the old 300).

## R-1A-S1 — Register an app

{
  "app_id": "app-regression-1a-7dd439",
  "name": "",
  "client_id": "r1-c2e187e66c06",
  "client_secret": "f6e2f173a9f7dcdd52176a4b6182afaea6b9f3d2e56f80541ba4a61d50be818a",
  "scopes": [
    "read:weather:*",
    "write:logs:*"
  ],
  "token_ttl": 1800,
  "status": ""
}


## R-1A-S6 — Developer authenticates with app credentials

{
    "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhcHA6YXBwLXJlZ3Jlc3Npb24tMWEtN2RkNDM5IiwiZXhwIjoxNzcyNzM1NzI1LCJuYmYiOjE3NzI3MzM5MjUsImlhdCI6MTc3MjczMzkyNSwianRpIjoiM2M0ZDc0MjdhMDYzOTI2YTQ0ZmIzZTg1MGEyYmNhM2IiLCJzY29wZSI6WyJhcHA6bGF1bmNoLXRva2VuczoqIiwiYXBwOmFnZW50czoqIiwiYXBwOmF1ZGl0OnJlYWQiXX0.2n43PvKsZdLVoYalJMBJ79JaC6Nr_xZmjXjKw75o0NdFRnaDSVp2DeGe37xI35FBQrxwmfaU6uUA8z6ExeuWBA",
    "expires_in": 1800,
    "token_type": "Bearer",
    "scopes": [
        "app:launch-tokens:*",
        "app:agents:*",
        "app:audit:read"
    ]
}

## R-1A-S7 — Bad credentials return 401

{
    "type": "urn:agentauth:error:unauthorized",
    "title": "Unauthorized",
    "status": 401,
    "detail": "Authentication failed",
    "instance": "/v1/app/auth",
    "error_code": "unauthorized",
    "request_id": "4650fb402d1f0a61"
}

## R-1A-S11 — Admin auth still works

{
    "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhZG1pbiIsImV4cCI6MTc3MjczNDIyNSwibmJmIjoxNzcyNzMzOTI1LCJpYXQiOjE3NzI3MzM5MjUsImp0aSI6IjI1ODZmYmU1ZDZlMGMyOTMyOWMwNGViNDk3MGZmZjI3Iiwic2NvcGUiOlsiYWRtaW46bGF1bmNoLXRva2VuczoqIiwiYWRtaW46cmV2b2tlOioiLCJhZG1pbjphdWRpdDoqIl19.qS8bm-LbbmWohJ3YYdu_EmYxFAoXKYsJsbQ7ea2su3dbmQtHs9ccO2adMrRKk94rBuNCszf7QyCjbm5mYzeLAg",
    "expires_in": 300,
    "token_type": "Bearer"
}

## Verdict

PASS — All Phase 1A core functionality works after TD-006. App registration returns token_ttl: 1800 (new default). Developer auth returns expires_in: 1800 (changed from 300 — this is the intended TD-006 behavior). Bad credentials return 401 with generic message. Admin auth still returns 300s tokens (admin TTL unchanged). No regressions.
