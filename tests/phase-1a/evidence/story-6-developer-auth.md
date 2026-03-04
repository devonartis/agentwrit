# Story 6 — Developer authenticates app using credentials received from the operator

## What we did

As the developer, used the `client_id` and `client_secret` that the operator gave us (from Story 1) to authenticate with the broker via the REST API. No admin key, no `aactl` — just `curl`.

## Command and output

```
$ curl -s -X POST http://127.0.0.1:8080/v1/app/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id": "wb-0753894ae326", "client_secret": "fb73ece075af892fe758e1dc03a56a41ae67eb01a40011eb31b38616253671e0"}'

{
  "access_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_in": 300,
  "token_type": "Bearer",
  "scopes": ["app:launch-tokens:*", "app:agents:*", "app:audit:read"]
}
```

## JWT payload (decoded)

```json
{
  "iss": "agentauth",
  "sub": "app:app-weather-bot-b4065c",
  "exp": 1772594203,
  "nbf": 1772593903,
  "iat": 1772593903,
  "jti": "12e13c4f9e67cdd95ac9574c542721ff",
  "scope": ["app:launch-tokens:*", "app:agents:*", "app:audit:read"]
}
```

## Acceptance criteria check

| Criteria | Result |
|----------|--------|
| 200 response with valid JSON | PASS |
| access_token present | PASS |
| expires_in: 300 | PASS |
| token_type: "Bearer" | PASS |
| scopes: app:launch-tokens:*, app:agents:*, app:audit:read | PASS — exactly these 3 |
| JWT sub = app:{app_id} | PASS — `app:app-weather-bot-b4065c` |
| JWT exp ~5 minutes from iat | PASS — `exp - iat = 300` seconds |

## Verdict

PASS — Developer authenticated with credentials from the operator. JWT has correct `app:` scopes, correct subject, 5-minute expiry. No admin key needed.
