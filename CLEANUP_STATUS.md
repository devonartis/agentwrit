# Cleanup Status — CC v4 Plan Execution

**Plan:** `.plans/designs/CC-2026-04-04-repo-cleanup-plan-v4.md`
**Started:** 2026-04-04

---

## Phase 1: Repo Renaming

- [x] **Step 1.1:** Rename enterprise repo on GitHub (`devonartis/agentauth` → `devonartis/agentauth-ENT`)
- [x] **Step 1.1a:** Rename local folder (`~/proj/agentauth` → `~/proj/agentauth-ENT`)
- [x] **Step 1.1b:** Update enterprise remote (`git@github.com:devonartis/agentauth-ENT.git`)
- [x] **Step 1.1c:** Push unpushed `fix/pre-modularize-security` branch (184 commits)
- [x] **Step 1.2:** Rename core repo on GitHub (`devonartis/agentauth-core` → `devonartis/agentauth`)
- [ ] **Step 1.3:** Rename local folder — DEFERRED (keep working in `~/proj/agentauth-core` until cleanup done)
- [x] **Step 1.4:** Update core remote (`git@github.com:devonartis/agentauth.git`)
- [x] **Step 1.4a:** Removed stale remotes (`github` → old agentauth-core, `legacy` → dead Devin workspace)

## Phase 2: Repo Cleanup

- [x] **Batch 1.5:** Update go.mod and Go imports (`divineartis` → `devonartis`, 154 occurrences, 46 files). Build passes. All 15 packages PASS.
- [x] **Batch 1:** Deleted sensitive/obsolete files (docs/patent/, COWORK_SESSION.md, COWORK_DOCS_AUDIT.md, tests/FUCKING QUETIONS.MD, .DS_Store files). Dev files (MEMORY, FLOW, TECH-DEBT, CLAUDE, .plans, .claude, .agents) stay on develop — stripped to main via Batch 7.
- [ ] **Batch 2:** Fix CRITICAL docs (TD-S08 field names, TD-S09 secrets, TD-S14 OpenAPI, TD-012 roles.md)
- [ ] **Batch 3:** Fix version/component drift (v1.2→v1.3, 7→8 components)
- [ ] **Batch 4:** Scripts cleanup (remove gates.sh, test_batch.sh, verify_*.sh)
- [ ] **Batch 5:** Review draft docs and CHANGELOG
- [ ] **Batch 6:** Cosmetic "agentauth-core" text replacement + Docker check
- [ ] **Batch 7:** Update .gitignore + create merge-to-main strip script
- [ ] **Batch 8:** Final verification

## Phase 3: Multi-Agent Review

- [ ] Multi-agent review complete
- [ ] Human final approval
- [ ] Make `devonartis/agentauth` public
