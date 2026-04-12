# Contributing to AgentAuth

Thank you for your interest in contributing to AgentAuth. This guide covers everything you need to set up your environment, follow our standards, and submit contributions that can be reviewed and merged with confidence.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [License and CLA](#license-and-cla)
- [Project Structure](#project-structure)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Testing](#testing)
- [Code Style](#code-style)
- [Pull Request Process](#pull-request-process)
- [Security Contributions](#security-contributions)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## License and CLA

The core AgentAuth server is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**. By contributing, you agree that your contributions will be licensed under AGPL-3.0.

Substantial contributions (anything beyond typo fixes or minor doc corrections) require accepting the **[Contributor License Agreement](CLA.md)**. The CLA grants the project maintainer additional rights for commercial and enterprise use while you retain full ownership of your contributions. See `CLA.md` for the complete terms.

**We cannot merge non-trivial contributions unless the CLA has been accepted.**

## Project Structure

```
agentauth/
├── cmd/
│   ├── broker/              # Credential broker HTTP server (main binary)
│   └── awrit/               # Operator CLI — admin auth, app management, audit
├── internal/
│   ├── admin/               # Admin authentication (bcrypt, shared secret)
│   ├── app/                 # App registration, credential lifecycle, launch tokens
│   ├── audit/               # Tamper-evident audit trail (hash-chain)
│   ├── authz/               # Scope enforcement, permission checks
│   ├── cfg/                 # Configuration parsing (YAML, env vars, CLI flags)
│   ├── deleg/               # Delegation chains — task-scoped token derivation
│   ├── handler/             # HTTP handlers (admin, app, token, health)
│   ├── identity/            # SPIFFE-format agent identity, challenge-response
│   ├── keystore/            # Ed25519 key management (persistent signing key)
│   ├── mutauth/             # Mutual TLS / mTLS support
│   ├── obs/                 # Observability — structured logging, Prometheus metrics
│   ├── problemdetails/      # RFC 9457 error responses
│   ├── revoke/              # Token revocation at 4 levels (token, agent, task, chain)
│   ├── store/               # SQLite persistence layer
│   └── token/               # JWT issuance, verification, renewal (Ed25519/EdDSA)
├── docs/                    # User-facing documentation
├── scripts/                 # Build, test, and deployment scripts
├── tests/                   # Acceptance test evidence and user stories
├── go.mod
├── LICENSE                  # AGPL-3.0
├── CLA.md                   # Contributor License Agreement
└── ENTERPRISE_LICENSE.md    # Commercial licensing summary
```

## Prerequisites

Before you start, ensure you have:

- **[Go 1.24+](https://go.dev/dl/)** — the broker and CLI are pure Go
- **[Docker](https://docs.docker.com/get-docker/)** and **[Docker Compose](https://docs.docker.com/compose/install/)** — required for integration testing
- **Git** — we use GitFlow branching

## Getting Started

1. **Fork the repository** to your GitHub account

2. **Clone your fork locally:**

   ```bash
   git clone https://github.com/<your-username>/agentauth.git
   cd agentauth
   ```

3. **Install dependencies:**

   ```bash
   go mod download
   ```

4. **Verify your setup — build both binaries:**

   ```bash
   go build -o bin/broker ./cmd/broker/
   go build -o bin/awrit  ./cmd/awrit/
   ```

5. **Run unit tests:**

   ```bash
   go test ./...
   ```

6. **Start a local broker (Docker) to verify end-to-end:**

   ```bash
   export AA_ADMIN_SECRET="$(openssl rand -base64 32)"
   ./scripts/stack_up.sh
   curl -s http://localhost:8080/v1/health | jq .
   # Should return {"status":"ok", ...}
   ./scripts/stack_down.sh
   ```

If all of the above pass, you are ready to contribute.

## Development Workflow

### Branch Model

Day-to-day work happens on **`develop`**. The **`main`** branch is the public release branch — it does not include internal tracking files. Development files are stripped automatically on merge to `main` via `scripts/strip_for_main.sh`.

**Do not commit dev-only files directly to `main`.**

### Creating a Branch

Create a branch from `develop` using the appropriate prefix:

```bash
git checkout develop
git pull origin develop
git checkout -b <type>/<description>
```

| Prefix | Purpose |
|--------|---------|
| `feature/` | New features or capabilities |
| `fix/` | Bug fixes |
| `docs/` | Documentation changes |
| `refactor/` | Code refactoring (no behavior change) |
| `test/` | Test improvements |
| `security/` | Security fixes or hardening |

Examples: `feature/key-rotation`, `fix/renew-ttl-preservation`, `security/rate-limit-app-auth`

### Making Changes

1. **Open an issue first** describing the bug, feature, or refactor you are proposing. For larger changes, discuss the approach in the issue before writing code.

2. **Write your code** following the [Code Style](#code-style) guidelines.

3. **Write tests** for your changes (see [Testing](#testing)).

4. **Update documentation** if your changes affect user-facing behavior. Documentation must update in the same branch as the code — no "fix docs later." The docs to check: `docs/api.md`, `docs/architecture.md`, `docs/concepts.md`, `docs/implementation-map.md`, `docs/scenarios.md`, `docs/api/openapi.yaml`.

5. **Run quality gates** before committing:

   ```bash
   gofmt -l ./...                              # Format check (no output = clean)
   go vet ./...                                # Static analysis
   go test ./...                               # Unit tests
   go build -o bin/broker ./cmd/broker/        # Broker builds
   go build -o bin/awrit  ./cmd/awrit/         # CLI builds
   ```

### Commit Messages

Write clear, descriptive commit messages:

```
type(scope): short summary (50 chars or less)

Longer explanation of what changed and why. Explain the problem
you're solving and how this commit solves it.

- Bullet points are fine for multiple related changes
- Keep lines under 72 characters
- Reference issues with "Fixes #123" or "Related to #456"
```

Type prefixes: `feat`, `fix`, `docs`, `refactor`, `test`, `security`, `chore`

Examples:
- `feat(deleg): add max delegation depth configuration`
- `fix(token): preserve TTL on renewal instead of resetting`
- `security(admin): add rate limiting to admin auth endpoint`
- `docs(api): update revocation endpoint examples`

## Testing

All contributions must include appropriate tests. AgentAuth has two layers of contributor testing and a third layer maintained by the project team.

### Unit Tests (required for all code changes)

- **Table-driven tests** with `t.Run` subtests — non-negotiable
- Test files live next to the code they test: `foo_test.go` beside `foo.go`
- Run with `go test ./...` or `go test ./... -short` to skip long-running tests

Example:

```go
func TestScopeIsSubset(t *testing.T) {
    tests := []struct {
        name     string
        request  []string
        ceiling  []string
        want     bool
    }{
        {"exact match", []string{"read:data:x"}, []string{"read:data:x"}, true},
        {"wildcard ceiling", []string{"read:data:x"}, []string{"read:data:*"}, true},
        {"exceeds ceiling", []string{"write:data:x"}, []string{"read:data:*"}, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := authz.ScopeIsSubset(tt.request, tt.ceiling)
            if got != tt.want {
                t.Errorf("ScopeIsSubset(%v, %v) = %v, want %v",
                    tt.request, tt.ceiling, got, tt.want)
            }
        })
    }
}
```

### Integration Evidence (required for broker-facing changes)

If your change affects HTTP handlers, token issuance, revocation, audit, or any behavior visible through the broker's API:

1. **Build the binary** — always test with compiled binaries, never `go run`
2. **Start the broker** via Docker (`./scripts/stack_up.sh`) or VPS mode (bare binary)
3. **Run your test against the live broker** and capture the terminal output
4. **Include the evidence in your PR description** — paste the terminal output showing the test passing

This is how maintainers verify that your change works against a real broker, not just in unit tests.

Example of what to include in your PR:

```
## Integration Evidence

Broker started via Docker, tested endpoint:

$ curl -s -X POST http://localhost:8080/v1/admin/auth \
    -H "Content-Type: application/json" \
    -d '{"secret":"..."}' | jq .
{
  "access_token": "eyJ...",
  "expires_in": 300,
  "token_type": "Bearer"
}

$ curl -s -X POST http://localhost:8080/v1/revoke \
    -H "Authorization: Bearer eyJ..." \
    -H "Content-Type: application/json" \
    -d '{"level":"token","target":"<jti>"}' | jq .
{
  "status": "revoked",
  "level": "token"
}
```

### Acceptance Tests (maintained by the project team)

Formal acceptance tests live in `tests/<feature>/` and follow the process defined in `tests/LIVE-TEST-TEMPLATE.md`. These are written and maintained by the project team after a contribution is merged. They use a specific methodology with personas, executive-readable banners, and structured evidence files.

**Contributors do not need to write acceptance tests.** Your unit tests and integration evidence give maintainers what they need to write acceptance stories for your contribution.

### What Must Be Tested

| Change type | Required from contributor |
|-------------|-------------------------|
| New endpoint | Unit tests for handler + integration evidence against live broker |
| Bug fix | Regression test that fails without the fix, passes with it |
| New config option | Unit test for parsing + integration evidence showing behavior change |
| Security fix | Unit test + integration evidence showing the vulnerability is closed |
| Refactor | Existing tests must pass unchanged (no test modifications unless behavior changes) |
| Documentation only | No tests required |

### Test Coverage

Generate coverage reports:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Code Style

### Go Standards

- **`gofmt`** is the formatter — no exceptions
- **Error wrapping:** `fmt.Errorf("context: %w", err)` — always wrap with context
- **Return errors, don't panic.** Reserve `panic` for truly unrecoverable programmer errors
- **Context:** pass `ctx context.Context` as the first parameter
- **No `init()` functions.** Explicit initialization only
- **No global mutable state.** Pass dependencies via constructors
- **Interfaces at the consumption site,** not the definition site
- **All crypto uses Go stdlib.** No third-party crypto libraries

### Standard Library Preference

Prefer the Go standard library over external frameworks:

- `net/http` instead of web frameworks
- `encoding/json` instead of third-party serializers
- `testing` package instead of external testing frameworks
- `crypto/ed25519` instead of third-party crypto

External dependencies should only be introduced for functionality that cannot reasonably be implemented with the standard library. **Open a discussion issue before adding any new dependency.**

### Error Responses

Use RFC 9457 Problem Details for all HTTP error responses via the `problemdetails` package:

```go
// Correct: RFC 9457 structured error
return problemdetails.New(
    http.StatusBadRequest,
    "invalid_token",
    "The provided token could not be validated",
).WriteResponse(w)

// Wrong: unstructured error
http.Error(w, "invalid token", http.StatusBadRequest)
```

### Code Comments

Comments must explain what **reading the code alone would NOT tell you**:

- **Who** calls this and why — which role (Admin, App, Agent), which endpoint, which scope
- **Why** this exists — the business reason, the security property, the design decision
- **Boundaries** — what this code is NOT responsible for, what the caller must ensure

```go
// Bad — restates the function name:
// handleCreateLaunchToken handles launch token creation.

// Good — tells you things you can't learn from reading the body:
// Called by: Apps (POST /v1/app/launch-tokens, scope app:launch-tokens:*) and
// Admin (POST /v1/admin/launch-tokens, scope admin:launch-tokens:*).
// App callers are constrained by their scope ceiling (ScopeIsSubset check).
// Admin callers bypass the ceiling — this is a bootstrapping/dev convenience.
```

### Structured Logging

Use the `obs` package for structured logging:

```go
import "github.com/devonartis/agentwrit/internal/obs"

// Correct: Structured with context
obs.Log(ctx, "token_validated",
    obs.String("token_id", tokenID),
    obs.String("scope", scope),
)

// Wrong: Printf-style
log.Printf("Token %s validated for scope %s", tokenID, scope)
```

### Security Rules (non-negotiable)

- Never leak internal state in error responses — no stack traces, file paths, or internal identifiers
- Never log secrets — no client_secret, API keys, or tokens in audit records or log output
- Constant-time comparison for all secret/token comparisons
- Tokens must expire — no indefinite tokens, reject TTL of 0 or negative
- Request body size limits on all endpoints — no unbounded reads
- Security-sensitive events (auth failures, scope violations, revocations) must get audit entries
- Security headers on all HTTP responses

## Pull Request Process

1. **Ensure your branch is up to date** with `develop`:

   ```bash
   git fetch origin
   git rebase origin/develop
   ```

2. **Verify all quality gates pass:**

   ```bash
   gofmt -l ./...                              # No output = clean
   go vet ./...                                # No warnings
   go test ./...                               # All pass
   go build -o bin/broker ./cmd/broker/        # Broker builds
   go build -o bin/awrit  ./cmd/awrit/         # CLI builds
   ```

3. **Submit your PR** against **`develop`** (not `main`) with:

   - A clear title following commit message conventions
   - Description of **what** changed and **why**
   - Reference to the issue it addresses (e.g., "Fixes #42")
   - Integration evidence for broker-facing changes (terminal output in PR description)
   - Documentation updates if behavior changed

4. **CLA acceptance** — if this is your first substantial contribution, you will be asked to confirm that you accept the [CLA](CLA.md)

5. **Code review** — all PRs require approval from at least one maintainer. Be responsive to feedback.

### PR Checklist

Before marking your PR as ready for review:

- [ ] Code compiles (`go build ./...`)
- [ ] All unit tests pass (`go test ./...`)
- [ ] New code has unit tests (table-driven with subtests)
- [ ] Integration evidence included for broker-facing changes
- [ ] `gofmt` and `go vet` are clean
- [ ] Comments explain who/why/boundaries (not what the code does)
- [ ] No new dependencies without prior discussion
- [ ] Documentation updated if behavior changed
- [ ] Commit messages follow conventions
- [ ] PR targets `develop`, not `main`
- [ ] CLA accepted (first-time contributors)

## Security Contributions

If you discover a security vulnerability, **do not open a public issue.** See [SECURITY.md](SECURITY.md) for responsible disclosure instructions.

Security-related PRs (hardening, new security checks, vulnerability fixes) are welcome and receive priority review. Include:

- A clear description of the security property being added or fixed
- Integration evidence that the fix works (terminal output showing the vulnerability is closed)
- Assessment of whether the fix could break existing behavior

## Questions?

- **Open an issue** for bugs or feature requests
- **Start a discussion** for architectural questions
- **See [SECURITY.md](SECURITY.md)** for security concerns

Thank you for helping improve AgentAuth.
