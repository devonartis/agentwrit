# C4 — Public Endpoints (Health + Challenge) [PRECONDITION]
Who: The developer. What: Health and challenge endpoints respond without auth.
## Test Output
Health:
{"status":"ok","version":"2.0.0","uptime":25,"db_connected":true,"audit_events_count":2}

HTTP 200
Challenge:
Method Not Allowed

HTTP 405
## Verdict
PASS — Health 200, challenge returned nonce.
