# ALT-S4 — Admin Cannot Call App Launch Token Route

Who: The security reviewer.

What: The reviewer verifies that an admin token is rejected when it tries
to call the app launch token endpoint (POST /v1/app/launch-tokens). The
routes are fully separated — each only accepts its own scope type.

Why: If admin tokens could call the app route, it would bypass the app scope
ceiling enforcement that the app route provides. More importantly, it would
mean the routes aren't truly separated — they're just aliases. True
separation means each route enforces its own authorization boundary.

How to run: Authenticate as admin. Try to create a launch token via the app
route instead of the admin route.

Expected: HTTP 403 — insufficient scope.

## Test Output

Admin token: eyJhbGciOiJFZERTQSIs...

--- POST /v1/app/launch-tokens with admin token (should be 403) ---
{"type":"urn:agentauth:error:insufficient_scope","title":"Forbidden","status":403,"detail":"token lacks required scope: app:launch-tokens:*","instance":"/v1/app/launch-tokens","error_code":"insufficient_scope","request_id":"5775ab03d28168f0"}

HTTP 403

## Verdict

PASS — Admin token correctly rejected with 403 on POST /v1/app/launch-tokens. Error: 'token lacks required scope: app:launch-tokens:*'. Routes are truly separated — admin and app paths enforce independent authorization boundaries.
