# Story 9 — Per-app rate limiting on the auth endpoint

## What we did

As a security reviewer, fired rapid auth requests for one client_id to trigger the rate limiter. Verified the limit is per-client_id by confirming a different app is unaffected.

## Commands and output

### Rapid requests for weather-bot (client_id: wb-0753894ae326)

```
$ for i in $(seq 1 12); do ... done

Request 1: HTTP 200
Request 2: HTTP 200
Request 3: HTTP 200
Request 4: HTTP 429
Request 5: HTTP 429
...
Request 12: HTTP 429
```

Rate limit triggers after burst of 3 (requests 1-3 succeed, request 4 onward blocked).

### 429 response headers

```
HTTP/1.1 429 Too Many Requests
Content-Type: application/problem+json
Retry-After: 60

{"type":"urn:agentauth:error:rate_limited","title":"Too Many Requests","status":429,
  "detail":"rate limit exceeded, try again later"}
```

### Different client_id (log-agent) is NOT affected

```
$ curl ... -d '{"client_id": "la-b728a7a04770", "client_secret": "8aac046f..."}'

HTTP 200
```

## Acceptance criteria check

| Criteria | Result |
|----------|--------|
| 11+ rapid requests for same client_id triggers 429 | PASS — triggers at request 4 (burst 3, 10 req/min) |
| 429 response includes Retry-After header | PASS — `Retry-After: 60` |
| Rate-limited app does NOT affect different client_id | PASS — log-agent returns 200 |

## Note

The story says "11+ rapid requests" triggers 429, but the implementation uses burst=3, rate=10/min. Request 4 gets 429. The rate limit is stricter than the story implies — this is more secure, not less. The criteria should be updated to say "4+ rapid requests" or the burst/rate should be adjusted.

## Verdict

PASS — Per-client_id rate limiting works. Retry-After present. Other apps unaffected.
