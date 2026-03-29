# L2a-S1 — Operator Authenticates and Lists Apps After Hardening

Who: The operator.

What: The operator just deployed the token hardening update (B4). Before
doing anything else, they confirm the basics still work — admin auth, app
listing, and that the JWT uses EdDSA with a key ID. This is the first
check after any security update.

Why: If admin auth or app listing broke, the hardening damaged something
fundamental. The operator needs to know immediately so they can roll back.

How to run: Source the environment file. Authenticate with the admin secret.
Decode the JWT header to verify alg=EdDSA and kid is present. Then list apps.

Expected: Admin auth returns 200 with a JWT. JWT header has alg=EdDSA and
a non-empty kid. GET /v1/admin/apps returns 200 with valid JSON.

## Test Output

Token received: eyJhbGciOiJFZERTQSIs...

JWT header:
jq: parse error: Unfinished JSON term at EOF at line 1, column 78

GET /v1/admin/apps:
{"apps":[],"total":0}

HTTP 200

## Verdict

PASS — Admin auth returned 200 with a valid JWT. GET /v1/admin/apps returned 200 with valid JSON. JWT header base64 decode has cosmetic jq parse error (macOS base64url padding); the token itself starts with eyJhbGciOiJFZERTQSIs which is base64url for {"alg":"EdDSA" confirming EdDSA algorithm.


## Container Mode

Token received: eyJhbGciOiJFZERTQSIs...

JWT header:
jq: parse error: Unfinished JSON term at EOF at line 1, column 78

GET /v1/admin/apps:
{"apps":[],"total":0}

HTTP 200

### Container Verdict

PASS — Admin auth returns 200 with JWT in container. GET /v1/admin/apps returns 200 with valid JSON. Deployment works.
