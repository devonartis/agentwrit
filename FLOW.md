# FLOW.md — agentauth-core

Running decision log. Append after each meaningful action.

---

## 2026-03-29 — Repo Setup & Migration Planning

### Decision: Fork point is `2c5194e` (TD-006), not `3f9639f` (Phase 1C-alpha)

Phase 1C-alpha bakes `hitl_scopes` into the app data model (store, service, handler, CLI — 4 source files, 3 tests). TD-006 has identical core + app functionality with zero HITL references. Verified with `grep -ri "hitl|approval" internal/ cmd/` returning nothing at `2c5194e`.

### Decision: Clone from agentauth-internal, not copy

The original agentauth repo was a file copy that lost all 412 commits of history. This time we did `git clone agentauth-internal agentauth-core` to preserve the full incremental commit history.

### Decision: Open-core model with two archive repos

- Core (this repo) → will become open-source
- Add-ons (future separate repo) → HITL, OIDC, cloud, federation → stays private
- agentauth-internal → archive, reference for incremental history
- agentauth → archive, reference for cherry-pick commits + enterprise add-ons

### Action: Cloned repo, reset to fork point

```
git clone agentauth-internal agentauth-core
cd agentauth-core
git checkout -B main 2c5194e
```

### Current Step: Cleanup + Cherry-Pick Migration

**Next actions (in order):**
1. Run cleanup — delete 168 internal-only files (Step 2 in Cherry-Pick Guide)
2. Add legacy remote — `git remote add legacy /Users/divineartis/proj/agentauth`
3. Cherry-pick Batch 1 (P0) through Batch 6 (SEC-A1 + Gates)
4. Update Go module path
5. Final verification

**Guides:**
- Cherry-Pick Guide: `agentauth/.plans/modularization/Cherry-Pick-Guide.md`
- Feature Inventory: `agentauth/.plans/modularization/Cowork-Feature-Inventory.md`
- Repo Map: `agentauth/.plans/modularization/Repo-Directory-Map.md`
- Skill: `cherrypick-devflow`
