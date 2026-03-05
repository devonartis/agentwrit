# TD-006 Regression — Phase 1B Key Stories

Who: The developer and the operator.

What: After TD-006, we verify that app-scoped launch tokens still work:
a developer can authenticate, create a launch token with attenuated scopes,
and the broker rejects scopes outside the app's ceiling. Also verify
admin launch tokens still work independently.

Why: TD-006 changed the app JWT TTL, which could affect downstream operations
that use the app JWT (like creating launch tokens). If the JWT structure or
scopes changed, launch token creation would break.

How to run: Source the environment file. Use the regression-1a app from the
Phase 1A regression. Authenticate, create a launch token, try an out-of-scope
request, and create an admin launch token.

Expected: Launch token creation works. Scope ceiling enforcement works.
Admin tokens unaffected.

## R-1B-S1 — Developer creates a launch token

{
    "launch_token": "a64dd3ff857b20752ced9c1262e3f7164b1132c2e8e992f12a817d8945ecd4da",
    "expires_at": "2026-03-05T18:06:26Z",
    "policy": {
        "allowed_scope": [
            "read:weather:current"
        ],
        "max_ttl": 300
    }
}

## R-1B-S2 — Scope ceiling enforced (out-of-scope rejected)

{
    "type": "urn:agentauth:error:forbidden",
    "title": "Forbidden",
    "status": 403,
    "detail": "requested scopes exceed app ceiling; allowed: [read:weather:* write:logs:*]",
    "instance": "/v1/admin/launch-tokens",
    "error_code": "forbidden",
    "request_id": "7353bf6d2b5bc265"
}

## R-1B-S6 — Admin launch token still works

{
    "launch_token": "12dc53ec53abbf8ee4baa8f81f1781213b8a07275cd8498e578e4114bb76cd26",
    "expires_at": "2026-03-05T18:06:27Z",
    "policy": {
        "allowed_scope": [
            "read:weather:*"
        ],
        "max_ttl": 300
    }
}

## Verdict

PASS — All Phase 1B core functionality works after TD-006. Developer can authenticate and create launch tokens with attenuated scopes. Scope ceiling enforcement rejects out-of-scope requests with 403. Admin launch tokens work independently with no app affiliation. No regressions.
