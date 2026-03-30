# ALT-S2 — App Cannot Call Admin Launch Token Route

Who: The security reviewer.

What: The reviewer verifies that an app token is rejected when it tries to
call the admin launch token endpoint (POST /v1/admin/launch-tokens). Before
this fix, both admin and app tokens hit the same endpoint. Now they're
separated — the admin route only accepts admin:launch-tokens:* scope.

Why: If an app can call the admin route, the separation is meaningless. The
admin route has no scope ceiling enforcement — an app could bypass its own
ceiling by calling the admin endpoint instead of the app endpoint. This is a
privilege escalation.

How to run: Use an app token. Try to create a launch token via the admin
route instead of the app route.

Expected: HTTP 403 — insufficient scope.

## Test Output

App token: eyJhbGciOiJFZERTQSIs...

--- POST /v1/admin/launch-tokens with app token (should be 403) ---
{"type":"urn:agentauth:error:insufficient_scope","title":"Forbidden","status":403,"detail":"token lacks required scope: admin:launch-tokens:*","instance":"/v1/admin/launch-tokens","error_code":"insufficient_scope","request_id":"c22d364ec8c1a602"}

HTTP 403

## Verdict

PASS — App token correctly rejected with 403 on POST /v1/admin/launch-tokens. Error: 'token lacks required scope: admin:launch-tokens:*'. Route separation enforced — app cannot bypass scope ceiling via admin endpoint.
