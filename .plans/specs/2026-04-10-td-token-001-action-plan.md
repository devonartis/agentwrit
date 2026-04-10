# TD-TOKEN-001 — Remove Hardcoded JWT Issuer Literal — Action Plan

**Date:** 2026-04-10
**Tech debt:** TD-TOKEN-001 (see `TECH-DEBT.md`)
**Audit:** `.plans/reviews/2026-04-10-hardcoded-identity-audit.md`
**Branch:** `fix/td-token-001-remove-issuer-hardcode` off `develop`

---

## Goal

Remove the hardcoded `"agentauth"` literal from the JWT `iss` claim. Make the issuer driven by config. **That is the entire scope.**

The bug is the literal in source code. Removing it is the fix. Anything else (launcher UX, validation engineering, doc rebrand sweep, federation thinking) is a separate concern handled by other tech-debt items or future cycles.

---

## Out of scope (explicitly)

- **Required-field validation** — if the operator doesn't set `AA_ISSUER`, that's user error. We don't engineer fail-fast UX around it. Empty config is acceptable; broker boots with empty issuer.
- **`aactl init` changes** — separate concern. The launcher already exists and is referenced by the existing admin-secret error pattern. Extending it for `AA_ISSUER` can happen later.
- **Compose file updates / env.sh updates** — not needed if we're not requiring the field.
- **Doc rebrand sweep** — JWT example payloads in `docs/api.md`, `docs/common-tasks.md`, `docs/foundations.md`, `docs/implementation-map.md` show `"iss": "agentauth"`. Those are documentation drift, separate cleanup item.
- **TrustDomain, DBPath, Audience hardcodes** — TD-CFG-001, TD-TOKEN-002. Separate TDs, separate fixes.
- **`aactl` → `awrit` binary rename** — TD-CLI-001. Separate fix.
- **Module path rename** (`github.com/devonartis/agentauth` → ...) — out of scope until the GitHub repo is renamed.

---

## Files changed

| # | File | Change |
|---|------|--------|
| 1 | `internal/cfg/cfg.go` | Add `Issuer string` field to `Cfg` struct. Add `Issuer: os.Getenv("AA_ISSUER")` to `Load()`. No default, no validation, no required check. Empty when unset. Update the inline doc-comment env var list at lines 10–23 to include `AA_ISSUER`. |
| 2 | `internal/token/tkn_svc.go:141` | Replace `Iss: "agentauth"` with `Iss: s.cfg.Issuer`. |
| 3 | `internal/token/tkn_claims.go` | Remove the issuer check from `Validate()` at line 62. Delete the doc comments at lines 8 and 59 that say "the issuer is always 'agentauth'". `Validate()` becomes purely structural (sub, jti, exp, nbf checks only). |
| 4 | `internal/token/tkn_svc.go` (Verify, ~line 227) | After the existing `claims.Validate()` call, add an issuer check: if `s.cfg.Issuer != ""`, require `claims.Iss == s.cfg.Issuer`. If `s.cfg.Issuer == ""`, skip the check. This mirrors the existing Audience pattern (`cfg.go:22` says "empty = skip"). |
| 5 | `internal/token/tkn_svc_test.go` | Three groups of edits: (a) line ~50 where `cfg.Cfg{}` is constructed in test setup, add `Issuer: "test-issuer"`. (b) Lines 89, 208, 824 where tests assert `claims.Iss == "agentauth"`, change to assert against `"test-issuer"` (the fixture value). (c) Restore equivalents of the deleted tests mentioned in the tombstone at line 521 — add a small `TestIssClaimMatchesConfig` table-driven test that constructs cfg with various `Issuer` values and verifies issued tokens carry them, and a `TestVerifyRejectsWrongIssuer` that constructs a token with a mismatched `Iss` and verifies `Verify()` rejects it. |
| 6 | `internal/authz/val_mw_test.go` | Three claim fixtures at lines 227, 249, 341 currently use `Iss: "agentauth"`. Change to `Iss: "test-issuer"` to match the test cfg pattern. |
| 7 | `internal/deleg/deleg_svc_test.go:25` | The `cfg.Cfg{}` literal in test setup needs `Issuer: "test-issuer"` added so the deleg tests work after the change. |
| 8 | `internal/admin/admin_svc_test.go:35` | Same — add `Issuer: "test-issuer"` to the cfg literal in test setup. |
| 9 | `CHANGELOG.md` | One line under the develop section: `Added cfg.Issuer / AA_ISSUER env var — JWT iss claim is now operator-configurable. Previously hardcoded as "agentauth".` |

**Total: 9 files.** No production-code changes outside `internal/cfg/`, `internal/token/`. No test changes outside the four test files that touch claims/cfg directly. No docs, no compose, no env.sh, no scripts, no `aactl init`.

---

## Verification — local before push

Run in order, all must pass:

1. `go build ./cmd/broker ./cmd/aactl` — both binaries compile
2. `go vet ./...` — no vet warnings
3. `gofmt -l internal/ cmd/` — must return empty (no formatting drift)
4. `go test ./internal/cfg/...` — cfg tests pass after the new field
5. `go test ./internal/token/...` — token tests pass with the rewritten assertions and the restored TestIssClaimMatchesConfig / TestVerifyRejectsWrongIssuer
6. `go test ./internal/authz/...` — val_mw tests pass after fixture rewrites
7. `go test ./internal/deleg/...` — deleg tests pass after the cfg literal update
8. `go test ./internal/admin/...` — admin tests pass after the cfg literal update
9. `go test ./...` — full suite, no regressions in any package
10. `go test ./... -race` — race detector clean
11. `./scripts/gates.sh task` — local fast gates pass

