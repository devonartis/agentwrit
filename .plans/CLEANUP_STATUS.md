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
- [x] **Batch 2:** TD-S08 already resolved. TD-S09 fixed stack_up.sh. TD-S14 (sidecar) already resolved. Added /v1/app/launch-tokens + single_use to OpenAPI. Renamed token-roles.md → roles.md for TD-012. (Broken link to deleted token-concept.md flagged for Batch 5.)
- [x] **Batch 3:** Fixed v1.2→v1.3 and 7→8 components in README.md (2 spots), docs/concepts.md, docs/architecture.md, docs/getting-started-operator.md. All public docs consistent.
- [x] **Batch 4:** Deleted 3 broken/obsolete scripts: test_batch.sh (migration tool), live_test.sh (references deleted cmd/smoketest), live_test_docker.sh (tests non-existent sidecar endpoints). Removed live_test reference from README.md. Verified `gates.sh task` still passes. gen_test_certs.sh already clean (TD-S12 resolved).
- [x] **Batch 5:** Renamed 4 cc-*.md files (dropped cc- prefix), fixed cross-refs, replaced roles.md internal link to credential-model.md. Deleted KNOWN-ISSUES.md (all 4 known issues were sidecar-related, obsolete). Rewrote CHANGELOG.md (732 → 128 lines): removed all Cherry-Pick Details, Tech Debt Discovered, Contamination notes, B0-B6 refs, legacy sidecar/HITL/OIDC content. Kept feature/security content grouped under [Unreleased] and [2.0.0].
- [x] **Batch 6:** Fixed clone URLs + cd commands in README.md and docs/getting-started-user.md (divineartis/agentauth-core → devonartis/agentauth). Docker check: Dockerfile and docker-compose*.yml clean (zero refs). Test comment in internal/token/tkn_svc_test.go:523 references "agentauth-core" — DEFERRED to post-freeze (code change not allowed now).
- [x] **Batch 7:** Rewrote .gitignore — only OS/tool junk, build artifacts, *.db, .env, Python caches, worktrees, dolt. Removed all dev-file and internal-artifact ignores. Created scripts/strip_for_main.sh — removes MEMORY.md, FLOW.md, TECH-DEBT.md, .plans/, .claude/, .agents/, audit/, AGENTS.md, CLAUDE.md, CLEANUP_STATUS.md, docx reports, DOC-AUDIT-REPORT.md files. Includes build verification after stripping.
- [x] **Batch 8:** Final verification PASS. go build/test all 15 packages OK. Zero sidecar/hitl/oidc/federation in Go code (1 stale comment flagged for post-freeze). Zero divineartis, zero agentauth-core in public docs/README/CHANGELOG. Admin auth uses {secret}. docs/roles.md exists. v1.3/8-components consistent everywhere. Fixed README badge URLs (divineartis→devonartis).
- [x] **Root cleanup:** Deleted 5 items (Archive.zip, AgentAuth_Code_Review.docx, audit/AgentAuth_Doc_Audit.docx, bin/, audit-reports/). Moved 2 items (CLEANUP_STATUS.md → .plans/, SOUL.md → .claude/). Removed CLEANUP_STATUS.md from strip_for_main.sh (now covered by .plans/ strip).
- [x] **.plans/ cleanup:** Deleted .plans/cherry-pick/ (7 files), old CC plan iterations v1-v3, redundant designs, migration tracker, code-comments-audit, unlogged-branches, duplicate SPEC-TEMPLATE. Archived 4 personal drafts to .plans/archive/. .plans/ went 30 → 12 items.
- [x] **audit/ deletion:** Removed entire audit/ directory (4 obsolete doc audit reports from March 29). Updated strip_for_main.sh. Docs work already complete via fix/docs-overhaul + Batches 2-3. Root went 34 → 28 items.

## Phase 3: Multi-Agent Review

- [ ] Multi-agent review complete
- [ ] Human final approval
- [ ] Make `devonartis/agentauth` public
