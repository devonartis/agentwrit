# Contributing

How to contribute to AgentAuth. Covers development setup, coding conventions, and the pull request process.

---

## Development Setup

### Prerequisites

- **Go 1.24+** — [Download](https://go.dev/dl/)
- **Docker** — [Download](https://docs.docker.com/get-docker/)
- **docker-compose** — Usually included with Docker Desktop

### Clone and Build

```bash
git clone https://github.com/devonartis/agentauth-core.git
cd agentauth-core

# Build all binaries
go build ./cmd/broker
go build ./cmd/sidecar
go build ./cmd/aactl

# Run tests
go test ./...
```

### Run Locally

```bash
# Start with Docker Compose
export AA_ADMIN_SECRET="$(openssl rand -hex 32)"
docker compose up -d

# Or run directly
export AA_ADMIN_SECRET="dev-secret"
go run ./cmd/broker &
go run ./cmd/sidecar &
```

---

## Project Structure

```
cmd/           # Entry points (main.go for each binary)
  broker/      # Broker server
  sidecar/     # Sidecar proxy
  aactl/       # CLI tool

internal/      # Private packages (not importable by external code)
  broker/      # Broker implementation
    handler/   # HTTP handlers
    service/   # Business logic
    store/     # Data access (SQLite)
    middleware/ # Auth, logging, rate limiting
    config/    # Configuration
  sidecar/     # Sidecar implementation
  crypto/      # Ed25519, JWT
  audit/       # Audit trail
  scope/       # Scope parsing and matching
  spiffe/      # SPIFFE ID generation

docs/          # Documentation
scripts/       # Utility scripts
```

---

## Coding Conventions

### General

- **Standard library first:** Prefer `net/http`, `crypto/ed25519`, `encoding/json` over third-party alternatives
- **No frameworks:** HTTP routing uses stdlib patterns
- **GoDoc comments:** All exported types and functions must have GoDoc comments
- **Error handling:** Return errors, don't panic. Use `fmt.Errorf("context: %w", err)` for wrapping.

### Error Responses

All HTTP errors must use RFC 7807 format:

```go
func writeError(w http.ResponseWriter, status int, errCode, detail, hint string) {
    w.Header().Set("Content-Type", "application/problem+json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]any{
        "type":       fmt.Sprintf("urn:agentauth:error:%s", errCode),
        "title":      http.StatusText(status),
        "status":     status,
        "detail":     detail,
        "error_code": errCode,
        "hint":       hint,
    })
}
```

### Naming

- **Packages:** lowercase, single word (`scope`, `audit`, `crypto`)
- **Files:** lowercase with underscores (`token_service.go`)
- **Interfaces:** verb-like names (`Store`, `Signer`, `Validator`)
- **Handlers:** `{Resource}{Action}Hdl` (e.g., `TokenRenewHdl`)

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run a specific package
go test ./internal/scope/...

# Verbose output
go test -v ./internal/broker/service/...
```

---

## Adding a New Endpoint

1. **Define the handler** in `internal/broker/handler/`
2. **Register the route** in the router setup
3. **Add middleware** (auth, rate limiting) as needed
4. **Add audit events** for the operation
5. **Write tests** in the same package
6. **Update documentation** in `docs/api.md`

---

## Adding a New Audit Event

1. Define the event type constant
2. Call the audit logger with the event details
3. Include: event type, agent ID (if applicable), outcome, detail string
4. The hash chain is computed automatically

---

## Pull Request Process

### Branch Naming

Follow GitFlow conventions:
- `feature/description` — new features
- `fix/description` — bug fixes
- `docs/description` — documentation
- `refactor/description` — code restructuring

### PR Requirements

- [ ] All tests pass (`go test ./...`)
- [ ] Code builds cleanly (`go build ./...`)
- [ ] GoDoc comments on all exported items
- [ ] RFC 7807 error format for any new HTTP errors
- [ ] Audit events for any new operations
- [ ] Documentation updated if behavior changes

### Review Process

1. Open a PR against `main`
2. At least one maintainer review required
3. CI must pass (tests, build, lint)
4. Squash merge preferred

---

## Getting Help

- **Issues:** https://github.com/devonartis/agentauth-core/issues
- **Security:** security@agentauth.dev
- **Docs:** [[Home]] (this wiki)

---

## Next Steps

- [[Architecture]] — Understand the codebase
- [[API Reference]] — Endpoint documentation
- [[Home]] — Back to wiki home
