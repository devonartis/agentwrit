# TD006-S4 — Operator Is Rejected for Out-of-Bounds TTL

Who: The operator.

What: The operator tries to register and update apps with TTL values that
are outside the safe bounds. The broker enforces a minimum of 60 seconds
and a maximum of 86400 seconds (24 hours). Values below or above those
limits should be rejected with a clear error message. The operator tries
three bad values: 30 seconds (below minimum at registration), 100000
seconds (above maximum at registration), and 5 seconds (below minimum
at update).

Why: Without bounds enforcement, an operator could accidentally set a TTL
of 1 second (tokens expire before they're useful) or 1 year (tokens live
far too long). Both are security risks. The bounds are safety rails that
prevent misconfiguration.

How to run: Source the environment file. Try to register an app with
--token-ttl 30. Try to register another with --token-ttl 100000. Then
try to update the app from S2 with --token-ttl 5. All three should fail
with an error.

Expected: All three commands are rejected with an error about TTL bounds.
No app is created with an out-of-bounds TTL.

## Test Output — Register with TTL 30 (below minimum)

Error: HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got 30","instance":"/v1/admin/apps","error_code":"invalid_ttl","request_id":"53b20a73f849880f"}

Usage:
  aactl app register [flags]

Flags:
  -h, --help            help for register
      --name string     app name (required)
      --scopes string   comma-separated scope ceiling (required)
      --token-ttl int   app JWT TTL in seconds (default: global AA_APP_TOKEN_TTL)

Global Flags:
      --json   output raw JSON

HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got 30","instance":"/v1/admin/apps","error_code":"invalid_ttl","request_id":"53b20a73f849880f"}


## Test Output — Register with TTL 100000 (above maximum)

Error: HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got 100000","instance":"/v1/admin/apps","error_code":"invalid_ttl","request_id":"19972967cd57b04a"}

Usage:
  aactl app register [flags]

Flags:
  -h, --help            help for register
      --name string     app name (required)
      --scopes string   comma-separated scope ceiling (required)
      --token-ttl int   app JWT TTL in seconds (default: global AA_APP_TOKEN_TTL)

Global Flags:
      --json   output raw JSON

HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got 100000","instance":"/v1/admin/apps","error_code":"invalid_ttl","request_id":"19972967cd57b04a"}


## Test Output — Update with TTL 5 (below minimum)

Error: HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got 5","instance":"/v1/admin/apps/app-ttl-custom-s2-5a8397","error_code":"invalid_ttl","request_id":"b4eb770ea400bead"}

Usage:
  aactl app update [flags]

Flags:
  -h, --help            help for update
      --id string       app ID to update (required)
      --scopes string   comma-separated new scope ceiling
      --token-ttl int   new app JWT TTL in seconds

Global Flags:
      --json   output raw JSON

HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got 5","instance":"/v1/admin/apps/app-ttl-custom-s2-5a8397","error_code":"invalid_ttl","request_id":"b4eb770ea400bead"}


## Verdict

PASS — All three out-of-bounds TTL values were rejected with HTTP 400. The broker returned RFC 7807 error responses with error_code: invalid_ttl and a clear detail message stating the bounds (60–86400 seconds). Both register and update paths enforce the same validation. No apps were created with unsafe TTL values.
