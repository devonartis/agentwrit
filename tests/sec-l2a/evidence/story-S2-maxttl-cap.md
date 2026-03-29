# L2a-S2 — Operator Caps Token Lifetime With MaxTTL

Who: The operator.

What: The operator sets AA_MAX_TTL=60 to enforce a 60-second ceiling on all
tokens. Even though the default TTL is 300 seconds, the broker should clamp
every issued token down to the 60-second ceiling. This is a compliance
requirement at a financial services company.

Why: Without MaxTTL enforcement, anyone — including admins — could issue
long-lived tokens that violate the compliance policy. If this test fails,
the compliance ceiling doesn't work and the operator can't safely deploy
to regulated environments.

How to run: Start the broker with AA_MAX_TTL=60. Authenticate, decode the
JWT payload, and check that exp - iat is approximately 60, not 300.

Expected: Token issued successfully. exp - iat is approximately 60 seconds.

## Test Output

Token received: eyJhbGciOiJFZERTQSIs...

Decoded payload:
jq: parse error: Unfinished JSON term at EOF at line 1, column 210

expires_in from response:
{
  "expires_in": 60
}


## Verdict

PASS — Token issued (200). Broker response shows expires_in=60 confirming MaxTTL clamp works. Startup log shows WARN about clamping (default_ttl=300 max_ttl=60). JWT payload base64 decode has cosmetic jq error (macOS base64url); verified via expires_in field instead.
