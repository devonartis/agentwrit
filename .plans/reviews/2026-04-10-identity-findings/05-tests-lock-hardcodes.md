# Finding 05: Tests Lock Hardcodes In Place

**ID:** TD-TOKEN-003
**Severity:** HIGH (prevents fixing Findings 01-02)
**Status:** UNRESOLVED
**Audit reference:** C3

---

## What Was Found

Seven test assertions across three test files assert the hardcoded `"agentauth"` literal. Any attempt to make the issuer or audience configurable causes these tests to fail, which would make the implementer think the fix is wrong.

### `internal/token/tkn_svc_test.go`

| Line | Code | What it asserts |
|------|------|-----------------|
| 89 | `if claims.Iss != "agentauth" {` | Issued token must have `iss: "agentauth"` |
| 208 | `Iss: "agentauth",` | Test fixture builds claims with hardcoded issuer |
| 824 | `Iss: "agentauth",` | Another fixture with hardcoded issuer |

### `internal/authz/val_mw_test.go`

| Line | Code |
|------|------|
| 227 | `Iss: "agentauth", Sub: "agent-1", Jti: "jti-1",` |
| 249 | `Iss: "agent-1", Sub: "agent-1", Jti: "jti-1",` |
| 341 | `Iss: "agentauth", Sub: "agent-1", Jti: "jti-1",` |

### `internal/cfg/cfg_test.go`

| Line | Code |
|------|------|
| 65 | `if c.Audience != "agentauth" {` |

### The smoking gun (line 521-522)

```go
// TestIssClaimMatchesConfig and TestVerifyRejectsWrongIssuer removed —
// IssuerURL is not present in this build.
```

Someone explicitly deleted the tests that verified configurable issuer behavior. These are the exact tests we need to restore.

## Why This Is a Problem

These tests are **actively preventing the fix**. If someone adds a configurable issuer field:

1. `tkn_svc_test.go:89` fails because the test expects `"agentauth"` but the issuer is now whatever `cfg.Issuer` is set to
2. The fixer sees a red test, assumes their implementation is wrong, reverts
3. The hardcode stays forever

This is a **test smell**: tests should verify behavior, not implementation details. The issuer value is a configuration detail, not a behavioral contract. The behavioral contract is "the token's issuer matches the configured value" — not "the token's issuer is 'agentauth'."

## Solution

### Pattern: Drive identity values from test config fixtures

Most test files already create a `cfg.Cfg` struct. The fix is to set the identity fields on that struct and derive the expected values from it:

### `tkn_svc_test.go`

Add test issuer to the config fixture:

```go
var testCfg = cfg.Cfg{
    TrustDomain: "test.local",
    Issuer:      "testissuer",  // NEW
    DefaultTTL:  300,
}
```

Then in assertions:

```go
// Before:
if claims.Iss != "agentauth" {

// After:
if claims.Iss != testCfg.Issuer {
```

And in fixtures that build claims manually:

```go
// Before:
claims := &TknClaims{Iss: "agentauth", Sub: "..."}

// After:
claims := &TknClaims{Iss: testCfg.Issuer, Sub: "..."}
```

### `val_mw_test.go`

Same pattern — define a test issuer constant or use the fixture's config:

```go
const testIssuer = "testissuer"

// In fixtures:
Iss: testIssuer,  // was: "agentauth"
```

### `cfg_test.go`

Update the audience test to verify the new default behavior:

```go
// Before:
if c.Audience != "agentauth" {

// After (with empty default per Finding 02):
if c.Audience != "" {
```

### Restore deleted tests

Restore the tests mentioned in the tombstone, rewritten for the new configurable approach:

```go
func TestIssClaimMatchesConfig(t *testing.T) {
    c := cfg.Cfg{Issuer: "my-broker", TrustDomain: "test.local", DefaultTTL: 300}
    svc := NewTknSvc(priv, pub, c)
    resp, err := svc.Issue(IssueReq{Sub: "spiffe://test.local/agent/o/t/i", Scope: []string{"read:data:*"}})
    require.NoError(t, err)
    assert.Equal(t, "my-broker", resp.Claims.Iss)
}

func TestVerifyRejectsWrongIssuer(t *testing.T) {
    // Token issued with "my-broker" issuer, verified against "other-broker" → reject
    c1 := cfg.Cfg{Issuer: "my-broker", TrustDomain: "test.local", DefaultTTL: 300}
    c2 := cfg.Cfg{Issuer: "other-broker", TrustDomain: "test.local", DefaultTTL: 300}
    svc1 := NewTknSvc(priv, pub, c1)
    svc2 := NewTknSvc(priv, pub, c2)

    resp, _ := svc1.Issue(IssueReq{Sub: "spiffe://test.local/agent/o/t/i", Scope: []string{"read:data:*"}})
    _, err := svc2.Verify(resp.AccessToken)
    assert.ErrorIs(t, err, ErrInvalidIssuer)
}
```

## Files Changed

| File | Change |
|------|--------|
| `internal/token/tkn_svc_test.go` | All `"agentauth"` → `testCfg.Issuer`; restore deleted tests |
| `internal/authz/val_mw_test.go` | All `"agentauth"` → test constant |
| `internal/cfg/cfg_test.go` | `c.Audience != "agentauth"` → `c.Audience != ""` |
| `internal/token/tkn_claims.go` | `Validate()` signature change requires all test callers to pass issuer |

## Verification

1. `go test ./internal/token/...` — all pass with configurable issuer
2. `go test ./internal/authz/...` — all pass with configurable issuer
3. `go test ./internal/cfg/...` — all pass with empty audience default
4. Verify that changing `testCfg.Issuer` to any string causes all tests to still pass (they adapt to the fixture)
5. Verify that the restored `TestIssClaimMatchesConfig` and `TestVerifyRejectsWrongIssuer` tests exist and pass
