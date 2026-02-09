# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added

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

### Changed

- **Errors**: Standardized all error responses to include `error_code` and `request_id` fields
- **Admin**: Refactored admin handlers to use shared standardized error helpers
- **Admin/Sidecar**: Added validation and replay-protection error semantics for activation flow:
  - `ErrActivationScopeEmpty`
  - `ErrActivationTokenInvalid`
  - `ErrActivationTokenReplayed`
- **Admin/Sidecar**: Activation exchange now enforces one-time token consumption via `SqlStore.ConsumeActivationToken(...)` and issues a bounded sidecar token carrying broker-derived `sid`.
- **Token/Sidecar**: Exchange flow now enforces sidecar scope ceilings (`sidecar:scope:*`) and rejects scope escalation with stable `scope_escalation_denied` error code.
- **Token**: Added optional audience propagation in `IssueReq -> TknClaims.Aud` to support intent-bound activation tokens.

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
