# CLAUDE.md

Read `MEMORY.md` first every session.

## AgentAuth

Go broker issuing short-lived scoped JWTs to AI agents via Ed25519 challenge-response. Sidecar proxy handles bootstrap/renewal. Audit trail persists to SQLite.

Module: `github.com/divineartis/agentauth`

## Repo Layout

- `cmd/broker/` — broker entry point
- `cmd/sidecar/` — sidecar proxy
- `internal/` — all domain packages (admin, audit, authz, handler, store, token, etc.)
- `docs/` — enterprise docs (v3)
- `scripts/` — gates, live tests, Docker stack
- `CHANGELOG.md` — all changes
- `MEMORY.md` — session work log

For deeper context on architecture, API, or operations see `docs/`.

## Workflow

- GitFlow: `main` -> `develop` -> feature branches
- Run `./scripts/gates.sh task` before every PR
- Test with Docker (`./scripts/stack_up.sh`), not `go run`
- Show terminal evidence when claiming tests pass
- Update docs + CHANGELOG with every feature
