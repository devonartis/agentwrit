# Story 7 — Developer receives clear error on bad credentials

## What we did

As the developer, tried three failure scenarios: wrong secret, unknown client_id, and deregistered app. All should return the same generic 401 — no way to tell which field is wrong.

## Commands and output

### Wrong client_secret

```
$ curl -s -X POST http://127.0.0.1:8080/v1/app/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id": "wb-0753894ae326", "client_secret": "wrong_secret_here..."}'

{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,
  "detail":"Authentication failed","instance":"/v1/app/auth"}
```

### Unknown client_id

```
$ curl -s -X POST http://127.0.0.1:8080/v1/app/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id": "nonexistent-id-000", "client_secret": "doesnt_matter..."}'

{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,
  "detail":"Authentication failed","instance":"/v1/app/auth"}
```

### Deregistered app (alert-service from Story 5)

```
$ curl -s -X POST http://127.0.0.1:8080/v1/app/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id": "as-b188a0881d44", "client_secret": "5766bc3c..."}'

{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,
  "detail":"Authentication failed","instance":"/v1/app/auth"}
```

## Acceptance criteria check

| Criteria | Result |
|----------|--------|
| Wrong secret → 401 with RFC 7807, "Authentication failed" | PASS |
| Unknown client_id → 401 with same generic message | PASS — identical response, no enumeration |
| Deregistered app → 401 with same generic message | PASS |
| No way to distinguish which field is wrong | PASS — all three return identical body |

## Verdict

PASS — All failure modes return the same generic 401. No credential enumeration possible.
