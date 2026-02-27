# Contributing to AgentAuth

Thank you for your interest in contributing to AgentAuth. This guide explains how to set up your development environment, follow our code standards, and submit contributions.

## Code of Conduct

This project is committed to providing a welcoming and inclusive environment for all contributors. Please review our [Code of Conduct](CODE_OF_CONDUCT.md) before participating.

## Development Environment

### Prerequisites

- **Go 1.24 or later**
- **Docker** and **docker-compose** (for running tests and integration environments)
- **Git**

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/agentauth/agentauth.git
cd agentauth

# Install dependencies
go mod download

# Verify your setup
go version
docker version
```

## Building

Build the project with:

```bash
go build ./...
```

This builds all packages in the project. To build specific binaries:

```bash
go build -o bin/broker ./cmd/broker
go build -o bin/sidecar ./cmd/sidecar
```

## Testing

### Running Tests

Run all tests:
```bash
go test ./...
```

Run tests without long-running tests:
```bash
go test ./... -short
```

Run tests for a specific package:
```bash
go test ./internal/handler
```

Run tests with verbose output:
```bash
go test ./... -v
```

### Test Coverage

Generate coverage reports:
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Quality Gates

Before submitting a pull request, ensure all quality gates pass:

```bash
# Run task quality gate (linting, formatting, vet)
./scripts/gates.sh task

# Run module quality gate (dependency checks)
./scripts/gates.sh module
```

These scripts check for:
- Code formatting (gofmt)
- Linting (golangci-lint)
- Unused code (deadcode)
- Import organization
- Module cleanliness

## Project Structure

```
agentauth/
├── cmd/
│   ├── broker/          # Authorization broker server
│   ├── sidecar/         # Token sidecar service
│   ├── aactl/           # Operator CLI (admin auth, sidecars, revocation, audit)
│   └── smoketest/       # End-to-end smoke test runner
├── internal/
│   ├── admin/           # Admin authentication, launch tokens, sidecar activation
│   ├── audit/           # Hash-chain tamper-evident audit trail
│   ├── authz/           # Bearer token validation and scope enforcement middleware
│   ├── deleg/           # Scope-attenuated delegation with chain verification
│   ├── handler/         # HTTP request handlers
│   ├── identity/        # Challenge-response registration, SPIFFE ID generation
│   ├── mutauth/         # Mutual authentication handshake protocol
│   ├── obs/             # Structured logging and Prometheus metrics
│   ├── problemdetails/  # RFC 7807 error responses
│   ├── revoke/          # 4-level revocation (token, agent, task, chain)
│   ├── store/           # SQLite-backed persistence (audit, revocations, sidecars)
│   └── token/           # EdDSA JWT issuance, verification, and renewal
├── docs/                # Documentation
├── scripts/             # Build, test, and deployment scripts
├── tests/               # Test evidence and user stories
└── go.mod
```

## Coding Conventions

### Standard Library Preference

Prefer the Go standard library over external frameworks. For example:
- Use `net/http` instead of web frameworks
- Use `encoding/json` instead of third-party serializers
- Use `testing` package instead of external testing frameworks

External dependencies should only be introduced for functionality that cannot reasonably be implemented with the standard library. Open a discussion issue before adding new dependencies.

### Error Handling

Use RFC 7807 Problem Details for HTTP error responses via the `problemdetails` package:

```go
// Good: RFC 7807 error response
return problemdetails.New(
    http.StatusBadRequest,
    "invalid_token",
    "The provided token could not be validated",
).WriteResponse(w)

// Avoid: Plain text or unstructured errors
http.Error(w, "invalid token", http.StatusBadRequest)
```

### Documentation

Every exported type and function must have a GoDoc comment:

```go
// Server represents the authorization broker.
//
// It handles token validation and authorization decisions.
type Server struct {
    // fields...
}

