# TD-010 — Promote `adminTTL` Const to Configurable Field — Action Plan

**Date:** 2026-04-10
**Tech debt:** TD-010 (carried forward from agentauth-internal, see `TECH-DEBT.md`)
**Branch:** `fix/td-010-admin-token-ttl-config` off `develop`
**Decided:** Operator-tunable. Confirmed by user 2026-04-10.

---

## Goal

Replace the magic number `const adminTTL = 300` in `internal/admin/admin_svc.go` with an operator-configurable field driven by env var `AA_ADMIN_TOKEN_TTL`. The default is preserved as a **named, typed `time.Duration` constant** — not a magic number.

This is **two improvements stacked into one fix**:
1. **Style fix (always correct):** the magic number `300` (no unit, no type — you have to read the call site to know if it's seconds, minutes, or arbitrary) is replaced with `5 * time.Minute` named as a typed `const`. Self-documenting.
2. **Configurability fix (operator decision, confirmed yes):** the value is exposed as `cfg.AdminTokenTTL` so operators can tune it via env var. Use cases: longer sessions during incidents, shorter for tighter security postures.

---

## Out of scope (explicitly)

- **Migrating other TTL fields** (`DefaultTTL`, `MaxTTL`, `AppTokenTTL`) from `int` seconds to `time.Duration`. They use the existing int-seconds convention which is also bad Go style, but refactoring them is a separate cleanup item — flag as a follow-up TD if you want, but not in this PR.
- **Other constants in the codebase.** This PR is for `adminTTL` only. Other magic numbers stay where they are pending their own decisions.
- **Validation logic** (e.g., refuse `AA_ADMIN_TOKEN_TTL=0`). The existing TTL fields don't have this and it's not part of this fix. Operator responsibility — if they set 0, they get 0.

---

## Files changed

| # | File | Change |
|---|------|--------|
| 1 | `internal/cfg/cfg.go` | Add `AdminTokenTTL time.Duration` field to `Cfg` struct. Define `const defaultAdminTokenTTL = 5 * time.Minute` near the top of the file (or in a `const ()` block grouped with related defaults). Add a new helper `envDurationOr(key string, fallback time.Duration) time.Duration` if it doesn't exist (parses with `time.ParseDuration`, falls back if unset or unparseable). Update `Load()` to populate `c.AdminTokenTTL = envDurationOr("AA_ADMIN_TOKEN_TTL", defaultAdminTokenTTL)`. Add the new env var to the inline comment block at lines 10–23. |
| 2 | `internal/admin/admin_svc.go` | Delete `const adminTTL = 300`. Update `AdminSvc` struct or its constructor to accept the configured TTL (either as a `time.Duration` field on `AdminSvc`, or by passing `cfg.Cfg` if the constructor already takes it — verify the existing constructor signature first). Update the call site that uses `adminTTL` (the place that issues admin tokens) to use the new field. |
| 3 | `cmd/broker/main.go` | If `AdminSvc` constructor signature changed, update the wiring to pass `cfg.AdminTokenTTL` (or `cfg`) through. Verify no other call sites construct `AdminSvc` directly. |
| 4 | `internal/admin/admin_svc_test.go` | Update tests that exercise admin token issuance to drive the TTL from a test cfg fixture instead of relying on the deleted const. Add a focused test that constructs `AdminSvc` with a non-default TTL (e.g. `10 * time.Minute`) and verifies the issued token's `Exp - Iat == 600`. |
| 5 | `internal/cfg/cfg_test.go` | Add tests for the new field: (a) `AdminTokenTTL` defaults to `5 * time.Minute` when env unset, (b) `AA_ADMIN_TOKEN_TTL=10m` sets it to `10 * time.Minute`, (c) `AA_ADMIN_TOKEN_TTL=600s` parses correctly, (d) malformed value falls back to default (or errors — match the convention used by `envIntOr`). |
| 6 | `CHANGELOG.md` | One line under develop: `Added cfg.AdminTokenTTL / AA_ADMIN_TOKEN_TTL — admin token TTL is now operator-tunable. Default 5m, previously hardcoded as 300 seconds.` |

**Total: 6 files.**

---

## Why `time.Duration` and not `int` seconds

The existing TTL fields (`DefaultTTL`, `MaxTTL`, `AppTokenTTL`) all use `int` (seconds). Using `time.Duration` for `AdminTokenTTL` is inconsistent with that pattern.

**Choosing inconsistency on purpose, because:**
- The existing int-seconds pattern is itself bad Go style (you just told me magic numbers and unit-less values are wrong).
- `time.Duration` is self-documenting at the type level — you can't accidentally pass milliseconds where seconds were expected.
- The env var format becomes user-friendly: `AA_ADMIN_TOKEN_TTL=5m` instead of `AA_ADMIN_TOKEN_TTL=300`. Operators don't need to do mental math.
- Adding one correct field is better than continuing to add wrong ones for "consistency."
- The existing fields can be migrated to `time.Duration` in a follow-up cleanup TD if you want — flagged at the bottom of this doc.

**Trade-off accepted:** the codebase has two TTL conventions until the cleanup happens.

---

## Verification — local before push

Run in order, all must pass:

1. `go build ./cmd/broker ./cmd/aactl`
2. `go vet ./...`
3. `gofmt -l internal/ cmd/` returns empty
4. `go test ./internal/cfg/...` — new env var parsing tests pass
5. `go test ./internal/admin/...` — admin svc tests pass with new TTL plumbing
6. `go test ./...` — full suite, no regressions
7. `go test ./... -race`
8. `./scripts/gates.sh task` — fast gates locally

---

## Behavioral test — non-negotiable

Unit tests catch wiring bugs but don't prove the env var → cfg → service → issued token chain actually works end-to-end. **Run this manually before declaring the fix done:**

1. Build broker: `go build -o bin/broker ./cmd/broker`
2. Start broker with overridden TTL:
   ```
   AA_ADMIN_SECRET="$(openssl rand -base64 32)" \
   AA_ADMIN_TOKEN_TTL=10m \
   AA_ISSUER=test-broker \
   ./bin/broker
   ```
   (The `AA_ISSUER` here is to satisfy whatever TD-TOKEN-001 lands as. If TD-TOKEN-001 hasn't merged yet, omit it.)
3. From another terminal, authenticate as admin and grab the token:
   ```
   curl -s -X POST http://localhost:8080/v1/admin/auth -d '{"secret":"<the secret you set above>"}' | jq -r .access_token
   ```
4. Decode the token's payload (middle base64 segment) and verify:
   ```
   echo "<token>" | cut -d. -f2 | base64 -d | jq '. | {iat, exp, ttl: (.exp - .iat)}'
   ```
5. **Expected:** `ttl: 600` (10 minutes in seconds).
6. Stop the broker, restart with no `AA_ADMIN_TOKEN_TTL` set, repeat steps 3–4.
7. **Expected:** `ttl: 300` (5 minutes default).

**Both expectations must hold** before the fix is declared complete. If either fails, the wiring is broken and the unit tests passed for the wrong reason.

This is the test the user explicitly called out — "you are changing code but it should still be configurable so you would truly need to test it afterwards." Unit tests are not enough. The configurability has to be proven against a running broker.

---

## CI gates — must all pass on the branch

Same 13 M-sec gates + aggregators as TD-TOKEN-001:
- `format`, `build`, `vet`, `lint`, `unit-tests`, `unit-tests-race`, `gosec`, `govulncheck`, `go-mod-verify`, `contamination`, `docker-build`, `smoke-l25`, `sbom`
- `gate-parity`, `gates-passed`, `changelog`, `contribution-policy`

**PLUS a new CI job: `config-matrix`** — see next section. This is part of the TD-010 PR scope, not a follow-up.

---

## NEW: CI must verify config tunables (added 2026-04-10 per user)

**The gap this closes:** Existing CI runs the broker once with a single fixed configuration in the `smoke-l25` job. There is no CI proof that any `AA_*` env var actually flows through to runtime behavior. If a refactor silently broke the env var → cfg → service plumbing, all unit tests would still pass (they exercise the wiring at compile time), the smoke test would still pass (it doesn't override anything), and the bug would ship. The user explicitly called this out: *"the CI I need to run with different settings to ensure we can still change"*.

This is a **standing weakness across the entire codebase**, not specific to TD-010. Every existing cfg field (`AA_DEFAULT_TTL`, `AA_MAX_TTL`, `AA_BIND_ADDRESS`, `AA_LOG_LEVEL`, etc.) has the same risk today. Adding `AA_ADMIN_TOKEN_TTL` without a CI verification path would propagate the gap, not close it.

**Fix included in this PR:**

Add a new CI job called `config-matrix` (or extend `smoke-l25` with a matrix strategy — implementation detail to decide during the PR; whichever is cleaner with the existing workflow shape). The job:

1. Builds the broker binary
2. Runs the broker **twice** (sequentially or as a matrix), each run with a different `AA_ADMIN_TOKEN_TTL`:
   - **Run A:** No override. Authenticates as admin, decodes the issued token, asserts `Exp - Iat == 300` (the default).
   - **Run B:** `AA_ADMIN_TOKEN_TTL=10m`. Authenticates as admin, decodes the issued token, asserts `Exp - Iat == 600`.
3. Both runs must pass for the job to be green.

**Implementation shape:**
- New script: `scripts/smoke/config-matrix.sh` — takes the expected TTL as a parameter, boots broker with whatever env it inherits, hits `/v1/admin/auth`, decodes the token, asserts the TTL.
- New CI job in `.github/workflows/ci.yml`: `config-matrix` runs the script twice, once with defaults, once with `AA_ADMIN_TOKEN_TTL=10m`. Adds the job to the `gates-passed` aggregator's `needs:` list so it becomes a required check.
- The script is generic in its assertion shape — when future cfg fields are added (TD-TOKEN-001's `AA_ISSUER`, TD-CFG-001's `AA_TRUST_DOMAIN` neutral default, etc.), each one extends the matrix with another assertion. The pattern scales.

**Why this is in the TD-010 PR and not a separate TD:**
- TD-010 is the first new tunable we're adding under the new "no hardcoded values" discipline. It would be incoherent to add the field without proving it works at runtime.
- Splitting this into a separate "CI improvements" PR means the TD-010 PR ships unverified — exactly the failure mode the user just flagged.
- The CI job is small (~40 lines of bash + ~15 lines of YAML). Worth it as a one-time setup that pays back on every future config field.

**Ongoing rule (worth adding to project standards):** every new `cfg.Cfg` field added to the codebase MUST come with a `config-matrix` assertion that proves the env var override actually changes runtime behavior. Unit tests of cfg parsing are not sufficient — the test must exercise the full env-var → cfg → service → observable-output chain against a running broker.

This rule is being added to project memory as a feedback/standing entry so future cycles don't slip back into "unit tests are enough."

---

## Branch + merge process

1. Create branch: `git checkout develop && git pull && git checkout -b fix/td-010-admin-token-ttl-config`
2. Implement in order: cfg.go → admin_svc.go → main.go wiring → admin_svc_test.go → cfg_test.go → CHANGELOG.md. Commit per logical group.
3. Run all 8 local checks. Do not push until clean.
4. Run the **behavioral test** locally (build broker, two restarts, two TTL verifications). Do not push until both pass.
5. Push: `git push -u origin fix/td-010-admin-token-ttl-config`
6. Open PR to develop. Title: `fix(admin): promote adminTTL const to configurable cfg.AdminTokenTTL (TD-010)`. Body links to TD-010 entry and this plan.
7. CI runs all gates. `gates-passed` must be green.
8. Merge to develop (merge commit, no squash).
9. Mark TD-010 as RESOLVED 2026-04-10 in `TECH-DEBT.md`.
10. develop → main strip merge happens at the next batch.

---

## Risk and rollback

**Risk profile:** LOW.
- Default behavior preserved (5 minutes = 300 seconds = no observable change for operators who don't set the new env var)
- New code path is opt-in via env var
- No wire-protocol change, no database change, no migration

**Rollback:** revert the merge commit.

---

## Definition of done

- [ ] All 8 local checks pass
- [ ] Behavioral test passes both cases (default + override)
- [ ] All 13 CI gates pass on the branch
- [ ] `gates-passed` aggregator green
- [ ] PR merged to develop
- [ ] TD-010 marked RESOLVED in `TECH-DEBT.md`

---

## Follow-up cleanup (separate TD, not this PR)

The other TTL fields (`DefaultTTL`, `MaxTTL`, `AppTokenTTL`) use `int` seconds with magic-number defaults at `cfg.go:78,79,132`. They have the same style problem this PR is fixing for `adminTTL`. Worth a separate TD entry: **TD-CFG-003 (proposed)** — migrate all TTL fields to `time.Duration` with named const defaults, deprecate the int-seconds env var format with a transition period. Not in this PR's scope. Flag for later.
