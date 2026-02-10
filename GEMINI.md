# AgentAuth: Gemini CLI Context

AgentAuth is an ephemeral agent credentialing broker that issues short-lived, scope-attenuated tokens to AI agents. It implements the Ed25519 challenge-response pattern and SPIFFE identifiers to eliminate long-lived credential exposure.

## Project Overview

- **Language:** Go (1.24+)
- **Architecture:** Clean architecture with logic in `internal/` and HTTP handlers in `internal/handler/`.
- **Key Technologies:**
    - **Cryptography:** Ed25519 for signing and challenge-response.
    - **Identity:** SPIFFE (via `go-spiffe`) for agent identifiers.
    - **Metrics:** Prometheus for observability.
    - **Standards:** RFC 7807 for error reporting (`application/problem+json`).
- **Core Components:**
    - `identity`: Manages agent registration and challenge-response.
    - `token`: Issues and validates EdDSA JWTs.
    - `authz`: Middleware for Bearer token validation and scope enforcement.
    - `revoke`: Handles 4-level revocation (token, agent, task, delegation).
    - `audit`: Maintains a tamper-evident audit trail.
    - `deleg`: Supports scope-attenuated token delegation.

## Building and Running

### Prerequisites
- Go 1.24 or later.
- Set `AA_ADMIN_SECRET` environment variable (required for startup).

### Commands
- **Build:** `go build ./...`
- **Run Broker:** `AA_ADMIN_SECRET="change-me" go run ./cmd/broker`
- **Unit Tests:** `go test ./... -short`
- **All Tests:** `go test ./...`
- **Quality Gates (Task):** `./scripts/gates.sh task` (Build + Lint + Unit Tests)
- **Quality Gates (Module):** `./scripts/gates.sh module` (Task + Integration + Live Tests)

## Configuration

Configuration is managed via environment variables with the `AA_` prefix (see `internal/cfg/cfg.go`):

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_PORT` | `8080` | HTTP listen port |
| `AA_LOG_LEVEL` | `verbose` | `quiet`, `standard`, `verbose`, `trace` |
| `AA_TRUST_DOMAIN`| `agentauth.local` | SPIFFE trust domain |
| `AA_ADMIN_SECRET`| (Required) | Secret for admin authentication |
| `AA_SEED_TOKENS` | `false` | If true, prints seed tokens on startup (Dev only) |

## Development Conventions

- **Clean Architecture:** Keep business logic in `internal/` packages. Services should be injectable into handlers.
- **Error Handling:** Use `handler.WriteProblem` to return RFC 7807 compliant error responses.
- **Testing:** Every package should have corresponding `_test.go` files. Use `-short` to skip long-running integration tests.
- **Observability:** Use the `internal/obs` package for structured logging and metrics.
- **Standard Library:** Prefer the Go standard library (e.g., `net/http` for routing) over external frameworks when possible.
