# Finding 06: Test SPIFFE Fixtures Leak Brand

**ID:** TD-TEST-001
**Severity:** MEDIUM (inconsistency, not a bug)
**Status:** UNRESOLVED
**Audit reference:** C4

---

## What Was Found

Some test files use `spiffe://agentauth.local/...` in their test fixtures, while others use the neutral `spiffe://test/...` or `spiffe://test.local/...`. The inconsistency means a find-and-replace for the rebrand could miss some occurrences or produce confusing diffs.

### Files using `agentauth.local` (needs cleanup)

| File | Approximate count | Example |
|------|-------------------|---------|
| `internal/token/tkn_svc_test.go` | ~30 | `spiffe://agentauth.local/agent/orch-1/task-1/abc123` |
| `internal/mutauth/mut_auth_hdl_test.go` | ~8 | `spiffe://agentauth.local/agent/orch-1/task-1/inst-a` |
| `internal/mutauth/heartbeat_test.go` | 6 | Same shape |
| `internal/mutauth/discovery_test.go` | 6 | Same shape |
| `internal/identity/id_svc_test.go` | 2 | `TrustDomain: "agentauth.local"` |
| `internal/admin/admin_hdl_test.go` | 1 | `spiffe://agentauth.local/agent/orch/task/inst` |

### Files already using neutral test domains (correct, no change needed)

| File | Convention |
|------|-----------|
| `internal/store/sql_store_test.go` | `spiffe://test/agent/...` |
| `internal/store/sql_store_revoke_test.go` | `spiffe://test/agent/...` |
| `internal/deleg/deleg_svc_test.go` | `spiffe://test/agent/o/t/...` |
| `internal/revoke/rev_svc_test.go` | `spiffe://example/agent/...` |
| `internal/handler/handler_test.go` | `spiffe://test.local/...` and `spiffe://test/...` |
| `internal/authz/val_mw_test.go` | Mostly `spiffe://test/agent/...` |

## Why This Is a Problem

1. **Search-and-replace hazard during rebrand.** If someone does a global replace of `agentauth.local` → `agentwrit.local`, these test strings get changed unnecessarily. If they DON'T do the global replace, these strings remain as the only `agentauth` references in the codebase — confusing for future readers.
2. **Inconsistency.** Half the tests use neutral domains, half use the brand. No reason for the difference.
3. **Tests should be brand-agnostic.** Test fixtures should verify behavior, not carry the product name.

## Solution

**Purely mechanical sed replacement.** Replace `agentauth.local` with `test.local` in all test files:

```bash
# One-liner for all test files:
sed -i '' 's/agentauth\.local/test.local/g' internal/token/tkn_svc_test.go \
    internal/mutauth/mut_auth_hdl_test.go \
    internal/mutauth/heartbeat_test.go \
    internal/mutauth/discovery_test.go \
    internal/identity/id_svc_test.go \
    internal/admin/admin_hdl_test.go
```

Also update the `TrustDomain` values in the `cfg.Cfg` fixtures within those tests:

```go
// Before:
cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300}

// After:
cfg.Cfg{TrustDomain: "test.local", DefaultTTL: 300}
```

**Wait until Finding 03 (configurable TrustDomain) is resolved first.** If TrustDomain becomes required, all test fixtures will need to set it explicitly anyway. Doing this cleanup at the same time avoids touching the same files twice.

## Files Changed

| File | Change |
|------|--------|
| `internal/token/tkn_svc_test.go` | `agentauth.local` → `test.local` in ~30 SPIFFE URIs + TrustDomain fixture |
| `internal/mutauth/mut_auth_hdl_test.go` | `agentauth.local` → `test.local` in ~8 URIs + TrustDomain fixture |
| `internal/mutauth/heartbeat_test.go` | `agentauth.local` → `test.local` in 6 URIs |
| `internal/mutauth/discovery_test.go` | `agentauth.local` → `test.local` in 6 URIs |
| `internal/identity/id_svc_test.go` | `agentauth.local` → `test.local` in TrustDomain arg |
| `internal/admin/admin_hdl_test.go` | `agentauth.local` → `test.local` in 1 URI |

## Verification

1. `grep -rn 'agentauth\.local' internal/ --include='*_test.go'` → returns zero results
2. `go test ./...` — all tests pass
3. `grep -rn 'test\.local\|spiffe://test/' internal/ --include='*_test.go'` → consistent usage across all test files
