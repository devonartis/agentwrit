# L2b-S1 — App Receives a Generic Error When Sending a Bad Token [ACCEPTANCE]

**Mode:** VPS

Who: The app. In production, an app receives tokens from agents and sends
them to the broker's validate endpoint to check whether the agent should be
trusted. This happens automatically, hundreds of times per minute.

What: The app sends a token to the broker that is clearly invalid — it's
not a real token at all, just garbage text. Before this fix, the broker
would tell the app exactly what was wrong with the token (e.g., "token
contains an invalid number of segments"). Now the broker gives a generic
message that reveals nothing about the token's structure.

Why: If the broker reveals why a token failed, an attacker who intercepts
error messages can use that information to craft better forged tokens. For
example, knowing "invalid signature" vs "expired" tells the attacker the
token format is correct and they just need a better signing key. A generic
message ("token is invalid or expired") gives the attacker nothing to work
with.

How to run: We emulate what the app does in production — it sends an HTTP
request to the broker's validate endpoint with a token it received. In this
case, the token is deliberately garbage to trigger an error.

Expected: The broker responds with `{"valid": false}` and a generic error
message: "token is invalid or expired". The response must NOT contain words
like "segment", "signature", "malformed", or any other internal detail.

## Test Output

{
  "valid": false,
  "error": "token is invalid or expired"
}

## Verdict

PASS — The broker returned valid=false with the generic message "token is invalid or expired". No internal details (segment, signature, malformed, algorithm) appear in the response. An attacker learns nothing about why the token failed.
