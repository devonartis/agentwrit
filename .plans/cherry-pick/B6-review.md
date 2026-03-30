# B6 Review -- SEC-A1 + Gates

**Date:** 2026-03-30
**Reviewer:** Code Review Agent (Opus 4.6)
**Branch:** `fix/sec-a1`
**Base:** `develop`

---

## Summary

B6 is clean. The two cherry-picked commits land exactly the right changes with no contamination. The uncommitted additions (regression tests, code comments, doc updates, tech debt tracker, acceptance tests) are thorough and well-executed. One important issue found in the unit test, and a few suggestions below.

---

## 1. Cherry-Pick Fidelity

### Commit 1: `9422e7c` -- TTL carry-forward on renewal

**Status: CORRECT**

The diff in `internal/token/tkn_svc.go` adds exactly what was planned:
- Computes `originalTTL := int(claims.Exp - claims.Iat)` before calling `Issue`
- Guards against `<= 0` with fallback to `s.cfg.DefaultTTL`
- Passes `TTL: originalTTL` in the `IssueReq`
- MaxTTL clamp in `Issue()` still applies (line 131-133)

No extraneous fields (AppID, AppName, OriginalPrincipal) leaked in. The conflict resolution noted in the CHANGELOG was handled correctly.

### Commit 2: `e395a15` -- gates.sh regression subcommand

**Status: CORRECT**

- Usage string updated: `{task|module|regression}`
- Mode validation updated to accept `regression`
- New `regression` block discovers `tests/*/regression.sh` scripts, runs them, reports pass/fail
- Summary section correctly exits before reaching the task/module summary block
- No contamination

---

## 2. Contamination Check

**CLEAN** -- `grep -ri "hitl\|approval\|oidc\|federation\|cloud\|sidecar" internal/ cmd/ --include="*.go"` returns nothing. No enterprise add-on references in Go source.

---

## 3. Regression Unit Test Review

File: `internal/token/tkn_svc_test.go` (uncommitted)

**All 5 subtests pass.** Table-driven with `t.Run`, follows project conventions.

### Important Issue

**I-1: Unused outer `svc` variable in `TestRenew_PreservesTTL`** (lines 730-733 and 824)

The test creates an outer `svc := NewTknSvc(priv, pub, c)` at the top of the function, but every subtest creates its own `svc` inside the `t.Run` block. The outer `svc` is never used for any assertion. Line 824 has `_ = svc // silence unused` to suppress the compiler error. This is a code smell -- the outer setup serves no purpose. Remove lines 730-733 (the outer `c` and `svc`) and line 824 (`_ = svc`).

**Severity:** Important (should fix) -- dead code in tests obscures intent and sets a bad example.

### Suggestion

**S-1: Add a "corrupted claims -- negative TTL" subtest**

The guard `if originalTTL <= 0` covers a defensive case, but no test exercises it directly. Consider adding a subtest that manually constructs a token with `Exp < Iat` (e.g., via direct signing) and verifies that renewal falls back to `DefaultTTL` rather than passing a negative TTL to `Issue`. This would fully cover the guard. That said, constructing such a token requires bypassing `Issue()` (which always sets `Exp > Iat`), so this may not be worth the complexity.

**Severity:** Suggestion (nice to have)

---

## 4. Code Comments Review

File: `internal/token/tkn_svc.go` (uncommitted changes)

**Status: EXCELLENT**

The updated comments follow the new standard from `.claude/rules/golang.md`:

- **TknSvc**: Lists all callers by role (AdminSvc, AppSvc, IdentitySvc, RenewHdl, ValMw/ValHdl), explains the trust boundary ("does NOT enforce who can call it -- that's the handler/middleware layer's job").
- **IssueReq**: Explains Sub semantics per role, Scope examples by role, TTL fallback/clamp behavior.
- **Issue**: Documents the three calling roles with their scope semantics, the authorization boundary, and MaxTTL ceiling.
- **Verify**: Explains it's role-agnostic, lists callers, describes apps using ValHdl for agent token validation.
- **Renew**: Documents SEC-A1 TTL carry-forward rationale, predecessor revocation ordering, failure semantics.

These comments answer "who calls this, why, and what are the boundaries" without restating the code. This is exactly the standard the project rules describe.

---

## 5. Documentation Updates Review

All four doc files accurately describe the new behavior:

| File | Change | Accurate? |
|------|--------|-----------|
| `docs/api.md` | Renew endpoint description now mentions TTL preservation + MaxTTL clamp | Yes |
| `docs/concepts.md` | Token renewal bullet adds "and original TTL (clamped by MaxTTL)" | Yes |
| `docs/implementation-map.md` | Renew flow now shows step 3 (compute originalTTL) and step 4 (Issue with TTL) | Yes |
| `docs/scenarios.md` | Comment added to renewal example: "New token preserves the original 300s TTL (clamped by MaxTTL)" | Yes |
| `docs/api/openapi.yaml` | Renew operation description updated with TTL preservation | Yes |

