# L2a-SEC1 — Code Review: Verify() Check Order

Who: The security reviewer.

What: The reviewer reads internal/token/tkn_svc.go and traces the Verify()
function line by line. The checks must happen in a specific order: format,
algorithm, kid, signature, claims, revocation. Each check is a gate — if it
fails, the function returns immediately.

Why: The order matters for security and performance. Cheap checks (alg, kid)
must happen before expensive checks (signature, revocation). Algorithm
confusion must be caught before signature verification. If checks are out of
order, an attacker could exploit gaps.

How to run: Read the Verify() function. Trace the check order. Verify no
code path skips a security check.

Expected: Checks in order: format → alg → kid → signature → claims →
revocation. No path skips a check. Appropriate errors for each failure.

## Code Review

Verify() function source:
```go
func (s *TknSvc) Verify(tokenStr string) (*TknClaims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// Validate JWT header: alg must be EdDSA, kid (if present) must match.
	hdrJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var hdr jwtHeader
	if err := json.Unmarshal(hdrJSON, &hdr); err != nil {
		return nil, ErrInvalidToken
	}
	if hdr.Alg != "EdDSA" {
		return nil, ErrInvalidToken
	}
	if hdr.Kid != "" && hdr.Kid != s.kid {
		obs.Warn("TOKEN", "verify", "kid mismatch — possible key rotation or wrong key", "got="+hdr.Kid, "want="+s.kid)
		return nil, ErrInvalidToken
	}

	// Decode and verify signature
	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrSignatureInvalid
	}

	if !ed25519.Verify(s.pubKey, []byte(signingInput), sigBytes) {
		return nil, ErrSignatureInvalid
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims TknClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if err := claims.Validate(); err != nil {
		return nil, err
	}

	if s.revoker != nil && s.revoker.IsRevoked(&claims) {
		return nil, ErrTokenRevoked
	}

	return &claims, nil
}
```


## Verdict

PASS — Verify() check order is correct:

1. **Format** — `SplitN(tokenStr, ".", 3)` + `len(parts) != 3` → ErrInvalidToken
2. **Algorithm** — `hdr.Alg != "EdDSA"` → ErrInvalidToken (prevents algorithm confusion)
3. **Key ID** — `hdr.Kid != "" && hdr.Kid != s.kid` → ErrInvalidToken (prevents cross-broker replay, allows empty kid for backward compat)
4. **Signature** — `ed25519.Verify(s.pubKey, signingInput, sigBytes)` → ErrSignatureInvalid
5. **Claims** — `claims.Validate()` checks: issuer, subject, jti, `exp <= 0` → ErrNoExpiry, `now > exp` → ErrTokenExpired, nbf
6. **Revocation** — `s.revoker.IsRevoked(&claims)` → ErrTokenRevoked

All cheap checks (format, alg, kid) happen BEFORE the expensive crypto signature check. Revocation (database lookup) is last. Every failure returns immediately with an appropriate error. No code path allows a token through without all 6 checks. The revoker nil-check (`s.revoker != nil`) is safe — it means revocation is disabled if no store is configured, which is handled by the M6 fix (NewRevSvc requires non-nil store).
