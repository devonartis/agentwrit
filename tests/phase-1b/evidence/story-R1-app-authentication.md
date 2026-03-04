# Story R1 — Developer authenticates with app credentials (Phase 1a regression)

## Purpose

Verify that Phase 1a app authentication still works after Phase 1b changes. A developer
with valid `client_id` and `client_secret` should be able to authenticate and receive a
JWT with the correct app-level scopes.

## Preconditions

- Broker running in Docker (`./scripts/stack_up.sh`)
- App `weather-bot` registered by operator with scope ceiling `["read:weather:*"]`
- Developer has been given `client_id` and `client_secret` by the operator

## Steps

### Step 1: Authenticate as app developer

**What this does:** Sends the app's credentials to the broker's app auth endpoint.
The broker validates the credentials and returns a short-lived JWT.

```bash
curl -s -X POST http://127.0.0.1:8080/v1/app/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id": "wb-009efbc75c6a", "client_secret": "<secret>"}'
```

**HTTP Status:** 200

**Response:**
```json
{
    "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...",
    "expires_in": 300,
    "token_type": "Bearer",
    "scopes": [
        "app:launch-tokens:*",
        "app:agents:*",
        "app:audit:read"
    ]
}
```

### Step 2: Decode the JWT to verify claims

**What this does:** Decodes the JWT payload (base64) to inspect the claims the broker
embedded. We're checking that the `sub` field identifies this as an app token and that
the scopes match what Phase 1a defined.

**JWT Payload:**
```json
{
  "iss": "agentauth",
  "sub": "app:app-weather-bot-cbd117",
  "exp": 1772619732,
  "nbf": 1772619432,
  "iat": 1772619432,
  "jti": "bf53e013f9eae22b639ea9d138e4ab56",
  "scope": [
    "app:launch-tokens:*",
    "app:agents:*",
    "app:audit:read"
  ]
}
```

## Acceptance Criteria

| # | Criterion | Expected | Actual | Result |
|---|-----------|----------|--------|--------|
| 1 | POST /v1/app/auth returns 200 | HTTP 200 | HTTP 200 | PASS |
| 2 | JWT carries correct scopes | `["app:launch-tokens:*", "app:agents:*", "app:audit:read"]` | Matches exactly | PASS |
| 3 | JWT `sub` is `app:<app_id>` | `app:app-weather-bot-cbd117` | `app:app-weather-bot-cbd117` | PASS |

## Verdict

**PASS** — App authentication works identically to Phase 1a. JWT contains correct
subject, scopes, and expiry (5 minutes).
