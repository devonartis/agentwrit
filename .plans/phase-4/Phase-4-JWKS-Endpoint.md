# Phase 4: JWKS Endpoint

**Status:** Spec
**Priority:** P1 — removes token validation as a single point of failure
**Effort estimate:** 1 day
**Depends on:** Phase 5 (key persistence) for full value, but can be built first
**Architecture doc:** `../.plans/CoWork-Architecture-Direct-Broker.md`

---

## Overview: What We're Building and Why

AgentAuth's token validation is currently a single point of failure. Every time a resource server needs to check whether an agent's JWT is valid, it calls `POST /v1/token/validate` on the broker. If the broker is down — maintenance, restart, network issue — no service can validate any token. Authorization stops system-wide.

**Phase 4 adds a standard JWKS (JSON Web Key Set) endpoint** at `GET /.well-known/jwks.json` that exposes the broker's Ed25519 public key. Resource servers fetch this key (and cache it), then validate token signatures locally — no broker call needed. This removes the broker as a SPOF for the most common validation case: "is this token's signature valid and is it not expired?"

There's an important nuance: local validation checks signature and expiry but does NOT check revocation. A revoked token will still pass local validation until it expires. Services handling sensitive operations should still use online validation (`POST /v1/token/validate`) for revocation checking. Most read-heavy services can use local validation safely.

**What's built:** One new unauthenticated endpoint returning the Ed25519 public key in RFC 7517 JWK format, `kid` (key ID) added to JWT headers for future key rotation support, cache headers for efficient client-side caching.

**What stays the same:** Online validation endpoint unchanged. Token format unchanged (just adding `kid` to the header). All existing validation behavior preserved.

**Relationship to Phase 5:** JWKS works immediately — but without key persistence (Phase 5), the signing key changes every broker restart, invalidating all cached JWKS keys. Phase 5 makes JWKS production-grade.

---

## Problem Statement

Today, every token validation requires a call to the broker (`POST /v1/token/validate`). If the broker is down, no service can validate any token — making the broker a single point of failure for authorization decisions.

A JWKS (JSON Web Key Set) endpoint exposes the broker's public signing key so that resource servers can validate token signatures locally without calling the broker. This removes the validation SPOF.

**Important caveat:** JWKS-based local validation checks the signature and expiry but does NOT check revocation status. Services needing revocation checks must still call the broker's online validation endpoint.

---

## Goals

1. Resource servers can validate token signatures locally by fetching the broker's public key
2. The JWKS endpoint follows the standard RFC 7517 format at `GET /.well-known/jwks.json`
3. Local validation works for the common case (valid signature + not expired)
4. Services that need revocation checking still use online validation
5. The public key is cacheable with appropriate cache headers

---

## Non-Goals

1. **Replacing online validation** — JWKS enables local signature checks, but revocation still needs the broker
2. **Key rotation via JWKS** — automated key rotation is a future enhancement
3. **Multiple keys in the JWKS** — single key for now (rotation support adds multiple keys later)
4. **OIDC discovery** — `/.well-known/openid-configuration` is a future consideration
5. **Client-side JWKS caching policy** — clients decide how often to refresh

---

## User Stories

### Developer Stories

1. **As a resource server developer**, I want to fetch the broker's public key from a standard JWKS endpoint so that I can validate agent tokens locally without calling the broker for every request.

2. **As a developer**, I want the JWKS response to include the key algorithm (`EdDSA`) and key type (`OKP`) so that my JWT library can automatically select the right verification method.

3. **As a developer**, I want token validation to work even if the broker is temporarily unreachable so that my service doesn't fail during broker maintenance windows.

### Operator Stories

4. **As an operator**, I want the JWKS endpoint to be unauthenticated so that any service in my infrastructure can fetch the public key without credentials.

5. **As an operator**, I want the JWKS response to include cache headers so that clients don't hammer the broker for the key on every request.

6. **As an operator**, I want to know that local validation does NOT check revocation so that I can decide which services need online vs local validation.

### Security Stories

7. **As a security reviewer**, I want the JWKS endpoint to only expose the PUBLIC key so that the signing private key is never transmitted.

8. **As a security reviewer**, I want the JWKS key ID (`kid`) to be included in issued JWTs so that key rotation (future) can work seamlessly.

---

## What Needs to Be Done

### 1. JWKS Endpoint

A new unauthenticated endpoint at `GET /.well-known/jwks.json` that returns the broker's Ed25519 public key in JWK format:

```json
{
    "keys": [
        {
            "kty": "OKP",
            "crv": "Ed25519",
            "x": "<base64url-encoded public key>",
            "kid": "<key identifier>",
            "use": "sig",
            "alg": "EdDSA"
        }
    ]
}
```

Response headers should include:
- `Cache-Control: public, max-age=3600` (1 hour cache)
- `Content-Type: application/json`

### 2. Key ID (`kid`) in Issued JWTs

All JWTs issued by the broker should include a `kid` claim in the JWT header. This allows clients to match the token to the correct key in the JWKS response — essential for key rotation support in the future.

The `kid` should be a deterministic identifier derived from the public key (e.g., a truncated SHA-256 hash of the key bytes).

### 3. Documentation: Local vs Online Validation

Clear documentation explaining:
- **Local validation** (JWKS): checks signature + expiry. Fast, no broker dependency. Does NOT check revocation.
- **Online validation** (`POST /v1/token/validate`): checks signature + expiry + revocation. Requires broker connectivity.
- When to use which: most read-heavy services can use local validation; services handling sensitive operations should use online validation for revocation checking.

---

## Success Criteria

- `GET /.well-known/jwks.json` returns the Ed25519 public key in RFC 7517 format
- Any JWT library (Go, Python, JavaScript) can use the JWKS response to validate tokens
- Issued JWTs include `kid` in the header
- Endpoint is unauthenticated (no Bearer token required)
- Response includes appropriate cache headers
- Endpoint works regardless of whether key persistence (Phase 5) is implemented
- Existing online validation endpoint unchanged

---

## Relationship to Phase 5

JWKS works today — the broker's in-memory Ed25519 key is exposed as a public key. However, without Phase 5 (key persistence), the key changes every time the broker restarts, which means all cached JWKS keys become invalid on restart. Phase 5 makes JWKS reliable for long-term caching.

Recommendation: Build Phase 4 first (it's quick), then Phase 5 makes it robust.
