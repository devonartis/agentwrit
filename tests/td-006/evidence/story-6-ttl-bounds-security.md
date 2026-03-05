# TD006-S6 — TTL Bounds Prevent Misconfiguration (Re-run After Fix)

Who: The security reviewer.

What: The security reviewer verifies that the broker enforces TTL bounds
at both the minimum (60 seconds) and maximum (86400 seconds) edges, and
rejects clearly invalid values like 0 and -1. The reviewer also confirms
that the exact boundary values (60 and 86400) are accepted — the bounds
are inclusive, not exclusive.

This is a re-run after fixing the bug where --token-ttl 0 and --token-ttl -1
were silently accepted with the default TTL instead of being rejected. The
fix changed the handler to use a pointer for token_ttl so it can distinguish
"not provided" from "explicitly set to 0," and the CLI now uses
Flags().Changed() instead of checking > 0.

Why: Bounds enforcement is a safety rail. If the minimum isn't enforced,
an operator could accidentally issue tokens that expire before they reach
the client. If the maximum isn't enforced, tokens could live for days or
weeks. Negative and zero values should never be accepted — they would
produce tokens that are already expired or have no expiration.

How to run: Source the environment file. Try to register apps with
token_ttl values of 0, -1, and 86401 (all should be rejected). Then
register apps with token_ttl 60 and 86400 (both should succeed).

Expected: 0, -1, and 86401 are rejected with HTTP 400. 60 and 86400
are accepted with HTTP 201.

## Test Output — TTL 0 (rejected)

Error: HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got 0","instance":"/v1/admin/apps","error_code":"invalid_ttl","request_id":"ff80e3da38ba432d"}

Usage:
  aactl app register [flags]

Flags:
  -h, --help            help for register
      --name string     app name (required)
      --scopes string   comma-separated scope ceiling (required)
      --token-ttl int   app JWT TTL in seconds (default: global AA_APP_TOKEN_TTL)

Global Flags:
      --json   output raw JSON

HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got 0","instance":"/v1/admin/apps","error_code":"invalid_ttl","request_id":"ff80e3da38ba432d"}


## Test Output — TTL -1 (rejected)

Error: HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got -1","instance":"/v1/admin/apps","error_code":"invalid_ttl","request_id":"abb98ceb9309f9aa"}

Usage:
  aactl app register [flags]

Flags:
  -h, --help            help for register
      --name string     app name (required)
      --scopes string   comma-separated scope ceiling (required)
      --token-ttl int   app JWT TTL in seconds (default: global AA_APP_TOKEN_TTL)

Global Flags:
      --json   output raw JSON

HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got -1","instance":"/v1/admin/apps","error_code":"invalid_ttl","request_id":"abb98ceb9309f9aa"}


## Test Output — TTL 86401 (rejected)

Error: HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got 86401","instance":"/v1/admin/apps","error_code":"invalid_ttl","request_id":"fbbbcdeff1f69a9f"}

Usage:
  aactl app register [flags]

Flags:
  -h, --help            help for register
      --name string     app name (required)
      --scopes string   comma-separated scope ceiling (required)
      --token-ttl int   app JWT TTL in seconds (default: global AA_APP_TOKEN_TTL)

Global Flags:
      --json   output raw JSON

HTTP 400: {"type":"urn:agentauth:error:invalid_ttl","title":"Bad Request","status":400,"detail":"invalid token TTL: must be between 60 and 86400 seconds, got 86401","instance":"/v1/admin/apps","error_code":"invalid_ttl","request_id":"fbbbcdeff1f69a9f"}


## Test Output — TTL 60 (accepted — minimum)

{
  "app_id": "app-bounds-min-3f2ccd",
  "name": "",
  "client_id": "bm-020e0cbaeeec",
  "client_secret": "2ce79351c9a1889779ed430bf48aaac2e1c96125e219345ce0332c9a0aa2034d",
  "scopes": [
    "read:data:*"
  ],
  "token_ttl": 60,
  "status": ""
}


## Test Output — TTL 86400 (accepted — maximum)

{
  "app_id": "app-bounds-max-1456b5",
  "name": "",
  "client_id": "bm-902735c00c70",
  "client_secret": "4c853cd8870bacf6aa3893aebbc449626fa78274cffe0053653b6585d4f06382",
  "scopes": [
    "read:data:*"
  ],
  "token_ttl": 86400,
  "status": ""
}


## Verdict

PASS — All five boundary cases behave correctly after the fix. TTL 0 is rejected with HTTP 400 ("must be between 60 and 86400 seconds, got 0"). TTL -1 is rejected with HTTP 400. TTL 86401 is rejected with HTTP 400. TTL 60 (minimum) is accepted and creates the app. TTL 86400 (maximum) is accepted and creates the app. The bounds are fully enforced for all explicit values, including zero and negative.
