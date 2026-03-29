# P1-S5b — Broker Starts with Config File — Container Mode [ACCEPTANCE]

Who: The operator.

What: Same as S5a but in Docker. Config file mounted as read-only volume.
If S5a passes but this fails, the bug is in Docker config, not application.

Why: Proves the deployment path works — config file crosses the filesystem
boundary (host → container volume mount).

How to run: Start Docker container with mounted config dir, authenticate.

Expected: Same behavior as S5a — Health 200, correct secret → 200, wrong → 401.

## Test Output

Starting container with mounted config...
b2b237d4d7ef20a77ad2529b67b2eacd7d308b708aec13d5e050cce165e23a14
Health check:
{
  "status": "ok",
  "version": "2.0.0",
  "uptime": 3,
  "db_connected": true,
  "audit_events_count": 0
}

Auth with correct secret:
{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhZG1pbiIsImF1ZCI6WyJhZ2VudGF1dGgiXSwiZXhwIjoxNzc0Nzk5MDUyLCJuYmYiOjE3NzQ3OTg3NTIsImlhdCI6MTc3NDc5ODc1MiwianRpIjoiOWU2ZjY2YThkNGY1OWE0Y2M5OWRhM2IwOTRkNDQ5NjIiLCJzY29wZSI6WyJhZG1pbjpsYXVuY2gtdG9rZW5zOioiLCJhZG1pbjpyZXZva2U6KiIsImFkbWluOmF1ZGl0OioiXX0.RtZBbTOwerKcpwj-aoik63IPGKFmNknQfNeiQhCuyKYcvtl14VAPjmUrhIb5TpSN6YGx-gPe5GzGTfj2Wb_DAA","expires_in":300,"token_type":"Bearer"}

HTTP 200

Auth with wrong secret:
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"invalid credentials","instance":"/v1/admin/auth","error_code":"unauthorized","request_id":"364ef29a9b163adb"}

HTTP 401aa-p1-container
aa-p1-container


## Verdict

PASS — Container mode: broker started with mounted config (ro volume). Health 200. Correct secret → HTTP 200. Wrong secret → HTTP 401. Identical behavior to VPS mode (S5a).
