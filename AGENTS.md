# AGENTS.md

**Last updated:** 2026-04-08  
**Purpose:** Handoff notes for the next agent session.

---

## Current State

**✅ Repo cleanup is COMPLETE.** Public release preparation is **documented**; flip the repo public only after maintainer review of `.plans/reviews/public-release-review-2026-04-08.md` and license decision in `FLOW.md`.

- 8 batches executed (go module rename, sensitive file removal, critical docs fixed, version drift fixed, scripts cleanup, cosmetic fixes, gitignore + strip script, final verification)
- `develop` → `main` merge completed (last merge: v2.0.0)
- `main` is clean: ~199 files, no dev artifacts (MEMORY, FLOW, `.plans/`, etc. stripped)
- Remote: `devonartis/agentauth` (private until you publish)
- Strip script verified; build passes after strip

### Public release readiness (develop-only artifacts)

| Artifact | Purpose |
|----------|---------|
| `.plans/release-readiness.md` | Merge checklist, license options (Apache vs source-available), grep audits |
| `.plans/reviews/public-release-review-2026-04-08.md` | Security / docs / surface-area review snapshot |

**Rule:** Internal recommendations stay in stripped paths or `.plans/` — never only on `main` unless intentionally public (README, LICENSE, `docs/`).

---

## What Was Done (Summary)

See `FLOW.md` for **decisions only** — not full history. Key outcomes:

1. **Repo renamed:** `agentauth-core` → `agentauth` (canonical name reserved for OSS core)
2. **Enterprise repo:** `agentauth` → `agentauth-ENT` (private, contains HITL/OIDC for future extraction)
3. **Go module:** Updated to `github.com/devonartis/agentauth` (154 occurrences, 46 files)
4. **Dev files organized:** All session tracking (MEMORY, FLOW, TECH-DEBT, AGENTS, .claude/, .plans/) on `develop`, stripped from `main`
5. **Public docs aligned:** v1.2→v1.3, 7→8 components, API contracts corrected
6. **Dead code removed:** Sidecar references, obsolete scripts, duplicate templates

---

## Project Structure (Canonical)

```
devonartis/agentauth        ← This repo (OSS core, private until review)
  main    → clean public release (199 files)
  develop → full dev workspace (233 files)

devonartis/agentauth-ENT    ← Enterprise code (HITL, OIDC), private
devonartis/agentauth-internal ← Golden history archive
```

---

## Next Phase: Multi-Agent Review

**Before going public**, this repo needs review by multiple agents. Focus areas:

### 1. Security Review
- Review `internal/token/` — JWT handling, Ed25519 signing, revocation
- Review `internal/authz/` — scope enforcement, rate limiting
- Review `internal/admin/` — bcrypt auth, admin secret handling
- Check for any hardcoded secrets, weak defaults, or bypass paths

### 2. Documentation Review
- Read `README.md` as a first-time visitor — does it make sense?
- Check `docs/` for any remaining "agentauth-core" or "v1.2" references
- Verify all code examples work against the actual broker
- Look for stale references to removed features (sidecar, HITL, OIDC)

### 3. Code Quality Review
- Check Go comments — do they explain role/boundary/intent per `.claude/rules/golang.md`?
- Look for missing error handling
- Verify test coverage (run `go test ./... -cover`)
- Check for any TODO/FIXME comments that should be resolved

### 4. Public Surface Review
- What files ship to `main`? (run `git ls-tree -r main --name-only`)
- Any internal artifacts accidentally included?
- Does `strip_for_main.sh` catch everything it should?

---

## File Purposes (Don't Mix These Up)

| File | Purpose | Branch |
|------|---------|--------|
| `AGENTS.md` | **This file** — handoff notes for next session | develop |
| `MEMORY.md` | Session context, lessons learned, current state | develop |
| `FLOW.md` | **Decision log only** — what was decided and why | develop |
| `TECH-DEBT.md` | Known issues, deferred work, severity tracking | develop |
| `SOUL.md` | Project principles, philosophy | develop |

**Important:** FLOW.md is for **decisions**, not history dumps. When adding to FLOW.md:
- Good: "Decision: Renamed repo to agentauth to reserve canonical name"
- Bad: "Then I ran git status and saw 15 files changed so I checked the diff and..."

---

## If Resuming Cold

1. Read `MEMORY.md` — current state and recent lessons
2. Read `FLOW.md` — key decisions (skim the headers)
3. Read `README.md` — as if you're a new contributor
4. Run `./scripts/gates.sh task` — verify build/tests pass
5. Check `git log main -10` — what's on the public branch

Then start your work.

---

## Constraints

- **Never** commit dev files (MEMORY, FLOW, AGENTS, .claude/, .plans/) to `main`
- The `strip_for_main.sh` script enforces this — use it before any main commit
- `develop` is private — `main` will eventually be public
- Enterprise code (HITL, OIDC) is in `agentauth-ENT`, not this repo
