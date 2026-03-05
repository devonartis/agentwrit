# P1B-S2 — Developer Is Rejected When Requesting Scopes Outside Ceiling

Who: The developer.

What: The developer tries to create a launch token requesting a scope that is
outside their app's allowed ceiling. The weather-bot app has a ceiling of
"read:weather:*" — meaning it can only read weather data. The developer asks for
"write:data:all" which is a completely different permission. The broker should reject
this because apps can only create launch tokens within their own scope ceiling.

Why: This is the core security boundary of the app model. If a developer could
request scopes outside their ceiling, the scope attenuation system is broken. Any
compromised app could escalate to full broker access.

How to run: Authenticate as the weather-bot app to get a JWT. Then request a launch
token with scope "write:data:all" (outside the ceiling). The broker should return 403.
Then request a valid scope "read:weather:current" to confirm the app isn't locked out
after a rejection.

Expected: The first request returns 403 with an explanation of the ceiling. The
second request with a valid scope still succeeds (201).

## Test Output — Step 1: Request Scope Outside Ceiling

{"type":"urn:agentauth:error:forbidden","title":"Forbidden","status":403,"detail":"requested scopes exceed app ceiling; allowed: [read:weather:*]","instance":"/v1/admin/launch-tokens","error_code":"forbidden","request_id":"2c857bd2bd0f4cac"}

HTTP 403

## Test Output — Step 2: Valid Scope Still Works After Rejection

{"launch_token":"6b106837b28420b0c418db913aca870ecebaffb9c6c4b71419ac0ba8c250a801","expires_at":"2026-03-05T02:27:40Z","policy":{"allowed_scope":["read:weather:current"],"max_ttl":300}}

HTTP 201

## Verdict

PASS — Request for scope outside ceiling (write:data:all) returned 403 with clear explanation: "requested scopes exceed app ceiling; allowed: [read:weather:*]". Follow-up request with valid scope (read:weather:current) returned 201 — app is not locked out after rejection. Scope ceiling enforcement works correctly.
