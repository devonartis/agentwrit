# Decision 011: Develop Stays Messy, Main Stays Clean

**Date:** 2026-04-04
**Status:** Final

## Decision

Development files (MEMORY.md, FLOW.md, TECH-DEBT.md, .plans/, decisions/) live on `develop` with full version history. On merge to `main`, `scripts/strip_for_main.sh` removes them. A pre-commit hook blocks accidental dev file commits to `main`.

`.gitignore` only blocks OS/tool junk (`.DS_Store`, `*.swp`, `bin/`). It does NOT block development files.

## Why

Earlier plans had MEMORY.md and FLOW.md in `.gitignore`, which would have blocked them on `develop` too — losing version history for internal artifacts that matter during development.

The strip script is the right boundary: `develop` tracks everything useful, `main` shows only what the public needs. Contributors on `develop` see the full picture. Visitors on `main` see a clean repo.

## How it works

- `scripts/strip_for_main.sh` — removes dev files, has `--dry-run` flag, refuses to run on `develop`
- `.githooks/pre-commit` — blocks commits of dev files to `main`
- Fast-forward merge from `develop` to `main`, then strip, then commit
