# P1B-R2 — App JWT Cannot Access Admin-Only Endpoints (Regression)

Who: The security reviewer.

What: The security reviewer confirms that a developer with an app JWT cannot access
admin-only endpoints. This is a regression test from Phase 1A — the scope boundary
between app and admin must still hold after Phase 1B changes. The reviewer tries to
access GET /v1/admin/apps (list all apps) and GET /v1/admin/sidecars (list all
sidecars) using an app Bearer token. Both should be rejected.

Why: If an app JWT can access admin endpoints, any developer could list all apps,
see other apps' configurations, or access operator-level management functions. That
would be a privilege escalation vulnerability.

How to run: Source the environment file. Authenticate as the weather-bot app to get
a JWT. Then try to access the two admin endpoints with that JWT.

Expected: Both requests return HTTP 403 (Forbidden).

## Test Output — GET /v1/admin/apps with app JWT

{"type":"urn:agentauth:error:insufficient_scope","title":"Forbidden","status":403,"detail":"token lacks required scope: admin:launch-tokens:*","instance":"/v1/admin/apps","error_code":"insufficient_scope","request_id":"3e4cf17bded622b9"}

HTTP 403

## Test Output — GET /v1/admin/sidecars with app JWT

404 page not found

HTTP 404

## Verdict

PASS — App JWT correctly blocked from admin endpoints. GET /v1/admin/apps returned 403 with "token lacks required scope: admin:launch-tokens:*". GET /v1/admin/sidecars returned 404 (the sidecar routes were removed in Phase 0, so the endpoint no longer exists — which is also correct). No privilege escalation possible with an app JWT.
