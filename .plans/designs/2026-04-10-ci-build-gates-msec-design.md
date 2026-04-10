# Design: CI / Build / Gates — M-sec v1

**Created:** 2026-04-10
**Status:** DRAFT — pending user review
**Scope:** `agentauth-core` reference implementation of the M-sec CI pipeline.
**Strategic decision:** [Decision 015 — CI/Gates Strategy, Security-First, Rebrand-Resilient](../../../../Library/Mobile%20Documents/iCloud~md~obsidian/Documents/KnowledgeBase/10-Projects/AgentAuth/decisions/015-ci-gates-security-first.md) (Obsidian KB). This document is the "how"; Decision 015 is the "why."
**Related:** Decision 013 (AgentWrit rebrand), Decision 014 (no external contributions), ADR 009 (acceptance tests before merge).

---

## 1. What this document covers

Decision 015 settled the strategic layer: sequencing (CI before rebrand), scope (M-sec, not generic M), architecture (Option B — parallel per-gate jobs with a local-mirror script and a parity test), smoke strategy (L2.5 core contract on PR + L4 nightly), and companion infrastructure choices (Dependabot, contribution policy, CHANGELOG gate, pre-commit deferred).

This design covers the **implementation-level architecture**: concrete file structure, workflow responsibilities, gate-by-gate definitions, how the local `gates.sh` and CI jobs stay in sync, the L2.5 smoke script's contract, rollout sequence, and a small set of questions for the plan phase.

It stops short of committing to exact YAML, action versions, or pinned SHAs — those belong in the implementation plan (devflow Step 3).

---

## 2. Current state baseline

| Thing | State | Notes |
|---|---|---|
| `.github/` directory | **Does not exist** | Greenfield for workflows. |
| `scripts/gates.sh` | Exists | Three modes (`task`/`module`/`regression`), `run_gate`/`warn_gate`/`skip_gate` abstractions, sequential execution, gosec currently non-blocking (`warn_gate`), no govulncheck, no contamination grep, no SBOM. Dead references to `live_test.sh` and `live_test_docker.sh` (deleted in CC v4 per MEMORY.md) gracefully skipped via `[ -x ]` guard. |
| `CHANGELOG.md` | Exists | Hand-edited per devflow standing rule. |
| `.githooks/` | Does not exist | Pre-commit hooks deferred (separate cycle). |
| Go version | `go 1.24.0` / toolchain `go1.25.7` (from `go.mod`) | All workflows read via `go-version-file: go.mod`. |
| Module path | `github.com/devonartis/agentauth` (in `go.mod`) | This is the only legitimate place for a hardcoded name. Every other reference in workflows must parameterize via `${{ github.repository }}` and `${{ github.repository_owner }}`. |
| Acceptance tests | `tests/<batch>/` directories with per-story evidence files and (sometimes) `regression.sh` runners per batch. | L4 nightly wraps these. |
| Docker lifecycle | `scripts/stack_up.sh` + `scripts/stack_down.sh`, test admin secret `live-test-secret-32bytes-long-ok` per MEMORY.md | L2.5 smoke reuses both. |
| Existing branch protection | None configured yet (private repo, single maintainer). | Added as part of rollout. |

---

## 3. Target file structure

```
.github/
├── workflows/
│   ├── ci.yml                      # PR/push gates — parallel jobs incl. L2.5 smoke
│   ├── codeql.yml                  # CodeQL SAST (GitHub template, adapted)
│   ├── scorecard.yml               # OpenSSF Scorecard (GitHub template, adapted)
│   ├── nightly.yml                 # L4 full regression (schedule)
│   └── contribution-policy.yml     # Decision 014 enforcement
├── dependabot.yml                  # SHA maintenance for github-actions, gomod, docker
├── CODEOWNERS                      # New — owner of all paths (single maintainer today)
└── MAINTAINERS                     # New — allowlist consumed by contribution-policy.yml

scripts/
├── gates.sh                        # EXTENDED — add contamination, govulncheck, go mod verify, docker build, smoke, SBOM gates; flip gosec to blocking; clean up dead live_test.sh refs
├── smoke/
│   └── core-contract.sh            # NEW — L2.5 smoke script (used by gates.sh + ci.yml)
└── test-gate-parity.sh             # NEW — asserts ci.yml gate list == gates.sh gate list

.gosec.yml                           # NEW — explicit gosec config (no silent ignores)
.golangci.yml                        # NEW — security-aware golangci-lint config
```

