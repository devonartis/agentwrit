# Story R2 — App JWT cannot access admin-only endpoints (Phase 1a regression)

## Purpose

Verify that an app-issued JWT is blocked from accessing admin-only endpoints. This
confirms that Phase 1b's changes to the launch token endpoint (which now accepts both
admin and app JWTs) did not accidentally open other admin endpoints to app callers.

## Preconditions

- Broker running in Docker (`./scripts/stack_up.sh`)
- App `weather-bot` registered by operator with ceiling `["read:weather:*"]`
- Developer has authenticated and holds a valid app JWT

## How to reproduce

### Step 1: Get an app JWT

Authenticate as the developer using the app credentials the operator provided.

```bash
source tests/phase-1b/env.sh

APP_JWT=$(curl -s -X POST http://127.0.0.1:8080/v1/app/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id": "<client_id>", "client_secret": "<client_secret>"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
```

The app JWT carries scopes `["app:launch-tokens:*", "app:agents:*", "app:audit:read"]`.
None of these are admin scopes. Admin endpoints require scopes starting with `admin:`.

### Step 2: Try to list all apps (admin-only endpoint)

This endpoint lists every registered app. Only operators with admin scope should see this.
An app developer should not be able to enumerate other apps.

```bash
curl -s http://127.0.0.1:8080/v1/admin/apps \
  -H "Authorization: Bearer $APP_JWT"
```

**Expected:** HTTP 403 with `insufficient_scope` error.

**Actual:**
```json
{
  "type": "urn:agentauth:error:insufficient_scope",
  "title": "Forbidden",
  "status": 403,
  "detail": "token lacks required scope: admin:launch-tokens:*",
  "instance": "/v1/admin/apps",
  "error_code": "insufficient_scope",
  "request_id": "655b443ff747ec55"
}
```

HTTP Status: **403**

### Step 3: Try to deregister the app (admin-only endpoint)

This is a destructive action — deregistering an app. An app should never be able to
deregister itself or other apps. Only the operator should have this power.

```bash
curl -s -X DELETE http://127.0.0.1:8080/v1/admin/apps/app-weather-bot-cbd117 \
  -H "Authorization: Bearer $APP_JWT"
```

**Expected:** HTTP 403 with `insufficient_scope` error.

**Actual:**
```json
{
  "type": "urn:agentauth:error:insufficient_scope",
  "title": "Forbidden",
  "status": 403,
  "detail": "token lacks required scope: admin:launch-tokens:*",
  "instance": "/v1/admin/apps/app-weather-bot-cbd117",
  "error_code": "insufficient_scope",
  "request_id": "ea27fe70852f5027"
}
```

HTTP Status: **403**

## Acceptance Criteria

| # | Criterion | Expected | Actual | Result |
|---|-----------|----------|--------|--------|
| 1 | `GET /v1/admin/apps` with app Bearer → 403 | 403 | 403 | PASS |
| 2 | `DELETE /v1/admin/apps/{id}` with app Bearer → 403 | 403 | 403 | PASS |

## Verdict

**PASS** — App JWTs are correctly blocked from admin-only endpoints. The broker returns
clear `insufficient_scope` errors with RFC 7807 problem details. Phase 1b's changes to
the launch token endpoint did not weaken access control on other admin routes.
