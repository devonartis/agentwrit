# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
