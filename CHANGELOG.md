# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added — B6 (SEC-A1 + Gates): TTL Carry-Forward on Renewal

- **TTL carry-forward on renewal** (`internal/token/tkn_svc.go`): Renew now preserves the original token's TTL instead of falling back to DefaultTTL. Computes `originalTTL = Exp - Iat` from the predecessor token and passes it to Issue. MaxTTL clamp still applies. Closes a privilege escalation where an agent issued with TTL=120 could renew and get TTL=300 (the default).
- **gates.sh regression subcommand** (`scripts/gates.sh`): New `regression` mode discovers and runs all `tests/*/regression.sh` scripts, reports per-phase pass/fail summary.
- **Regression unit tests**: `TestRenew_PreservesTTL` with 5 subtests — custom TTL preserved, default TTL preserved, MaxTTL clamps, under-limit unchanged, escalation-to-default blocked.

#### Cherry-Pick Details

- Source: 2 commits from `agentauth` legacy repo (B6 batch — final batch)
- Conflicts resolved: `tkn_svc.go` — incoming commit had `AppID`, `AppName`, `OriginalPrincipal` fields not present in core's `IssueReq` struct. Kept TTL fix, dropped three add-on fields.
- Contamination: CLEAN

#### Tech Debt Discovered

- TD-012 (CRITICAL): Missing role model documentation — no doc explains Admin vs App vs Agent roles, scopes, or boundaries
- TD-013 (HIGH): `POST /v1/admin/launch-tokens` lets admin create agents without scope ceiling enforcement
- TD-014 (CRITICAL): Code comments across `internal/` don't explain roles, scopes, or security boundaries

### Added — B5 (SEC-L2b): HTTP Security Hardening

