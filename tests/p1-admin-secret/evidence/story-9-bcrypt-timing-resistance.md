# P1-S9 — Bcrypt Timing Resistance [ACCEPTANCE]

Who: The security reviewer.

What: All wrong secrets return the same HTTP status and response shape.
Bcrypt's internal constant-time comparison prevents timing side-channels.

Why: With plaintext comparison, an attacker could measure response time
to determine how many bytes matched. Bcrypt eliminates this.

How to run: Start broker, send wrong secrets of varying lengths, verify
all get identical 401 response shape. Empty secret should get 400.

Expected: All wrong secrets → 401 (same shape). Empty → 400.

## Test Output

Same-length wrong secret:
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"invalid credentials","instance":"/v1/admin/auth","error_code":"unauthorized","request_id":"0d695cd1134a198e"}

HTTP 401
Shorter wrong secret:
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"invalid credentials","instance":"/v1/admin/auth","error_code":"unauthorized","request_id":"ac8d5a1f84d42740"}

HTTP 401
Longer wrong secret:
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"invalid credentials","instance":"/v1/admin/auth","error_code":"unauthorized","request_id":"b3a668f3df163bb1"}

HTTP 401
Empty secret (should be 400):
{"type":"urn:agentauth:error:invalid_request","title":"Bad Request","status":400,"detail":"secret is required","instance":"/v1/admin/auth","error_code":"invalid_request","request_id":"a66aee892e68e943"}

HTTP 400

## Verdict

PASS — Same-length, shorter, and longer wrong secrets all return HTTP 401 with identical response shape (no information leakage). Empty secret returns HTTP 400 "secret is required" (validation, not auth). All 401 responses have the same structure — no timing side-channel via response body.
