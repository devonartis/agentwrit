# CC-2026-04-04: Repo Cleanup & Rename Plan v3

**Author:** Claude Code
**Date:** 2026-04-04
**Status:** DRAFT — awaiting user review
**Supersedes:** CC-2026-04-04-repo-cleanup-plan-v2.md
**Informed by:** PI-2026-04-04-merged-cleanup-plan.md (staging mindset, rename-not-delete, local folder rename, multi-agent review gate)

---

## Goal

Make `divineartis/agentauth-core` become `divineartis/agentauth` on GitHub. The old `divineartis/agentauth` (HITL, OIDC, enterprise code) gets renamed to `agentauth-ENT` and stays private. Clean this repo so it's presentable as open-source.

Enterprise extraction is future work — we have the code in `agentauth-ENT`, we'll catalog it when we need it.

**Everything stays PRIVATE until multi-agent review approves going public.**

---

## Core Principles

| Principle | Rationale |
|-----------|-----------|
| **All repos stay PRIVATE during cleanup** | Safe to iterate and fix. No public exposure until ready. |
| **Human review after every batch** | No batch proceeds without explicit user approval. Catches mistakes early. |
| **Rename, don't delete** | Enterprise repo becomes `agentauth-ENT` — keeps GitHub history, issues, stars as reference. |
| **GitHub rename first, then local** | GitHub is source of truth. Local folders match after. |
| **No enterprise extraction map** | Scope creep. Code is in `agentauth-ENT`. Catalog when needed. |

---

## Key Fact: Go Module Path Already Correct

`go.mod` says `module github.com/divineartis/agentauth`. No Go import changes needed. Only cosmetic text references to "agentauth-core" in docs/evidence files.

---

## Canonical Public Story

### Product Identity

| Aspect | Canonical Value |
|--------|-----------------|
| **Product Name** | AgentAuth |
| **Repository** | `divineartis/agentauth` (after rename) |
| **Pattern** | Ephemeral Agent Credentialing v1.3 |
| **Components** | 8 (not 7) |
| **Broker Version** | 2.0.0 |

### The 8 Components

1. Identity — hash-chain identity
2. Authentication — Ed25519 signing
3. Delegation — scope-attenuated credential passing
4. Token Lifecycle — issue, renew, release with preserved TTL
5. Revocation — token, agent, task, chain-level
6. Scope Model — hierarchical capability attenuation
7. Audit Trail — credential lifecycle and security events
8. App Registration — multi-tenant app isolation with scope ceilings

### Role Model

| Role | Purpose | CAN do | CANNOT do |
|------|---------|--------|-----------|
| **Admin** | Manages system | Register apps, revoke tokens, audit | Create launch tokens, act as app |
| **App** | Manages its agents | Create launch tokens (within ceiling), manage agents | Exceed scope ceiling, register other apps |
| **Agent** | Does work | Validate tokens, renew, delegate | Escalate scope, access admin endpoints |

---

## Phase 1: Repo Renaming (GitHub First, Then Local)

### Step 1.1: Rename Enterprise Repo on GitHub

**Action:** Rename `divineartis/agentauth` → `divineartis/agentauth-ENT`

**Validation:**
- GitHub UI shows new name
- Old URL redirects to new name
- Repo remains private

**⏸ HUMAN REVIEW — confirm rename succeeded before proceeding.**

### Step 1.2: Rename Core Repo on GitHub

**Action:** Rename `divineartis/agentauth-core` → `divineartis/agentauth`

**Validation:**
- GitHub UI shows new name
- Old URL redirects to new name
- Repo remains private

**⏸ HUMAN REVIEW — confirm rename succeeded before proceeding.**

### Step 1.3: Rename Local Folders (order matters)

Enterprise folder first (frees the `agentauth` name), then core:

```bash
mv ~/proj/agentauth ~/proj/agentauth-ENT
mv ~/proj/agentauth-core ~/proj/agentauth
```

**Validation:** `ls ~/proj/` shows `agentauth` and `agentauth-ENT`

**⏸ HUMAN REVIEW — confirm local folders correct before proceeding.**

### Step 1.4: Update Git Remotes

In `~/proj/agentauth-ENT`:
```bash
git remote set-url origin git@github.com:divineartis/agentauth-ENT.git
git remote -v   # validate
git fetch        # validate connection
```

In `~/proj/agentauth`:
```bash
git remote set-url origin git@github.com:divineartis/agentauth.git
git remote -v   # validate
git fetch        # validate connection
```

**⏸ HUMAN REVIEW — confirm both remotes work before proceeding to Phase 2.**

### Phase 1 Result

| Location | GitHub Repo | Local Folder |
|----------|-------------|--------------|
| Enterprise code | `divineartis/agentauth-ENT` (private) | `~/proj/agentauth-ENT` |
| Open-source core | `divineartis/agentauth` (private) | `~/proj/agentauth` |