No inaccuracies found. The implementation-map.md change is particularly good -- it shows the exact computation and the MaxTTL clamp relationship.

---

## 6. Acceptance Tests Review

4 stories in `tests/sec-a1/evidence/`:

| Story | Type | Persona | Verdict | Assessment |
|-------|------|---------|---------|------------|
| A1-S1 | ACCEPTANCE | Security Reviewer | PASS | Tests admin flow TTL carry-forward. Banner is executive-readable. |
| A1-S2 | ACCEPTANCE | Security Reviewer | PASS | Tests production app flow. Good -- explicitly notes S1 uses admin shortcut. |
| A1-S3 | ACCEPTANCE | Security Reviewer | PASS | Scope boundary test (app cannot use admin endpoint). |
| A1-R1 | ACCEPTANCE | App | PASS | Full lifecycle regression. |

### What's Good

- S2 explicitly tests the production flow (app creates agent), not just the admin shortcut
- S3 is a valuable boundary test that validates the scope model
- All banners follow Who/What/Why/How/Expected format with plain language
- Verdicts cite specific evidence (expires_in values, HTTP status codes, JTI changes)
- R1 validates the renewed token via `/v1/token/validate` -- proves the token is actually usable

### Important Issue

**I-2: Story S3 appears in evidence but not in `user-stories.md`**

The `tests/sec-a1/user-stories.md` file lists S1, S2, and R1 but does not list S3 (app cannot use admin endpoint). The evidence file `story-S3-app-cannot-use-admin-endpoint.md` exists and is well-written. The story index should be updated to include S3.

**Severity:** Important (should fix) -- story index must match evidence files.

### Suggestion

**S-2: No Container mode evidence**

The user-stories.md specifies `Mode: VPS, Container` for S1 and R1, but all evidence is VPS-only. This is likely acceptable for B6 (TTL carry-forward is a pure code change with no Docker-specific behavior), but the story specs should either be updated to say VPS-only or container evidence should be added for completeness.

**Severity:** Suggestion (nice to have) -- the TTL logic has no container-specific path.

---

## 7. Supporting Changes Review

### TECH-DEBT.md (moved to root)

Good decision. Root-level visibility is correct for a project-wide tracker. The new entries (TD-011 through TD-014) are well-written with clear severity, file references, and actionable detail. TD-012 (missing role model doc) and TD-013 (admin creating agents without ceiling) are genuine findings that surfaced during this batch -- good that they were documented rather than ignored.

### CHANGELOG.md

Accurately describes what B6 added. Cherry-pick details section is helpful for future contributors. Tech debt section correctly references the new findings.

### MEMORY.md

Correctly points to `TECH-DEBT.md` instead of inline tech debt table. New standing rules about code comments and role model are appropriate.

### .claude/skills/cherrypick-devflow/SKILL.md

Adds Step 3 (Regression Unit Tests) and code comments requirements. The additions are well-motivated by the B6 experience. Step numbering is correctly shifted (old 3-7 becomes 4-8).

### .plans/cherry-pick/TESTING.md

B6 gates G1-G6 all marked PASS. Consistent with verified build and test results.

---

## 8. Issues Summary

### Important (should fix)

| ID | Description | File | Action |
|----|-------------|------|--------|
| I-1 | Unused outer `svc` variable with `_ = svc` suppression | `internal/token/tkn_svc_test.go` | Remove outer `c`/`svc` setup (lines ~730-733) and `_ = svc` (line ~824) |
| I-2 | Story S3 missing from `user-stories.md` index | `tests/sec-a1/user-stories.md` | Add S3 entry |

### Suggestions (nice to have)

| ID | Description | File | Action |
|----|-------------|------|--------|
| S-1 | No test for negative originalTTL guard | `internal/token/tkn_svc_test.go` | Add subtest with crafted `Exp < Iat` token |
| S-2 | Container mode evidence missing for S1/R1 | `tests/sec-a1/evidence/` | Update story specs to VPS-only or add container evidence |

---

## 9. Verdict

**B6 is ready to commit and merge** after fixing I-1 and I-2 (both are quick fixes). The cherry-picked code is correct, the contamination check is clean, the regression tests are comprehensive, the code comments are exemplary, and the acceptance tests cover both the security fix and the production flow. The tech debt documentation (TD-011 through TD-014) shows good engineering discipline -- surfacing real issues rather than hiding them.

The gates.sh regression subcommand is clean and additive. No concerns.
