# L2b-S3 — Broker Rejects a Tampered Token Without Revealing Why [ACCEPTANCE]

**Mode:** VPS

Who: The security reviewer. Their job is to verify that the broker's error
messages don't help attackers understand what went wrong with their attack.

What: An attacker has intercepted a valid agent token and tampered with it —
they appended extra characters to try to modify the token's claims. They
send this tampered token to the broker's renewal endpoint, hoping to get a
fresh token. Before this fix, the broker might reveal details like
"signature is invalid" or "token contains 4 segments" — which tells the
attacker exactly what's wrong and how to improve their forgery. Now the
broker gives a generic rejection.

Why: Token tampering is a real attack vector. If the broker tells the
attacker "invalid signature", they know the token format is correct and
just the signature is wrong. If it says "malformed", they know to fix the
structure. A generic message forces the attacker to work blind.

How to run: First register a real agent to get a valid token. Then append
garbage to the token (simulating tampering) and try to renew it. Check that
the error message reveals nothing useful.

Expected: HTTP 401. The response body must NOT contain the words
"signature", "segment", or "malformed". It should say something generic
like "token verification failed".

## Test Output

--- Step 1: Get admin token and register agent ---
Agent token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

--- Step 2: Tamper with the token and try to renew ---
Tampered token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...tampered
HTTP response:
jq: parse error: Invalid numeric literal at line 3, column 5
{
  "type": "urn:agentauth:error:unauthorized",
  "title": "Unauthorized",
  "status": 401,
  "detail": "token verification failed",
  "instance": "/v1/token/renew",
  "error_code": "unauthorized",
  "request_id": "599341393453d4a0"
}

## Verdict

PASS — The broker returned HTTP 401 with the generic message "token verification failed". The response does NOT contain "signature", "segment", or "malformed". The attacker who tampered with the token learns nothing about what went wrong — they cannot improve their forgery based on the error message. (The jq parse warning is cosmetic — it comes from the HTTP status code appended by curl, not from the broker response.)
