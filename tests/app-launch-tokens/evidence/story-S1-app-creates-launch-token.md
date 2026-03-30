# ALT-S1 — App Creates Launch Token on App Route

Who: The developer's application.

What: The app authenticates with the broker using its client credentials,
then creates a launch token for one of its agents using the dedicated app
route (POST /v1/app/launch-tokens). This is the normal production path.
Apps manage their own agents within their scope ceiling.

Why: If the app can't create launch tokens through its own route, the entire
app-driven agent lifecycle is broken. Every app would need an operator to
manually create launch tokens, which doesn't scale.

How to run: Authenticate as admin. Register an app with a scope ceiling.
Authenticate as the app. Create a launch token via the app route.

Expected: App auth returns 200. Launch token creation returns 201.

## Test Output

Admin token: eyJhbGciOiJFZERTQSIs...

--- Register app ---
{
  "app_id": "app-test-pipeline-8d276e",
  "name": "",
  "client_id": "tp-4e2401785ac1",
  "client_secret": "c4c17d4cc39188d9850205e085b6daac9fdb9072b1df5e16884fbbd00000999f",
  "scopes": [
    "read:data:*"
  ],
  "token_ttl": 1800,
  "status": ""
}

--- App auth ---
App token: eyJhbGciOiJFZERTQSIs...

--- POST /v1/app/launch-tokens ---
{"launch_token":"a6e1faa3061e4731daba2085c5a54e39b432dafd079091c62d1752cfdd6e77d3","expires_at":"2026-03-30T12:54:48Z","policy":{"allowed_scope":["read:data:*"],"max_ttl":300}}

HTTP 201

## Verdict

PASS — App authenticated (200), registered app with scope ceiling read:data:*. App created launch token via POST /v1/app/launch-tokens — returned 201 with launch token and policy showing allowed_scope within ceiling.
