# Story 8 — App credentials are scoped, not the master key

## What we did

As a security reviewer, verified that a developer's app JWT cannot access admin endpoints. The app JWT carries only `app:` scopes — it should be rejected by any endpoint requiring `admin:` scopes.

## Commands and output

### Get app JWT (developer auth)

```
$ curl -s -X POST http://127.0.0.1:8080/v1/app/auth \
  -d '{"client_id": "wb-0753894ae326", "client_secret": "fb73ece..."}'

→ access_token with scopes: ["app:launch-tokens:*", "app:agents:*", "app:audit:read"]
```

### Try admin sidecars endpoint with app token

```
$ curl -s -H "Authorization: Bearer $APP_TOKEN" http://127.0.0.1:8080/v1/admin/sidecars

HTTP 403 — {"title":"Forbidden","detail":"token lacks required scope: admin:launch-tokens:*"}
```

### Try admin apps endpoint with app token

```
$ curl -s -H "Authorization: Bearer $APP_TOKEN" http://127.0.0.1:8080/v1/admin/apps

HTTP 403 — {"title":"Forbidden","detail":"token lacks required scope: admin:launch-tokens:*"}
```

## Acceptance criteria check

| Criteria | Result |
|----------|--------|
| App JWT carries app: scopes only — never admin: | PASS |
| App JWT cannot be used on admin endpoints (403) | PASS |
| Master key not required to authenticate as app | PASS — Story 6 used only client_id + client_secret |
| Deregistering one app doesn't affect others | PASS — Story 5 deregistered alert-service, weather-bot still works |
| client_secret_hash never in any API response | PASS — checked in Stories 1-5 |

## Verdict

PASS — App tokens are properly scoped. Admin endpoints reject them with 403.