---

## Phase 2: Repo Cleanup (What Ships Publicly)

All work happens in `~/proj/agentauth`. Enterprise repo stays untouched.

### Batch 1: Remove internal artifacts

**Remove these files/directories — agent session state, internal coordination, or sensitive:**

| File/Dir | Why |
|----------|-----|
| `MEMORY.md` | Agent session state |
| `MEMORY_ARCHIVE.md` | Agent session state |
| `FLOW.md` | Internal decision log |
| `AGENTS.md` | Agent config |
| `CLAUDE.md` | Claude Code context |
| `COWORK_SESSION.md` | Agent coordination |
| `COWORK_DOCS_AUDIT.md` | Agent audit |
| `TECH-DEBT.md` | Internal tracking (has design details) |
| `SOUL.md` | Review first — project principles may have value |
| `Archive.zip` | Unknown internal archive |
| `.DS_Store` | macOS junk |
| `.agents/` | Agent skills directory |
| `.claude/` | Claude Code config |
| `.plans/` | All planning docs, tracker, designs |
| `docs/patent/` | **NEVER ship** |
| `tests/FUCKING QUETIONS.MD` | Remove |

**Verification:** `git status` shows deletions only. `go build ./...` and `go test ./...` still pass.

**⏸ HUMAN REVIEW — confirm deletions are correct. No code was removed, only internal artifacts.**

### Batch 2: Fix CRITICAL docs

These are the first things any user hits. Broken examples = nobody can try the project.

**TD-S08 — Wrong API field names:**

| File | Lines | Wrong | Correct |
|------|-------|-------|---------|
| `docs/api.md` | 52, 248, 255 | `client_id`, `client_secret` | `secret` |
| `docs/getting-started-operator.md` | 467, 489, 604 | `client_id`, `client_secret` | `secret` |

**TD-S09 — Rejected secret in examples:**

