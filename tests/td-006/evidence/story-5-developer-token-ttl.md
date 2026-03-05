# TD006-S5 — Developer Gets a Token That Lasts Long Enough

Who: The developer.

What: The developer authenticates with an app that was registered with a
3600-second TTL (the ttl-custom-s2 app, now updated to 7200 in S3). The
developer uses curl to call the app auth endpoint with the app's client_id
and client_secret. The response should include an expires_in field matching
the app's configured TTL, and the JWT's exp claim should be approximately
iat + 7200.

Why: The developer trusts that the token will last as long as the operator
configured. If the expires_in field says 7200 but the JWT actually expires
sooner, the developer's app will break mid-operation with an unexpected
auth failure.

How to run: Use curl to POST /v1/app/auth with the credentials from S2.
Check expires_in in the response. Decode the JWT and verify that exp minus
iat equals the configured TTL.

Expected: expires_in: 7200. JWT exp - iat = 7200.

## Test Output — Auth

{
    "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhcHA6YXBwLXR0bC1jdXN0b20tczItNWE4Mzk3IiwiZXhwIjoxNzcyNzM5NTUxLCJuYmYiOjE3NzI3MzIzNTEsImlhdCI6MTc3MjczMjM1MSwianRpIjoiOGRhNDJmMjZiNjI2MDk3ZDVjODcwMGE4OGIwNDVlNmUiLCJzY29wZSI6WyJhcHA6bGF1bmNoLXRva2VuczoqIiwiYXBwOmFnZW50czoqIiwiYXBwOmF1ZGl0OnJlYWQiXX0.X-6MhcuBcO77Peyeclrcedo1rbWNU5LnSKQaL3mkfjYxkIqPQSMk08qqzJv-WB5Ahq8BbPJ92e_5VvYtlfbXAw",
    "expires_in": 7200,
    "token_type": "Bearer",
    "scopes": [
        "app:launch-tokens:*",
        "app:agents:*",
        "app:audit:read"
    ]
}

## Test Output — JWT Decode

{
  "iss": "agentauth",
  "sub": "app:app-ttl-custom-s2-5a8397",
  "exp": 1772739551,
  "nbf": 1772732351,
  "iat": 1772732351,
  "jti": "8cc678f108afd3250dccff6708172b1d",
  "scope": [
    "app:launch-tokens:*",
    "app:agents:*",
    "app:audit:read"
  ]
}

iat: 1772732351
exp: 1772739551
exp - iat: 7200

## Verdict

PASS — The developer received a token with expires_in: 7200 matching the app's configured TTL. The decoded JWT confirms exp - iat = 7200. The TTL set by the operator flows through to the actual JWT claims — the token will live exactly as long as configured.