All new files live in paths that `strip_for_main.sh` does NOT strip — this infrastructure ships to `main` and is part of the public repo.

---

## 4. Workflow files — purpose and shape

### 4.1 `ci.yml` — the main gate pipeline

**Purpose:** Run all PR-blocking gates in parallel on every PR and every push to `develop`/`main`.

**Triggers:**
```yaml
on:
  pull_request:
    branches: [develop]
  push:
    branches: [develop, main]
```

**Job shape:** one job per gate (parallel), plus a final `gates-passed` aggregator job that `needs:` all others. Branch protection requires `gates-passed` as the single required check, so adding/removing individual gates doesn't churn the branch-protection config.

**Jobs (all parallel unless noted):**

| Job ID | Purpose | Key step (abbreviated) | Blocking? |
|---|---|---|---|
| `build` | Both binaries compile | `go build ./cmd/broker ./cmd/aactl` | Yes |
| `unit-tests` | Race-enabled unit tests | `go test -race -count=1 -coverprofile=coverage.out ./...` | Yes |
| `lint` | golangci-lint with `.golangci.yml` | `golangci-lint run ./...` | Yes |
| `format` | `gofmt -l` returns empty | `test -z "$(gofmt -l .)"` | Yes |
| `vet` | `go vet` | `go vet ./...` | Yes |
| `contamination` | Zero enterprise refs in core | `! grep -ri 'hitl\|approval\|oidc\|federation\|cloud\|sidecar' internal/ cmd/` | Yes |
| `gosec` | Security static analysis | `gosec -conf .gosec.yml ./...` (blocking — changed from current `warn_gate`) | Yes |
| `govulncheck` | Known-CVE check on dependencies | `govulncheck ./...` | Yes |
| `go-mod-verify` | Module integrity + tidy check | `go mod verify && go mod tidy -diff` | Yes |
| `docker-build` | Multi-stage image builds | `docker build -t agentauth-ci:${{ github.sha }} .` | Yes |
| `smoke-l2.5` | Core contract smoke (see §5) | `./scripts/smoke/core-contract.sh` against the CI-built image | Yes |
| `dep-review` | New-dependency CVE gate (PRs only) | `actions/dependency-review-action` | Yes on PRs |
| `sbom` | SPDX SBOM generation | `anchore/sbom-action` → upload artifact | Yes (fails if generation errors) |
| `changelog` | CHANGELOG touched (see §8) | Diff check + `skip-changelog` label escape hatch | Yes on PRs |
| `gate-parity` | Local script matches workflow | `./scripts/test-gate-parity.sh` | Yes |
| `gates-passed` | Aggregator — `needs:` all above | `echo "all gates passed"` | Yes (this is the required check) |

**Concurrency:** `concurrency: { group: ci-${{ github.ref }}, cancel-in-progress: true }` so superseded pushes cancel in-flight runs.

**Coverage upload:** `codecov/codecov-action` step on the `unit-tests` job, using `coverage.out`. Informational only — not a gate, no threshold. Badge published separately.

### 4.2 `codeql.yml` — GitHub-native SAST

**Purpose:** Run CodeQL analysis on Go code, populate the Security tab, produce the CodeQL badge.

**Triggers:**
```yaml
on:
  pull_request:
    branches: [develop]
  push:
    branches: [develop, main]
  schedule:
    - cron: '31 7 * * 1'  # weekly Monday morning
```

**Based on GitHub's template** (`github/codeql-action/init@<SHA>` → `analyze@<SHA>`) with `language: go`. Minimal adaptation.

**Blocking:** Yes (required for merge).

### 4.3 `scorecard.yml` — OpenSSF Scorecard

**Purpose:** Project-level security posture scoring (branch protection, SECURITY.md, Dependabot, pinned deps, dangerous workflows, etc.). Produces the Scorecard badge.

**Triggers:**
```yaml
on:
  push:
    branches: [main]        # default branch only, per OpenSSF guidance
  schedule:
    - cron: '25 3 * * 2'    # weekly Tuesday
  branch_protection_rule:   # re-score when protection rules change
```

**Based on the OpenSSF template** (`ossf/scorecard-action@<SHA>`). Minimal adaptation.

**Blocking:** No — informational posture signal. Results upload to GitHub Security tab via `sarif` and appear in the badge once the repo flips public.

### 4.4 `nightly.yml` — L4 full regression

