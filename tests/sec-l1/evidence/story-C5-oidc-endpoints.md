# C5 — OIDC Discovery and JWKS Still Work

Who: The developer.

What: The developer hits the OIDC discovery and JWKS endpoints. These are
used by external consumers (AWS STS, Python validators) to discover the
broker's issuer URL and fetch the public key for token verification.

Why: OIDC endpoints are public and critical for federation. If the bind
address or timeout changes affected response handling, external consumers
would fail to validate tokens. These endpoints must return correct JSON
with the right structure.

How to run: GET /.well-known/openid-configuration and GET /v1/jwks.

Expected: Discovery returns issuer matching the broker URL. JWKS returns
an EC/P-256 key with ES256 algorithm.

## Test Output

--- GET /.well-known/openid-configuration ---
{
  "issuer": "http://localhost:8080",
  "jwks_uri": "http://localhost:8080/v1/jwks",
  "id_token_signing_alg_values_supported": ["ES256"],
  "claims_supported": ["sub","iss","aud","exp","iat","nbf","jti","scope","sid","task_id","orch_id"]
}

--- GET /v1/jwks ---
{
  "keys": [{
    "kty": "EC",
    "crv": "P-256",
    "use": "sig",
    "kid": "1iLIoJfYRekpFF46--b1agHq8_RV0rx-pZo1CMGQmmI",
    "x": "nEYvgwfIgpcbaodSijkvvoP4tIYpmpMnxqQEfTUB0vY",
    "y": "5fGWk8xHDV_YBXoYXAOfRjaXxVUdd1VOvTnG22s_cRc"
  }]
}

## Verdict

PASS — Discovery returned correct issuer (http://localhost:8080), jwks_uri pointing to /v1/jwks, and ES256 signing algorithm. JWKS returned an EC/P-256 key with kid, x, y coordinates. Both OIDC endpoints work correctly with the hardened server.
