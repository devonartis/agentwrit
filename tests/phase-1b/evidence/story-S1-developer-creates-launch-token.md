# P1B-S1 — Developer Creates a Launch Token Using App Credentials

Who: The developer.

What: The developer has received app credentials (client_id and client_secret) from
the operator for the weather-bot app. The developer first authenticates with the
broker to get an app JWT, then uses that JWT to create a launch token for their
agent. This is the core Phase 1B feature — before this, only the operator could
create launch tokens. Now the developer can do it themselves, scoped to their app's
permissions.

Why: If this doesn't work, developers can't self-serve launch tokens and must ask the
operator every time they want to register a new agent. That defeats the purpose of
the app model.

How to run: First POST to /v1/app/auth with the app's client_id and client_secret to
get an app JWT. Then POST to /v1/admin/launch-tokens with that JWT as the Bearer
token, requesting a launch token with scope "read:weather:current" (which is within
the app's ceiling of "read:weather:*").

Expected: The app auth call returns 200 with a JWT. The launch token call returns 201
with a launch_token, expires_at, and policy.allowed_scope.

## Test Output — Step 1: App Authentication

{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhcHA6YXBwLXdlYXRoZXItYm90LWZmZmFkMCIsImV4cCI6MTc3MjY3Nzg5MSwibmJmIjoxNzcyNjc3NTkxLCJpYXQiOjE3NzI2Nzc1OTEsImp0aSI6ImRiZjQ5NjFmZTAxMjhjNGE5ZDM0NTJhYWMwZTkzNGEwIiwic2NvcGUiOlsiYXBwOmxhdW5jaC10b2tlbnM6KiIsImFwcDphZ2VudHM6KiIsImFwcDphdWRpdDpyZWFkIl19.BOsmg1Yy1EiNoRZNxrFFibf1EcIf_vfCM2Bn4k0BomvOPTJpQrQ3AgvgL18rQxnUJOrIGVt7XpESoxg0kwBvCA","expires_in":300,"token_type":"Bearer","scopes":["app:launch-tokens:*","app:agents:*","app:audit:read"]}

HTTP 200

## Test Output — Step 2: Create Launch Token

{"launch_token":"378d089fbd38fb79a6ed3939f24b3934f514abc26b062b0f6f92a075c2ffbb40","expires_at":"2026-03-05T02:27:13Z","policy":{"allowed_scope":["read:weather:current"],"max_ttl":300}}

HTTP 201

## Verdict

PASS — App auth returned 200 with JWT carrying app scopes. Launch token creation returned 201 with launch_token, expires_at, and policy.allowed_scope=["read:weather:current"]. The developer can self-serve launch tokens scoped within their app ceiling.