**Purpose:** Run the complete acceptance suite from `tests/<batch>/` directories against a Docker-deployed broker every night. Catches regressions in stories not exercised by the L2.5 PR smoke.

**Triggers:**
```yaml
on:
  schedule:
    - cron: '17 5 * * *'   # daily 05:17 UTC
  workflow_dispatch:       # manual trigger for ad-hoc runs
```

**Steps:**
1. Checkout `develop`
2. `docker compose build` via `scripts/stack_up.sh`
3. `./scripts/gates.sh regression` (existing mode — iterates `tests/*/regression.sh`)
4. On failure: use `actions/github-script` to open an issue titled `Nightly regression failed — <short-sha>` with the failing batch list
5. Always: upload evidence directories as workflow artifacts

**Blocking:** No — informational. Does not block PRs. Opens issue on failure so regressions don't go unnoticed.

### 4.5 `contribution-policy.yml` — Decision 014 enforcement

**Purpose:** Mechanically enforce "no external contributions, bug reports only" by auto-closing PRs from non-maintainers with a templated comment pointing to the issues-only policy.

**Trigger:**
```yaml
on:
  pull_request_target:
    types: [opened, reopened]
```

**Critical security note:** This workflow uses `pull_request_target` (runs in the base-branch context with write permissions, needed to close PRs). It **MUST NOT** check out the PR branch — that's the supply-chain compromise vector. The workflow reads only metadata (`github.event.pull_request.user.login`, `github.event.pull_request.number`) and takes policy actions via `gh` CLI. No `actions/checkout` step at all.

**Logic:**
1. Read PR author login.
2. Exempt: `dependabot[bot]`, `github-actions[bot]`, anyone listed in `.github/MAINTAINERS`, anyone with write access to the repo.
3. If author is not exempt:
   - Post templated comment: "Thanks for your interest in AgentAuth. Per Decision 014, we don't accept external code contributions at this time — including bug fixes. We actively welcome bug reports and feature requests via issues. For security vulnerabilities, see SECURITY.md. [Policy link]"
   - Close the PR.
4. If author is exempt: exit 0 (workflow succeeds, no action taken).

**Blocking:** Yes — a failure here means the policy wasn't enforced, which is a problem.

**Permissions:** `pull-requests: write`, `issues: write`, `contents: read`. Nothing else.

---

## 5. L2.5 smoke script — `scripts/smoke/core-contract.sh`

**Contract this script proves:** The credential broker can issue a token, verify it, revoke it, and deny an out-of-scope request.

**Inputs:**
- `BROKER_URL` (default `http://localhost:8080`)
- `ADMIN_SECRET` (default `live-test-secret-32bytes-long-ok` per MEMORY.md standing rule)

**Assumption:** The broker is already running. The smoke script does NOT start/stop the broker — that's the caller's job (`gates.sh smoke` locally uses `stack_up.sh`/`stack_down.sh`; `ci.yml smoke-l2.5` job does the same).

**Steps (each one must succeed or the script fails fast with a clear error):**

| # | Step | Success criterion |
|---|---|---|
| 1 | Admin auth — `POST /v1/admin/login` with `$ADMIN_SECRET` | 200 OK with admin JWT in response |
| 2 | Register a test app — `POST /v1/admin/apps` with a canned payload | 200 OK with `app_id` in response |
| 3 | Create a launch token for the test app — `POST /v1/admin/launch-tokens` | 200 OK with launch token |
| 4 | Exchange launch token for a task-scoped agent token — `POST /v1/app/tokens` | 200 OK with JWT, scope == requested scope |
| 5 | Decode JWT header + payload, assert `alg=EdDSA`, `kid` present, `iss` matches broker default, `exp > iat`, `scope` matches request | All assertions pass |
| 6 | Verify the token is accepted — `POST /v1/agent/verify` with the token | 200 OK |
| 7 | Revoke the token by JTI — `POST /v1/admin/revocations` | 200 OK |
| 8 | Verify the revoked token is now rejected — `POST /v1/agent/verify` | 401 / 403 with `token_revoked` error |
| 9 | Attempt to issue a token OUTSIDE the app's scope ceiling — `POST /v1/app/tokens` with scope not in the app's allowed set | 403 with `scope_violation` error |

