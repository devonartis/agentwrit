# A1-S3 — App Cannot Create Launch Tokens via Admin Endpoint [ACCEPTANCE]

**Mode:** VPS

Who: The security reviewer, verifying the boundary between app and admin.

What: The broker has two launch token endpoints: /v1/admin/launch-tokens
(admin only) and /v1/app/launch-tokens (apps only). An app that
authenticates with client credentials should be REJECTED when hitting the
admin endpoint. The app token has scope app:launch-tokens:*, not
admin:launch-tokens:*.

Why: If an app can use admin endpoints, the scope model is broken. Admin
endpoints have no scope ceiling enforcement — an app reaching them could
create agents with any scope, bypassing the ceiling the admin set.

How to run: Reuse the app from S2. Send the app token to the admin
launch-token endpoint. Check that the broker rejects it.

Expected: 403 or scope violation. The app token must NOT work on the
admin endpoint.

## Test Output

--- Step 1: Admin authenticates ---
Admin token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

--- Step 2: Register app and authenticate as app ---
App token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

--- Step 3: App tries ADMIN launch-token endpoint ---
{"type":"urn:agentauth:error:insufficient_scope","title":"Forbidden","status":403,"detail":"token lacks required scope: admin:launch-tokens:*","instance":"/v1/admin/launch-tokens","error_code":"insufficient_scope","request_id":"7f76049e9c3daddf"}

HTTP_STATUS:403

--- Step 4: App tries APP launch-token endpoint (should work) ---
{"launch_token":"7a9dde7de3d66cb99ee2ae62710883200618736bb701c239bd5d5cf96584eaf5","expires_at":"2026-03-30T18:16:55Z","policy":{"allowed_scope":["read:data:*"],"max_ttl":120}}

HTTP_STATUS:201

## Verdict

PASS — App token was rejected (403 insufficient_scope: "token lacks required scope: admin:launch-tokens:*") on the admin endpoint. Same app token succeeded (201) on the app endpoint. The scope boundary between admin and app is enforced.
