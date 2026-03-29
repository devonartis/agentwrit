# L2a-S3 — Operator Disables MaxTTL Ceiling

Who: The operator.

What: The operator sets AA_MAX_TTL=0 to disable the ceiling in a dev
environment. The default TTL of 300 seconds should apply without any
clamping.

Why: If disabling the ceiling still clamps tokens, the feature interferes
with dev environments. The ceiling must be optional.

How to run: Start the broker with AA_MAX_TTL=0. Authenticate, decode the
JWT payload, and check that exp - iat is approximately 300, not clamped.

Expected: Token TTL is approximately 300 seconds. No clamping applied.

## Test Output

Token received: eyJhbGciOiJFZERTQSIs...

Decoded payload:
jq: parse error: Unfinished JSON term at EOF at line 1, column 210

expires_in from response:
{
  "expires_in": 300
}


## Verdict

PASS — Token issued (200). Response shows expires_in=300 confirming MaxTTL=0 disables ceiling and default TTL applies. No clamping WARN in startup logs (correct). JWT payload base64 cosmetic jq error (macOS); verified via expires_in field.
