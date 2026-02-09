# AgentAuth

AgentAuth is an ephemeral agent credentialing broker that issues short-lived, scope-attenuated tokens to AI agents. It implements the [Ephemeral Agent Credentialing](plans/Security-Pattern-That-Is-Why-We-Built-AgentAuth.md) security pattern: each agent proves identity via Ed25519 challenge-response, receives a SPIFFE-format identifier, and operates with only the permissions its task requires. Tokens expire in minutes, not hours, eliminating the credential exposure window that plagues traditional IAM approaches to AI agent security.

## Quick Start

```bash
# 1. Build
go build ./...

# 2. Configure (optional -- defaults are safe for local dev)
export AA_ADMIN_SECRET="change-me-in-production"

# 3. Run
go run ./cmd/broker

# 4. Test health
curl http://localhost:8080/v1/health
```

The broker starts on port 8080 by default. Set `AA_PORT` to change it.

## Architecture

```
                          AgentAuth Broker (:8080)
                         +-------------------------+
                         |                         |
  Agent                  |  Identity   Token       |
  +----------+           |  Service    Service     |   Resource
  | Ed25519  |--challenge-->  |           |        |   Server
  | key pair |--register---->  |           |        |   +------+
  |          |<--JWT token----+-----------+        |   |      |
  |          |--request + Bearer token-------------------> |  |
  +----------+           |  Authz    Revoke        |   +------+
                         |  Middleware Service      |
  Admin                  |     |         |         |
  +----------+           |  Audit    Delegation    |
  | client   |--auth---->|  Log      Service       |
  | secret   |<--admin-->|     |         |         |
  +----------+  token    |  Prometheus Metrics      |
                         +-------------------------+
```

**Key components:**

| Component | Package | Purpose |
|-----------|---------|---------|
| Identity Service | `internal/identity` | Challenge-response registration, SPIFFE ID generation, Ed25519 key management |
| Token Service | `internal/token` | EdDSA JWT issuance, verification, and renewal |
| Authz Middleware | `internal/authz` | Bearer token validation, scope enforcement on every request |
| Revocation Service | `internal/revoke` | 4-level revocation (token, agent, task, delegation chain) |
| Audit Log | `internal/audit` | Hash-chain tamper-evident audit trail with PII sanitization |
| Delegation Service | `internal/deleg` | Scope-attenuated delegation with chain verification |
| Admin Service | `internal/admin` | Admin authentication, launch token lifecycle |
| Observability | `internal/obs` | Structured logging, Prometheus metrics |

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/v1/challenge` | None | Obtain a cryptographic nonce (30s TTL) |
| `POST` | `/v1/register` | Launch token | Register agent with signed nonce and public key |
| `POST` | `/v1/token/validate` | None | Verify a token and return decoded claims |
| `POST` | `/v1/token/renew` | Bearer | Renew a token with fresh timestamps |
| `POST` | `/v1/delegate` | Bearer | Create scope-attenuated delegation token |
| `POST` | `/v1/revoke` | Bearer + `admin:revoke:*` | Revoke tokens at 4 levels |
| `GET` | `/v1/audit/events` | Bearer + `admin:audit:*` | Query the audit trail |
| `POST` | `/v1/admin/auth` | None | Authenticate admin with shared secret |
| `POST` | `/v1/admin/launch-tokens` | Bearer + `admin:launch-tokens:*` | Create launch tokens |
| `GET` | `/v1/health` | None | Health check (status, version, uptime) |
| `GET` | `/v1/metrics` | None | Prometheus metrics |

All error responses use [RFC 7807](https://tools.ietf.org/html/rfc7807) `application/problem+json`.

See [docs/api/openapi.yaml](docs/api/openapi.yaml) for the full machine-readable API specification.

## Configuration

All environment variables are prefixed with `AA_`:

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_PORT` | `8080` | HTTP listen port |
| `AA_LOG_LEVEL` | `verbose` | Logging level: `quiet`, `standard`, `verbose`, `trace` |
| `AA_TRUST_DOMAIN` | `agentauth.local` | SPIFFE trust domain for agent IDs |
| `AA_DEFAULT_TTL` | `300` | Default token TTL in seconds (5 minutes) |
| `AA_ADMIN_SECRET` | *(empty)* | Shared secret for admin authentication (required in production) |
| `AA_SEED_TOKENS` | `false` | Print seed launch/admin tokens on startup (dev only) |

## Running Tests

```bash
go test ./...                     # all tests
go test ./... -short              # unit tests only (skip integration)
go test ./internal/token/...      # single package
```

## Docker

```bash
docker compose up --build         # starts broker on :8080
```

The image uses a multi-stage build (golang:1.23-alpine builder, alpine:3.19 runtime) producing a static binary.

## Documentation

- [API Reference](docs/API_REFERENCE.md) -- endpoint details and examples
- [Developer Guide](docs/DEVELOPER_GUIDE.md) -- architecture, conventions, contributing
- [User Guide](docs/USER_GUIDE.md) -- workflows and integration patterns
- [OpenAPI Spec](docs/api/openapi.yaml) -- machine-readable API contract
- [Security Pattern](plans/Security-Pattern-That-Is-Why-We-Built-AgentAuth.md) -- the "why" behind AgentAuth
- [Changelog](CHANGELOG.md) -- release history

## License

See [LICENSE](LICENSE) for details.
