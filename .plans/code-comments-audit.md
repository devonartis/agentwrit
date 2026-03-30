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
| 8 | `internal/handler/val_hdl.go` | 6 | done — explains token introspection purpose for apps/resource servers |
| 9 | `internal/handler/reg_hdl.go` | 3 | done — agent entry point, launch token flow |
| 10 | `internal/handler/renew_hdl.go` | 4 | done — session extension, SEC-A1 TTL carry-forward |
| 11 | `internal/handler/revoke_hdl.go` | 5 | done — admin kill switch, 4 revocation levels |
| 12 | `internal/handler/release_hdl.go` | 3 | done — already clear (self-revocation) |
| 13 | `internal/handler/challenge_hdl.go` | 4 | done — step 1 of registration flow |
| 14 | `internal/handler/audit_hdl.go` | 4 | done — already clear |
| 15 | `internal/handler/deleg_hdl.go` | 3 | done — scope attenuation, delegation chain provenance |
| 16 | `internal/handler/health_hdl.go` | 5 | done — already clear |
| 17 | `internal/handler/security_hdl.go` | 1 | done — already clear |
| 18 | `internal/handler/metrics_hdl.go` | 1 | done — already clear |
| 19 | `internal/handler/logging.go` | 4 | done — already clear |
| 20 | `internal/handler/doc.go` | 0 | done — reorganized by audience (public/agent/admin) |
| 21 | `cmd/broker/main.go` | 3 | done — already excellent route table |
| 22 | `cmd/broker/serve.go` | 3 | done — timeout rationale, cipher suite reasoning |

## Chunk 3: Authorization & Revocation
*Security-critical paths.*

| # | File | Symbols | Status |
|---|------|---------|--------|
| 23 | `internal/authz/scope.go` | 3 | done — already clear (scope model, attenuation rule) |
| 24 | `internal/authz/val_mw.go` | 13 | done — already clear (middleware chain, scope checks) |
| 25 | `internal/authz/rate_mw.go` | 7 | done — fixed package comment, explained per-client_id purpose |
| 26 | `internal/revoke/rev_svc.go` | 7 | done — already excellent (4-level revocation, persistence) |
| 27 | `internal/audit/audit_log.go` | 18 | done — added event constant intent, rest already solid |

## Chunk 4: Supporting Infrastructure

| # | File | Symbols | Status |
|---|------|---------|--------|
| 28 | `internal/store/sql_store.go` | 37 | done — package doc + SqlStore + AppRecord updated for dual storage model |
| 29 | `internal/deleg/deleg_svc.go` | 8 | done — already excellent (attenuation, chain, depth limits) |
| 30 | `internal/identity/id_svc.go` | 9 | done — already excellent (10-step registration flow) |
| 31 | `internal/identity/spiffe.go` | 2 | done — already clear |
| 32 | `internal/cfg/cfg.go` | 5 | done — already excellent (env vars, bcrypt, security notes) |
| 33 | `internal/cfg/configfile.go` | 4 | done — already clear (symlink rejection, permission checks) |

## Chunk 5: Low Priority

| # | File | Symbols | Status |
|---|------|---------|--------|
| 34 | `internal/keystore/keystore.go` | 3 | done — already clear (PEM, O_EXCL, 0600 permissions) |
| 35 | `internal/mutauth/mut_auth_hdl.go` | 8 | done — already excellent (3-step handshake protocol) |
| 36 | `internal/mutauth/discovery.go` | 6 | done — already clear (binding, identity consistency) |
| 37 | `internal/mutauth/heartbeat.go` | 8 | done — already clear (liveness, auto-revocation) |
| 38 | `internal/obs/obs.go` | 9 | done — already excellent (logging levels, metrics reference) |
| 39 | `internal/problemdetails/problemdetails.go` | 8 | done — already clear (RFC 7807, MaxBytesBody, RequestID) |
| 40 | `cmd/aactl/apps.go` | 1 | done — already clear (cobra docs per command) |
| 41 | `cmd/aactl/audit.go` | 1 | done — already clear |
| 42 | `cmd/aactl/client.go` | 9 | done — already clear (auth flow, doPostWithToken purpose) |
| 43 | `cmd/aactl/init_cmd.go` | 3 | done — already clear (dev vs prod mode) |
| 44 | `cmd/aactl/main.go` | 1 | done — already clear |
| 45 | `cmd/aactl/output.go` | 2 | done — already clear |
| 46 | `cmd/aactl/revoke.go` | 1 | done — already clear |
| 47 | `cmd/aactl/root.go` | 2 | done — already clear |
| 48 | `cmd/aactl/token.go` | 1 | done — already clear (self-revocation, idempotency) |
