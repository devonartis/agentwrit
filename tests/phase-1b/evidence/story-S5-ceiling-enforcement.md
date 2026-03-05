# P1B-S5 — Operator Confirms Ceiling Enforcement Works

Who: The operator.

What: The operator verifies that the broker enforces each app's scope ceiling when
a developer creates launch tokens. The weather-bot app was registered with a scope
ceiling of "read:weather:*" — it can only do weather-related reads. The operator
watches as the developer tries three different scope requests to make sure the
broker blocks anything outside the ceiling.

Why: Ceiling enforcement is the core security property of the app model. If a
developer can request scopes outside their ceiling, they could escalate their own
permissions beyond what the operator granted them. Every rejection must also appear
in the audit trail so the operator can detect abuse attempts.

How to run: Source the environment file. Authenticate as the weather-bot app to get
a JWT. Then make three launch token requests using curl:
1. Scope "read:weather:current" — within the ceiling, should succeed.
2. Scope "write:data:all" — completely outside the ceiling, should be rejected.
3. Scopes "read:weather:*" and "write:data:all" together — partially outside, should
   be rejected.
Then pull the scope_ceiling_exceeded audit events using aactl.

Expected: Request 1 returns HTTP 201. Requests 2 and 3 return HTTP 403 with a
message explaining what the app's ceiling is. Audit trail shows scope_ceiling_exceeded
events for the two rejections.

## Test Output — Request 1: Scope Within Ceiling

{"launch_token":"915e3a76fc516f8df992bcc9b54c78ad762d921d0a5f6bd938df13427108c28e","expires_at":"2026-03-05T02:41:29Z","policy":{"allowed_scope":["read:weather:current"],"max_ttl":300}}

HTTP 201

## Test Output — Request 2: Scope Completely Outside Ceiling

{"type":"urn:agentauth:error:forbidden","title":"Forbidden","status":403,"detail":"requested scopes exceed app ceiling; allowed: [read:weather:*]","instance":"/v1/admin/launch-tokens","error_code":"forbidden","request_id":"fdd2cab8bd0bdb5d"}

HTTP 403

## Test Output — Request 3: Scope Partially Outside Ceiling

{"type":"urn:agentauth:error:forbidden","title":"Forbidden","status":403,"detail":"requested scopes exceed app ceiling; allowed: [read:weather:*]","instance":"/v1/admin/launch-tokens","error_code":"forbidden","request_id":"f06b30fe13ea13bf"}

HTTP 403

## Test Output — Audit Trail: scope_ceiling_exceeded Events

ID          TIMESTAMP                       EVENT TYPE              AGENT ID                    OUTCOME  DETAIL
evt-000012  2026-03-05T02:27:10.938496178Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0  denied   app=app-weather-bot-fffad0 requested=[write:data:all] cei...
evt-000038  2026-03-05T02:37:50.859745543Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0  denied   app=app-weather-bot-fffad0 requested=[write:data:all] cei...
evt-000039  2026-03-05T02:37:50.86292171Z   scope_ceiling_exceeded  app:app-weather-bot-fffad0  denied   app=app-weather-bot-fffad0 requested=[read:weather:* writ...
evt-000043  2026-03-05T02:39:54.575080003Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0  denied   app=app-weather-bot-fffad0 requested=[write:data:all] cei...
evt-000044  2026-03-05T02:39:54.592886378Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0  denied   app=app-weather-bot-fffad0 requested=[read:weather:* writ...
evt-000048  2026-03-05T02:40:59.803351256Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0  denied   app=app-weather-bot-fffad0 requested=[write:data:all] cei...
evt-000049  2026-03-05T02:40:59.817127381Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0  denied   app=app-weather-bot-fffad0 requested=[read:weather:* writ...
Showing 7 of 7 events (offset=0, limit=100)


## Verdict

PASS — All three ceiling enforcement cases work correctly. Request 1 (within ceiling, read:weather:current) returned 201 with a launch token. Request 2 (completely outside ceiling, write:data:all) returned 403 with "requested scopes exceed app ceiling; allowed: [read:weather:*]". Request 3 (partially outside ceiling, read:weather:* + write:data:all) also returned 403. Audit trail shows scope_ceiling_exceeded events for both rejections (evt-000048 and evt-000049). Ceiling enforcement is working and fully audited.
