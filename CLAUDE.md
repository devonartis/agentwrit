# CLAUDE.md

Read `MEMORY.md` first every session.

## AgentAuth

Go broker issuing short-lived scoped JWTs to AI agents via Ed25519 challenge-response. Sidecar proxy handles bootstrap/renewal. Audit trail persists to SQLite.

Module: `github.com/divineartis/agentauth`

## Repo Layout

- `cmd/broker/` — broker entry point
- `cmd/sidecar/` — sidecar proxy
- `internal/` — all domain packages (admin, audit, authz, handler, store, token, etc.)
- `docs/` — enterprise docs (v3). **Only application documentation belongs here** — no plans, roadmaps, session artifacts, or other non-application content.
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

## Delegation Rules

Always delegate these operations to sub-agents instead of running inline:

- **Gate checks**: Use the `gate-runner` sub-agent for running `gates.sh`
- **Codebase research**: Use `Explore` sub-agents for searching and understanding code
- **Code review**: Use `code-reviewer` sub-agents after completing a feature
- **Multi-step implementation**: Use `general-purpose` sub-agents for independent tasks in a plan

Keep the main conversation focused. Sub-agents handle verbose output (test logs, search results, review details) and return only summaries.
