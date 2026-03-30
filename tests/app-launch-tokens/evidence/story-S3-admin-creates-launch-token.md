# ALT-S3 — Admin Creates Launch Token on Admin Route

Who: The operator.

What: The operator authenticates with the admin secret and creates a launch
token using the admin route (POST /v1/admin/launch-tokens). This is the
platform management path — used for broker bootstrap, initial app setup,
dev/testing environments, and emergency break-glass scenarios.

Why: Without the admin path, there's no way to bootstrap the system. Before
any apps are registered, someone needs to create the first launch token so
the first agent can register. The operator is that someone.

How to run: Authenticate as admin. Create a launch token via the admin route.

Expected: HTTP 201 with a launch token.

## Test Output

Admin token: eyJhbGciOiJFZERTQSIs...

--- POST /v1/admin/launch-tokens with admin token ---
{"launch_token":"040c5f506b7ac8f10e42e160ede55f2e514bb68cbcddf8d783fd29cd95843ec6","expires_at":"2026-03-30T13:38:30Z","policy":{"allowed_scope":["read:data:*","write:logs:*"],"max_ttl":600}}

HTTP 201

## Verdict

PASS — Admin token created launch token via POST /v1/admin/launch-tokens — returned 201 with launch token, policy showing allowed_scope and max_ttl. Platform bootstrap path works.
