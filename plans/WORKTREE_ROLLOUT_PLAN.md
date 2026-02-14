# AgentAuth Worktree Rollout Plan

## Objective
Create a stable Git worktree workflow so we can run parallel efforts without branch-switching churn, while keeping `develop` as the integration baseline.

## Why We Are Doing This
- Keep the primary checkout on `develop` for integration checks and shared verification.
- Isolate active work by branch to avoid accidental carry-over changes.
- Reduce friction when context-switching between module work, hotfixes, and docs.

## Proposed Layout
Assume current repo root is `/Users/divineartis/proj/agentAuth`.

- Main checkout: `/Users/divineartis/proj/agentAuth` on `develop`
- Worktree root: `/Users/divineartis/proj/agentAuth-worktrees`
- Per-branch worktrees:
  - `/Users/divineartis/proj/agentAuth-worktrees/mXX-<topic>` for module/feature branches
  - `/Users/divineartis/proj/agentAuth-worktrees/hotfix-<topic>` for urgent fixes
  - `/Users/divineartis/proj/agentAuth-worktrees/docs-<topic>` for doc-only work

## Branch Naming
Use `codex/*` branch names for newly created work branches.

Examples:
- `codex/m09-observability-hardening`
- `codex/hotfix-token-validation`
- `codex/docs-rc-checklist-refresh`

## Standard Flow
1. Ensure `/Users/divineartis/proj/agentAuth` is on `develop` and up to date.
2. Create a worktree for each planned stream.
3. Perform all changes, tests, and commits inside that worktree.
4. Merge back through PR flow into `develop`.
5. Remove worktree after branch is merged.

## Command Reference
```bash
# from the main repo
mkdir -p /Users/divineartis/proj/agentAuth-worktrees

# create a new branch + worktree from develop
git worktree add /Users/divineartis/proj/agentAuth-worktrees/mXX-topic -b codex/mXX-topic develop

# list active worktrees
git worktree list

# remove completed worktree
git worktree remove /Users/divineartis/proj/agentAuth-worktrees/mXX-topic

# delete local branch after merge
git branch -d codex/mXX-topic
```

## Guardrails
- Never do active feature work directly in the main checkout.
- Keep one concern per worktree branch.
- Re-run module/task gates before opening or merging PRs.
- Remove stale worktrees weekly to avoid drift.

## Immediate Next Step
After this document is committed to `develop`, create the first worktree for the next active module stream.
