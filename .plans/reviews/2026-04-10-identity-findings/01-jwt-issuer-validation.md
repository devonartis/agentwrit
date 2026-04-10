# Finding 01: JWT Issuer — The Only Truly Hardcoded Value

**ID:** TD-TOKEN-001
**Severity:** HIGH (blocks rebrand — this is the ONE finding that is genuinely hardcoded)
**Status:** UNRESOLVED
**Audit reference:** C1-1, C1-2

---

## Important Context: Hardcoded vs. Branded Default

Most of the "hardcoded identity" findings in the audit are **NOT truly hardcoded.** They're branded defaults — the string `"agentauth"` appears as the fallback value in `envOr()`, but the operator can override it via an environment variable:

| Field | Env var | Overridable? | Truly hardcoded? |
|-------|---------|-------------|-----------------|
| `TrustDomain` | `AA_TRUST_DOMAIN` | ✅ Yes | No — branded default |
| `DBPath` | `AA_DB_PATH` | ✅ Yes | No — branded default |
| `Audience` | `AA_AUDIENCE` | ✅ Yes | No — branded default |
| **`Issuer`** | **Does not exist** | ❌ **No** | **YES — genuinely hardcoded** |

An operator who sets `AA_TRUST_DOMAIN=mycompany.com` never sees `"agentauth.local"`. The brand only leaks if you run without setting env vars — which is a docs/defaults problem, not a hardcoding problem.

**The issuer is different.** There is no `AA_ISSUER` env var. There is no `Issuer` field in `cfg.Cfg`. The string `"agentauth"` is baked into two code paths with zero override path.

---

## What Was Found

### Issuance — `internal/token/tkn_svc.go:141`

```go
claims := &TknClaims{
    Iss: "agentauth",  // literal string, no config lookup
    ...
}
```

Every token this broker mints gets `iss: "agentauth"`. No env var, no config field, no way to change it without editing source code.

### Validation — `internal/token/tkn_claims.go:62`

```go
func (c *TknClaims) Validate() error {
    if c.Iss != "agentauth" {  // literal string, no config lookup
        return ErrInvalidIssuer
    }
    // ...
}
```

Every token verified by this broker must have `iss: "agentauth"` or it's rejected. No way to change the expected value.

### Doc comments reinforce the hardcode

```go
// The issuer is always "agentauth". Subjects are SPIFFE-format agent IDs
// ...
// It returns an error if the issuer is not "agentauth", the subject or JTI
```

## Why This Is a Problem

- **Blocks rebrand:** You cannot rename the product to AgentWrit without changing the issuer. Since there's no config path, it requires source code changes + recompile.
- **Blocks multi-instance:** Two separate deployments both emit `iss: "agentauth"` — tokens are indistinguishable.
- **Wire-protocol lock-in:** Any external verifier must know the literal `"agentauth"` string.

## Why This Happened

The tombstone at `internal/token/tkn_svc_test.go:521-522`:

```go
// TestIssClaimMatchesConfig and TestVerifyRejectsWrongIssuer removed —
// IssuerURL is not present in this build.
```

During the open-core split, someone deleted `IssuerURL` (which was coupled to OIDC) and replaced it with the hardcoded literal. They treated the JWT `iss` claim as OIDC-specific when it's a general JWT concern every token issuer needs.

## Solution

### 1. Add `Issuer` field to `cfg.Cfg`

In `internal/cfg/cfg.go`:

```go
type Cfg struct {
    // ... existing fields ...
    Issuer string // AA_ISSUER: JWT issuer claim (required)
    // ...
}
```

### 2. Make it required — empty = startup failure

In the `Load()` function, after populating all fields:

```go
if c.Issuer == "" {
    return Cfg{}, fmt.Errorf(
        "AA_ISSUER is required. Set it to the issuer name for this broker " +
        "(e.g. 'agentwrit'). Previously this defaulted to 'agentauth'.",
    )
}
```

### 3. Use `cfg.Issuer` in `TknClaims.Validate()`

In `internal/token/tkn_claims.go`, change `Validate` to accept the expected issuer:

```go
func (c *TknClaims) Validate(expectedIssuer string) error {
    if c.Iss != expectedIssuer {
        return ErrInvalidIssuer
    }
    // ... rest unchanged ...
}
```

### 4. Thread the issuer through `TknSvc`

`TknSvc` already holds `cfg.Cfg`, so update `Issue()` to use `s.cfg.Issuer`:

```go
// In Issue():
claims := &TknClaims{
    Iss: s.cfg.Issuer,  // was: "agentauth"
    // ...
}
```

And in `Verify()`, pass the issuer to `Validate`:

```go
if err := claims.Validate(s.cfg.Issuer); err != nil {
    return nil, err
}
```

### 5. Update all callers of `Validate()`

Search for all direct calls to `.Validate()` on `TknClaims` and pass the issuer. In tests, pass the test fixture's issuer string (e.g., `"testissuer"`).

### 6. Fix the doc comments

```go
// The issuer is configurable via AA_ISSUER. Subjects are SPIFFE-format agent IDs
// ...
// It returns an error if the issuer does not match the configured value, the subject or JTI
```

## Files Changed

| File | Change |
|------|--------|
| `internal/cfg/cfg.go` | Add `Issuer string` field, `AA_ISSUER` env var, required check |
| `internal/cfg/cfg_test.go` | Add test for required-issuer startup failure |
| `internal/token/tkn_claims.go` | `Validate()` takes `expectedIssuer string` param |
| `internal/token/tkn_svc.go` | Use `s.cfg.Issuer` in `Issue()` and `Verify()` |
| `internal/token/tkn_svc_test.go` | All test fixtures set `Issuer`, pass to `Validate()` |
| `internal/authz/val_mw_test.go` | Update `Validate()` calls with test issuer |

## Migration Note

**BREAKING CHANGE.** Existing deployments must set `AA_ISSUER=agentauth` to preserve backward compatibility with previously issued tokens, OR accept that all existing tokens will be rejected after upgrade and re-authenticate.

CHANGELOG entry:
```
### BREAKING
- `AA_ISSUER` environment variable is now required. Previously the JWT issuer
  was hardcoded to "agentauth". Set `AA_ISSUER=agentauth` to preserve
  compatibility with existing tokens, or set `AA_ISSUER=<your-name>` for new
  deployments.
```

## Verification

After the fix:
1. `go test ./internal/token/...` — all tests pass with configurable issuer
2. `go test ./internal/authz/...` — all validation middleware tests pass
3. Start broker without `AA_ISSUER` → FATAL with clear error message
4. Start broker with `AA_ISSUER=agentwrit` → tokens have `iss: "agentwrit"`
5. Verify a token with `iss: "agentwrit"` against a broker configured with `AA_ISSUER=agentwrit` → accepted
6. Verify a token with `iss: "agentauth"` against the same broker → rejected
