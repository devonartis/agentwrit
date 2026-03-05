# TD006-S2 — Operator Registers App with Custom TTL

Who: The operator (registration) and the developer (authentication).

What: The operator registers a new app called ttl-custom-s2 and sets the token
TTL to 3600 seconds (1 hour) using the --token-ttl flag. This is for apps
that need longer-lived tokens — for example, a long-running data pipeline
that shouldn't have to re-authenticate every 30 minutes. After registration,
a developer authenticates with the app's credentials to prove the issued
JWT actually uses the custom TTL, not the default.

Why: If custom TTLs don't work at registration, operators can't tune token
lifetimes per app. Every app would be stuck with the global default, which
defeats the purpose of the feature. And if the stored TTL doesn't flow
through to the issued JWT, the setting is cosmetic — tokens would still
expire at the wrong time.

How to run: Source the environment file. Run aactl app register with
--token-ttl 3600. Check that the response shows token_ttl: 3600. Then
use curl to POST /v1/app/auth with the app's client_id and client_secret.
Check that the response has expires_in: 3600.

Expected: The register response shows token_ttl: 3600. The developer auth
response shows expires_in: 3600.

## Test Output — Register

{
  "app_id": "app-ttl-custom-s2-5a8397",
  "name": "",
  "client_id": "tcs-c67f93ed098b",
  "client_secret": "d4358ab56eae086c2693a0cad106e8ab889d2bc4c35f46ac434d2929aa3d0840",
  "scopes": [
    "read:data:*"
  ],
  "token_ttl": 3600,
  "status": ""
}


## Test Output — Developer Auth

{
    "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhcHA6YXBwLXR0bC1jdXN0b20tczItNWE4Mzk3IiwiZXhwIjoxNzcyNzM1ODQwLCJuYmYiOjE3NzI3MzIyNDAsImlhdCI6MTc3MjczMjI0MCwianRpIjoiYTkyNzljYzdiNzE1ZmI0YzJmYTNjZjY2MTg1ZTdiNmEiLCJzY29wZSI6WyJhcHA6bGF1bmNoLXRva2VuczoqIiwiYXBwOmFnZW50czoqIiwiYXBwOmF1ZGl0OnJlYWQiXX0.EzOjCprevXlta6uUrPO2uymSw2fpJ6gLfA2LXShS9GqEYaymejKfrnXCaMZdNbpKon7bJ-5bdyHY_N6D0TBLDw",
    "expires_in": 3600,
    "token_type": "Bearer",
    "scopes": [
        "app:launch-tokens:*",
        "app:agents:*",
        "app:audit:read"
    ]
}

## Verdict

PASS — The operator registered an app with --token-ttl 3600. The register response shows token_ttl: 3600. The developer authenticated with the app's credentials and received a JWT with expires_in: 3600. The custom TTL flows from registration through to issued tokens.
