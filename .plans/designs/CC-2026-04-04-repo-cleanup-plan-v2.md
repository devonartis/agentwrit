# CC-2026-04-04: Repo Cleanup & Rename Plan v2

**Author:** Claude Code
**Date:** 2026-04-04
**Status:** DRAFT — awaiting user review
**Supersedes:** CC-2026-04-04-repo-cleanup-plan.md (v1 — overcomplicated, enterprise extraction map was scope creep)
**Informed by:** PI-2026-04-04-merged-cleanup-plan.md (good detail, same scope creep problem)

---

## Goal

Make `divineartis/agentauth-core` become `divineartis/agentauth` on GitHub. The old `divineartis/agentauth` (which has HITL, OIDC, enterprise code) gets backed up locally and removed from GitHub. Clean this repo so it's presentable as open-source.

That's it. Enterprise extraction is future work — we have the code, we'll catalog it when we need it.

---

## Key Fact: Go Module Path Already Correct

`go.mod` says `module github.com/divineartis/agentauth`. No Go import changes needed. Only cosmetic text references to "agentauth-core" in docs/evidence files.

---

## Part 1: GitHub Operations (do once)

| Step | What | Command / Action |
|------|------|-----------------|
| 1 | Back up old `divineartis/agentauth` locally | `git clone --mirror git@github.com:divineartis/agentauth.git ~/backups/agentauth-enterprise/` |
| 2 | Make old repo private (or delete) on GitHub | GitHub Settings → Danger Zone → Make private |
| 3 | Rename `agentauth-core` → `agentauth` on GitHub | GitHub Settings → Repository name → `agentauth` |
| 4 | Update local git remote | `git remote set-url origin git@github.com:divineartis/agentauth.git` |

**Note on Step 2:** Making private is safer than deleting — you keep the GitHub issue history, stars, etc. as reference. Enterprise code (HITL, OIDC) should not be publicly visible.

---

## Part 2: Repo Cleanup (what ships publicly)

### Batch 1: Remove internal artifacts

**Remove these files — they're agent session state, internal coordination, or sensitive:**

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

**Verification:** `git status` shows deletions only, build still passes.

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

**TD-S14 — OpenAPI spec has 51 stale sidecar endpoints:**

| File | Action |
|------|--------|
| `docs/api/openapi.yaml` | Remove all sidecar/token-exchange routes, add missing app endpoints, fix auth field names |

**TD-012 — Missing roles doc:**

| File | Action |
|------|--------|
| `docs/roles.md` | **CREATE** — Admin/App/Agent definitions, scopes, production flow, CAN/CANNOT matrix |

**Verification:** Test example commands against live broker.

### Batch 3: Fix version/component drift

| File | Current | Target |
|------|---------|--------|
| `README.md` | v1.2, 7 components | v1.3, 8 components |
| `docs/architecture.md` | 7 components | 8 components |
| `docs/concepts.md` | 7 components (inconsistent) | 8 components |
| `docs/getting-started-operator.md` | 7 components | 8 components |

The 8 components of Ephemeral Agent Credentialing v1.3:
1. Identity
2. Authentication
3. Delegation
4. Token Lifecycle
5. Revocation
6. Scope Model
7. Audit Trail
8. App Registration

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

### Batch 5: Review draft docs

These came from recent agent sessions. Decision needed for each:

| File | Action needed |
|------|--------------|
| `docs/cc-foundations.md` | Ship, improve, or remove? |
| `docs/cc-scope-model.md` | Ship, improve, or remove? |
| `docs/cc-token-concept.md` | Ship, improve, or remove? |
| `docs/cc-design-decisions.md` | Ship, improve, or remove? |
| `docs/token-roles.md` | Merge into new roles.md or remove? |
| `docs/agentauth-explained.md` | Ship, improve, or remove? |
| `KNOWN-ISSUES.md` | Ship sanitized or remove? |

### Batch 6: Cosmetic "agentauth-core" text replacement

Grep for remaining "agentauth-core" references in surviving files and replace with "agentauth". Most of the original 26 files will already be deleted by Batch 1. Remaining will be README, test evidence, and a few docs.

### Batch 7: Update .gitignore

Add patterns so deleted internal artifacts don't get recreated by contributors or agent tools:

```
# Internal tooling — not part of open-source release
.agents/
.claude/
.plans/
MEMORY.md
MEMORY_ARCHIVE.md
FLOW.md
TECH-DEBT.md
COWORK_SESSION.md
```

### Batch 8: Final verification

- Run full test suite (`go test ./...`)
- Docker build + health check
- Grep for remaining sidecar/HITL/OIDC references in Go code (contamination check)
- Grep for `change-me-in-production` (should be zero)
- Grep for `client_id` / `client_secret` in docs (should be zero)
- Verify `docs/roles.md` exists
- Verify all docs say v1.3 and 8 components
- Read through README as a first-time visitor

---

## Open Questions (Require Your Decision)

1. **SOUL.md** — ship (project principles) or remove?
2. **KNOWN-ISSUES.md** — ship sanitized or remove?
3. **Draft docs (cc-*.md, token-roles.md, agentauth-explained.md)** — ship, improve, or remove?
4. **Test evidence directories** — ship (demonstrates rigor) or remove (exposes internal process)?
5. **Old repo** — make private or delete on GitHub?
6. **live_test.sh / live_test_docker.sh** — fix or remove?

---

## Success Criteria

- [ ] Old `divineartis/agentauth` backed up locally and off GitHub (or private)
- [ ] This repo renamed to `divineartis/agentauth` on GitHub
- [ ] No internal files in repo (MEMORY, FLOW, .plans, .claude, patent, etc.)
- [ ] No broken examples (rejected secrets, wrong field names)
- [ ] OpenAPI spec matches actual API routes
- [ ] `docs/roles.md` exists
- [ ] All docs say v1.3, 8 components
- [ ] Build and tests pass
- [ ] Repo presentable to a first-time open-source visitor

---

**END OF PLAN**

**Authorization Required:** No actions until explicitly authorized.