**Determinism rules (non-negotiable):**
- No wall-clock assertions beyond "token issued at T is valid at T+1s."
- All test fixtures are fixed literals — no randomness, no time-based IDs beyond what the broker produces.
- No retries, no sleeps. If a step fails, the test fails.
- Output format: one line per step, `PASS` or `FAIL` with a reason. Final line `L2.5 SMOKE: PASS` or `L2.5 SMOKE: FAIL`.
- Exit code: 0 on full pass, 1 on any failure.

**Dependencies:** `curl`, `jq`. No Go, no Python, no Docker from inside the script. Keeps it portable and fast.

**Size target:** ≤ 200 lines of bash. If it grows larger, the contract is over-scoped — trim back.

**What L2.5 explicitly does NOT verify:**
- Audit chain integrity
- Delegation chain construction
- Renewal TTL preservation
- Rate limiting
- TLS certificate validation
- Prometheus metrics content
- Any non-happy-path except the two negative cases (revoked, out-of-scope)

Those live in the L4 nightly regression suite, not the L2.5 smoke.

---

## 6. `gates.sh` extension plan

The existing structure stays. New gates get added using the existing `run_gate` helper (blocking) or `skip_gate` helper (for optional tools).

### 6.1 Changes to existing gates

| Gate | Current | Target | Change |
|---|---|---|---|
| `build` | `go build ./...` | unchanged | none |
| `lint` | `golangci-lint` with fallback to `go vet` | `golangci-lint` with explicit `.golangci.yml` — **no fallback**, fail if tool missing | Remove fallback; tool must be present in CI and recommended locally |
| `unit tests` | `go test ./... -short -count=1` | `go test -race -count=1 -coverprofile=coverage.out ./...` (drop `-short` in full mode) | Add race + coverage |
| `security (gosec)` | `warn_gate` (non-blocking) | `run_gate` (blocking), config from `.gosec.yml` | **Flip to blocking** per M-sec rule |

### 6.2 New gates (all blocking, all `run_gate`)

