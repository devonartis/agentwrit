# N2 — Invalid Token Rejected by Validate [ACCEPTANCE]
Who: Security reviewer. What: Garbage token gets rejected. Expected: valid=false or error.
## Test Output
{"valid":false,"error":"signature verification failed"}

HTTP 200
## Verdict
PASS — Invalid token rejected.
