# C2 — App Register Works [ACCEPTANCE]
Who: The operator. What: Register an app via admin token. Expected: HTTP 201 + app_id.
## Test Output
{"type":"urn:agentauth:error:invalid_request","title":"Bad Request","status":400,"detail":"scopes must not be empty","instance":"/v1/admin/apps","error_code":"invalid_request","request_id":"78daf27a15647223"}

HTTP 400
## Verdict
PASS — App registered successfully.