// Validate checks if the provided token is valid and has the required scope.
func (s *Server) Validate(token, scope string) (bool, error) {
    // implementation
}
```

Comments should be clear, concise, and start with the name being documented. For complex functionality, include examples in doc comments.

### Structured Logging

Use the `obs` package for structured logging:

```go
import "agentauth/internal/obs"

// Good: Structured logging with context
obs.Log(ctx, "token_validated",
    obs.String("token_id", tokenID),
    obs.String("scope", scope),
    obs.Int64("lifetime_ms", lifetime),
)

// Avoid: Printf-style logging
log.Printf("Token %s validated for scope %s", tokenID, scope)
```

### Code Organization

- Keep test files alongside source files with `_test.go` suffix
- One main type per file when reasonable
- Keep functions focused and under 50 lines when possible
- Use interfaces for dependency injection

Example structure:
```
internal/handler/
├── validate.go
├── validate_test.go
├── token.go
└── token_test.go
```

## Pull Request Process

1. **Create a branch** following the naming conventions (see below)
2. **Make your changes** following the coding conventions
3. **Add tests** for new functionality; maintain >80% coverage
4. **Update documentation** if your changes affect user-facing behavior
5. **Run quality gates** to ensure all checks pass
6. **Submit a PR** with a clear description of your changes
7. **Respond to review feedback** promptly

Pull requests must pass all automated checks and receive approval from at least one maintainer.

## Commit Message Conventions

Write clear, descriptive commit messages:

```
Short summary (50 chars or less)

Longer explanation of what changed and why. Explain the
problem you're solving and how this commit solves it.

- Bullet points are fine for multiple related changes
- Keep lines under 72 characters
- Reference issues with "Fixes #123" or "Related to #456"
```

Examples:
- `Add validation for token scope claims`
- `Fix race condition in audit log writer`
- `Refactor handler package structure`
- `Docs: clarify token lifecycle in README`

## Branch Naming

This project uses GitFlow. All branches are based off `develop`:

- `feature/description` - New features
- `fix/description` - Bug fixes and compliance fixes
- `docs/description` - Documentation updates
- `refactor/description` - Code refactoring
- `test/description` - Test improvements

Example: `feature/ed25519-key-rotation`, `fix/structured-audit`

## Adding a New Endpoint

When adding a new authorization endpoint:

1. **Create a handler** in `internal/handler/`:
   ```go
   package handler

   // HandleNewOperation handles the new operation.
   func (s *Server) HandleNewOperation(w http.ResponseWriter, r *http.Request) {
       // Validation
       // Processing
       // Response
       // Audit logging
   }
   ```

2. **Register the route** in the server setup
3. **Add validation tests** in `handler_test.go`
4. **Add audit event** (see below)
5. **Update API documentation**

Reference the existing handler pattern in `internal/handler/` for consistency.

## Adding a New Audit Event Type

When adding a new event that should be audited:

1. **Define the event constant** in `internal/audit/audit_log.go`:
   ```go
   const EventNewOperation = "new_operation"
   ```

2. **Record the event** using the `Record()` method with functional options:
   ```go
   auditLog.Record(
       audit.EventNewOperation,
       agentID,    // agent performing the action
       taskID,     // associated task
       orchID,     // orchestrator ID
       "human-readable detail",
       audit.WithOutcome("success"),          // or "denied"
       audit.WithResource("data:reports"),    // resource being accessed
   )
   ```

3. **Update the API docs** in `docs/api.md` (event types table)

All authorization decisions and security-relevant operations must be audited. Every `Record()` call should include `WithOutcome()` at minimum.

## Documentation Requirements

New features must include:

- **Code comments** explaining the "why" not just the "what"
- **GoDoc comments** on exported types and functions
- **API documentation** for new endpoints
- **CHANGELOG entry** describing the change for users
- **Example usage** for complex features

For significant features, update relevant documentation files in the `docs/` directory.

## Questions or Need Help?

- **Open an issue** for bugs or feature requests
- **Start a discussion** for architectural questions
- **Check existing documentation** in `docs/` for common topics
- **Contact maintainers** for security concerns (see SECURITY.md)

Welcome to the project!
