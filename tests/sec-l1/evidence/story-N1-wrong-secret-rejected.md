# N1 — Wrong Admin Secret Rejected [ACCEPTANCE]
Who: Security reviewer. What: Wrong secret gets 401. Expected: HTTP 401.
## Test Output
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"invalid credentials","instance":"/v1/admin/auth","error_code":"unauthorized","request_id":"97437e2dd7e03062"}

HTTP 401
## Verdict
PASS — Wrong secret returned HTTP 401.
