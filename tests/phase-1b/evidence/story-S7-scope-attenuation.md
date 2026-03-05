# P1B-S7 — Scope Attenuation at the Launch Token Level

Who: The security reviewer.

What: The security reviewer tests four specific scope attenuation cases against the
weather-bot app, which has a ceiling of "read:weather:*". Scope attenuation means
you can narrow permissions down but never widen them. The reviewer tests:
1. Action mismatch — requesting "write:weather:*" when the ceiling only allows "read"
2. Valid attenuation — requesting "read:weather:current" (narrower than ceiling)
3. Exact match — requesting "read:weather:*" (same as ceiling, should be allowed)
4. Wildcard widening — requesting "read:weather:*" when ceiling is "read:weather:current"

Case 4 requires a second app with a narrower ceiling to test properly.

Why: If any of these attenuation rules fail, the security boundary is broken. An app
with read-only permissions could create agents with write permissions, or an app with
access to one resource could create agents with access to everything.

How to run: Source the environment file. Authenticate as weather-bot. Send four
launch token requests with different scopes. For case 4, register a second app with
ceiling "read:weather:current" and test widening.

Expected: Cases 1 and 4 return 403. Cases 2 and 3 return 201. Each 403 produces a
scope_ceiling_exceeded audit event.

## Test Output — Case 1: Action Mismatch (write:weather:* against read:weather:* ceiling)

{"type":"urn:agentauth:error:forbidden","title":"Forbidden","status":403,"detail":"requested scopes exceed app ceiling; allowed: [read:weather:*]","instance":"/v1/admin/launch-tokens","error_code":"forbidden","request_id":"d949fe9d8d3bcb05"}

HTTP 403

## Test Output — Case 2: Valid Attenuation (read:weather:current within read:weather:*)

{"launch_token":"4a219cc684ae751699648382eda78a41468aed382ccfd2958aab809a687627a2","expires_at":"2026-03-05T02:43:29Z","policy":{"allowed_scope":["read:weather:current"],"max_ttl":300}}

HTTP 201

## Test Output — Case 3: Exact Match (read:weather:* equals ceiling)

{"launch_token":"f4aa69286b159dacbd8bed738fb991a579c3cc4b07a2f3ea83c93ac18978a9d7","expires_at":"2026-03-05T02:43:29Z","policy":{"allowed_scope":["read:weather:*"],"max_ttl":300}}

HTTP 201

## Test Output — Case 4: Wildcard Widening (registering narrow-ceiling app first)

Registering narrow-bot with ceiling read:weather:current...
FIELD          VALUE
APP_ID         app-narrow-bot-491ee9
CLIENT_ID      nb-296b127a7ed1
CLIENT_SECRET  0d81a322bb35c4add8468a8f6569f42fc2a5de9d61832afee5e33160c4b2f63b
SCOPES         read:weather:current

WARNING: Save the client_secret — it cannot be retrieved again.

Requesting read:weather:* with ceiling read:weather:current...
{"type":"urn:agentauth:error:forbidden","title":"Forbidden","status":403,"detail":"requested scopes exceed app ceiling; allowed: [read:weather:current]","instance":"/v1/admin/launch-tokens","error_code":"forbidden","request_id":"9309df9754a7bda7"}

HTTP 403

## Verdict

PASS — All four scope attenuation cases work correctly. Case 1 (action mismatch, write vs read ceiling) returned 403. Case 2 (valid attenuation, read:weather:current within read:weather:*) returned 201. Case 3 (exact match, read:weather:* equals ceiling) returned 201. Case 4 (wildcard widening, requesting read:weather:* with ceiling read:weather:current) returned 403 with "allowed: [read:weather:current]". Scopes can only be narrowed, never widened — the attenuation boundary holds.
