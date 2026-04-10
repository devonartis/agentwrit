# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added — CI/build/gates (M-sec v1)

- **`.gosec.yml`** — explicit gosec configuration with documented rule
  exclusions (G117, G304, G101) rationalized for a credential broker's API
  surface. Every excluded rule carries a reviewer-auditable rationale.
- **`.golangci.yml`** — security-aware `golangci-lint` config (errcheck,
  gosec, govet, ineffassign, staticcheck, unused, gosimple, bodyclose,
  misspell, gofmt, goimports) with tuned govet subchecks (fieldalignment
  and shadow disabled with rationale) and mirrored gosec excludes.
- **`scripts/smoke/core-contract.sh`** — L2.5 core contract smoke test.
  10-step verification (health, admin auth, launch token, challenge,
  Ed25519 challenge-response register, JWT structure, validate-accepted,
  revoke, validate-rejected, out-of-scope denied) against a running
  broker. Uses `python3 + cryptography` for the Ed25519 signing step.
- **`scripts/test-gate-parity.sh`** — enforces gate list alignment
  between `scripts/gates.sh --list-gates` and `.github/workflows/ci.yml`
  `GATE_LIST_START/END` block. Prevents silent drift.
- **`syft scan` baseline** — SBOM generation integrated into the local
  `gates.sh full` pipeline (SPDX-2.3, 27 packages at baseline).

### Changed — CI/build/gates (M-sec v1)

- **`scripts/gates.sh`** — extended from 4 gates to 13. New blocking
  gates: `contamination` (enterprise refs grep), `govulncheck` (stdlib
  and dependency vulnerabilities), `go-mod-verify` (module integrity +
  tidy drift), `vet`, `format`, plus `full`-mode-only: `unit-tests-race`,
  `docker-build`, `smoke-l2.5`, `sbom`. `gosec` flipped from warn-only
  to blocking. `module` renamed to `full` (deprecated alias retained).
  Dead references to `live_test.sh`/`live_test_docker.sh` removed.
  `golangci-lint` and `gosec` are now required (no fallback). Added
  `--list-gates` for parity enforcement. Honors `BROKER_URL` for
  smoke-l2.5 on non-default ports.
- **`TECH-DEBT.md`** — recorded TD-VUL-001..004 (four Go stdlib CVEs
  fixed by bumping `go.mod` toolchain from `go1.25.7` to `go1.25.9`,
  scheduled for landing at the first CI push).

### Fixed — CI/build/gates (M-sec v1)

- **gofmt drift** — 24 pre-existing gofmt-dirty files normalized in a
  single style commit. No behavior change. Surfaced by adding `format`
  as a blocking gate.
- **`internal/keystore/parseKey`** — defensive type-assertion on
  `priv.Public().(ed25519.PublicKey)` to satisfy `errcheck
  check-type-assertions`. Unreachable on the happy path.
- **`internal/mutauth/heartbeat.sweep`** — heartbeat auto-revoke
  failures are now logged via `obs.Warn` instead of being silently
  dropped. Previously `_, _ = h.revSvc.Revoke(...)` was followed by an
  unconditional "agent auto-revoked" log line, even when the revocation
  actually failed.
- **`cmd/aactl/client`** — `json.Marshal` and `io.ReadAll` errors are
  now propagated as wrapped errors instead of being discarded. Affects
  `authenticate()` (two sites) and `doPostWithToken()`.
- **`internal/store/sql_store.QueryAuditEvents`** — documented `#nosec
  G202` on the audit query SELECT, explaining why the fragment
  concatenation is safe (fixed template, parameterized values).
- **`internal/admin/admin_svc_test.TestLaunchTokenRecord_SpecCompliance`** —
  clarified the exhaustive-literal intent in a doc comment and silenced
  `govet unusedwrite` with `_ = rec`.

### Added

**Security hardening**

- **Token TTL enforcement** — `AA_MAX_TTL` configuration sets the maximum token lifetime ceiling (default 86400s, set to 0 to disable). The broker clamps any requested TTL to this ceiling.
- **TTL carry-forward on renewal** — Renewed tokens preserve the original token's TTL instead of falling back to the default. Closes a privilege escalation path where a short-lived token could be renewed to the broker default.
- **JWT algorithm validation** — The broker rejects tokens with `alg != EdDSA`, preventing the `alg:none` and HS256/RS256 algorithm confusion attacks.
- **JWT key ID validation** — The broker rejects tokens with a mismatched `kid`, preventing cross-broker token replay.
- **Revocation check in Verify()** — Every token verification path checks the revocation list. Defense in depth.
- **Transactional renewal** — Predecessor token is revoked before the new token is issued. If revocation fails, renewal fails.
- **Startup warning when DefaultTTL > MaxTTL** — Surfaces silent clamping at startup.
- **Token expiry required** — Tokens with `exp=0` or missing `exp` are rejected.

**HTTP hardening**

