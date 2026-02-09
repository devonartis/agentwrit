# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/) and this project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.0.0] - 2026-02-09

### Core Broker (Go)

#### Identity
- SPIFFE ID generation, validation, and parsing (`spiffe://{domain}/agent/{orch}/{task}/{instance}`)
- Ed25519 key management with JWK public-key parsing
- Launch token creation and single-use consumption
- Challenge-response registration: `GET /v1/challenge` and `POST /v1/register`

#### Tokens
- Signed JWT issue/verify/renew with EdDSA (Ed25519)
- Scope model: `action:resource:identifier` with wildcard matching and subset logic
- Token claims: `sub`, `scope`, `task_id`, `orchestration_id`, `delegation_chain`, `jti`, `exp`
- Endpoints: `POST /v1/token/validate` and `POST /v1/token/renew`

#### Authorization
- Zero-trust middleware (`ValMw`) with bearer token verification on every request
- Route-level scope injection via `WithRequiredScope`
- Protected resource endpoint: `GET /v1/protected/customers/12345`

#### Revocation
- 4-level token revocation: token, agent, task, delegation chain
- `POST /v1/revoke` endpoint with admin scope requirement
- `RevChecker` interface for pluggable revocation backends
- Real-time enforcement via authorization middleware integration

#### Audit
- Immutable hash-chain audit log with SHA-256 integrity verification
- PII sanitization (email, phone, customer ID hashing)
- 7 event types: `credential_issued`, `access_granted`, `access_denied`, `token_revoked`, `delegation_created`, `delegation_revoked`, `anomaly_detected`
- `GET /v1/audit/events` with filtering (agent, task, event type, time range) and pagination

#### Mutual Authentication
- 3-step agent-to-agent handshake protocol
- Discovery binding registry for endpoint mapping and MITM prevention
- Heartbeat/liveness monitoring with optional auto-revocation
- Identity cross-checks: `ErrInitiatorMismatch`, `ErrPeerMismatch`, `ErrResponderMismatch`

#### Delegation
- Scope attenuation: permissions narrow at each delegation hop, never expand
- Delegation token issuance with TTL enforcement and depth limits (max 3 hops)
- Chain verification with Ed25519 signature validation per hop
- SHA-256 chain hash for tamper detection and chain-level revocation
- `POST /v1/delegate` endpoint

#### Observability
- Centralized RFC 7807 `application/problem+json` error responses
- Prometheus metrics with `aa_*` prefix
- `GET /v1/health` endpoint (200 healthy, 503 degraded/unhealthy)
- `GET /v1/metrics` endpoint (Prometheus exposition format)

### Demo Application (Python)

#### Resource Server
- FastAPI server with 4 endpoints: customers, orders, tickets, notifications
- Dual-mode auth middleware: insecure (API-Key) and secure (Bearer + broker validation)
- Pre-seeded sample data (5 customers, 10 orders, 3 tickets)
- Scope mapping: URL paths resolve to required scopes

#### Demo Agents
- BrokerClient: async HTTP wrapper for all broker REST endpoints
- AgentBase: Ed25519 ephemeral key generation and challenge-response registration
- Agent A (DataRetriever): scoped customer data retrieval
- Agent B (Analyzer): order analysis with scope delegation to Agent C
- Agent C (ActionTaker): uses delegated token to close tickets and send notifications
- Orchestrator: sequences A->B->C workflow with per-agent timing

#### Attack Simulator
- 5 adversarial scenarios: credential theft, lateral movement, impersonation, privilege escalation, accountability
- Dual-mode execution: insecure (attacks succeed) vs. secure (attacks blocked)
- CLI entrypoint: `python -m attacks --mode secure|insecure`

#### Dashboard
- Web-based demo dashboard with HTMX frontend and SSE real-time events
- Demo control endpoints (run/reset/status)
- Agent workflow visualization and attack results display

### Infrastructure

- Multi-stage Dockerfile (golang:1.23-alpine build, alpine:3.19 runtime)
- Docker Compose configuration with health checks
- Quality gate system (`scripts/gates.sh`): build, lint, unit, security (gosec + govulncheck), docs
- Live test infrastructure with seed tokens and smoke test client
- Structured logging: `[AA:MODULE:LEVEL] TIMESTAMP | COMPONENT | MESSAGE | CONTEXT`

### Security

- `LoadSigningKey` validates path safety: no symlinks, regular files only, rejects group/other-readable keys
- Admin endpoints protected by zero-trust middleware with required `admin:Broker:*` scope
- Token subject cross-checks on all handshake steps to prevent identity spoofing
- Delegation chain propagation through token issuance and renewal
