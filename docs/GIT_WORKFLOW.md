# Git Workflow

## Branching model

This repository uses a GitFlow-style model:

- `main`: production-ready history only
- `develop`: integration branch for upcoming release
- `feature/*`: feature and module work branches from `develop`
- `release/*`: stabilization branches from `develop` before merge to `main`
- `hotfix/*`: urgent fixes from `main`, merged back to both `main` and `develop`

## Naming rules

- Feature branches: `feature/mXX-short-topic` (example: `feature/m01-identity-challenge`)
- Release branches: `release/vX.Y.Z`
- Hotfix branches: `hotfix/vX.Y.Z-short-topic`

## Commit standards

Use Conventional Commits:

- `feat(scope): ...`
- `fix(scope): ...`
- `docs(scope): ...`
- `test(scope): ...`
- `chore(scope): ...`

Examples:
- `feat(identity): add challenge nonce handler`
- `docs(api): add register endpoint schema`

## Pull request rules

Each PR must include:

1. Scope summary with module/task ids (for example `M01-T02`)
2. Gate evidence (`./scripts/gates.sh task` at minimum)
3. Test evidence (new/updated tests)
4. Documentation updates (`README`, user/developer/api docs)
5. `CHANGELOG.md` update under `[Unreleased]`

## Merge strategy

- Feature PRs merge into `develop` using squash merge.
- Release branch merges:
  - `release/*` -> `main` (tagged release)
  - `release/*` -> `develop` (back-merge)
- Hotfix branch merges:
  - `hotfix/*` -> `main`
  - `hotfix/*` -> `develop`

## Required local checks before PR

```bash
./scripts/gates.sh task
```

For module completion:

```bash
./scripts/gates.sh module
```

## Gate enforcement

- `task` and `module` gates fail unless current branch is `feature/*` based on local `develop`.
- `task` and `module` also enforce module alignment via `.active_module`:
  - example: `.active_module` = `M01` requires branch `feature/m01-*`
- `milestone` and `all` gates require `develop`, `release/*`, or `hotfix/*`.
- Detached HEAD is always rejected.

## Active module control

Set module context before starting a module:

```bash
./scripts/set_active_module.sh M01
```

This updates `.active_module`, which is enforced by `GITFLOW` gate.