- **SecurityHeaders middleware** — All responses carry `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `X-Frame-Options: DENY`. HSTS added on TLS/mTLS deployments.
- **Request body size limit** — Global 1MB limit on all endpoints, enforced by eager buffering so streaming decoders can't bypass it. Returns 413 on oversized bodies.
- **Error sanitization** — Token validation, renewal, and auth middleware errors return generic messages to the client. Full errors are recorded in the audit trail with a correlation `request_id`.
- **Bind address safety** — Broker defaults to `127.0.0.1`; warns at startup when binding to `0.0.0.0` without TLS.
- **HTTP server timeouts** — Read, write, and idle timeouts prevent slowloris-style attacks.
- **TLS 1.2 minimum + AEAD-only ciphers** — Enforced when TLS is enabled.
- **Weak secret denylist** — The broker refuses to start with a known-weak admin secret (empty, `change-me-in-production`, etc.). Use `aactl init` or generate a strong value.

**Operator tooling**

- **`aactl init` command** — Generates a secure admin secret and config file in dev or prod mode. Atomic file creation with `O_EXCL`, rejects symlinks, enforces 0600 file / 0700 directory permissions.
- **Config file support** — KEY=VALUE format at `AA_CONFIG_PATH` > `/etc/agentauth/config` > `~/.agentauth/config`. Rejects insecure permissions like SSH/GPG does.
- **Bcrypt admin authentication** — Admin secret stored as a bcrypt hash; plaintext only shown once at init. Dev mode supports plaintext config for convenience, bcrypt is derived at startup.
- **`gates.sh` developer tool** — Build + lint + unit tests + gosec in one command (`./scripts/gates.sh task`). Module mode adds full tests and Docker E2E. Regression mode runs all phase regression suites.

**App credential lifecycle**

- **`POST /v1/app/launch-tokens`** — App-facing endpoint for creating launch tokens within the app's scope ceiling. Scope ceiling enforcement prevents apps from escalating beyond what the operator granted at registration.
- **`POST /v1/admin/launch-tokens`** — Admin-facing endpoint for bootstrapping and break-glass scenarios. No ceiling enforcement (admin is the root of trust).
- **App scope ceiling** — Operators set a scope ceiling when registering an app; the broker enforces it on every `POST /v1/app/launch-tokens` call.
- **App traceability** — `app_id`, `app_name`, and `original_principal` claims flow through launch tokens into agent JWTs, preserved through delegation.

**Production foundations**

- **Persistent signing key** — Ed25519 signing key loaded from disk at startup (`AA_SIGNING_KEY_PATH`), generated with 0600 permissions on first start. Agent tokens survive broker restart.
- **Graceful shutdown** — SIGINT/SIGTERM triggers clean shutdown: HTTP server drains, SQLite closed.
- **Corrupt key fails fast** — Broker refuses to start with a malformed signing key, surfacing the problem at deploy time.
- **Token predecessor revocation on renewal** — Prevents two valid tokens existing for the same agent.
- **JTI blocklist pruning** — Background goroutine removes expired revocation entries so memory doesn't grow unbounded.
- **Agent record expiry** — Agent records marked expired when their token TTL elapses.

**Audit and observability**

- **Structured audit fields** — Audit events carry `resource`, `outcome`, `deleg_depth`, `deleg_chain_hash`, and `bytes_transferred` via a backward-compatible options pattern. Hash chain tamper evidence covers all structured fields.
- **Outcome filtering** — Query the audit trail by outcome via `--outcome` on `aactl audit events` or `?outcome=` on `GET /v1/audit/events`.
- **Enforcement event coverage** — Audit events emitted for every denial path: missing auth, invalid scheme, token verification failure, revoked token access, scope violations, delegation attenuation violations, scope ceiling exceeded.
- **New Prometheus metrics** — `agentauth_audit_events_total`, `agentauth_audit_write_duration_seconds`, `agentauth_db_errors_total`, `agentauth_audit_events_loaded`, `agentauth_admin_auth_total`.

**Persistence**

- **SQLite audit persistence** — Audit events persist to SQLite via `modernc.org/sqlite` (pure Go, no CGo). Hash chain is rebuilt from disk on startup. Configurable via `AA_DB_PATH` (default `./agentauth.db`).
- **Revocation persistence** — Revocations stored in SQLite so tokens stay revoked across broker restarts.

### Changed

- **Documentation accuracy** — Corrected public documentation to match the current broker contract for agent registration, renewal, release, app authentication, launch token creation, and health responses. Fixed copy/paste examples with stale payload shapes and outdated event names.
- **Direct HTTP integration** — Go developers get explicit pre-SDK guidance with end-to-end examples for registration, renewal, and release in `docs/getting-started-developer.md`.
- **Authorization middleware** — `WithRequiredScope()` standalone function replaced by `ValMw.RequireScope()` method. Scope checking now emits `scope_violation` audit events on denial.

---

## [2.0.0] — 2026-02-09

Complete rewrite implementing the Ephemeral Agent Credentialing security pattern.

### Added

**Identity and authentication**

- Challenge-response agent registration with Ed25519 cryptographic verification
- SPIFFE-format agent IDs (`spiffe://{domain}/agent/{orch}/{task}/{instance}`)
- EdDSA-signed JWT tokens with configurable TTL (default 5 minutes)
- Token verification endpoint returning decoded claims
- Token renewal with fresh timestamps and new JTI

**Authorization**

- `ValMw` middleware enforcing Bearer token + scope on every request
- Scope format `action:resource:identifier` with wildcard support

**Revocation**

- 4-level token revocation (token/JTI, agent/SPIFFE ID, task, delegation chain)

**Audit**

- Hash-chain tamper-evident audit trail with SHA-256 linking
- Automatic PII sanitization (secrets, passwords, private keys, token values)
- 12 event types covering admin auth, registration, token lifecycle, delegation, resource access
- Query endpoint with filtering (agent, task, event type, time range) and pagination

**Delegation**

- Scope-attenuated token delegation with chain verification
- Maximum delegation depth of 5 hops
- Cryptographic delegation chain embedded in token claims

**Admin**

- Admin authentication via shared secret with constant-time comparison
- Launch token creation with policy (allowed scope, max TTL, single-use flag)
- Admin bootstrap flow for initial system setup

**Observability**

- Prometheus metrics (registrations, revocations, active agents)
- Structured logging via `obs` package
- RFC 7807 `application/problem+json` error responses on all endpoints
- Health check endpoint reporting status, version, and uptime
- Prometheus exposition format at `/v1/metrics`

**Configuration**

- `AA_*` environment variable configuration with sensible defaults