- **SecurityHeaders middleware** (`internal/handler/security_hdl.go`): Sets `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `X-Frame-Options: DENY` on all responses. Conditionally adds HSTS (`Strict-Transport-Security: max-age=63072000; includeSubDomains`) when TLS mode is `tls` or `mtls`. Handlers can override `Cache-Control` (last-writer-wins).
- **Global MaxBytesBody**: Moved from per-route wrappers to global middleware stack. All endpoints now enforce 1MB request body limit. Returns 413 `Request Entity Too Large` for oversized payloads. Eager body buffering prevents streaming JSON decoders from bypassing the limit.
- **Error sanitization — val_hdl**: Token validation errors return generic `"token is invalid or expired"` instead of leaking internal error details. Full error logged with `request_id` for debugging.
- **Error sanitization — renew_hdl**: Token renewal errors return generic `"token renewal failed"` instead of leaking internal error details. Full error logged to audit trail.
- **Error sanitization — ValMw**: Authentication middleware errors return generic `"token verification failed"` instead of leaking internal error details. Full error logged to audit trail.
- **Global middleware ordering**: `mux → SecurityHeaders → MaxBytesBody → LoggingMiddleware → RequestIDMiddleware` (outermost to innermost).

#### Cherry-Pick Details

- Source: 5 commits from `agentauth` legacy repo (B5 batch)
- Applied: 4 commits (1 skipped as empty — content already present from prior batch)
- Conflicts resolved: `main.go` (OIDC/cloud routes dropped), `handler_test.go` (kept comprehensive test suite from prior commits), `renew_hdl.go` (kept obs.Warn log line)
- Contamination: CLEAN (zero HITL/OIDC/cloud/federation references)

### Changed — Documentation Accuracy and Pre-SDK Integration Guidance

- Corrected public documentation to match the current broker contract for agent registration, renewal, release, app authentication, launch token creation, and health responses.
- Added explicit pre-SDK guidance for Go developers: direct HTTP integration is the supported path today, with end-to-end Go examples for registration, token renewal, and token release in `docs/getting-started-developer.md`.
- Updated public-facing docs to reflect persistent signing key behavior, current `AACTL_*` operator environment variables, current health payload fields, and the current `2.0.0` broker version reported by the running service.
- Fixed multiple copy/paste examples that previously used stale payload shapes, stale response examples, or outdated audit event names.

### Added — B4 (SEC-L2a): Token Hardening

- `AA_MAX_TTL` configuration: maximum token lifetime ceiling (default 86400s, set to 0 to disable)
- JWT algorithm validation: broker rejects tokens with alg != EdDSA (prevents CVE-2015-9235 algorithm confusion)
- JWT key ID validation: broker rejects tokens with mismatched kid (prevents cross-broker replay)
- Revocation check in Verify(): defense-in-depth — every code path that validates a token also checks revocation
- Transactional renewal: predecessor token revoked BEFORE issuing new token; renewal fails if revocation fails
- Startup warning when DefaultTTL > MaxTTL (silent clamping detection)
- Empty kid backward compatibility: tokens issued before B4 (without kid) are still accepted
- Error sanitization: renewal handler returns generic error to client, logs full details internally

### Added — P1: Admin Secret (Bcrypt + `aactl init`)

- `aactl init` command for secure admin secret generation (`--mode=dev|prod`, `--force`, `--config-path`)
- Config file support: KEY=VALUE format at `AA_CONFIG_PATH` > `/etc/agentauth/config` > `~/.agentauth/config`
- Config file permissions: 0600 (file), 0700 (directory)
- `Cfg.Mode`, `Cfg.AdminSecretHash`, `Cfg.ConfigPath` fields
- Bcrypt hashing for admin secret comparison (always-on, replaces constant-time byte comparison)
- Dev mode: plaintext in config, bcrypt hash derived at startup
- Prod mode: only bcrypt hash stored, plaintext shown once during `aactl init`
- `golang.org/x/crypto/bcrypt` promoted from indirect to direct dependency
- Backward compatible: `AA_ADMIN_SECRET` env var continues to work, overrides config file

### Added — Documentation Enhancement (Session 42 cont.)

**Enhanced mermaid diagrams across all docs:**
- All mermaid blocks across documentation files upgraded with professional styling
- Added `classDef` color-coded classes, emoji indicators, `rect` phase backgrounds, styled subgraphs
- Consistent color palette: green (success), blue (security/info), orange (operations), red (errors), purple (admin), yellow (decisions)

**Godoc improvements:**
- Added package and function documentation to `cmd/aactl/client.go`
- Added field-level comments to handler request/response structs
- Improved documentation on `internal/approval/` package exports
- Added godoc to `cmd/aactl/apps.go` command functions

### Added — Phase 1C-beta: Human-in-the-Loop (HITL) Approval

See `docs/concepts.md` HITL section and `docs/integration-patterns.md` Pattern 7 for background.

**New endpoints (all require `app:launch-tokens:*` scope):**
- `GET /v1/app/approvals` — list pending/resolved HITL approval requests (filter by status, limit)
- `GET /v1/app/approvals/{id}` — get single approval request
- `POST /v1/app/approvals/{id}/approve` — submit human approval with `principal` identity
- `POST /v1/app/approvals/{id}/deny` — submit denial with optional `reason`

**Changed: `POST /v1/app/launch-tokens`:**
- Returns `403 hitl_approval_required` (JSON body with `approval_id`, `expires_at`) when `allowed_scope` overlaps the app's `hitl_scopes` and no `approval_token` supplied
- New `approval_token` request field: one-time-use broker-signed JWT from approve endpoint; 300s TTL; scope-locked + app-locked
- Admin JWT callers bypass HITL gate unconditionally

**New config:** `AA_HITL_APPROVAL_TTL` (approval request TTL in seconds, default 300)

**New audit events:** `hitl_approval_required`, `hitl_approved`, `hitl_denied`, `hitl_expired`

**New packages:** `internal/approval` — `ApprovalSvc` (8 methods) + `ApprovalHdl` (4 HTTP handlers), 35 tests

**Store changes:**
- `approval_requests` SQLite table with FK to apps, JSON scope, RFC3339 timestamps, consumed flag
- `LaunchTokenRecord.OriginalPrincipal` — human identity flows approval → launch token → agent JWT `original_principal` claim

**Background goroutine:** prunes expired pending approvals every 30s, emits `hitl_expired` audit events

**Documentation updated:** api.md, concepts.md, getting-started-developer.md (Python HITL example), getting-started-operator.md (hitl_scopes config), common-tasks.md (HITL task), integration-patterns.md (Pattern 7), architecture.md (route table)



### Added — Fix: App Launch Tokens Endpoint

- `POST /v1/app/launch-tokens` — developer-facing endpoint for creating launch tokens. Requires `app:launch-tokens:*` scope. Same handler as admin endpoint but under the correct namespace.

### Changed — Fix: App Launch Tokens Endpoint

- `POST /v1/admin/launch-tokens` — now requires `admin:launch-tokens:*` scope only. App JWTs (`app:launch-tokens:*`) are no longer accepted. Developers should use `POST /v1/app/launch-tokens`.

### Fixed — Fix: App Launch Tokens Endpoint

- `POST /v1/app/auth` response documentation corrected to show actual JWT scopes (`app:launch-tokens:*`, `app:agents:*`, `app:audit:read`) instead of the app's data scope ceiling.
- `docs/integration-patterns.md` — replaced all references to non-existent endpoints (`/v1/apps/{client_id}/launch-tokens`, `/v1/apps/{client_id}/token`).
- `docs/getting-started-*.md` — all developer examples now use `/v1/app/launch-tokens`.
- `docs/examples/customer-support.md` — fixed non-existent endpoint paths and incorrect registration flow.

### Added — Phase 1C-alpha: JWT Claims + Token Hygiene

- `app_id`, `app_name`, `original_principal` claims in agent JWTs (Stories 6, 9, 11)
- `original_principal` propagates unchanged through delegation chain
- Token predecessor revocation on renewal (Story 17, resolves TD-008)
- JTI blocklist pruning — background goroutine removes expired entries (Story 18, resolves TD-009)
- Agent record expiry — agents marked expired when token TTL elapses (Story 19)
- Validate endpoint returns new claims automatically via `omitempty` struct tags (Story 12)
### Added — Phase 1B: App-Scoped Launch Tokens

**Summary:** Developers can now create launch tokens using their app credentials instead of
requesting them from the operator. The broker enforces each app's scope ceiling on launch
token creation, preventing scope escalation. Agents registered via app-created launch tokens
carry full `app_id` traceability through the audit trail.

**11/11 user stories verified via Docker live tests.** Evidence: `tests/phase-1b/evidence/`

#### New: `internal/handler/app_launch_hdl.go`
- `AppLaunchHdl` — handles `POST /v1/admin/launch-tokens` with app JWT Bearer auth
- Extracts `app_id` and app scope ceiling from JWT claims
- Validates requested scopes are a subset of the app's ceiling via `authz.ScopeIsSubset()`
- On ceiling violation: returns 403 with explanation, records `scope_ceiling_exceeded` audit event
- On success: delegates to `AdminSvc.CreateLaunchToken()` with `app_id` annotation

#### Modified: `internal/admin/admin_svc.go`
- `CreateLaunchToken()` now accepts optional `app_id` parameter
- Launch token policy carries `app_id` when created by an app (nil for admin-created tokens)
- `app_id` propagated to audit events: `launch_token_issued` detail includes `created_by=app:<id>`

#### Modified: `internal/identity/id_svc.go`
- `Register()` propagates `app_id` from launch token policy to agent record
- Audit events `agent_registered` and `token_issued` include `app_id` when present

#### Modified: `internal/authz/val_mw.go`
- App JWT tokens (with `app:launch-tokens:*` scope) routed to `AppLaunchHdl` for launch token creation
- Admin tokens continue to use the existing unrestricted path (backward compatible)

#### Modified: `cmd/broker/main.go`
- Wired `AppLaunchHdl` into the launch token route with scope-based dispatch

#### Verification Evidence (2026-03-04)
- Docker live tests: **11/11 PASS** — `./scripts/live_test.sh --docker`
- Stories S1-S3 (Developer): App auth → launch token → agent registration with app traceability
- Stories S4-S6 (Operator): Audit traceability, ceiling enforcement (3 cases), backward compat
- Stories S7-S8 (Security): Scope attenuation (4 cases incl. wildcard widening), full traceability chain
- Stories R1-R3 (Regression): App auth, admin endpoint blocking, hash chain integrity

### Added — New Enterprise Documentation

- **docs/integration-patterns.md**: 6 real-world integration patterns with mermaid diagrams,
  production Python code, security analysis, and dangerous-path comparisons: Multi-Agent Pipeline,
  Sidecar-Per-Microservice, Token Release as Task Completion, Delegation Chain with Scope Narrowing,
  Emergency Revocation Cascade, BYOK Registration
- **docs/aactl-reference.md**: Complete aactl CLI reference with all commands (`audit events`,
  `token release`, `revoke`, `sidecars list`, `sidecars ceiling get/set`), flags, output examples
  (table + JSON), common workflows, security best practices, and FAQ
- **docs/api/openapi.yaml**: Major update — added `POST /v1/token/release`, `GET /v1/admin/sidecars`,
  `GET/PUT /v1/admin/sidecars/{id}/ceiling`, `outcome` query parameter, structured audit fields,
  `token_released` and `scopes_ceiling_updated` event types, `db_connected`/`audit_events_count`
  in HealthResponse, sidecar management schemas. Fixed license (MIT → Apache 2.0), removed
  duplicate response keys

### Changed — Documentation Upgrade (Pre-Release Polish)

**Summary:** Comprehensive documentation review and upgrade for demo readiness.
All docs updated to reflect the current codebase including all 6 compliance fixes.

- **README.md**: Professional badge suite (Go Reference, Report Card, Docker, Security Policy,
  Ed25519, SPIFFE), added aactl CLI section, native TLS/mTLS production deployment guide,
  UDS mode documentation, missing endpoints (`/v1/token/release`, `/v1/admin/sidecars`),
  expanded configuration table (`AA_AUDIENCE`, `AA_TLS_*`), updated architecture diagram
  with aactl, added Store component to package table
- **docs/architecture.md**: Updated store description from "in-memory" to "hybrid persistence"
  (SQLite + in-memory), added listener.go and token.go to directory layout, added token
  release and admin sidecars to middleware stack, updated Pattern Components table with
  `RecordOption`/`ReleaseHdl`, corrected Security Assumptions for persistent revocations
- **docs/api.md**: Added `POST /v1/token/release` endpoint documentation, added `outcome`
  query parameter to audit events, added 5 structured audit fields to AuditEvent schema,
  added `token_released` event type (23 event types total)
- **docs/getting-started-operator.md**: Added `aactl token release` to quick reference,
  `--outcome` flag for audit events, new "Token Release" section
- **docs/getting-started-developer.md**: Added token release subsection with Python example
- **docs/common-tasks.md**: Added "Release a Token" task, outcome filter examples
- **docs/concepts.md**: Updated audit persistence description, structured fields,
  token lifecycle diagram with release state
- **docs/troubleshooting.md**: Added UDS socket permission troubleshooting, TLS/mTLS
  certificate diagnostics with openssl examples
- **Go code documentation**: Added package-level godoc to `cmd/sidecar` (19-line overview
  of features and configuration) and `internal/mutauth` (comprehensive protocol description
  with 3-step handshake, DiscoveryRegistry, HeartbeatMgr)

### Added — Fix 6: Structured Audit Log Fields (P1 Compliance)

**Summary:** Audit events now carry structured fields (`resource`, `outcome`, `deleg_depth`,
`deleg_chain_hash`, `bytes_transferred`) via a backward-compatible functional options pattern
(`RecordOption`). All ~20 `Record()` call sites annotated with `WithOutcome`. Hash chain
tamper evidence covers all structured fields. Outcome filtering available via query API and
`aactl audit events --outcome`.

#### Modified: `internal/audit/audit_log.go`
- `AuditEvent` struct: 5 new fields (`Resource`, `Outcome`, `DelegDepth`, `DelegChainHash`, `BytesTransferred`)
- `RecordOption` type + 5 option functions (`WithResource`, `WithOutcome`, `WithDelegDepth`, `WithDelegChainHash`, `WithBytesTransferred`)
- `Record()` accepts variadic `...RecordOption` — all existing callers unchanged
- `computeHash()` includes structured fields for tamper evidence
- `QueryFilters.Outcome` field + filter clause in `Query()`

#### Modified: `internal/store/sql_store.go`
- 5 new `audit_events` columns (`resource`, `outcome`, `deleg_depth`, `deleg_chain_hash`, `bytes_transferred`) with `DEFAULT NULL`
- `SaveAuditEvent`, `LoadAllAuditEvents`, `QueryAuditEvents` updated with nullable types
- `idx_audit_outcome` index for efficient outcome filtering

#### Modified: `internal/handler/audit_hdl.go`
- `outcome` query param mapped to `QueryFilters.Outcome`

#### Modified: `cmd/aactl/audit.go`
- `--outcome` flag on `aactl audit events` command
- `OUTCOME` column in table output

#### Modified: All `Record()` callers
- `authz/val_mw.go`, `identity/id_svc.go`, `deleg/deleg_svc.go`, `handler/*.go`, `admin/admin_svc.go`, `admin/admin_hdl.go`
- Every call site annotated with `WithOutcome("success"|"denied")`
- Delegation callers include `WithDelegDepth`, `WithDelegChainHash`
- Validation middleware includes `WithResource`

### Added — ADR-002: Sidecar Architecture Decision

**Summary:** Collaborative 4-agent architecture review resolved 6 open questions about the
sidecar model. Decision: keep sidecars as the primary and only current model. Direct broker
access deferred as future work (blocked by broker code changes). Admin secret blast radius
identified as highest priority security fix (KI-001).

- ADR-002: sidecar architecture decision document (archived during pre-release cleanup)
- `KNOWN-ISSUES.md` — 4 known issues: admin secret blast radius (KI-001), TCP default (KI-002),
  audit indistinguishability (KI-003), ephemeral registry (KI-004)

### Added — Fix 5: Sidecar UDS Listen Mode (P1 Compliance — Pattern v1.2 §3.3)

**Summary:** The sidecar previously only listened on TCP, exposing agent-to-sidecar
traffic on the network and requiring unique port allocation per sidecar. This fix adds
`AA_SOCKET_PATH` — when set, the sidecar listens on a Unix domain socket instead of TCP.
Socket permissions (`0660`) restrict access to owner + group. Stale sockets are cleaned
on startup. TCP fallback logs a WARN for operator awareness.

#### New: `cmd/sidecar/listener.go`
- `startListener()` — creates UDS or TCP listener based on `socketPath` parameter.
- UDS mode: removes stale socket, binds, sets `0660` permissions, returns cleanup func.
- TCP mode: binds on port, logs WARN about network exposure.

#### Modified: `cmd/sidecar/config.go`
- New `SocketPath` field on `sidecarConfig`, loaded from `AA_SOCKET_PATH`.

#### Modified: `cmd/sidecar/main.go`
- Replaced inline `http.ListenAndServe` with `startListener()` + `http.Serve`.
- Deferred socket cleanup on shutdown.

#### Modified: `docker-compose.yml`
- `AA_SOCKET_PATH` env var passed through to sidecar container.

#### Modified: `docs/getting-started-operator.md`
- `AA_SOCKET_PATH` added to sidecar configuration table.
- New "Unix domain socket (UDS) mode" section with examples.

#### New: `cmd/sidecar/listener_test.go`
- `TestStartListener_UDS` — creates socket, connects via UDS client, verifies HTTP response.
- `TestStartListener_TCP` — verifies TCP fallback with port 0.
- `TestStartListener_UDS_CleansUpStaleSocket` — verifies stale socket replacement.

#### New: `cmd/sidecar/listener_integration_test.go`
- `TestMultiSidecarUDS` — two sidecars on different UDS paths, concurrent client access.

### Added — Fix 1 (Sidecar): TLS/mTLS Client Support (P0 Compliance — Pattern v1.2 §3.3)

**Summary:** The broker-side TLS was already on `develop`, but the sidecar had no TLS
client support — it always connected over plain HTTP. This fix completes the TLS story
by adding client-side TLS to the sidecar's broker client. The sidecar can now verify
the broker's cert (one-way TLS) and present its own client cert (mTLS).

#### Modified: `cmd/sidecar/config.go`
- New `CACert`, `TLSCert`, `TLSKey` fields on `sidecarConfig`.
- Loaded from `AA_SIDECAR_CA_CERT`, `AA_SIDECAR_TLS_CERT`, `AA_SIDECAR_TLS_KEY`.

#### Modified: `cmd/sidecar/broker_client.go`
- `newBrokerClient()` now accepts CA cert, client cert, and client key paths.
- New `buildTLSConfig()` — builds `tls.Config` with CA trust pool and optional client cert.
- TLS 1.3 minimum enforced. Falls back to plain HTTP on config errors.

#### Modified: `cmd/sidecar/main.go`
- Passes TLS config from `sidecarConfig` to `newBrokerClient()`.

#### Modified: `docker-compose.yml`
- `AA_SIDECAR_CA_CERT`, `AA_SIDECAR_TLS_CERT`, `AA_SIDECAR_TLS_KEY` env vars for sidecar.

#### New: `docker-compose.tls.yml`, `docker-compose.mtls.yml`
- Compose overlays for TLS and mTLS Docker testing.

#### New: `scripts/gen_test_certs.sh`
- Generates CA, broker, and sidecar certs (ECDSA P-256, SHA-256) for testing.

#### Modified: `docs/getting-started-operator.md`
- Sidecar TLS client env vars added to configuration table.
- New "Sidecar TLS client" section with one-way TLS and mTLS examples.

### Added — Fix 4: Token Release Endpoint (P1 Compliance — Pattern v1.2 §4.4)

**Summary:** Agents had no way to explicitly surrender tokens after task completion.
This fix adds `POST /v1/token/release` — an agent presents its Bearer token and the
handler self-revokes by JTI, records a `token_released` audit event, and returns 204.
Idempotent. No admin scope required. Includes `aactl token release` operator tooling.

#### New: `internal/handler/release_hdl.go`
- `ReleaseHdl` extracts claims from context, calls `revSvc.Revoke("token", jti)`.
- Records `token_released` audit event with agent/task/orch IDs.
- Returns 204 No Content on success.

#### Modified: `internal/audit/audit_log.go`
- New `EventTokenReleased` constant.

#### Modified: `internal/authz/val_mw.go`
- New `ContextWithClaims()` test helper for injecting claims into context.

#### Modified: `cmd/broker/main.go`
- Wired `POST /v1/token/release` through `valMw.Wrap()` (Bearer auth, no scope gate).

#### New: `cmd/aactl/token.go`
- `aactl token release --token <jwt>` — operator CLI for testing/force-releasing tokens.
- Handles 204 success and 403 "already revoked" as idempotent outcomes.

#### Modified: `cmd/aactl/client.go`
- New `doPostWithToken()` for agent-facing endpoints that use a caller-supplied token.

### Added — Fix 3: Audience Validation (P1 Compliance — Pattern v1.2 §3.1)

**Summary:** Tokens were issued without an `aud` (audience) claim, meaning a token from
one broker instance could be replayed against another. This fix adds `AA_AUDIENCE`
configuration, populates `aud` in all token issuance paths, and validates it in the
authentication middleware. Backward compatible — unset or empty `AA_AUDIENCE` skips
the check.

#### Modified: `internal/cfg/cfg.go`
- New `Audience` field on `Cfg` struct.
- `Load()` uses `os.LookupEnv` to distinguish unset (default "agentauth") from empty (skip).

#### Modified: `internal/authz/val_mw.go`
- `ValMw` gains `audience` field; `NewValMw()` accepts audience param.
- `Wrap()` checks `claims.Aud` contains the configured audience after revocation check.
- New `containsAudience()` helper.

#### Modified: `internal/identity/id_svc.go`
- `IdSvc` gains `audience` field; `NewIdSvc()` accepts audience param.
- `Register()` populates `Aud` on issued tokens via `audienceSlice()`.

#### Modified: `internal/token/tkn_svc.go`
- `Renew()` preserves `Aud` from the original token across renewal.

#### Modified: `internal/deleg/deleg_svc.go`
- `Delegate()` propagates `Aud` from delegator to delegate token.

#### Modified: `internal/admin/admin_svc.go`
- `AdminSvc` gains `audience` field; `NewAdminSvc()` accepts audience param.
- `Authenticate()` and `ActivateSidecar()` populate `Aud` on issued tokens.

#### Modified: `internal/handler/token_exchange_hdl.go`
- Token exchange propagates `Aud` from caller (sidecar) token to issued agent token.

#### Modified: `docker-compose.yml`
- `AA_AUDIENCE` env var passed through to broker container.

### Added — Fix 2: Revocation Persistence (P0 Compliance — Pattern v1.2 §4.2)

**Summary:** Revocations were previously stored only in memory — a broker restart
silently cleared every revocation, allowing previously-revoked tokens to be accepted
again. This fix persists revocations to SQLite via a write-through pattern matching the
existing audit and sidecar persistence. On startup, the broker loads all revocations from
SQLite and rebuilds the in-memory maps.

#### Modified: `internal/store/sql_store.go`

- New `revocations` table (`level`, `target`, `revoked_at`) with `UNIQUE(level, target)`.
- New `RevocationEntry` type — represents a single persisted revocation.
- New `SaveRevocation(level, target)` — idempotent INSERT OR IGNORE.
- New `LoadAllRevocations()` — returns all entries ordered by id for startup rebuild.
- `InitDB()` now creates the revocations table alongside audit and sidecars.

#### Modified: `internal/revoke/rev_svc.go`

- New `RevocationStore` interface — single method `SaveRevocation(level, target)`.
- `NewRevSvc()` now accepts an optional `RevocationStore` parameter.
- `Revoke()` writes through to the store when non-nil (warn-on-failure, non-blocking).
- New `LoadFromEntries()` — bulk-loads level/target pairs into in-memory maps at startup.

#### Modified: `cmd/broker/main.go`

- Loads revocations from SQLite after sidecar loading, before service init.
- Passes `sqlStore` as `RevocationStore` to `NewRevSvc()`.
- Calls `LoadFromEntries()` to rebuild in-memory state on startup.

### Added — Fix 1: Native TLS/mTLS Transport (P0 Compliance — Pattern v1.2 §3.3)

**Summary:** The broker previously only supported plain HTTP regardless of deployment
environment. This fix adds first-class TLS and mutual TLS (mTLS) support directly to
the broker process, eliminating the unencrypted transport gap identified in the
4-reviewer compliance audit (2026-02-20). All changes are fully backward compatible —
default mode remains `none` (plain HTTP).

#### New: `cmd/broker/serve.go`

- New `serve(c cfg.Cfg, addr string, handler http.Handler) error` function — single
  dispatch point for all three TLS modes. `main()` now calls `serve()` instead of
  calling `http.ListenAndServe()` directly, keeping the startup path clean.
- New `loadCA(path string) (*x509.CertPool, error)` function — reads a PEM-encoded CA
  certificate file and returns an `x509.CertPool`. Extracted as an independently
  testable unit to enable unit coverage of the mTLS CA loading path without blocking
  on a live server.
- Mode `"none"` (default): delegates to `http.ListenAndServe` — zero regression for
  existing plain-HTTP deployments.
- Mode `"tls"`: delegates to `http.ListenAndServeTLS` using `AA_TLS_CERT` and
  `AA_TLS_KEY` paths.
- Mode `"mtls"`: builds a `tls.Config` with `ClientAuth: tls.RequireAndVerifyClientCert`
  and a CA pool loaded from `AA_TLS_CLIENT_CA`, then starts an `http.Server` with that
  config. Clients without a valid certificate signed by the configured CA are rejected
  at the TLS handshake.

#### New: `cmd/broker/serve_test.go`

Three unit tests covering all error paths of `loadCA()` (TDD Red-Green-Refactor):

- `TestLoadCA_MissingFile` — expects an error when the CA path does not exist.
- `TestLoadCA_InvalidPEM` — expects an error when the file contains non-PEM bytes.
- `TestLoadCA_ValidPEM` — generates a real self-signed CA certificate in-process using
  `crypto/ecdsa` + `crypto/x509` + `encoding/pem`, writes it to a temp file, and
  asserts that `loadCA()` returns a non-nil pool. No hardcoded cert bytes — cert
  generated programmatically so the test can never fail from an expired or truncated
  fixture.

#### Changed: `cmd/broker/main.go`

- Replaced inline `http.ListenAndServe(addr, rootHandler)` call with `serve(c, addr,
  rootHandler)`.
- Updated package-level godoc comment to document all three TLS modes and their
  required env vars (`AA_TLS_MODE`, `AA_TLS_CERT`, `AA_TLS_KEY`, `AA_TLS_CLIENT_CA`).
- Corrected stale doc reference from `docs/API_REFERENCE.md` to `docs/api.md`.

#### Changed: `internal/cfg/cfg.go`

Four new fields added to the `Cfg` struct with corresponding `Load()` wiring:

| Field         | Env Var             | Default | Description                              |
|---------------|---------------------|---------|------------------------------------------|
| `TLSMode`     | `AA_TLS_MODE`       | `none`  | Transport mode: `none`, `tls`, `mtls`    |
| `TLSCert`     | `AA_TLS_CERT`       | `""`    | Path to TLS certificate PEM file         |
| `TLSKey`      | `AA_TLS_KEY`        | `""`    | Path to TLS private key PEM file         |
| `TLSClientCA` | `AA_TLS_CLIENT_CA`  | `""`    | Path to client CA PEM file (mTLS only)   |

Updated package-level godoc to enumerate all four new env vars alongside existing ones.

#### Changed: `internal/cfg/cfg_test.go`

Three new unit tests (written before production code — TDD Red first):

- `TestLoad_TLSModeDefault` — asserts `TLSMode` is `"none"` when `AA_TLS_MODE` unset.
- `TestLoad_TLSModeSet` — asserts `TLSMode` is `"mtls"` when `AA_TLS_MODE=mtls`.
- `TestLoad_TLSFields` — asserts all three path fields are populated from their
  respective env vars.

#### Changed: `docker-compose.yml`

Four TLS env vars added to the `broker` service with safe empty defaults, so existing
`docker-compose up` workflows require no changes:

```yaml
- AA_TLS_MODE=${AA_TLS_MODE:-none}
- AA_TLS_CERT=${AA_TLS_CERT:-}
- AA_TLS_KEY=${AA_TLS_KEY:-}
- AA_TLS_CLIENT_CA=${AA_TLS_CLIENT_CA:-}
```

#### Changed: `scripts/live_test.sh`

Two new live test modes added (`--tls`, `--mtls`) covering all four user stories:

- `run_tls()` — generates an RSA-2048 self-signed cert with `openssl`, starts the
  broker binary with `AA_TLS_MODE=tls`, verifies the health endpoint responds over
  HTTPS (`curl --cacert`), and confirms that a misconfigured cert path causes a fast
  non-zero exit (Story 4).
- `run_mtls()` — generates a CA, a server cert signed by that CA, and a client cert
  signed by the same CA; starts the broker with `AA_TLS_MODE=mtls`; asserts that a
  client presenting the signed client cert receives a 200 OK response; asserts that a
  client presenting no cert is rejected at the TLS layer (curl exit 35/56).
- Both modes clean up temp files on exit via `trap`.

#### New: `tests/fix1-broker-tls-user-stories.md`

User stories written before any test code was authored (standing rule established
2026-02-24):

| Story | As a…         | I want…                                         | Acceptance Criteria                          |
|-------|---------------|-------------------------------------------------|----------------------------------------------|
| 1     | Operator      | Plain HTTP to still work with no TLS config     | `AA_TLS_MODE` unset → broker starts on HTTP  |
| 2     | Operator      | One-way TLS with `AA_TLS_MODE=tls`              | Health endpoint responds on HTTPS            |
| 3     | Security Eng  | mTLS with `AA_TLS_MODE=mtls`                    | Clients without cert rejected at TLS layer   |
| 4     | Operator      | Misconfigured cert path to fail at startup      | Broker exits non-zero, does not start silently|

Each story is mapped to the live test command (`live_test.sh --tls` / `--mtls`) that
covers it.

#### Changed: `docs/getting-started-operator.md` (Document Version 2.1)

- Added TLS env vars to the broker configuration reference table.
- New "TLS/mTLS Configuration" section covering:
  - When to use each mode (none / tls / mtls).
  - Step-by-step TLS mode example with `openssl` self-signed cert generation.
  - Step-by-step mTLS mode example with CA, server cert, and client cert chain.
  - Docker Compose override pattern for TLS deployments.
  - Production cert note (Let's Encrypt / internal PKI).

#### Changed: `docs/getting-started-developer.md` (Document Version 2.1)

- New "TLS Connections" section added to the Python SDK reference with working examples
  for connecting to TLS-enabled (`ssl_ca_certs`) and mTLS-enabled (`ssl_certfile` +
  `ssl_keyfile`) broker deployments.

#### Process: Standing Rules Established (2026-02-24)

During Fix 1 implementation the following standing rules were documented in `CLAUDE.md`,
`FLOW.md`, and `MEMORY.md`:

- **Live tests require Docker** — self-hosted binary tests are NOT live tests. The
  Docker stack (`./scripts/stack_up.sh`) must be running before any live test executes.
- **User stories before test code** — `tests/<fix-name>-user-stories.md` must exist
  before any live test file is written.
- **Docker Compose must be updated** when a fix adds new env vars.
- `CLAUDE.md` "Live Test Rules" section added to enforce these rules for all future
  contributors.

### Fixed

- **Compliance [Fix 1 — P0]**: Resolved unencrypted transport gap against
  Pattern v1.2 Section 3.3. The broker now enforces transport security when
  `AA_TLS_MODE` is set to `tls` or `mtls`. Default `none` preserves backward
  compatibility for development and internal deployments where a terminating proxy
  provides TLS externally.

#### Verification Evidence (2026-02-24)

- Unit tests: **8 pass** (5 cfg + 3 loadCA), 0 fail — `go test ./internal/cfg/... ./cmd/broker/...`
- Gates: **BUILD OK · lint OK · unit OK** · gosec WARN (non-blocking, pre-existing)
- Docker live test: **9 pass, 0 fail** — `./scripts/live_test.sh --docker`

- `aactl` operator CLI (`cmd/aactl/`) — cobra-based binary for managing the AgentAuth broker without hand-crafting curl + JWT
  - `aactl sidecars list` — list all registered sidecars (table or JSON)
  - `aactl sidecars ceiling get <id>` — get scope ceiling for a sidecar
  - `aactl sidecars ceiling set <id> --scopes s1,s2` — update scope ceiling
  - `aactl revoke --level <lvl> --target <t>` — revoke tokens at token/agent/task/chain granularity
  - `aactl audit events [flags]` — query audit trail with filters (agent-id, task-id, event-type, since, until, limit, offset)
  - Env-var auth: `AACTL_BROKER_URL` + `AACTL_ADMIN_SECRET` (stateless, no disk state)
  - Table output by default; `--json` flag for raw JSON (CI-friendly)
- **Sidecar Persistence [P1]**: `GET /v1/admin/sidecars` endpoint lists all known sidecars with their ID, allowed scopes, status, and activation timestamp. Requires `admin:manage` scope.
- **Sidecar Persistence [P1]**: SQLite sidecar persistence via dual-write pattern (same architecture as audit persistence). Sidecar records written to both in-memory ceiling map and SQLite on activation.
- **Sidecar Persistence [P1]**: Startup sidecar loading — `LoadAllSidecars()` populates the ceiling map from SQLite on broker start, so sidecar scope ceilings survive restarts.
- **Sidecar Persistence [P1]**: Store methods: `SaveSidecar()`, `ListSidecars()`, `UpdateSidecarCeiling()`, `UpdateSidecarStatus()`, `LoadAllSidecars()` for full sidecar lifecycle management in SQLite.
- **Sidecar Persistence [P1]**: `UpdateSidecarCeiling` syncs ceiling changes to SQLite when updated via `PUT /v1/admin/sidecars/{id}/ceiling`.
- **Observability [P1]**: 2 new Prometheus metrics: `agentauth_sidecars_total` (gauge, tracks active sidecar count), `agentauth_sidecar_list_duration_seconds` (histogram, list endpoint latency).
- **Testing [P1]**: Integration test `TestListSidecars_Integration` — full end-to-end through HTTP (admin auth, activate sidecar, list sidecars, verify response).

### Fixed

- **Bug [P0]**: Multi-scope sidecar activation — `AllowedScopePrefix` (string) → `AllowedScopes` ([]string). Comma-joined scope entries were stored as a single JWT claim, causing all multi-scope token exchanges to fail with `scope_escalation_denied`. Each scope now gets its own `sidecar:activate:X` and `sidecar:scope:X` claim entry. **Breaking change** to `POST /v1/admin/sidecar-activations` request body.
- **Security [P1]**: Removed dead `TknSvc.Exchange()` and `isScopeAllowed()` methods that used a weaker prefix-based scope check instead of `authz.ScopeIsSubset()`. Deleted associated sentinel errors and stale test.
- **Security [P2]**: Token exchange TTL=0 now clamps to `maxExchangeTTL` (900s) instead of delegating to `cfg.DefaultTTL`, preventing silent TTL cap bypass when `AA_DEFAULT_TTL` > 900.
- **Docker**: All Docker build scripts (`live_test_docker.sh`, `live_test_sidecar.sh`, `stack_up.sh`) now use `--no-cache` to prevent stale cached layers from masking code changes during E2E testing
- **Lint**: Resolved 18 errcheck findings across production and test code (token exchange handler, problem details, admin handler, store tests, revoke tests, handler tests, admin handler tests, logging test)
- **Lint**: Fixed ineffassign in `mut_auth_hdl_test.go` (unused `hdl` variable overwritten immediately)
- **Production code**: `json.Encode` errors now logged via `obs.Warn` or `log.Printf` in `token_exchange_hdl.go`, `admin_hdl.go`, and `problemdetails.go`

### Added

- **Audit Persistence [P0]**: SQLite-backed audit event persistence via `modernc.org/sqlite` (pure Go, zero CGo). Audit events now survive broker restarts. On startup, existing events are loaded from SQLite to rebuild the in-memory hash chain. Write-through model: events are written to both memory (for fast queries) and SQLite (for durability). Configurable via `AA_DB_PATH` env var (default `./agentauth.db`).
- **Audit Persistence [P0]**: `AuditStore` interface in `internal/audit/audit_log.go` — decouples audit log from storage backend. `SqlStore` implements this interface. Pass `nil` for memory-only mode (tests, dev).
- **Audit Persistence [P0]**: `NewAuditLogWithEvents()` constructor rebuilds hash chain from persisted events at startup. Counter and prevHash derived from the last loaded event so new events continue the chain seamlessly.
- **Audit Persistence [P0]**: `SqlStore.InitDB()`, `SaveAuditEvent()`, `LoadAllAuditEvents()`, `QueryAuditEvents()`, `HasDB()`, `Close()` methods for SQLite audit table management with 3 indexes (event_type, agent_id, timestamp).
- **Config [P0]**: `AA_DB_PATH` environment variable for configuring SQLite database location. Default: `./agentauth.db`.
- **Broker Health [P0]**: `GET /v1/health` now returns `db_connected` (bool) and `audit_events_count` (int) fields. `DBChecker` interface allows health handler to check database connectivity without importing the store package directly.
- **Sidecar Health [P0]**: `GET /v1/health` on sidecar now returns `sidecar_id` field, enabling programmatic discovery of the sidecar ID for ceiling management operations.
- **Observability [P0]**: 4 new Prometheus metrics: `agentauth_audit_events_total` (counter, by event_type), `agentauth_audit_write_duration_seconds` (histogram), `agentauth_db_errors_total` (counter, by operation), `agentauth_audit_events_loaded` (gauge, set at startup).
- **Docker [P0]**: `docker-compose.yml` — broker service now includes `AA_DB_PATH` env var and `broker-data` volume for SQLite persistence across container restarts.
- **Docs**: `docs/getting-started-operator.md` — "Runtime Ceiling Management" section explaining that `AA_SIDECAR_SCOPE_CEILING` is the bootstrap seed only, how to update the ceiling at runtime via `PUT /v1/admin/sidecars/{id}/ceiling`, propagation timing (4-12 minutes on next renewal cycle), emergency narrowing with automatic token revocation, and ceiling change audit queries
- **Docs**: `docs/getting-started-operator.md` — "Audit Persistence (AA_DB_PATH)" section documenting the new `AA_DB_PATH` env var for SQLite-backed audit event persistence, default value (`./agentauth.db`), Docker Compose volume mount pattern, and startup behavior (hash chain rebuild from SQLite)
- **Docs**: v2 documentation restructure — replaced monolithic v1 docs (`DEVELOPER_GUIDE.md`, `API_REFERENCE.md`, `USER_GUIDE.md`) with role-based v2 docs: `architecture.md`, `concepts.md`, `api.md`, `getting-started-developer.md`, `getting-started-operator.md`, `getting-started-user.md`, `common-tasks.md`, `troubleshooting.md`
- **Docs**: 4 real-world multi-agent example walkthroughs in `docs/examples/`: Data Pipeline (scope attenuation + delegation), Code Generation (branch-scoped write access), Customer Support (PII containment + audit), DevOps Automation (least-privilege deployment)
- **Audit**: 5 new enforcement audit event types: `token_auth_failed`, `token_revoked_access`, `scope_violation`, `scope_ceiling_exceeded`, `delegation_attenuation_violation`
- **Audit**: All `ValMw` middleware denial paths now produce audit events (missing auth header, invalid scheme, verification failed, revoked token access)
- **Audit**: Delegation scope attenuation violations now produce `delegation_attenuation_violation` audit events with delegator, target, requested, and allowed scope details
- **Audit**: Sidecar scope ceiling denials now include structured audit fields (`event_type`, `agent_name`, `task_id`) in log output
- **Docs**: "Enforcing Scopes in Your Resource Server" section added to `docs/getting-started-developer.md` with Python, Go, and TypeScript examples of the validate→check scope→act pattern
- **Sidecar Resilience — Failsafe Mode**: Circuit breaker with sliding-window failure tracking (Closed → Open → Probing states)
- **Sidecar Resilience**: Cached token fallback — serves previously-issued tokens during broker outage (`X-AgentAuth-Cached: true` header)
- **Sidecar Resilience**: Background health probe for automatic circuit breaker recovery
- **Sidecar Resilience**: Bootstrap retry with exponential backoff — sidecar no longer exits on broker unavailability at startup
- **Sidecar Resilience**: HTTP server starts pre-bootstrap — health endpoint responds during startup
- **Sidecar Resilience**: 3 new Prometheus metrics: `circuit_state`, `circuit_trips_total`, `cached_tokens_served_total`
- **Sidecar Resilience**: 4 new config vars: `AA_SIDECAR_CB_WINDOW`, `AA_SIDECAR_CB_THRESHOLD`, `AA_SIDECAR_CB_PROBE_INTERVAL`, `AA_SIDECAR_CB_MIN_REQUESTS`
- **Sidecar Observability**: Structured logging via `internal/obs` package — replaces all 15 raw `fmt.Printf` calls with leveled, structured log lines (`[AA:SIDECAR:LEVEL] TIMESTAMP | COMPONENT | MESSAGE | context`)
- **Sidecar Observability**: `AA_SIDECAR_LOG_LEVEL` now wired (was loaded but unused) — supports `quiet`, `standard`, `verbose`, `trace`
- **Sidecar Observability**: 6 Prometheus metrics in dedicated `cmd/sidecar/metrics.go`: bootstrap, renewals, token exchanges, scope denials, agents registered, request duration
- **Sidecar Observability**: `GET /v1/metrics` endpoint on sidecar for Prometheus scraping
- **Sidecar Observability**: Health endpoint now reports `agents_registered`, `last_renewal`, `uptime_seconds`
- **Testing**: Docker E2E live tests (`live_test_sidecar.sh`, `live_test_docker.sh`) are now mandatory module gates in `gates.sh` — blocks merge if any live test fails
- **Testing**: New `scripts/live_test_sidecar.sh` — 9-step Docker-based E2E covering all 5 sidecar endpoints (health, lazy reg, cache hit, scope ceiling, renew, challenge, BYOK register, BYOK token, broker validate)
- **Sidecar Phase 2**: Background auto-renewal goroutine for sidecar bearer token (80% TTL default, configurable via `AA_SIDECAR_RENEWAL_BUFFER`)
- **Sidecar Phase 2**: Per-agent registration — lazy on first `POST /v1/token` with sidecar-managed Ed25519 keypairs
- **Sidecar Phase 2**: BYOK registration: `GET /v1/challenge` proxy + `POST /v1/register` for developer-provided keys
- **Sidecar Phase 2**: In-memory ephemeral agent registry with per-agent locking for concurrent safety
- **Sidecar Phase 2**: Health endpoint now reports `status: "degraded"` (503) when token renewal fails
- **Sidecar Phase 2**: Graceful shutdown via SIGINT/SIGTERM with context cancellation
- **Sidecar Phase 2**: Thread-safe `sidecarState` with `sync.RWMutex` for renewal/handler concurrency
- **Sidecar**: Go sidecar binary (`cmd/sidecar/`) that auto-bootstraps with the broker and exposes a simple developer-facing API (`POST /v1/token`, `POST /v1/token/renew`, `GET /v1/health`)
- **Sidecar**: Scope ceiling enforcement — sidecar locally checks requested scope against its configured ceiling before calling the broker
- **Sidecar**: Auto-activation sequence — health check, admin auth, activation token, single-use exchange
- **Sidecar**: End-to-end integration test validating full developer flow against a real in-process broker
- **Docker**: Updated multi-stage Dockerfile with separate `broker` and `sidecar` targets
- **Docker**: Updated docker-compose.yml replacing placeholder sidecar with real binary, health-check dependency
- **Deployment**: Multi-stage Dockerfile for small, secure production images
- **Deployment**: Docker Compose configuration for local development and orchestration
- **Infrastructure**: Global Request-ID middleware for request correlation across logs and diagnostics
- **Infrastructure**: HTTP request logging middleware capturing method, path, status, and latency
- **Infrastructure**: Centralized `problemdetails` package for RFC 7807 compliance and cycle resolution
- **Identity**: Support for stable sidecar identity via the `sid` JWT claim
- **Security**: Activation token replay protection in `SqlStore` using JTI tracking
- **Tokens**: `IssueReq` now supports optional JWT audience (`aud`) so broker-issued tokens can carry endpoint-specific intent (used by sidecar activation contract).
- **Admin/Sidecar**: Added sidecar activation service models:
  - `CreateSidecarActivationReq` / `CreateSidecarActivationResp`
  - `ActivateSidecarReq` / `ActivateSidecarResp`
- **Admin/Sidecar**: Added service methods for sidecar bootstrap flow:
  - `CreateSidecarActivationToken(...)`
  - `ActivateSidecar(...)`
- **Token/Sidecar**: Added `POST /v1/token/exchange` for sidecar-mediated token issuance with broker-derived `sid` lineage.
- **Ops**: Added one-command stack scripts:
  - `scripts/stack_up.sh` (compose up broker + sidecar)
  - `scripts/stack_down.sh` (compose down)
- **Live Testing**: `scripts/live_test.sh` now always deploys Docker Compose (`broker` + `sidecar`) before executing E2E checks.
- **Testing**: Comprehensive token exchange test coverage: scope format validation, TTL bounds, lineage anti-spoof, `Sid` fallback, integration lifecycle, and activation replay protection.
- **Testing**: Request-ID propagation tests confirming `X-Request-Id` header round-trips through middleware and appears in every JSON error response.
- **Testing**: Method restriction tests returning 405 `method_not_allowed` for non-POST requests on token exchange and sidecar activation endpoints.
- **Testing**: Integration test mux wiring now mirrors `main.go` exactly (auth middleware, rate limiter, route order).
- **Audit**: Added audit coverage for 5 previously uncovered sidecar denial paths:
  - Unauthenticated exchange attempt (`sidecar_exchange_denied`)
  - Sidecar identity derivation failure (`sidecar_exchange_denied`)
  - Token issuance failure (`sidecar_exchange_denied`)
  - Sidecar activation token creation denial (`sidecar_activation_failed`)
  - Sidecar activation failure (`sidecar_activation_failed`)

### Changed

- **Authorization**: `WithRequiredScope()` standalone function replaced by `ValMw.RequireScope()` method — scope checking now has access to `auditLog` for recording `scope_violation` events on denial
- **Errors**: Standardized all error responses to include `error_code` and `request_id` fields
- **Admin**: Refactored admin handlers to use shared standardized error helpers
- **Admin/Sidecar**: Added validation and replay-protection error semantics for activation flow:
  - `ErrActivationScopeEmpty`
  - `ErrActivationTokenInvalid`
  - `ErrActivationTokenReplayed`
- **Admin/Sidecar**: Activation exchange now enforces one-time token consumption via `SqlStore.ConsumeActivationToken(...)` and issues a bounded sidecar token carrying broker-derived `sid`.
- **Token/Sidecar**: Exchange flow now enforces sidecar scope ceilings (`sidecar:scope:*`) and rejects scope escalation with stable `scope_escalation_denied` error code.
- **Token/Sidecar**: Scope format pre-validation on `POST /v1/token/exchange` rejects malformed scope entries with `invalid_scope_format` error code (400) before attempting scope attenuation.
- **Token/Sidecar**: Lineage anti-spoof hardening: client-supplied `sidecar_id` is always ignored; `sid` is derived from authenticated caller token's `Sid` field (falling back to `Sub`). Empty derivation returns 500 `sidecar_derivation_failed`.
- **Token/Sidecar**: TTL bounds enforcement: explicit negative or >900s TTL returns 400 `invalid_ttl`; TTL=0 clamps to `maxExchangeTTL` (900s).
- **Token**: Added optional audience propagation in `IssueReq -> TknClaims.Aud` to support intent-bound activation tokens.
- **Audit**: Added `sidecar_exchange_success` and `sidecar_exchange_denied` audit event types for token exchange operations.
- **Docker**: `docker-compose.yml` now includes a dedicated `sidecar` service for runtime and E2E integration flow testing.

## [2.0.0] - 2026-02-09

Complete rewrite implementing the Ephemeral Agent Credentialing security pattern.

### Added

- **Identity**: Challenge-response agent registration with Ed25519 cryptographic verification
- **Identity**: SPIFFE-format agent IDs (`spiffe://{domain}/agent/{orch}/{task}/{instance}`)
- **Tokens**: EdDSA-signed JWT tokens with configurable TTL (default 5 minutes)
- **Tokens**: Token verification endpoint returning decoded claims
- **Tokens**: Token renewal with fresh timestamps and new JTI
- **Authorization**: `ValMw` middleware enforcing Bearer token + scope on every request
- **Authorization**: Scope format `action:resource:identifier` with wildcard support
- **Revocation**: 4-level token revocation (token/JTI, agent/SPIFFE ID, task, delegation chain)
- **Audit**: Hash-chain tamper-evident audit trail with SHA-256 linking
- **Audit**: Automatic PII sanitization (secrets, passwords, private keys, token values)
- **Audit**: 12 event types covering admin auth, registration, token lifecycle, delegation, and resource access
- **Audit**: Query endpoint with filtering (agent, task, event type, time range) and pagination
- **Delegation**: Scope-attenuated token delegation with chain verification
- **Delegation**: Maximum delegation depth of 5 hops
- **Delegation**: Cryptographic delegation chain embedded in token claims
- **Admin**: Admin authentication via shared secret with constant-time comparison
- **Admin**: Launch token creation with policy (allowed scope, max TTL, single-use flag)
- **Admin**: Admin bootstrap flow for initial system setup
- **Observability**: Prometheus metrics (registrations, revocations, active agents)
- **Observability**: Structured logging via `obs` package (`Ok`/`Warn`/`Fail`/`Trace` levels)
- **Errors**: RFC 7807 `application/problem+json` error responses on all endpoints
- **Health**: Health check endpoint reporting status, version, and uptime
- **Metrics**: Prometheus exposition format at `/v1/metrics`
- **Config**: `AA_*` environment variable configuration with sensible defaults
- **API**: OpenAPI 3.0 specification at `docs/api/openapi.yaml`
