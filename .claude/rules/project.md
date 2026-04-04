# Project — AgentAuth Core

## Architecture

Two binaries, one Go module:

| Binary | Path | Purpose |
|--------|------|---------|
| `broker` | `cmd/broker/` | HTTP server — credential broker, token issuer, audit recorder |
| `aactl` | `cmd/aactl/` | CLI tool — operator interface for admin, app management, audit |

## Internal Packages

| Package | Responsibility |
|---------|---------------|
| `admin` | Admin authentication (bcrypt, shared secret) |
| `app` | App registration, credential lifecycle, launch tokens |
| `audit` | Tamper-evident audit trail (hash-chain) |
| `authz` | Scope enforcement, permission checks |
| `cfg` | Configuration parsing (YAML, env vars, CLI flags) |
| `deleg` | Delegation chains — task-scoped token derivation |
| `handler` | HTTP handlers (admin, app, token, health, OIDC discovery) |
| `identity` | SPIFFE-format agent identity |
| `keystore` | Ed25519 key management (persistent signing key) |
| `mutauth` | Mutual TLS / mTLS support |
| `obs` | Observability — Prometheus metrics |
| `problemdetails` | RFC 9457 error responses |
| `revoke` | Token revocation at 4 levels (token, agent, task, chain) |
| `store` | SQLite persistence layer |
| `token` | JWT issuance, verification, renewal (Ed25519/EdDSA) |

## Key Commands

```bash
# Build
go build -o bin/broker ./cmd/broker
go build -o bin/aactl ./cmd/aactl

# Unit tests
go test ./...
go test ./... -short          # skip long-running tests

# Start broker (Docker)
export AA_ADMIN_SECRET="$(openssl rand -base64 32)"
./scripts/stack_up.sh

# Start broker (VPS mode — compiled binary)
./bin/broker --admin-secret "$AA_ADMIN_SECRET"

# Health check
curl -s http://localhost:8080/v1/health | jq .
```

## Test Structure

Tests live in `tests/<phase-or-fix>/` with user stories, env files, and evidence directories. See `.claude/rules/testing.md` for the full process.

```
tests/
  p0-production-foundations/    # Phase 0 — graceful shutdown, persistent key
  p1-admin-secret/              # Phase 1 — bcrypt admin auth, config file
  sec-l1/                       # Security Level 1 — bind address, TLS, denylist
  sec-l2a/                      # Security Level 2a — token hardening
  sec-l2b/                      # Security Level 2b — headers, body limits, error sanitization
  app-launch-tokens/            # App credential lifecycle
  LIVE-TEST-TEMPLATE.md         # Canonical process — read this first
```

## Current State

Read `MEMORY.md` for migration status, current branch, cherry-pick progress, and tech debt. Read `FLOW.md` for decision history.
