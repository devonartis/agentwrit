# BUG-CLI-002 — `awrit init` writes config to wrong path

**ID:** BUG-CLI-002  
**Severity:** HIGH  
**Introduced in:** commit `4e197a5` (`fix(cli): rename aactl binary to awrit (TD-CLI-001)`)  
**Linked TD:** TD-CLI-002  
**Status:** Resolved 2026-04-12 — fixed on `fix/rebrand-runtime-doc-alignment`

---

## Summary

`awrit init` wrote the generated config file to `~/.agentauth/config`. The broker's
config auto-loader reads from `~/.broker/config`. These paths do not match. A user
who runs `awrit init` without `--config-path` ends up with a config file the broker
silently ignores, causing the broker to start with no admin secret and exit with a FATAL.

---

## Root Cause

Two commits touched the same logical concern but were not reconciled:

| Commit | File | What it did |
|--------|------|-------------|
| `07daa20` (TD-CFG-002) | `internal/cfg/configfile.go` | Updated broker auto-load search paths: `/etc/agentauth/config` → `/etc/broker/config`, `~/.agentauth/config` → `~/.broker/config` |
| `4e197a5` (TD-CLI-001) | `cmd/awrit/init_cmd.go` | Created `resolveConfigPath()` with the **old paths**: `/etc/agentauth/config` and `~/.agentauth/config` |

`07daa20` landed first. `4e197a5` was created after it, but introduced new code using
the stale paths — they were never synced.

---

## Affected Code

**File:** `cmd/awrit/init_cmd.go`  
**Function:** `resolveConfigPath()` (lines 53–64)

```go
func resolveConfigPath() string {
    if initConfigPath != "" {
        return initConfigPath
    }
    if p := os.Getenv("AA_CONFIG_PATH"); p != "" {
        return p
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "/etc/agentauth/config"   // ← should be /etc/broker/config
    }
    return home + "/.agentauth/config"  // ← should be /.broker/config
}
```

**Broker reads from** (`internal/cfg/configfile.go:24`):
```go
locs = append(locs, filepath.Join(home, ".broker", "config"))
```

---

## Impact

A user following the getting-started guide:

1. Runs `awrit init` → config written to `~/.agentauth/config`
2. Starts broker → broker searches `~/.broker/config`, finds nothing
3. `AA_ADMIN_SECRET` is unset → broker exits FATAL: `admin secret required`

The `--config-path` flag works around this, but no getting-started doc tells users
they need it because the default is supposed to Just Work.

---

## Fix

**File:** `cmd/awrit/init_cmd.go:53–64`

```go
func resolveConfigPath() string {
    if initConfigPath != "" {
        return initConfigPath
    }
    if p := os.Getenv("AA_CONFIG_PATH"); p != "" {
        return p
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "/etc/broker/config"
    }
    return home + "/.broker/config"
}
```

Two literal changes plus a unit test in `cmd/awrit/init_cmd_test.go`.

---

## Verification

After the fix, run:

```bash
# Build
go build -o bin/awrit ./cmd/awrit

# Run init
./bin/awrit init --mode dev

# Confirm path
ls -la ~/.broker/config           # must exist
cat ~/.broker/config              # must contain ADMIN_SECRET=...

# Start broker (reads ~/.broker/config automatically)
./bin/broker
# Expect: broker starts, no FATAL on admin secret
```

Unit test: `internal/cfg/configfile_test.go` already tests that `~/.broker/config`
is in the search path. Add a test in `cmd/awrit/init_cmd_test.go` (or similar)
that calls `resolveConfigPath()` with `HOME` set and asserts the returned path
ends in `.broker/config`.

---

## Branch

Fixed on `fix/rebrand-runtime-doc-alignment` off the updated docs branch/develop.
Needs: PR review and merge.
