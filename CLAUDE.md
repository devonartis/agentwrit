# CLAUDE.md

Read `MEMORY.md` and `FLOW.md` first every session.

## AgentAuth

Go broker issuing short-lived scoped JWTs to AI agents via Ed25519 challenge-response. Sidecar proxy handles bootstrap/renewal. Audit trail persists to SQLite.

Module: `github.com/divineartis/agentauth`

**Source Pattern:** [Ephemeral Agent Credentialing v1.2](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.2.md) — this is the security pattern AgentAuth implements. All design decisions trace back to this document.

## Repo Layout

- `cmd/broker/` — broker entry point
- `cmd/sidecar/` — sidecar proxy
- `cmd/aactl/` — aactl operator CLI
- `internal/` — all domain packages (admin, audit, authz, handler, store, token, etc.)
- `docs/` — enterprise docs (v3). **Only application documentation belongs here** — no plans, roadmaps, session artifacts, or other non-application content.
- `scripts/` — gates, live tests, Docker stack
- `CHANGELOG.md` — all changes
- `MEMORY.md` — session work log

For deeper context on architecture, API, or operations see `docs/`.

## Workflow

- GitFlow: `main` -> `develop` -> `fix/*` or `feature/*` branches
- Run `./scripts/gates.sh task` before every PR
- Test with Docker (`./scripts/stack_up.sh`), not `go run`
- Show terminal evidence when claiming tests pass
- Update docs + CHANGELOG with every feature

## Live Test Rules

**A live test means the Docker stack is running. No Docker = not a live test.**

- `./scripts/stack_up.sh` must be run first — app must be up in containers before any live test
- Self-hosted binary tests are NOT live tests — they are quick local integration checks only
- Every fix/feature MUST have a Docker live test before merge — `./scripts/live_test.sh --docker`
- Write user stories FIRST, save to `tests/<fix-or-feature-name>-user-stories.md`, before writing test code
- If a fix adds new env vars, update `docker-compose.yml` to pass them into the container
- Never claim a live test passes without showing Docker terminal evidence

## Delegation Rules

Always delegate these operations to sub-agents instead of running inline:

- **Gate checks**: Use the `gate-runner` sub-agent for running `gates.sh`
- **Codebase research**: Use `Explore` sub-agents for searching and understanding code
- **Code review**: Use `code-reviewer` sub-agents after completing a feature
- **Multi-step implementation**: Use `general-purpose` sub-agents for independent tasks in a plan

Keep the main conversation focused. Sub-agents handle verbose output (test logs, search results, review details) and return only summaries.