| File | Lines | Wrong | Correct |
|------|-------|-------|---------|
| `README.md` | 185 | `change-me-in-production` | `aactl init` or generate real secret |
| `docs/getting-started-operator.md` | 76 | `change-me-in-production` | `aactl init` |
| `docs/common-tasks.md` | 1145 | `change-me-in-production` | `aactl init` |
| `scripts/stack_up.sh` | 9 | `change-me-in-production` | `${AA_ADMIN_SECRET}` (require it, don't default) |

**TD-S14 — OpenAPI spec (51 stale sidecar endpoints):**
- Remove all sidecar/token-exchange routes
- Add missing app endpoints (`/v1/app/auth`, `/v1/app/launch-tokens`)
- Fix auth field names

**TD-012 — Create `docs/roles.md`:**
- Admin/App/Agent definitions, scopes, production flow, CAN/CANNOT matrix

**Verification:** Test example commands against live broker.

**⏸ HUMAN REVIEW — verify doc fixes are accurate. Review `docs/roles.md` for correctness.**

### Batch 3: Fix version/component drift

| File | Current | Target |
|------|---------|--------|
| `README.md` | v1.2, 7 components | v1.3, 8 components |
| `docs/architecture.md` | 7 components | 8 components |
| `docs/concepts.md` | 7 components (inconsistent) | 8 components |
| `docs/getting-started-operator.md` | 7 components | 8 components |

**Verification:** Grep for "7 component" and "v1.2" — should be zero in docs.

**⏸ HUMAN REVIEW — confirm all docs now say v1.3 and 8 components consistently.**

### Batch 4: Scripts cleanup

| Script | Action |
|--------|--------|
| `scripts/stack_up.sh` | **SHIP** — keep |
| `scripts/stack_down.sh` | **SHIP** — keep |
| `scripts/gen_test_certs.sh` | **SHIP** — remove sidecar cert generation (TD-S12) |
| `scripts/gates.sh` | Remove |
| `scripts/test_batch.sh` | Remove |
| `scripts/verify_compose.sh` | Remove |
| `scripts/verify_dockerfile.sh` | Remove |
| `scripts/live_test.sh` | Review — fix sidecar refs (TD-S01) or remove |
| `scripts/live_test_docker.sh` | Review — fix sidecar refs (TD-S03) or remove |

**Verification:** `scripts/stack_up.sh` and `scripts/stack_down.sh` still work.

**⏸ HUMAN REVIEW — confirm which scripts to keep. Decide on live_test.sh and live_test_docker.sh.**

### Batch 5: Review draft docs and CHANGELOG

Decision needed for each:

| File | Decision needed |
|------|----------------|
| `docs/cc-foundations.md` | Ship, improve, or remove? |
| `docs/cc-scope-model.md` | Ship, improve, or remove? |
| `docs/cc-token-concept.md` | Ship, improve, or remove? |
| `docs/cc-design-decisions.md` | Ship, improve, or remove? |
| `docs/token-roles.md` | Merge into new roles.md or remove? |
| `docs/agentauth-explained.md` | Ship, improve, or remove? |
| `KNOWN-ISSUES.md` | Ship sanitized or remove? |
| `CHANGELOG.md` | Review for internal refs (B0-B6 batch names, agent session details) — sanitize if needed |

**⏸ HUMAN REVIEW — decide on each file. This batch is entirely your call.**

### Batch 6: Cosmetic text replacement and Docker check

- Grep for remaining "agentauth-core" references in surviving files and replace with "agentauth"
- Check `Dockerfile` and `docker-compose*.yml` for `agentauth-core` in image names or labels

**Verification:** `grep -ri "agentauth-core" .` returns zero results (excluding `.git/`).

**⏸ HUMAN REVIEW — confirm all references updated.**

### Batch 7: Update .gitignore

Add patterns so deleted internal artifacts don't get recreated by contributors or agent tools:

```
# Internal tooling — not part of open-source release
.agents/
.claude/
.plans/
```

**Note:** Not gitignoring `MEMORY.md`, `FLOW.md`, etc. — these are generic filenames a contributor might legitimately use. The deletion + code review is sufficient.

**Verification:** `.gitignore` updated, no unintended side effects.

**⏸ HUMAN REVIEW — confirm .gitignore patterns are what you want.**

### Batch 8: Final verification

- Run full test suite (`go test ./...`)
- Docker build + health check
- Grep for sidecar/HITL/OIDC in Go code (contamination check)
- Grep for `change-me-in-production` (should be zero)
- Grep for `client_id` / `client_secret` in docs (should be zero)
- Grep for `agentauth-core` (should be zero)
- Verify `docs/roles.md` exists
- Verify all docs say v1.3 and 8 components
- Read through README as a first-time visitor

**⏸ HUMAN REVIEW — confirm all checks pass. This is the last gate before multi-agent review.**

---

## Phase 3: Multi-Agent Review (Before Going Public)

**Do NOT make repo public until multiple agents review and approve.**

Review checklist:
- [ ] All docs consistent (v1.3, 8 components, correct field names)
- [ ] No internal files remain
- [ ] No broken examples
- [ ] OpenAPI spec matches actual routes
- [ ] `docs/roles.md` complete and accurate
- [ ] CHANGELOG clean
- [ ] Build and tests pass
- [ ] First-time visitor experience is clean

**⏸ HUMAN REVIEW — final approval. When ready, make `divineartis/agentauth` public.**

---

## Open Questions (Require Your Decision)

1. **SOUL.md** — ship (project principles) or remove?
2. **KNOWN-ISSUES.md** — ship sanitized or remove?
3. **Draft docs (cc-*.md, token-roles.md, agentauth-explained.md)** — ship, improve, or remove?
4. **Test evidence directories** — ship (demonstrates rigor) or remove (exposes process)?
5. **live_test.sh / live_test_docker.sh** — fix or remove?
6. **CHANGELOG.md** — sanitize internal refs or ship as-is?

---

## Success Criteria

- [ ] Phase 1: GitHub repos renamed, local folders match, remotes work
- [ ] Batch 1: Internal artifacts removed
- [ ] Batch 2: Critical docs fixed (TD-S08, TD-S09, TD-S14, TD-012)
- [ ] Batch 3: All docs say v1.3, 8 components
- [ ] Batch 4: Scripts cleaned
- [ ] Batch 5: Draft docs decided
- [ ] Batch 6: All "agentauth-core" references replaced, Docker checked
- [ ] Batch 7: .gitignore updated
- [ ] Batch 8: Final verification passes
- [ ] Phase 3: Multi-agent review complete
- [ ] Repo public when human approves

---

## Quick Reference

### Cannot Ship
- `docs/patent/`, `MEMORY.md`, `MEMORY_ARCHIVE.md`, `FLOW.md`, `AGENTS.md`, `CLAUDE.md`, `TECH-DEBT.md`
- `COWORK_SESSION.md`, `COWORK_DOCS_AUDIT.md`
- `.agents/`, `.claude/`, `.plans/`
- `tests/FUCKING QUETIONS.MD`

### Must Fix
- `README.md` (v1.3, 8 components, secrets)
- `docs/api.md` (field names)
- `docs/getting-started-operator.md` (components, field names, secrets)
- `docs/api/openapi.yaml` (remove sidecar, fix routes)
- `CHANGELOG.md` (review for internal refs)

### Must Create
- `docs/roles.md`

### Must Check
- `Dockerfile` / `docker-compose*.yml` for `agentauth-core` image names

---

**END OF PLAN**

**Authorization Required:** No actions until explicitly authorized. Human review required after every batch.
