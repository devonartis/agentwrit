# Finding 02: JWT Audience Default Should Not Carry a Brand

**ID:** TD-TOKEN-002
**Severity:** MEDIUM (bad default, not a hardcode)
**Status:** UNRESOLVED
**Audit reference:** C2-3

---

## The Principle

**Production software should not silently inject branded values.** If a value is configurable, its default should be either neutral or empty — never the product name.

The audience field has a config path (`AA_AUDIENCE`), so this is NOT a hardcode. But the default when unset is `"agentauth"`, which silently ships the brand.

## Why This Is a Problem

1. **The default forces the brand into every token's `aud` claim.** An operator who never sets `AA_AUDIENCE` gets tokens with `aud: "agentauth"` — a brand they may not be using.
2. **The comment says "empty = skip"** but you can only get empty by explicitly setting the env var to empty string. The natural state (unset) gives you the branded default instead. This violates the principle of least surprise.
3. **If you're rebranding to AgentWrit, every existing deployment that doesn't set `AA_AUDIENCE` will continue to emit `"agentauth"` in the audience claim.**

## Solution

### Option A: Honor the contract — unset = empty = skip (RECOMMENDED)

Change the default to empty string. Unset `AA_AUDIENCE` means "no audience check":

```go
if v, ok := os.LookupEnv("AA_AUDIENCE"); ok {
    c.Audience = v // could be empty string → skip check
} else {
    c.Audience = "" // no default → skip check
}
```

This is the simplest fix and matches what the comment already promises. Operators who want audience validation must explicitly set `AA_AUDIENCE=agentwrit` (or whatever they want).

**Risk:** Some deployments may depend on the audience check happening by default. If they upgrade and don't set `AA_AUDIENCE`, the check stops happening. This is unlikely to be a real risk since the audience check is a defense-in-depth measure — the real security is the signature + issuer + expiry + revocation checks.

### Option B: Require `AA_AUDIENCE` (like `AA_ISSUER`)

Make it required, like the issuer. But this seems too strict — audience validation is optional in JWT spec (RFC 7519 §4.1.3 lists it as "RECOMMENDED", not "REQUIRED").

### Go with Option A.

## Files Changed

| File | Change |
|------|--------|
| `internal/cfg/cfg.go` | Change default from `"agentauth"` to `""`; update doc comment |
| `internal/cfg/cfg_test.go` | Update test at line 65: `c.Audience == ""` after defaults load |
| Any code that checks `c.Audience` | Verify it handles empty string correctly (skip the check) |

## Verification

1. `go test ./internal/cfg/...` — `c.Audience` is `""` when `AA_AUDIENCE` unset
2. `go test ./internal/cfg/...` — `c.Audience` is `"agentwrit"` when `AA_AUDIENCE=agentwrit`
3. `go test ./internal/cfg/...` — `c.Audience` is `""` when `AA_AUDIENCE=""`
4. Verify audience check is skipped when empty (no tokens rejected for audience mismatch)
5. Verify audience check works when set (tokens with wrong audience rejected)
