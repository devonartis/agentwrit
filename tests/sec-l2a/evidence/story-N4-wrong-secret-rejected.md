# L2a-N4 — Wrong Admin Secret Is Still Rejected

Who: The security reviewer.

What: Regression check after the token hardening. The reviewer sends a wrong
admin secret and then a correct one. The basic auth flow must still work.

Why: If the hardening broke the auth path, wrong secrets might be accepted
or correct secrets rejected. Either would be a critical regression.

How to run: Source env. Send admin auth with a wrong secret (expect 401).
Send admin auth with the correct secret (expect 200).

Expected: Wrong secret returns 401. Correct secret returns 200 with JWT.

## Test Output

--- Wrong secret (should be 401) ---
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"invalid credentials","instance":"/v1/admin/auth","error_code":"unauthorized","request_id":"b24cbd83a662257e"}

HTTP 401

--- Correct secret (should be 200) ---
{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCIsImtpZCI6IlhSa1dxUTMzazdGMmRYWVloQkJxeW5TV2ZJUGRaNlF4LUwtY2xvMG5JNjAifQ.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhZG1pbiIsImF1ZCI6WyJhZ2VudGF1dGgiXSwiZXhwIjoxNzc0ODE4MzU3LCJuYmYiOjE3NzQ4MTgwNTcsImlhdCI6MTc3NDgxODA1NywianRpIjoiODE3Y2M1MjgxMGY1ZmYwMjAyZWJjMjMxOWFlMTU2YTciLCJzY29wZSI6WyJhZG1pbjpsYXVuY2gtdG9rZW5zOioiLCJhZG1pbjpyZXZva2U6KiIsImFkbWluOmF1ZGl0OioiXX0.gIcrwoETUtejP0UvcD64iC33_MY7B1qYqYrDKpD_0mOwsCGczTMxCVws7DKjq2V_pQ57TBVcqNHIOHaEbBCKCA","expires_in":300,"token_type":"Bearer"}

HTTP 200

## Verdict

PASS — Wrong secret returns 401 ('invalid credentials'). Correct secret returns 200 with JWT. Auth path works correctly after hardening.

## Container Mode

--- Wrong secret (should be 401) ---
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"invalid credentials","instance":"/v1/admin/auth","error_code":"unauthorized","request_id":"9c784eb2803a409f"}

HTTP 401

--- Correct secret (should be 200) ---
{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCIsImtpZCI6Imx1S0RZeERVY3NaVERQYXk3eE1MUHhZekctTDdXOFYwNTFUcWVrN0N6RW8ifQ.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhZG1pbiIsImV4cCI6MTc3NDgyNDU1NywibmJmIjoxNzc0ODI0MjU3LCJpYXQiOjE3NzQ4MjQyNTcsImp0aSI6IjI5MzQ1ZjhiNDVmYzU0MDMzZjJjZDI2YTE1ZjY1ZTQwIiwic2NvcGUiOlsiYWRtaW46bGF1bmNoLXRva2VuczoqIiwiYWRtaW46cmV2b2tlOioiLCJhZG1pbjphdWRpdDoqIl19.BLOXwPIJQSOJY30z6YZD4bVGCgaqqQms7Vd2203Vp64wXe7hENlvOwi8Yn3d3OSovwa6GBzKsBjJBWjntiIKBg","expires_in":300,"token_type":"Bearer"}

HTTP 200

### Container Verdict

PASS — Wrong secret returns 401 ('invalid credentials'). Correct secret returns 200 with JWT. Auth path works correctly in container deployment.
