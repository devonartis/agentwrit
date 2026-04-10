# Decision 003: GitFlow Branching

**Date:** 2026-03-29
**Status:** Final

## Decision

All work happens on `fix/*` or `feature/*` branches off `develop`. Merged to `develop` after verification. `develop` merges to `main` periodically via `scripts/strip_for_main.sh`, which removes development files (MEMORY.md, FLOW.md, TECH-DEBT.md, .plans/).

## Why

`develop` is the working branch — it tracks everything useful including decision logs, memory, and tech debt. `main` is what the public sees — clean, no internal artifacts. The strip script enforces this boundary automatically.

This is better than `.gitignore` because development files get full version history on `develop`. A pre-commit hook blocks accidental commits of dev files to `main`.

## Trade-offs

- **Accepted:** Two long-lived branches to keep in sync. Mitigated by fast-forward merges and the strip script.
- **Rejected alternative:** `.gitignore` for dev files — would block them on `develop` too.
