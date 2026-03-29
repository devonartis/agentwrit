# L2a-S4 — Revoked Token Is Rejected Everywhere

Who: The security reviewer.

What: The reviewer verifies that token revocation works end to end. Before
this hardening, revocation was only checked in HTTP middleware. Now the check
is inside Verify() itself, so every code path that validates a token also
checks revocation. The reviewer issues a token, confirms it works, renews it
(which revokes the old one), and confirms the old token is dead.

Why: If a revoked token still works on any endpoint, an attacker who stole a
token can keep using it indefinitely even after the legitimate user renewed.
This is a critical security gap.

How to run: Source env. Get token A. Use it on /v1/admin/apps (should work).
Renew it to get token B (revokes A). Try token A again (should fail 401).
Try token B (should work 200).

Expected: Token A returns 200 before renewal, 401 after renewal. Token B
returns 200.

## Test Output

Token A: eyJhbGciOiJFZERTQSIs...

--- Token A before renewal ---
{"apps":[],"total":0}

HTTP 200

Token B (after renewal): eyJhbGciOiJFZERTQSIs...

--- Token A after renewal (should be 401) ---
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"token verification failed: token has been revoked","instance":"/v1/admin/apps","error_code":"unauthorized","request_id":"35053acd710e0cde"}

HTTP 401

--- Token B (should be 200) ---
{"apps":[],"total":0}

HTTP 200

## Verdict

PASS — Token A returns 200 before renewal. After renewal, Token A returns 401 ('token has been revoked'). Token B returns 200. Revocation is enforced end to end via Verify().


## Container Mode

Token A: eyJhbGciOiJFZERTQSIs...

--- Token A before renewal ---
{"apps":[],"total":0}

HTTP 200

Token B (after renewal): eyJhbGciOiJFZERTQSIs...

--- Token A after renewal (should be 401) ---
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"token verification failed: token has been revoked","instance":"/v1/admin/apps","error_code":"unauthorized","request_id":"604b770ee4abf047"}

HTTP 401

--- Token B (should be 200) ---
{"apps":[],"total":0}

HTTP 200

### Container Verdict

PASS — Token A returns 200 before renewal, 401 after ('token has been revoked'). Token B returns 200. Revocation enforced end to end in container deployment.
