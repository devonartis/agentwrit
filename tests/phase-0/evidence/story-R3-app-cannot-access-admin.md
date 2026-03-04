# P0-R3 — App JWT Cannot Access Admin Endpoints

Who: The security reviewer.

What: The security reviewer takes the app token that the developer got in R2 and tries to access the admin audit trail at GET /v1/audit/events. The app token only has app-level permissions — it does not have admin scopes. The broker should reject this request with a 403 Forbidden.

Why: This is the security boundary between apps and admins. An app should never be able to see admin-only data. If this fails, the Phase 0 cleanup broke the authorization checks.

How to run: Send a GET to /v1/audit/events using the app JWT from R2 in the Authorization header. The broker checks the token's scopes, sees it doesn't have admin permissions, and rejects the request.

Expected: HTTP 403 Forbidden.

## Test Output

{"type":"urn:agentauth:error:insufficient_scope","title":"Forbidden","status":403,"detail":"token lacks required scope: admin:audit:*","instance":"/v1/audit/events","error_code":"insufficient_scope","request_id":"8ad45ec15a943a47"}

HTTP 403

## Verdict

PASS — The broker returned 403 Forbidden. The app token was correctly rejected for the admin audit endpoint. Message: "token lacks required scope: admin:audit:*"
