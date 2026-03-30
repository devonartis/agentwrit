# SEC-L2b: Error Handling & Headers — Acceptance Stories (agentauth-core)

Adapted from legacy `agentauth/tests/fix-sec-l2b/user-stories.md`.
Removed S5 (HSTS/TLS — optional, cert-dependent) and S7 (JWKS Cache-Control — OIDC, not in core).

## Precondition

Broker running via Docker (`./scripts/stack_up.sh`) with standard test secret.

---

## S1 — Validate returns generic error on invalid token (H3)

| Field | Value |
|-------|-------|
| **Who** | Developer (curl) |
| **What** | POST /v1/token/validate with a garbage token |
| **Why** | H3: JWT library errors must not leak to callers |
| **How** | `curl -s -X POST http://127.0.0.1:8443/v1/token/validate -H "Content-Type: application/json" -d '{"token":"not-a-valid-token"}'` |
| **Expected** | `{"valid":false,"error":"token is invalid or expired"}` — no JWT internals |

## S2 — Validate returns generic error on revoked token (H3)

| Field | Value |
|-------|-------|
| **Who** | Developer (curl) |
| **What** | Register agent, revoke it, then validate |
| **Why** | H3: Revocation must not reveal "token has been revoked" |
| **How** | Register → revoke → validate |
| **Expected** | `{"valid":false,"error":"token is invalid or expired"}` |

## S3 — Renew rejects tampered token without leaking details (H4)

| Field | Value |
|-------|-------|
| **Who** | Developer (curl) |
| **What** | POST /v1/token/renew with tampered Bearer token |
| **Why** | H4: Renewal error must not include err.Error() internals |
| **How** | `curl -s -X POST http://127.0.0.1:8443/v1/token/renew -H "Authorization: Bearer <token>tampered"` |
| **Expected** | 401, body has no "signature", "segment", or "malformed" strings |

## S4 — Security headers present on all responses (H1)

| Field | Value |
|-------|-------|
| **Who** | Developer (curl) |
| **What** | Check response headers on health, metrics, validate |
| **Why** | H1: All responses must include security headers |
| **How** | `curl -sI` on `/v1/health`, `/v1/metrics`, `/v1/token/validate` |
| **Expected** | `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Cache-Control: no-store` |

## S5 — HSTS present when TLS enabled (H1) — OPTIONAL

| Field | Value |
|-------|-------|
| **Who** | Operator |
| **What** | Start broker with AA_TLS_MODE=tls, check headers |
| **Why** | H1: HSTS only when TLS active |
| **How** | Start with TLS mode, curl health |
| **Expected** | `Strict-Transport-Security: max-age=63072000; includeSubDomains` |
| **Note** | Requires TLS cert — SKIP if no cert available |

## S6 — Oversized body returns 413 (H7)

| Field | Value |
|-------|-------|
| **Who** | Developer (curl) |
| **What** | POST /v1/token/validate with 1MB+ body |
| **Why** | H7: Global body limit must cover all routes |
| **How** | Send 1MB+1 byte payload |
| **Expected** | HTTP 413 |

---

## Regression Tests (prior batches must still work)

### R1 — Admin auth works (B2)

| **How** | POST /v1/admin/auth with test secret |
| **Expected** | 200 + JWT token |

### R2 — Agent registration works (B0)

| **How** | Create launch token, register agent |
| **Expected** | 200 + agent token |

### R3 — Token renewal works (B1)

| **How** | Renew a valid agent token |
| **Expected** | 200 + new token |

### R4 — Token revocation + validate (B4)

| **How** | Register agent, revoke, validate |
| **Expected** | Revoke 200, validate returns valid=false |
