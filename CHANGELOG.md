# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Fixed

- **Bug [P0]**: Multi-scope sidecar activation — `AllowedScopePrefix` (string) → `AllowedScopes` ([]string). Comma-joined scope entries were stored as a single JWT claim, causing all multi-scope token exchanges to fail with `scope_escalation_denied`. Each scope now gets its own `sidecar:activate:X` and `sidecar:scope:X` claim entry. **Breaking change** to `POST /v1/admin/sidecar-activations` request body.
- **Security [P1]**: Removed dead `TknSvc.Exchange()` and `isScopeAllowed()` methods that used a weaker prefix-based scope check instead of `authz.ScopeIsSubset()`. Deleted associated sentinel errors and stale test.
- **Security [P2]**: Token exchange TTL=0 now clamps to `maxExchangeTTL` (900s) instead of delegating to `cfg.DefaultTTL`, preventing silent TTL cap bypass when `AA_DEFAULT_TTL` > 900.
- **Docker**: All Docker build scripts (`live_test_docker.sh`, `live_test_sidecar.sh`, `stack_up.sh`) now use `--no-cache` to prevent stale cached layers from masking code changes during E2E testing
- **Lint**: Resolved 18 errcheck findings across production and test code (token exchange handler, problem details, admin handler, store tests, revoke tests, handler tests, admin handler tests, logging test)
- **Lint**: Fixed ineffassign in `mut_auth_hdl_test.go` (unused `hdl` variable overwritten immediately)
- **Production code**: `json.Encode` errors now logged via `obs.Warn` or `log.Printf` in `token_exchange_hdl.go`, `admin_hdl.go`, and `problemdetails.go`

### Added

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
