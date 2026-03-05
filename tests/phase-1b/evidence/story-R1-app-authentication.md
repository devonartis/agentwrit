# P1B-R1 — Developer Authenticates with App Credentials (Regression)

Who: The developer.

What: The developer logs in to the broker using the app credentials the operator gave
them. This is a regression test from Phase 1A — app authentication must still work
after Phase 1B changes. The developer sends their client_id and client_secret to
the broker and gets back a JWT. The JWT should carry the app-level scopes
(app:launch-tokens:*, app:agents:*, app:audit:read) and the subject should be
"app:<app_id>".

Why: If app authentication broke during Phase 1B, developers can't do anything —
no login means no launch tokens, no agent registration, nothing.

How to run: Source the environment file. Send a POST to /v1/app/auth with the
weather-bot's client_id and client_secret. Check that the response is 200 with a
JWT, and that the JWT carries the correct scopes and subject.

Expected: HTTP 200 with access_token. Scopes include app:launch-tokens:*,
app:agents:*, app:audit:read. Subject is app:<app_id>.

## Test Output

{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhcHA6YXBwLXdlYXRoZXItYm90LWZmZmFkMCIsImV4cCI6MTc3MjY3ODk2MSwibmJmIjoxNzcyNjc4NjYxLCJpYXQiOjE3NzI2Nzg2NjEsImp0aSI6Ijk5ZjIwZGJmNWVlZjJmMjI5MWZhMjM5MzdlNDAxMzdiIiwic2NvcGUiOlsiYXBwOmxhdW5jaC10b2tlbnM6KiIsImFwcDphZ2VudHM6KiIsImFwcDphdWRpdDpyZWFkIl19.fTj40XQgGAl7J4bVmqEGhPWYcLpTjkJLsFLY-Rau9yjfJlhoznfvYH5SrESH4I3jSwcngxfavfGeY0_ZgrrvAQ","expires_in":300,"token_type":"Bearer","scopes":["app:launch-tokens:*","app:agents:*","app:audit:read"]}

HTTP 200

## Verdict

PASS — App authentication returned HTTP 200. JWT carries scopes [app:launch-tokens:*, app:agents:*, app:audit:read]. The sub claim in the JWT is "app:app-weather-bot-fffad0". No regression — app auth works after Phase 1B.
