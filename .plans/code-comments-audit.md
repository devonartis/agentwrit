# Code Comments Audit (TD-014)

**Goal:** Update all Go source comments in `internal/` and `cmd/` to the standard defined in `.claude/rules/golang.md`.

**Standard:** Comments tell you what reading the code alone would NOT tell you. Keep them natural — not a template, not a book. Focus on:
- **Intent** — why does this exist, what's the business/security reason
- **Context** — whom is this for, what role or flow triggers it
- If a design choice looks odd, explain why it's intentional (e.g. "see TD-013")

**Scope:** Source files only. Test files excluded (tests are self-documenting).

**Verification:** `go build ./...` after each chunk.

---

## Chunk 1: Core Token Flow — CRITICAL
*Where B6's role confusion happened. The Admin/App/Agent role model lives here.*

| # | File | Symbols | Status |
|---|------|---------|--------|
| 1 | `internal/token/tkn_svc.go` | 14 | done — minor tweaks, already well-commented from B6 |
| 2 | `internal/token/tkn_claims.go` | 3 | done — already clear, no changes needed |
| 3 | `internal/token/revoker.go` | 1 | done — already clear, no changes needed |
| 4 | `internal/app/app_svc.go` | 15 | done — package doc, AppSvc, UpdateApp, DeregisterApp, ListApps |
| 5 | `internal/app/app_hdl.go` | 17 | done — package doc, AppHdl, all handlers |
| 6 | `internal/admin/admin_svc.go` | 10 | done — package doc, AdminSvc, CreateLaunchToken |
| 7 | `internal/admin/admin_hdl.go` | 8 | done — AdminHdl, handleAuth, handleCreateLaunchToken |

## Chunk 2: HTTP Layer
*Where roles meet endpoints. Who hits what, with what scope.*

| # | File | Symbols | Status |
|---|------|---------|--------|
| 8 | `internal/handler/val_hdl.go` | 6 | pending |
| 9 | `internal/handler/reg_hdl.go` | 3 | pending |
| 10 | `internal/handler/renew_hdl.go` | 4 | pending |
| 11 | `internal/handler/revoke_hdl.go` | 5 | pending |
| 12 | `internal/handler/release_hdl.go` | 3 | pending |
| 13 | `internal/handler/challenge_hdl.go` | 4 | pending |
| 14 | `internal/handler/audit_hdl.go` | 4 | pending |
| 15 | `internal/handler/deleg_hdl.go` | 3 | pending |
| 16 | `internal/handler/health_hdl.go` | 5 | pending |
| 17 | `internal/handler/security_hdl.go` | 1 | pending |
| 18 | `internal/handler/metrics_hdl.go` | 1 | pending |
| 19 | `internal/handler/logging.go` | 4 | pending |
| 20 | `internal/handler/doc.go` | 0 | pending |
| 21 | `cmd/broker/main.go` | 3 | pending |
| 22 | `cmd/broker/serve.go` | 3 | pending |

## Chunk 3: Authorization & Revocation
*Security-critical paths.*

| # | File | Symbols | Status |
|---|------|---------|--------|
| 23 | `internal/authz/scope.go` | 3 | pending |
| 24 | `internal/authz/val_mw.go` | 13 | pending |
| 25 | `internal/authz/rate_mw.go` | 7 | pending |
| 26 | `internal/revoke/rev_svc.go` | 7 | pending |
| 27 | `internal/audit/audit_log.go` | 18 | pending |

## Chunk 4: Supporting Infrastructure

| # | File | Symbols | Status |
|---|------|---------|--------|
| 28 | `internal/store/sql_store.go` | 37 | pending |
| 29 | `internal/deleg/deleg_svc.go` | 8 | pending |
| 30 | `internal/identity/id_svc.go` | 9 | pending |
| 31 | `internal/identity/spiffe.go` | 2 | pending |
| 32 | `internal/cfg/cfg.go` | 5 | pending |
| 33 | `internal/cfg/configfile.go` | 4 | pending |

## Chunk 5: Low Priority

| # | File | Symbols | Status |
|---|------|---------|--------|
| 34 | `internal/keystore/keystore.go` | 3 | pending |
| 35 | `internal/mutauth/mut_auth_hdl.go` | 8 | pending |
| 36 | `internal/mutauth/discovery.go` | 6 | pending |
| 37 | `internal/mutauth/heartbeat.go` | 8 | pending |
| 38 | `internal/obs/obs.go` | 9 | pending |
| 39 | `internal/problemdetails/problemdetails.go` | 8 | pending |
| 40 | `cmd/aactl/apps.go` | 1 | pending |
| 41 | `cmd/aactl/audit.go` | 1 | pending |
| 42 | `cmd/aactl/client.go` | 9 | pending |
| 43 | `cmd/aactl/init_cmd.go` | 3 | pending |
| 44 | `cmd/aactl/main.go` | 1 | pending |
| 45 | `cmd/aactl/output.go` | 2 | pending |
| 46 | `cmd/aactl/revoke.go` | 1 | pending |
| 47 | `cmd/aactl/root.go` | 2 | pending |
| 48 | `cmd/aactl/token.go` | 1 | pending |
