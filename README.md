# AgentAuth

**Ephemeral agent credentialing for AI systems.**

AgentAuth is a Go broker that issues short-lived, scoped tokens to AI agents via cryptographic challenge-response identity flows. It implements the [Ephemeral Agent Credentialing](docs/SECURITY_PATTERN.md) security pattern to solve the AI agent identity crisis.

---

## The Problem

Traditional IAM (OAuth, AWS IAM, service accounts) was designed for long-lived services with static identities. AI agents break every assumption: they are ephemeral (minutes), non-deterministic, require task-specific permissions at runtime, operate autonomously, and need to authenticate each other.

A system with 100 concurrent agents using 15-minute OAuth tokens for 2-minute tasks creates **21,666 agent-hours of unnecessary credential exposure per day**. Static credentials shared across agents mean one compromise exposes everything. There is no per-agent accountability, no scope enforcement, and no way to revoke a single agent's access.

AgentAuth eliminates this exposure window.

## What AgentAuth Does

| Capability | Description |
|---|---|
| **Ephemeral Identity** | Each agent instance gets a unique SPIFFE ID, cryptographically bound via Ed25519 |
| **Task-Scoped Tokens** | JWTs scoped to `action:resource:identifier` with configurable TTL (1-15 min) |
| **Zero-Trust Enforcement** | Every request validated: signature, expiry, scope, revocation status |
| **4-Level Revocation** | Revoke by token, agent, task, or delegation chain -- takes effect immediately |
| **Immutable Audit Trail** | SHA-256 hash-chain audit log with PII sanitization |
| **Mutual Authentication** | 3-step agent-to-agent handshake with discovery binding |
| **Delegation Chains** | Scope attenuation across agent hops with cryptographic lineage proof |
| **Observability** | Prometheus metrics, structured logging, health endpoints |

## Architecture

```
                           AgentAuth Broker (:8080)
                          +-----------------------+
                          |                       |
  Agent                   |  Identity   Token     |
  +-----------+           |  Service    Service   |
  | Ed25519   |  Challenge|  (IdSvc)    (TknSvc)  |
  | Key Pair  |<--------->|                       |
  |           |  Register |  Authz MW   Revoke    |
  +-----------+---------->|  (ValMw)    (RevSvc)  |
        |                 |                       |
        | Bearer Token    |  Audit      Deleg     |
        +---------------->|  (AuditLog) (DelegSvc)|
        |                 |                       |
        v                 |  MutAuth    Obs       |
  Resource Server         |  (MutAuthHdl)(Metrics)|
                          +-----------------------+
```

**Request flow:**

1. Agent calls `GET /v1/challenge` to receive a nonce
2. Agent signs nonce with Ed25519 private key
3. Agent calls `POST /v1/register` with launch token + signed nonce + public key
4. Broker validates proof-of-possession, creates SPIFFE ID, issues scoped JWT
5. Agent uses JWT as Bearer token to access protected resources
6. Token auto-expires; broker can also revoke immediately

## Quick Start

### Prerequisites

- Go 1.23+
- Python 3.11+ (for demo application)
- Docker (optional)

### Run the Broker

```bash
git clone https://github.com/YOUR_ORG/agentauth.git
cd agentauth

# Build and run
go run ./cmd/broker

# Verify
curl http://localhost:8080/v1/health
```

### Run with Docker

```bash
docker compose up
```

### Run with Seed Tokens (Development)

```bash
AA_SEED_TOKENS=true go run ./cmd/broker
# Prints SEED_LAUNCH_TOKEN and SEED_ADMIN_TOKEN for development use
```

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/v1/health` | None | Broker liveness and readiness check |
| `GET` | `/v1/metrics` | None | Prometheus metrics endpoint |
| `GET` | `/v1/challenge` | None | Generate nonce for registration proof |
| `POST` | `/v1/register` | Launch token + signature | Register agent, receive SPIFFE ID and scoped JWT |
| `POST` | `/v1/token/validate` | None | Verify token signature, expiry, and scope |
| `POST` | `/v1/token/renew` | Valid token | Rotate token for long-running agents |
| `POST` | `/v1/revoke` | Admin scope | Revoke tokens at 4 levels |
| `POST` | `/v1/delegate` | Valid token | Delegate attenuated scope to another agent |
| `GET` | `/v1/audit/events` | Admin scope | Query immutable audit trail |

See [API Reference](docs/API_REFERENCE.md) for full request/response details.

## Demo Application

The `demo/` directory contains a Python application that demonstrates AgentAuth in action:

- **Resource Server** (FastAPI): Simulated customer database with 4 endpoints and dual-mode auth
- **Demo Agents**: Three agents (DataRetriever, Analyzer, ActionTaker) collaborating via delegation
- **Attack Simulator**: 5 adversarial scenarios showing the security gap (insecure) vs. the fix (secure)
- **Dashboard**: Web UI with real-time event stream and side-by-side comparison

```bash
# Install Python dependencies
cd demo && pip install -r requirements.txt

# Run demo agents in insecure mode (no broker)
python -m resource_server.main --mode insecure &
python -m agents --mode insecure --resource-url http://localhost:8090

# Run in secure mode (with broker)
AA_SEED_TOKENS=true go run ./cmd/broker &
python -m resource_server.main --mode secure &
python -m agents --mode secure \
  --launch-token "$SEED_LAUNCH_TOKEN" \
  --broker-url http://localhost:8080 \
  --resource-url http://localhost:8090
```

See the [User Guide](docs/USER_GUIDE.md) for detailed instructions.

## Configuration

All environment variables are prefixed with `AA_`:

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_PORT` | `8080` | Broker listen port |
| `AA_LOG_LEVEL` | `verbose` | Logging verbosity: `quiet`, `standard`, `verbose`, `trace` |
| `AA_TRUST_DOMAIN` | `agentauth.local` | SPIFFE trust domain |
| `AA_DEFAULT_TTL` | `300` | Token TTL in seconds |
| `AA_SEED_TOKENS` | `false` | Print dev seed tokens on startup |

## Development

```bash
# Build
go build ./...

# Test (all)
go test ./...

# Test (unit only)
go test ./... -short

# Test (single package)
go test ./internal/token/...

# Lint
golangci-lint run ./...

# Python demo tests
cd demo && python -m pytest -v
```

## Documentation

| Document | Purpose |
|----------|---------|
| [User Guide](docs/USER_GUIDE.md) | Operator and evaluator guide |
| [Developer Guide](docs/DEVELOPER_GUIDE.md) | Architecture, packages, coding standards |
| [API Reference](docs/API_REFERENCE.md) | Endpoint request/response details |
| [OpenAPI Spec](docs/api/openapi.yaml) | Machine-readable API contract |
| [Security Pattern](docs/SECURITY_PATTERN.md) | The security pattern AgentAuth implements |
| [Contributing](CONTRIBUTING.md) | How to contribute |
| [Security Policy](SECURITY.md) | Vulnerability reporting |

## License

This project is licensed under the [Apache License 2.0](LICENSE).
