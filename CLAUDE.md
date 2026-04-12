# AgentAuth Core

## Rules

**At session start, ALWAYS `Read` these rule files before doing anything else:**
- `.claude/rules/mission.md` — what this project is and why it exists
- `.claude/rules/project.md` — codebase layout and architecture
- `.claude/rules/security.md` — security expectations, non-negotiable
- `.claude/rules/testing.md` — LIVE test and acceptance test expectations
- `.claude/rules/golang.md` — Go coding standards
- `.claude/rules/mandatory-reading.md` — **Read this entirely and summarize it back to the user at session start.** Defines what goes in MEMORY.md vs FLOW.md vs TECH-DEBT.md. Do NOT mix them up.
- `.claude/rules/karpathy.md` — coding discipline (simplicity, surgical changes, goal-driven execution). Always active alongside the rules above.

## Defaults

- **jCodeMunch is the default** for all code lookup. Fall back to `Read`/`Grep`/`Glob` only when jCodeMunch has no results or you are about to `Edit`.
- **Index on session start.** Run `index_folder(incremental=true)` if stale. Index docs too — `docs/`, `tests/`, `MEMORY.md`, `FLOW.md` are first-class artifacts.
- **Read docs before code.** `MEMORY.md` has current state. `FLOW.md` has decision history.
