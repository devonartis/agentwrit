# Finding 03: Identity Defaults Should Be Required, Not Branded

**ID:** TD-CFG-001
**Severity:** HIGH (bad production practice — silent branded defaults)
**Status:** UNRESOLVED
**Audit reference:** C2-1, C2-2

---

## The Principle

**Production software should fail to start if identity values aren't explicitly configured.** No defaults. No branded fallbacks. If the operator didn't set it, the broker refuses to boot with a clear error.

These fields have `envOr()` fallbacks — so an operator CAN override them. But if they DON'T, the broker silently starts with `"agentauth"` branded values. That's bad practice. A production broker should never start in an unconfigured state.

---

## What Was Found

### TrustDomain — `internal/cfg/cfg.go:77`

```go
TrustDomain: envOr("AA_TRUST_DOMAIN", "agentauth.local"),
```

If `AA_TRUST_DOMAIN` is unset, the broker silently starts with `agentauth.local`. Every agent gets a SPIFFE ID like `spiffe://agentauth.local/...`. The operator may not realize their deployment is using a default identity.

### DBPath — `internal/cfg/cfg.go:82`

```go
DBPath: envOr("AA_DB_PATH", "./agentauth.db"),
```

The database file is named after the brand. This is infrastructure, not identity — a neutral default is fine.

## Why This Is a Problem

**TrustDomain is identity.** It defines the naming authority for the entire deployment. Two brokers that both silently default to `"agentauth.local"` can't distinguish each other's agents. An operator who forgets to set it gets a deployment that looks identical to every other default deployment.

**The fix isn't a different default — it's NO default.** Fail fast. Tell the operator to set it.

DBPath is lower-stakes — it's just a filename. Neutral default like `"./data.db"` is fine.

## Solution

### TrustDomain → Required (unset = startup failure)

```go
// In Load():
TrustDomain: os.Getenv("AA_TRUST_DOMAIN"),

// After populating all fields:
if c.TrustDomain == "" {
    return Cfg{}, fmt.Errorf(
        "AA_TRUST_DOMAIN is required. Set it to the SPIFFE trust domain for this " +
        "deployment (e.g. 'agentwrit.local' for dev, 'prod.example.com' for production).",
    )
}
```

No default. No fallback. The operator MUST declare who they are.

### DBPath → Neutral default `"./data.db"`

```go
DBPath: envOr("AA_DB_PATH", "./data.db"),
```

DBPath is infrastructure — a generic default is fine. The brand has no business being in a filename.

## Files Changed

| File | Change |
|------|--------|
| `internal/cfg/cfg.go` | `TrustDomain` from env only (no default), `DBPath` default → `"./data.db"` |
| `internal/cfg/cfg_test.go` | Update tests: `TrustDomain` required, `DBPath` = `"./data.db"` |
| Tests that create `cfg.Cfg` fixtures | All test files must now explicitly set `TrustDomain` in their fixture |

## Migration Note

**BREAKING CHANGE.** Two fields:

1. `AA_TRUST_DOMAIN` is now required. Existing deployments must set it. Suggested value for backward compatibility: `AA_TRUST_DOMAIN=agentauth.local` (preserves existing SPIFFE IDs). New deployments should use their own domain.
2. `AA_DB_PATH` default changes from `./agentauth.db` to `./data.db`. Existing deployments that relied on the default should set `AA_DB_PATH=./agentauth.db` explicitly, or rename the file.

## Verification

1. Start broker without `AA_TRUST_DOMAIN` → FATAL with clear error message
2. Start broker with `AA_TRUST_DOMAIN=agentwrit.local` → agents get `spiffe://agentwrit.local/...`
3. Start broker without `AA_DB_PATH` → uses `./data.db`
4. Start broker with `AA_DB_PATH=/custom/path.db` → uses `/custom/path.db`
5. `go test ./...` — all tests pass (all fixtures set `TrustDomain` explicitly)
