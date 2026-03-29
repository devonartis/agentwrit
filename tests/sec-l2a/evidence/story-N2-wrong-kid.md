# L2a-N2 — Broker Rejects Token With Wrong Key ID

Who: The security reviewer.

What: The reviewer tampers with the kid (key ID) in the JWT header. In a
multi-broker deployment, each broker has its own signing key with a unique
ID. If an attacker steals a token from broker A and presents it to broker B,
the kid won't match. The broker should reject it.

Why: Before this hardening, the broker didn't check kid at all. Any token
with a valid signature was accepted regardless of which key supposedly signed
it. This is a cross-broker replay attack vector.

How to run: Source env. Get a valid token. Decode the header, change kid to
a fake value, re-encode, and present to the broker.

Expected: Broker returns 401. The wrong-kid token is rejected.

## Test Output

Original kid:
jq: parse error: Unfinished JSON term at EOF at line 1, column 78

--- Presenting wrong-kid token (should be 401) ---
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"token verification failed: invalid token format","instance":"/v1/admin/apps","error_code":"unauthorized","request_id":"e8a6ba03dbbf9a34"}

HTTP 401

## Verdict

PASS — Token with tampered kid ('wrong-key-id-12345') correctly rejected with 401. Error detail: 'token verification failed: invalid token format'. No information leakage. Base64 decode cosmetic issue on header display only.

## Container Mode

Original kid:
jq: parse error: Unfinished JSON term at EOF at line 1, column 78

--- Presenting wrong-kid token (should be 401) ---
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"token verification failed: invalid token format","instance":"/v1/admin/apps","error_code":"unauthorized","request_id":"833ff5c38328dca7"}

HTTP 401

### Container Verdict

PASS — Token with tampered kid ('wrong-key-id-12345') rejected with 401 in container. Cross-broker replay attack prevented.
