# AgentAuth

AgentAuth is an ephemeral agent credentialing broker that issues short-lived, scope-attenuated tokens to AI agents. It implements the [Ephemeral Agent Credentialing](plans/Security-Pattern-That-Is-Why-We-Built-AgentAuth.md) security pattern: each agent proves identity via Ed25519 challenge-response, receives a SPIFFE-format identifier, and operates with only the permissions its task requires. Tokens expire in minutes, not hours, eliminating the credential exposure window that plagues traditional IAM approaches to AI agent security.

## Release Status

**Current release:** MVP Prototype (pattern validation release)

This release validates that AgentAuth is a working implementation of the target security pattern and is ready for controlled demos, integration testing, and senior-engineering productionization.

This is intentionally **not** a production-hardening release. Production controls (transport hardening, deployment architecture, and operations posture) are handled in a follow-on build-out phase.

For full release framing and handoff scope, see [plans/AgentAuth-MVP-Release-Writeup-v1.0.md](plans/AgentAuth-MVP-Release-Writeup-v1.0.md).

## Quick Start

```bash
# 1. Build
go build ./...

# 2. Configure (required -- AA_ADMIN_SECRET must be set)
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
| `AA_ADMIN_SECRET` | **(required)** | Shared secret for admin authentication. Broker exits on startup if unset. |
| `AA_SEED_TOKENS` | `false` | Print seed launch/admin tokens on startup (dev only) |

## Running Tests

```bash
go test ./...                     # all tests
go test ./... -short              # unit tests only (skip integration)
go test ./internal/token/...      # single package
```

## Production Deployment

The broker listens on plain HTTP by default. **Production deployments MUST use a TLS-terminating reverse proxy** (e.g., nginx, envoy, Caddy) or configure a load balancer with TLS termination. Native TLS support (`AA_TLS_CERT`, `AA_TLS_KEY`) is planned for a future release.

Example with nginx:

```nginx
server {
    listen 443 ssl;
    server_name agentauth.example.com;
    ssl_certificate     /etc/ssl/certs/agentauth.pem;
    ssl_certificate_key /etc/ssl/private/agentauth-key.pem;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Docker (Broker + Sidecar)

This repo includes a Docker Compose stack for the broker and sidecar runtime.
The demo app is intentionally separate and should run from its own repository.

One-command startup:

```bash
./scripts/stack_up.sh
```

One-command teardown:

```bash
./scripts/stack_down.sh
```

Run live E2E (always deploys compose stack first):

```bash
./scripts/live_test.sh
```

## Documentation

- [API Reference](docs/API_REFERENCE.md) -- endpoint details and examples
- [Agent Integration Guide](docs/AGENT_INTEGRATION_GUIDE.md) -- step-by-step Python/TypeScript agent integration
- [Developer Guide](docs/DEVELOPER_GUIDE.md) -- architecture, conventions, contributing
- [User Guide](docs/USER_GUIDE.md) -- workflows and integration patterns
- [OpenAPI Spec](docs/api/openapi.yaml) -- machine-readable API contract
- [Security Pattern](plans/Security-Pattern-That-Is-Why-We-Built-AgentAuth.md) -- the "why" behind AgentAuth
- [Changelog](CHANGELOG.md) -- release history

## License

See [LICENSE](LICENSE) for details.
