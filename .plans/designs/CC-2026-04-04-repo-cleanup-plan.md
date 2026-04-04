# CC-2026-04-04: Repo Cleanup & Rename Plan

**Author:** Claude Code
**Date:** 2026-04-04
**Status:** DRAFT — awaiting user review

---

## Context

Three repos exist:
- `devonartis/agentauth-internal` — golden history, 412 commits (archive #1, already archived)
- `divineartis/agentauth` — enterprise code: HITL approval flow, OIDC provider, cloud federation, migration planning docs
- `divineartis/agentauth-core` — open-source core, B0-B6 merged, all gates green

**Goal:** Archive the old `divineartis/agentauth`, rename `agentauth-core` → `divineartis/agentauth` so the open-source core gets the canonical name.

**Constraint:** The old `divineartis/agentauth` is NOT throwaway. It contains HITL and OIDC code that will be extracted later for the paid/enterprise binary.

---

## Key Finding: Go Module Path Already Correct

`go.mod` already says `module github.com/divineartis/agentauth` — no Go import path changes needed. The 26 files referencing "agentauth-core" are all docs, plans, and test evidence — cosmetic text replacements only.

---

## Sequence

### Step A: Write Enterprise Extraction Doc

**Why first:** Do while the old repo is fresh in mind. Once archived, nobody will remember what's in there without a map.

**Deliverable:** A document cataloging everything in old `divineartis/agentauth` that core doesn't have:
- HITL approval flow — modules, files, interfaces, how it plugs into core
- OIDC provider — modules, files, interfaces
- Cloud federation / credential exchange code
- Enterprise-specific tests
- How each module connects to core's extension points
- What work is needed to extract, test, and build the paid binary

**Where:** `.plans/designs/CC-enterprise-extraction-map.md` (stays in .plans/, does NOT ship)

### Step B: Fix CRITICAL Docs

These are the first things any user hits. Broken examples = nobody can try the project.

| ID | What | Impact |
|----|------|--------|
| TD-S08/S09 | Wrong API field names (`client_id`/`client_secret` → `secret`) and rejected secrets (`change-me-in-production`) in examples | Broker FAILs on startup if user copies example secret. API calls fail with wrong field names. |
| TD-012 | Missing `docs/roles.md` — who does what (Admin/App/Agent), scopes, production flow | Every contributor and agent misunderstands the system without this. |
| TD-S14 | OpenAPI spec has 51 stale sidecar endpoint references | API consumers get wrong picture of available endpoints. |

### Step C: Clean Internal Artifacts

**Remove before public release:**

| File/Dir | Reason |
|----------|--------|
| `MEMORY.md` | Agent session state — internal |
| `MEMORY_ARCHIVE.md` | Agent session state — internal |
| `FLOW.md` | Decision log — internal |
| `COWORK_SESSION.md` | Agent coordination — internal |
| `COWORK_DOCS_AUDIT.md` | Agent audit — internal |
| `TECH-DEBT.md` | Has internal details — sanitize or remove |
| `SOUL.md` | Internal |
| `Archive.zip` | Internal |
| `.plans/` | Entire directory — tracker, cherry-picks, designs, specs |
| `docs/patent/` | **NEVER ship** |
| `tests/FUCKING QUETIONS.MD` | Remove |
| `AGENTS.md` | Agent config — internal |

**Decision needed:** Do we keep these in a branch/tag before deleting, or just delete?

### Step D: Fix Remaining Docs & Scripts

**Docs to review/fix:**

| File | Issue |
|------|-------|
| `docs/cc-foundations.md` | Draft from recent session — ship, improve, or remove? |
| `docs/cc-scope-model.md` | Draft — ship, improve, or remove? |
| `docs/cc-token-concept.md` | Draft — ship, improve, or remove? |
| `docs/cc-design-decisions.md` | Draft — ship, improve, or remove? |
| `docs/token-roles.md` | Draft from other agent — ship, improve, or remove? |
| `docs/agentauth-explained.md` | Draft from other agent — ship, improve, or remove? |
| `README.md` | Needs update for open-source (references agentauth-core) |

**Scripts to fix or remove:**

| Script | Issue | Recommendation |
|--------|-------|---------------|
| `scripts/live_test.sh` | TD-S01: stale sidecar refs | Fix or remove |
| `scripts/live_test_docker.sh` | TD-S03: stale sidecar refs | Fix or remove (test_batch.sh may replace it) |
| `scripts/verify_compose.sh` | TD-S13: stale sidecar refs | Fix or remove |
| `scripts/gen_test_certs.sh` | TD-S12: generates sidecar certs | Remove sidecar cert gen, keep broker certs |
| `scripts/gates.sh` | TD-S13: references live_test_sidecar.sh | Fix |
| `scripts/test_batch.sh` | Migration tool | Remove or keep for CI? |

### Step E: Archive Old Repo on GitHub

Now safe — we documented what's in it (Step A).

1. Rename `divineartis/agentauth` → `divineartis/agentauth-enterprise-archive`
2. Mark as archived on GitHub (read-only)
3. Update any references in agentauth-core docs

### Step F: Rename Core → agentauth on GitHub

Last step — everything clean.

1. Rename `divineartis/agentauth-core` → `divineartis/agentauth` on GitHub
2. Update git remote: `git remote set-url origin git@github.com:divineartis/agentauth.git`
3. Find-and-replace "agentauth-core" → "agentauth" in 26 docs/plans/evidence files
4. Update `README.md` for open-source (badges, install instructions, etc.)

---

## Open Questions

1. **Draft docs (cc-*.md, token-roles.md, agentauth-explained.md):** Ship as-is, improve first, or remove? These came from recent sessions and may not be polished.
2. **Internal artifacts:** Tag/branch before removing, or just delete? A tag preserves the full history including MEMORY.md etc. for reference.
3. **Test evidence:** Ship the `tests/` directories with acceptance evidence? They demonstrate testing rigor but also expose internal process.
4. **Enterprise archive name:** `agentauth-enterprise-archive`? `agentauth-v1-enterprise`? Something else?
5. **Scripts:** Which scripts are worth fixing vs removing? `gates.sh` and `test_batch.sh` could become real CI if cleaned up.

---

## Enterprise Code Preservation Note

The archived `divineartis/agentauth` contains code for the paid/enterprise binary:

| Module | What It Does | Core Interface It Plugs Into |
|--------|-------------|------------------------------|
| HITL Approval Flow | Human-in-the-loop approval before agent gets token | Token issuance pipeline |
| OIDC Provider | OpenID Connect identity provider for agent tokens | Identity/auth layer |
| Cloud Federation | Exchange agent tokens for cloud provider credentials | Token exchange |
| Federation Bridge | Cross-org agent credential trust | Identity federation |

This code needs to be extracted, tested against current core, and built as the enterprise binary. That's future work — but the extraction map (Step A) ensures we know exactly what to pull and where it connects.
