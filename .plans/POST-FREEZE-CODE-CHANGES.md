# Post-Freeze Code Changes

Tracks code changes (even comments) deferred until code freeze ends.

---

## 1. Fix stale "agentauth-core" reference in test comment

**File:** `internal/token/tkn_svc_test.go:522-523`
**Found during:** Batch 6 (CC v4 cleanup plan, 2026-04-04)

**Current:**
```go
// TestIssClaimMatchesConfig and TestVerifyRejectsWrongIssuer removed —
// IssuerURL is not present in agentauth-core.
```

**Change to:**
```go
// TestIssClaimMatchesConfig and TestVerifyRejectsWrongIssuer removed —
// IssuerURL is not present in this build.
```

**Why:** The repo was renamed to `devonartis/agentauth`. The name "agentauth-core" only existed during migration. Comment is misleading to future readers.

**Impact:** Comment only. No behavior change. No test change.

---

## 2. Fix stale OIDC reference in tkn_svc.go comment

**File:** `internal/token/tkn_svc.go:289`
**Found during:** Batch 8 final verification (CC v4 cleanup plan, 2026-04-04)

**Current:**
```go
// PublicKey returns the Ed25519 public key so external services (resource
// servers, federated brokers) can verify tokens without calling back to
// the broker. Exposed via the OIDC discovery endpoint (/.well-known/jwks.json).
```

**Change to (suggested):**
```go
// PublicKey returns the Ed25519 public key so external services (resource
// servers) can verify tokens without calling back to the broker.
```

**Why:** The broker does NOT expose a JWKS endpoint in this build — the `/.well-known/jwks.json` mention is stale. JWKS/OIDC discovery lives in the enterprise fork. Also "federated brokers" implies federation, which also lives in enterprise.

**Impact:** Comment only. No behavior change.