If any of these fails, fix the cause and re-run from the failing step. Do not push to the branch until all 11 pass locally.

---

## CI gates — must all pass on the branch

**PLUS a new `config-matrix` job extension** — see TD-010 plan for the design. When TD-TOKEN-001 lands, the `config-matrix` job (added in TD-010 if it ships first, or added here if TD-TOKEN-001 ships first) gets a new assertion:

- **Run C:** Boot broker with `AA_ISSUER=ci-test-issuer`. Issue any token via the broker. Decode it. Assert `claims.iss == "ci-test-issuer"`.
- **Run D:** Boot broker with no override. Issue a token. Assert `claims.iss == ""` (empty-skip, since TD-TOKEN-001 chose not to require the field).

The job proves that `AA_ISSUER` actually flows through to runtime token claims, not just compiles. Whichever PR ships first creates the `scripts/smoke/config-matrix.sh` script and the workflow job; the other PR extends it with its own assertions.

All 13 M-sec CI gates plus the aggregators:

1. `format` — gofmt clean
2. `build` — both binaries build
3. `vet` — no vet warnings
4. `lint` — golangci-lint clean
5. `unit-tests` — full `go test ./...`
6. `unit-tests-race` — race detector clean
7. `gosec` — no new gosec findings
8. `govulncheck` — no new vulnerability findings
9. `go-mod-verify` — go.mod clean
10. `contamination` — no enterprise-module strings (this gate should still pass; the literal "agentauth" in source is what we're removing, not adding)
11. `docker-build` — image builds
12. `smoke-l25` — L2.5 core contract smoke test against running broker (will boot with empty `AA_ISSUER` since we're not requiring it; verify the contract still passes)
13. `sbom` — SBOM generation
14. `gate-parity` — branch protection gate-parity check
15. `gates-passed` — aggregator (required check on develop)
16. `changelog` — CHANGELOG entry present
17. `contribution-policy` — auto-pass for maintainer

If `smoke-l25` fails because of the empty issuer, that's a real signal that the empty-skip path needs adjustment — investigate before forcing through.

---

## Branch + merge process

1. **Create branch** off latest `develop`:
   `git checkout develop && git pull && git checkout -b fix/td-token-001-remove-issuer-hardcode`
2. **Implement** in the order: cfg.go → tkn_svc.go (issuance) → tkn_claims.go → tkn_svc.go (Verify) → test files → CHANGELOG. Commit after each logical group so the history is reviewable.
3. **Local validation** — run all 11 local checks above. Do not push until clean.
4. **Push** the branch: `git push -u origin fix/td-token-001-remove-issuer-hardcode`
5. **Open PR** to develop. Title: `fix(token): remove hardcoded JWT issuer literal (TD-TOKEN-001)`. Body summarizes the change, links to TECH-DEBT.md TD-TOKEN-001 entry and the audit doc.
6. **CI runs** all 13 gates plus aggregators. `gates-passed` must show success.
7. **Merge** to develop once gates are green. No squash — use a merge commit so the progression is preserved.
8. **Mark TD-TOKEN-001 as RESOLVED 2026-04-10** in `TECH-DEBT.md` in a follow-up commit on develop (or in the same PR if convenient).
9. **develop → main strip merge** happens at the next batch per existing strip-list discipline. Not part of this PR.

---

## Risk and rollback

**Risk profile:** LOW. The change is contained to two production files (`cfg.go`, `tkn_svc.go`, `tkn_claims.go`) and four test files. No external API changes, no wire-protocol break (tokens still have an `iss` claim, just driven by config), no database changes, no migrations.

**The only behavioral change** for an operator who upgrades without setting `AA_ISSUER`:
- Issued tokens will have `iss: ""` instead of `iss: "agentauth"`
- Verification will skip the issuer check (empty cfg.Issuer = skip)
- Tokens minted before the upgrade with `iss: "agentauth"` will be accepted by the new broker (because the check is skipped when cfg.Issuer is empty)
- Tokens minted after the upgrade will pass verification on any broker that also has empty `cfg.Issuer`

This is graceful degradation — the upgrade does nothing observable to operators who don't change anything. Operators who DO set `AA_ISSUER` get the configured value enforced.

**Rollback:** revert the merge commit on develop. The branch can also be re-opened and amended if a finding emerges in code review.

---

## Definition of done

- [ ] All 11 local checks pass
- [ ] All 13 CI gates pass on the branch
- [ ] `gates-passed` aggregator success on the branch
- [ ] PR merged to develop
- [ ] TD-TOKEN-001 marked RESOLVED in `TECH-DEBT.md`
- [ ] No follow-up TDs created (the scope is bounded; nothing should leak)

---

## What this plan does NOT promise

This plan removes the hardcoded literal. It does not:
- Make the broker fail loudly on missing `AA_ISSUER` (out of scope, user error)
- Update the launcher to prompt for the new field (separate concern)
- Update example JWT payloads in user-facing docs (separate cleanup)
- Solve the TrustDomain, DBPath, or Audience hardcodes (separate TDs)
- Rebrand anything

If those things matter, they're tracked separately. This plan is one item, one fix, one PR.