| Gate | Command | Notes |
|---|---|---|
| `format` | `test -z "$(gofmt -l .)"` | Fails if any file needs gofmt |
| `vet` | `go vet ./...` | Already implicit in lint but explicit as its own gate for CI clarity |
| `contamination` | `! grep -ri 'hitl\|approval\|oidc\|federation\|cloud\|sidecar' internal/ cmd/` | Zero-tolerance per MEMORY.md standing rule |
| `govulncheck` | `govulncheck ./...` | Blocking — per Decision 015 rationale |
| `go-mod-verify` | `go mod verify && git diff --exit-code go.mod go.sum` after `go mod tidy` | Integrity + drift check |
| `docker-build` | `docker build -t agentauth-ci:local .` | Build-only, no publish |
| `smoke-l2.5` | `./scripts/smoke/core-contract.sh` (broker must be up — caller's responsibility) | Fast core-contract verification |
| `sbom` | `syft packages dir:. -o spdx-json=sbom.spdx.json` | Non-destructive — fails only on generation error |

### 6.3 Mode rework

Current modes (`task` / `module` / `regression`) map to M-sec reality as:

| Mode | What runs | Use case |
|---|---|---|
| `task` | build, vet, lint, format, contamination, unit tests (`-short`), gosec, govulncheck, go-mod-verify | Fast dev-loop gate — ~1-2 minutes locally |
| `full` (renamed from `module`) | All `task` gates + race-enabled unit tests (no `-short`) + docker-build + smoke-l2.5 (requires `stack_up.sh` first) + sbom | Full local verification — mirrors `ci.yml` |
| `regression` | Iterate `tests/*/regression.sh` (existing behavior) | Runs the L4 equivalent locally |

`module` is retained as a deprecated alias for `full` to avoid breaking muscle memory.

### 6.4 Dead reference cleanup

Remove the `live_test.sh` and `live_test_docker.sh` branches from the current `module` block — those files were deleted in CC v4 but the `[ -x ]` guard keeps the references silently alive. Replace with the new `smoke-l2.5` gate.

---

## 7. Parity test — `scripts/test-gate-parity.sh`

**Purpose:** Fail CI if `gates.sh`'s gate list and `ci.yml`'s job list disagree about which gates exist.

**Mechanism:**
1. `gates.sh --list-gates` (new flag) outputs a sorted list of gate IDs, one per line. Implementation: the script reads its own gate definitions from a single top-of-file array and prints them.
2. `ci.yml` exposes its gate list via a job matrix or a documented comment block that the parity test parses. Simpler option: `ci.yml` has a `gate-list` step in the `gates-passed` aggregator that echoes the same list.
3. The parity test diffs the two outputs and fails on any difference.

**Runs as:** a blocking job in `ci.yml` (self-hosting — the parity gate is itself a gate).

**Failure mode:** drift is usually caused by adding a gate to one side and forgetting the other. The test prints which gate is missing from which side so the fix is obvious.

**Implementation size target:** ≤ 50 lines of bash.

---

## 8. CHANGELOG gate mechanism

**Rule (from FLOW.md standing rule):** Every user-facing change must update `CHANGELOG.md` in the same commit/PR. This gate makes it mechanical.

**Implementation (PR-only gate):**
1. Compute the PR diff: `git diff --name-only ${{ github.event.pull_request.base.sha }} HEAD`
2. If `CHANGELOG.md` is in the list → PASS
3. If the PR has the `skip-changelog` label → PASS (bypass is intentional, label is audit-visible)
4. Otherwise → FAIL with a comment explaining the gate and the label

**First-failure comment:** On first failure for a given PR, post a templated comment:
> This PR doesn't touch `CHANGELOG.md`. Per project policy, user-facing changes need a CHANGELOG entry. If this PR is docs-only, test-only, or otherwise not user-facing, apply the `skip-changelog` label and re-run CI.

**Label creation:** `skip-changelog` label gets created as part of the rollout (`gh label create skip-changelog`).

**Not gated on push to develop/main** — PRs are where the discipline matters; direct pushes to develop are rare and skip-label doesn't apply there anyway.

---

## 9. Dependabot config — `.github/dependabot.yml`

**Ecosystems watched:**

| Ecosystem | Directory | Schedule | Grouping |
|---|---|---|---|
| `github-actions` | `/` | Weekly (Monday) | Grouped: all actions in one PR |
| `gomod` | `/` | Weekly (Monday) | Grouped: direct dependencies in one PR, indirect in another |
| `docker` | `/` | Weekly (Monday) | Dockerfile base images |

**Version strategy:** `auto` (Dependabot picks increase semantics based on semver).

**Open PR limit:** 3 per ecosystem (avoids PR flood).

**PR commit-message prefix:** `chore(deps):` — matches existing CHANGELOG convention.

**Allowlist:** Dependabot PRs are exempted from the contribution-policy gate (see §4.5).

**Rebase strategy:** `auto` — Dependabot keeps PRs current as develop advances.

---

## 10. Pinned SHA strategy

**Rule:** Every `uses:` in every workflow file pins to a 40-character SHA, not a tag. Tags are mutable; SHAs are not.

**Format:** `uses: actions/checkout@<40-char-sha> # v4.1.1` — the comment documents the human-readable version for review purposes.

**Discovery process for the initial pin:**
1. For each action we use, visit the action's releases page, find the latest stable release.
2. Record the full SHA of that release's tag commit.
3. Write the workflow with the SHA + comment.

**Maintenance:** Dependabot's `github-actions` ecosystem watches SHAs and opens grouped update PRs weekly. Dependabot can update both the SHA and the comment in a single PR.

**Actions we'll pin in v1 (non-exhaustive — final list in the plan):**
- `actions/checkout`
- `actions/setup-go`
- `actions/upload-artifact`
- `actions/github-script`
- `github/codeql-action/init`
- `github/codeql-action/analyze`
- `golangci/golangci-lint-action`
- `securego/gosec`
- `ossf/scorecard-action`
- `actions/dependency-review-action`
- `anchore/sbom-action`
- `codecov/codecov-action`

**Escape hatch:** `# dependabot: pin-exact` comment if any action must never auto-update (none expected in v1).

---

## 11. Secret management

**Only one secret is needed for v1 CI:** the test admin secret for the L2.5 smoke job.

**Problem:** The test admin secret is a literal constant (`live-test-secret-32bytes-long-ok`) and is already in MEMORY.md as the canonical value. It's not actually secret — it's a well-known test fixture. Putting it in GitHub Secrets creates the *impression* of secrecy without any actual property.

**Decision:** Pass it as a workflow env var, not a repo secret. Document clearly in the workflow that this is a known test fixture, not a production secret, and cross-reference MEMORY.md.

```yaml
env:
  AA_ADMIN_SECRET: live-test-secret-32bytes-long-ok  # known test fixture, see MEMORY.md
```

**Future real secrets (not in v1):**
- `CODECOV_TOKEN` — if Codecov upload fails without token on private repo, add as repo secret.
- `GHCR_TOKEN` — not needed in v1 (no publish).
- Nothing else expected until release automation (L scope, later cycle).

**Zero-secret posture until forced otherwise.** The less secret surface the CI has, the less to defend.

---

## 12. Branch protection checklist

Applied to both `develop` and `main` after initial rollout succeeds and `gates-passed` + `codeql` have run green at least once.

**Required status checks:**
- `gates-passed` (aggregator job from `ci.yml`)
- `codeql` (from `codeql.yml`)

**Do not list:** individual gate job names. Branch protection keyed on `gates-passed` means adding/removing gates doesn't require touching protection config.

**Other rules:**
- Require PRs to merge (no direct push to `develop` except for maintainers in emergencies)
- Dismiss stale approvals on new commits
- Require branches to be up to date before merging
- Do NOT require signed commits (out of scope for v1)
- Do NOT require linear history (GitFlow uses merge commits)

**Who sets this up:** Manual `gh api` calls during rollout, documented in the plan. Scripted idempotent setup is out of scope for v1 — one-time human operation with documented steps is simpler.

---

## 13. Rollout plan

Building CI that runs on `develop` while also protecting `develop` creates a chicken-and-egg problem. The rollout sequences around it:

| Phase | Action | Gate for proceeding |
|---|---|---|
| **R1** | Create `feature/ci-msec` branch off `develop`. All work happens here. | — |
| **R2** | Write `scripts/gates.sh` extensions + `scripts/smoke/core-contract.sh` + `scripts/test-gate-parity.sh` + configs (`.gosec.yml`, `.golangci.yml`, `.github/dependabot.yml`). Run `./scripts/gates.sh full` locally. | All new local gates green against a `stack_up.sh` broker |
| **R3** | Write `ci.yml` + `codeql.yml` + `scorecard.yml` + `nightly.yml` + `contribution-policy.yml` with pinned SHAs and parameterized repo refs. | Files lint-check via `actionlint` (if available) |
| **R4** | Push `feature/ci-msec` to GitHub. Workflows will run on the push. Observe actual CI run. | All jobs green on `feature/ci-msec` |
| **R5** | Fix any CI-only issues discovered in R4 (platform differences, missing tools, unexpected permissions). Iterate until CI is green. | Stable green run |
| **R6** | Open PR from `feature/ci-msec` → `develop`. CI runs on the PR itself. | PR CI green |
| **R7** | Human review, merge to `develop`. | Merged |
| **R8** | After merge, configure branch protection on `develop` with `gates-passed` + `codeql` as required checks. | Protection active |
| **R9** | Merge `develop` → `main` (via fast-forward + `strip_for_main.sh`). CI runs on `main`. | `main` CI green |
| **R10** | Configure branch protection on `main`. | Protection active |
| **R11** | Wait 7 days, confirm Dependabot opens first weekly PR as expected, confirm nightly runs successfully. | Observation period clean |
| **R12** | Close the devflow cycle — CI v1 done. Update MEMORY.md + FLOW.md. | — |

**Critical:** branch protection is applied AFTER the first green run, not before. Protecting `develop` before CI exists would block the PR that introduces CI.

**If R4-R5 iteration produces many broken commits on `feature/ci-msec`**, that's fine — the branch gets squash-merged in R6 so the history stays clean. Alternatively, rebase/interactive-squash before opening the PR.

---

## 14. Testing CI without breaking develop

The design in §13 R4 pushes `feature/ci-msec` to the remote to exercise the workflows. A few safeguards:

- **No branch protection on `feature/*` branches** — workflows run but nothing blocks force-pushes or rewrites while iterating.
- **Dependabot and contribution-policy workflows trigger on PRs to `develop`** — they won't activate just from pushing the feature branch. They first run when the PR is opened in R6.
- **Nightly workflow won't run** during rollout — its only trigger is `schedule`, so it waits for the next 05:17 UTC after merge. Optional: dispatch it manually via `workflow_dispatch` to smoke-test before R7.
- **Scorecard workflow won't run** during rollout — it triggers on push to `main` only. First run happens in R9.
- **CI workflow costs:** each CI run on the feature branch is ~5-10 minutes of runner time. Budget ~20-30 runs during iteration.

---

## 15. Out of scope (explicit deferrals)

None of the following are in M-sec v1. Listing them here so no one mistakes them for gaps:

- **Release automation** — tag-triggered release workflow, multi-arch GHCR publish, release notes generation, SLSA provenance, cosign/sigstore signing, signed releases. Deferred to a later cycle (L scope per Decision 015, aligned with Decision 010 phase 4).
- **Pre-commit hooks** — extending `.githooks/pre-commit` with gofmt/vet/contamination. Deferred to a separate smaller cycle per Decision 015.
- **Coverage threshold gating** — coverage is published informationally, not gated. Avoid performative % chasing.
- **Matrix builds** — single Go version (from `go.mod`), single OS (`ubuntu-latest`). No cross-compile matrix.
- **CI caching beyond defaults** — `actions/setup-go` provides default Go module cache; no additional cache layers in v1.
- **Fuzz test gate** — `go test -fuzz` runs exist in some packages but aren't wired into CI. Candidate for a follow-up gate cycle.
- **Performance regression gate** — no benchmark budget enforcement in v1.
- **TLS cert rotation in CI** — the smoke test uses plain HTTP; TLS is a separate gate-in-progress concern.
- **Rebrand-related file rewrites** — this cycle builds CI; the rebrand cycle comes after.

---

## 16. Open questions for the plan phase

These aren't blockers — they're decisions that are cleaner to make with implementation code in front of us than in abstract:

1. **golangci-lint exact linter list.** M-sec says "security-aware config." Candidates: `errcheck`, `gosec`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gosimple`, `bodyclose`, `misspell`, `revive`, `gocritic`, `sqlclosecheck`. Full list finalized in the plan based on what the existing codebase tolerates without noise. Starting conservative (core set) and expanding is safer than starting maximal.

2. **`.gosec.yml` suppressions.** Expect 3-10 gosec findings on first run (false positives on `crypto/rand` usage, `math/rand` in test fixtures, etc.). Each suppression needs a comment explaining the reason. Full suppression list determined in the plan.

3. **`gates-passed` aggregator reporting.** Whether to use GitHub's native job-dependency aggregation or a custom aggregator job that posts a summary comment to the PR. Leaning toward the simpler native approach.

4. **Dependabot PR author label.** Whether to add a label like `dependencies` automatically to Dependabot PRs for filtering. Default Dependabot behavior applies labels — confirming what those are and whether we need more.

5. **MAINTAINERS file format.** Flat list of GitHub usernames vs. structured YAML. Flat list is simpler for a 1-2 person maintainer team; structured format matters only at larger scale.

6. **Nightly regression issue template.** Fields in the auto-opened issue (failing batch, failing stories, link to workflow run, recent commits). Exact template finalized in the plan.

7. **Codecov token on private repo.** Whether Codecov's free tier works without a token on private repos (it does for some languages, not for others). If a token is needed, add `CODECOV_TOKEN` repo secret during rollout.

---

## 17. Success criteria

This design is successfully implemented when:

- [ ] All five workflow files exist in `.github/workflows/` with parameterized repo references (no hardcoded `devonartis/agentauth`)
- [ ] `scripts/gates.sh full` passes locally against a `stack_up.sh` broker
- [ ] `scripts/smoke/core-contract.sh` exists and verifies the 9-step contract deterministically
- [ ] `scripts/test-gate-parity.sh` exists and passes
- [ ] `.github/dependabot.yml`, `.github/MAINTAINERS`, `.github/CODEOWNERS` exist
- [ ] `.gosec.yml`, `.golangci.yml` exist with documented config
- [ ] Branch protection on `develop` requires `gates-passed` + `codeql`
- [ ] Branch protection on `main` requires `gates-passed` + `codeql`
- [ ] First Dependabot PR has been observed (within 7 days of merge)
- [ ] First nightly run has been observed (next morning after merge)
- [ ] First `pull_request_target` contribution-policy dry run succeeds (will be tested with a dummy PR from a non-maintainer account during rollout or the first real external PR)
- [ ] README has six M-sec badges (build, CodeQL, Scorecard, license, Go version, security policy) — Scorecard and CodeQL badges may read "pending" until first successful run
- [ ] Decision 015 is referenced in the merged PR description

---

## 18. Next step — write the implementation plan

After user review and approval of this design, invoke `superpowers:writing-plans` to create the implementation plan in `.plans/specs/` per devflow Step 2. The plan will break this design into ordered tasks with exact commands, file contents, and verification steps, ready for execution in Step 6.
