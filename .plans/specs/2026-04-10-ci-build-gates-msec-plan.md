# M-sec CI / Build / Gates v1 — Implementation Plan

> **For agentic workers:** This plan implements the M-sec CI pipeline on `feature/ci-msec`. Work task-by-task, committing after each task unless a task says otherwise. Many tasks are "create file" + "run verification" + "commit" — lean but explicit.

**Goal:** Build a security-product-grade CI/build/gates pipeline for `agentauth-core` that runs on every PR and push to `develop`/`main`, with a local `gates.sh` mirror kept in sync via a parity test, ready to act as the safety net for the future AgentWrit rebrand PR.

**Architecture:** Parallel per-gate GitHub Actions jobs (Option B from Decision 015) in five workflow files. Local `scripts/gates.sh` mirrors the same gate set with a parity-test job enforcing drift-free alignment. L2.5 core contract smoke (issue/verify/revoke/deny) on every PR; L4 full regression nightly (informational). Pinned-SHA supply chain discipline with Dependabot maintenance.

**Tech Stack:** GitHub Actions, Go 1.24+ (from `go.mod`), `golangci-lint`, `gosec`, `govulncheck`, `syft` (SBOM), CodeQL, OpenSSF Scorecard, Dependabot, Docker, bash, `jq`, `curl`.

**Source of truth for the WHY:** Obsidian KB Decision 015 — "CI/Gates Strategy — Security-First, Rebrand-Resilient" (`10-Projects/AgentAuth/decisions/015-ci-gates-security-first.md`). Don't re-debate strategy in this plan — if a choice seems odd, check Decision 015 first.

**Source of truth for the HOW at architecture level:** `.plans/designs/2026-04-10-ci-build-gates-msec-design.md`. This plan is the task-level expansion of that design.

**Important notes before starting:**
1. **Pinned SHAs use placeholder tags in this plan.** The plan writes workflows with tag references like `actions/checkout@v4`. Task 22 pins every action to its 40-char SHA with a `# v4.x.y` comment before first push. Don't skip Task 22 — unpinned actions are a supply chain vulnerability for a security product.
2. **Don't push the feature branch until Task 22 is complete.** Pinning SHAs before the first push keeps the commit history clean and means Dependabot's first rotation works against the intended baseline.
3. **The L4 nightly and Scorecard workflows won't trigger during rollout** — they're `schedule`-based. That's fine; they'll fire after merge.
4. **Branch protection is applied AFTER the first green CI run**, not before. Protecting develop before CI exists would block the PR that introduces CI.

---

## File Structure

Files this plan creates or modifies:

```
Created:
  .gosec.yml                                       (gosec config)
  .golangci.yml                                    (golangci-lint config)
  .github/dependabot.yml                           (Dependabot config)
  .github/CODEOWNERS                               (ownership)
  .github/MAINTAINERS                              (contribution-policy allowlist)
  .github/workflows/ci.yml                         (main gate pipeline)
  .github/workflows/codeql.yml                     (CodeQL SAST)
  .github/workflows/scorecard.yml                  (OpenSSF Scorecard)
  .github/workflows/nightly.yml                    (L4 full regression)
  .github/workflows/contribution-policy.yml        (Decision 014 enforcement)
  scripts/smoke/core-contract.sh                   (L2.5 smoke script)
  scripts/test-gate-parity.sh                      (parity enforcement)

Modified:
  scripts/gates.sh                                 (extend with new gates, flip gosec to blocking, clean dead refs)
  CHANGELOG.md                                     (add Unreleased entry — each task's commit)
  README.md                                        (add six badge lines — final task)
```

---

## Phase A — Local infrastructure (Tasks 1–11)

Phase A runs locally on `feature/ci-msec` without any GitHub Actions interaction. Goal: `./scripts/gates.sh full` passes against a `stack_up.sh` broker.

---

### Task 1: Cut `feature/ci-msec` branch

**Files:** n/a — branch operation

- [ ] **Step 1: Verify clean working tree on develop**

```bash
git status --short
git rev-parse --abbrev-ref HEAD
```

Expected: no output from `git status --short`, and branch is `develop`.

- [ ] **Step 2: Pull latest develop**

```bash
git fetch origin develop
git pull --ff-only origin develop
```

Expected: up-to-date.

- [ ] **Step 3: Cut the feature branch**

```bash
git checkout -b feature/ci-msec
git rev-parse --abbrev-ref HEAD
```

Expected: `feature/ci-msec`.

- [ ] **Step 4: Verify the branch is based on the latest develop**

```bash
git log --oneline -3
```

Expected: top commit matches current `develop` HEAD.

**No commit yet** — branch cut is a git state change, not a commit.

---

### Task 2: Create `.gosec.yml` config

**Files:**
- Create: `.gosec.yml`

- [ ] **Step 1: Create the config file**

```yaml
# .gosec.yml — gosec configuration for agentauth-core
#
# Policy: no silent suppressions. Every entry here has a comment
# explaining WHY it's suppressed. Reviewers audit this file.
#
# Severity: fail on HIGH and MEDIUM findings. LOW is advisory.

global:
  nosec: false         # require explicit suppressions via config, not //nosec
  audit: false
  exclude-dir:
    - vendor           # third-party code is scanned separately
    - tests            # test fixtures may intentionally use weak crypto

# Run all rules by default. Uncomment to disable specific rules with reason.
# rules:
#   - G101  # Hardcoded credentials — enabled (catches accidental token leakage)
#   - G104  # Unhandled errors — enabled (matches linter rule)
#   - G404  # Weak random — enabled (crypto/rand required for token IDs)

# Severity levels to report: high, medium, low
severity: medium
confidence: medium
```

- [ ] **Step 2: Install gosec locally if missing**

```bash
if ! command -v gosec &>/dev/null; then
  go install github.com/securego/gosec/v2/cmd/gosec@latest
fi
gosec -version
```

Expected: gosec version printed.

- [ ] **Step 3: Run gosec with the new config against the repo**

```bash
gosec -conf .gosec.yml ./... 2>&1 | tail -20
```

Expected: findings report (likely some MEDIUM/HIGH findings — these are pre-existing and get addressed in Task 3.5 or suppressed in this file with justification).

- [ ] **Step 4: Document any findings**

Read through the gosec output. For each HIGH finding:
- If it's a real bug, file it as tech debt in `TECH-DEBT.md` and fix before merge
- If it's a false positive, add to `.gosec.yml` with a `# reason: ...` comment

For each MEDIUM finding:
- Same triage

**Do not ignore findings.** Document every decision.

- [ ] **Step 5: Commit**

```bash
git add .gosec.yml
git commit -m "chore(gates): add gosec config for M-sec pipeline"
```

---

### Task 3: Create `.golangci.yml` config

**Files:**
- Create: `.golangci.yml`

- [ ] **Step 1: Create the config file**

```yaml
# .golangci.yml — golangci-lint configuration for agentauth-core
#
# M-sec linter set: security-aware defaults plus the core Go linters.
# Conservative starting set — expand after first clean run.

run:
  timeout: 5m
  tests: true
  modules-download-mode: readonly

linters:
  disable-all: true
  enable:
    - errcheck         # unchecked errors
    - gosec            # security — also gated separately
    - govet            # go vet
    - ineffassign      # unused assignments
    - staticcheck      # static analysis
    - unused           # unused code
    - gosimple         # code simplification
    - bodyclose        # HTTP body close (critical for broker HTTP client)
    - misspell         # typos in comments/strings
    - gofmt            # formatting (also gated separately)
    - goimports        # import ordering

linters-settings:
  errcheck:
    check-type-assertions: true
    check-blank: true
  gosec:
    config-file: .gosec.yml
  govet:
    enable-all: true
  misspell:
    locale: US

issues:
  exclude-dirs:
    - vendor
  exclude-rules:
    # Test files allowed to use weak random and panic for fixtures
    - path: _test\.go
      linters:
        - gosec
```

- [ ] **Step 2: Install golangci-lint if missing**

```bash
if ! command -v golangci-lint &>/dev/null; then
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi
golangci-lint version
```

Expected: golangci-lint version printed.

- [ ] **Step 3: Run lint with the new config**

```bash
golangci-lint run ./... 2>&1 | tail -30
```

Expected: some findings likely on first run. Triage:
- Fix real issues in place
- Add targeted `//nolint:linter_name // reason: ...` comments ONLY with justification
- Add broad exclusions to `.golangci.yml` ONLY if a whole linter is producing noise you plan to address later

- [ ] **Step 4: Iterate until clean or documented**

Re-run until `golangci-lint run ./...` exits 0. This may take several iterations on first pass.

- [ ] **Step 5: Commit**

```bash
git add .golangci.yml $(git diff --name-only)  # include any in-place fixes
git commit -m "chore(gates): add golangci-lint M-sec config"
```

---

### Task 4: Install `govulncheck` locally and baseline

**Files:** none (tool install + baseline check)

- [ ] **Step 1: Install govulncheck**

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck -version
```

Expected: govulncheck version printed.

- [ ] **Step 2: Run govulncheck against the module**

```bash
govulncheck ./... 2>&1 | tee /tmp/govulncheck-baseline.txt
```

Expected: either "No vulnerabilities found" OR a list of vulnerabilities.

- [ ] **Step 3: Triage vulnerabilities**

If vulnerabilities are reported:
- For each: update the dependency (`go get <pkg>@latest && go mod tidy`) or document in `TECH-DEBT.md` with severity and rationale
- Re-run govulncheck until clean or every remaining vuln has a tech-debt entry

**CI will block on govulncheck — this baseline must be clean before Phase B.**

- [ ] **Step 4: Commit dependency updates (if any)**

```bash
git add go.mod go.sum
git commit -m "chore(deps): update dependencies for govulncheck baseline"
```

Skip commit if no changes.

---

### Task 5: Extend `scripts/gates.sh` — add new blocking gates

**Files:**
- Modify: `scripts/gates.sh`

The current `gates.sh` has modes `task`/`module`/`regression` and uses `run_gate`/`warn_gate`/`skip_gate`. We preserve that structure and add new gates.

- [ ] **Step 1: Read current `scripts/gates.sh`**

```bash
cat scripts/gates.sh
```

Understand the existing structure before modifying.

- [ ] **Step 2: Rewrite `scripts/gates.sh` with M-sec extensions**

Replace the entire file with this content:

```bash
#!/usr/bin/env bash
set -euo pipefail

# gates.sh — quality gate runner for AgentAuth (M-sec)
#
# Usage:
#   ./scripts/gates.sh task        Fast dev-loop gates (build/vet/lint/format/contamination/short tests/security)
#   ./scripts/gates.sh full        Full CI-mirror gates (task + race tests + docker-build + smoke-l2.5 + sbom)
#   ./scripts/gates.sh regression  L4 full regression — iterate tests/*/regression.sh
#   ./scripts/gates.sh --list-gates  Print gate IDs (one per line) for parity test
#
# 'module' is retained as a deprecated alias for 'full'.
#
# Local/CI parity: this script's gate IDs must match ci.yml's job matrix.
# scripts/test-gate-parity.sh enforces this.

MODE="${1:-}"

# Authoritative gate list — single source of truth.
# scripts/test-gate-parity.sh reads this array; ci.yml's gate-list step echoes
# the same strings. If you add/remove/rename a gate, update BOTH.
GATES_TASK=(
  build
  vet
  lint
  format
  contamination
  unit-tests
  gosec
  govulncheck
  go-mod-verify
)
GATES_FULL=(
  "${GATES_TASK[@]}"
  unit-tests-race
  docker-build
  smoke-l2.5
  sbom
)

if [[ "$MODE" == "--list-gates" ]]; then
  for g in "${GATES_FULL[@]}"; do echo "$g"; done
  exit 0
fi

if [[ -z "$MODE" ]]; then
  echo "Usage: $0 {task|full|regression|--list-gates}"
  exit 1
fi

# Alias: module -> full (deprecated)
if [[ "$MODE" == "module" ]]; then
  echo "NOTE: 'module' is deprecated, use 'full'." >&2
  MODE="full"
fi

if [[ "$MODE" != "task" && "$MODE" != "full" && "$MODE" != "regression" ]]; then
  echo "Error: unknown mode '$MODE'. Use 'task', 'full', 'regression', or '--list-gates'."
  exit 1
fi

PASS=0
FAIL=0
SKIP=0

run_gate() {
  local name="$1"
  shift
  echo ""
  echo "=== GATE: $name ==="
  if "$@"; then
    echo "--- PASS: $name ---"
    PASS=$((PASS + 1))
  else
    echo "--- FAIL: $name ---"
    FAIL=$((FAIL + 1))
  fi
}

skip_gate() {
  local name="$1"
  local reason="$2"
  echo ""
  echo "=== GATE: $name ==="
  echo "--- SKIP: $reason ---"
  SKIP=$((SKIP + 1))
}

# --- TASK gates ---

run_gate "build" go build ./cmd/broker ./cmd/aactl

run_gate "vet" go vet ./...

# Lint: require golangci-lint (no fallback — M-sec policy)
if command -v golangci-lint &>/dev/null; then
  run_gate "lint" golangci-lint run ./...
else
  echo "ERROR: golangci-lint not installed. Install via: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
  exit 1
fi

# Format: gofmt -l must return empty
run_gate "format" bash -c 'test -z "$(gofmt -l .)"'

# Contamination: zero enterprise refs in core
run_gate "contamination" bash -c "! grep -ri 'hitl\|approval\|oidc\|federation\|cloud\|sidecar' internal/ cmd/ 2>/dev/null"

run_gate "unit-tests" go test -short -count=1 ./...

# Security: gosec (BLOCKING — flipped from warn per Decision 015)
if command -v gosec &>/dev/null; then
  run_gate "gosec" gosec -quiet -conf .gosec.yml ./...
else
  echo "ERROR: gosec not installed. Install via: go install github.com/securego/gosec/v2/cmd/gosec@latest"
  exit 1
fi

# Vulnerability check: govulncheck (BLOCKING)
if command -v govulncheck &>/dev/null; then
  run_gate "govulncheck" govulncheck ./...
else
  echo "ERROR: govulncheck not installed. Install via: go install golang.org/x/vuln/cmd/govulncheck@latest"
  exit 1
fi

# Module integrity + tidy drift
run_gate "go-mod-verify" bash -c 'go mod verify && go mod tidy && git diff --exit-code go.mod go.sum'

# --- FULL gates (only if mode is full) ---

if [[ "$MODE" == "full" ]]; then
  run_gate "unit-tests-race" go test -race -count=1 -coverprofile=coverage.out ./...

  # Docker build: multi-stage image builds cleanly
  if docker info >/dev/null 2>&1; then
    run_gate "docker-build" docker build -t agentauth-ci:local .
  else
    skip_gate "docker-build" "Docker daemon not running"
  fi

  # L2.5 smoke: core contract (issue/verify/revoke/deny)
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  if [[ -x "$SCRIPT_DIR/smoke/core-contract.sh" ]]; then
    if docker info >/dev/null 2>&1; then
      # Caller must ensure broker is running via stack_up.sh
      if curl -sf http://localhost:8080/v1/health >/dev/null 2>&1; then
        run_gate "smoke-l2.5" "$SCRIPT_DIR/smoke/core-contract.sh"
      else
        skip_gate "smoke-l2.5" "broker not reachable at localhost:8080 — run scripts/stack_up.sh first"
      fi
    else
      skip_gate "smoke-l2.5" "Docker daemon not running"
    fi
  else
    skip_gate "smoke-l2.5" "scripts/smoke/core-contract.sh not found or not executable"
  fi

  # SBOM: syft SPDX output
  if command -v syft &>/dev/null; then
    run_gate "sbom" syft packages dir:. -o spdx-json=sbom.spdx.json --quiet
  else
    skip_gate "sbom" "syft not installed — install: brew install syft or https://github.com/anchore/syft"
  fi
fi

# --- REGRESSION gates (only if mode is regression) ---

if [[ "$MODE" == "regression" ]]; then
  echo ""
  echo "=== REGRESSION: Running all previous phase tests ==="
  reg_pass=0
  reg_fail=0
  for test_dir in tests/*/; do
    phase=$(basename "$test_dir")
    runner=""
    if [ -f "$test_dir/regression.sh" ]; then
      runner="$test_dir/regression.sh"
    else
      echo "  SKIP $phase (no regression.sh runner)"
      continue
    fi
    echo "  RUN  $phase ($runner)"
    if bash "$runner"; then
      echo "  PASS $phase"
      reg_pass=$((reg_pass + 1))
    else
      echo "  FAIL $phase"
      reg_fail=$((reg_fail + 1))
    fi
  done
  echo ""
  echo "=== REGRESSION SUMMARY: $reg_pass passed, $reg_fail failed ==="
  if [[ $reg_fail -gt 0 ]]; then
    echo "RESULT: FAILED"
    exit 1
  else
    echo "RESULT: PASSED"
    exit 0
  fi
fi

# --- Summary ---

echo ""
echo "==============================="
echo "  GATE SUMMARY ($MODE mode)"
echo "==============================="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
echo "  SKIP: $SKIP"
echo "==============================="

if [[ $FAIL -gt 0 ]]; then
  echo "RESULT: FAILED"
  exit 1
else
  echo "RESULT: PASSED"
  exit 0
fi
```

- [ ] **Step 3: Make sure it's still executable**

```bash
chmod +x scripts/gates.sh
ls -l scripts/gates.sh
```

Expected: `-rwxr-xr-x` permissions visible.

- [ ] **Step 4: Run `./scripts/gates.sh --list-gates`**

```bash
./scripts/gates.sh --list-gates
```

Expected output (one per line):
```
build
vet
lint
format
contamination
unit-tests
gosec
govulncheck
go-mod-verify
unit-tests-race
docker-build
smoke-l2.5
sbom
```

- [ ] **Step 5: Run `./scripts/gates.sh task` and verify it passes**

```bash
./scripts/gates.sh task
```

Expected: all task-mode gates pass. If any fail, fix them before moving on. The smoke and docker-build gates won't run in task mode, so this test focuses on the fast gates only.

- [ ] **Step 6: Commit**

```bash
git add scripts/gates.sh
git commit -m "chore(gates): extend gates.sh with M-sec gate set

- Add contamination grep, govulncheck, go-mod-verify as blocking gates
- Flip gosec from warn_gate to run_gate (blocking per Decision 015)
- Add docker-build, smoke-l2.5, sbom in 'full' mode
- Add --list-gates flag for parity test
- Rename 'module' to 'full' (retain as deprecated alias)
- Remove dead references to live_test.sh / live_test_docker.sh
- Require golangci-lint and gosec (no fallback — M-sec policy)"
```

---

### Task 6: Create `scripts/smoke/core-contract.sh` — L2.5 smoke script

**Files:**
- Create: `scripts/smoke/core-contract.sh`

This is the L2.5 core contract smoke — it verifies issue + verify + revoke + deny-out-of-scope in ≤ 200 lines of bash against a running broker.

- [ ] **Step 1: Create the smoke directory**

```bash
mkdir -p scripts/smoke
```

- [ ] **Step 2: Create `scripts/smoke/core-contract.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

# core-contract.sh — L2.5 smoke test for agentauth-core
#
# Verifies the credential broker's core contract:
#   1. Admin can authenticate
#   2. Admin can register an app
#   3. App can receive a launch token
#   4. App can exchange launch token for an agent token
#   5. Agent token has correct JWT structure (EdDSA, kid, iss, scope)
#   6. Agent token is accepted by /v1/agent/verify
#   7. Admin can revoke the token
#   8. Revoked token is rejected
#   9. Out-of-scope token request is denied
#
# Caller's responsibility: start the broker before calling this script.
# This script does NOT start/stop the broker.
#
# Determinism: fixed fixtures, no random values, no sleeps, no retries.

BROKER_URL="${BROKER_URL:-http://localhost:8080}"
ADMIN_SECRET="${AA_ADMIN_SECRET:-live-test-secret-32bytes-long-ok}"

# Dependencies
for dep in curl jq; do
  if ! command -v $dep &>/dev/null; then
    echo "FAIL: missing dependency: $dep"
    exit 1
  fi
done

# Fixtures (fixed — no randomness)
APP_NAME="smoke-test-app"
APP_SCOPE_CEILING='["tasks:read","tasks:write"]'
REQUESTED_SCOPE='["tasks:read"]'
OUT_OF_SCOPE='["admin:all"]'

step=0
pass() { step=$((step+1)); echo "  [$step] PASS: $1"; }
fail() { step=$((step+1)); echo "  [$step] FAIL: $1 — $2"; echo "L2.5 SMOKE: FAIL"; exit 1; }

echo "=== L2.5 Core Contract Smoke ==="
echo "Broker: $BROKER_URL"

# --- Step 1: Admin auth ---
ADMIN_TOKEN=$(curl -sf -X POST "$BROKER_URL/v1/admin/login" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$ADMIN_SECRET\"}" \
  | jq -r '.token // empty')
if [[ -z "$ADMIN_TOKEN" ]]; then
  fail "admin login" "no token in response"
fi
pass "admin login (got admin JWT)"

# --- Step 2: Register app ---
APP_RESPONSE=$(curl -sf -X POST "$BROKER_URL/v1/admin/apps" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$APP_NAME\",\"scope_ceiling\":$APP_SCOPE_CEILING}")
APP_ID=$(echo "$APP_RESPONSE" | jq -r '.app_id // empty')
APP_SECRET=$(echo "$APP_RESPONSE" | jq -r '.app_secret // empty')
if [[ -z "$APP_ID" ]]; then
  fail "app registration" "no app_id in response: $APP_RESPONSE"
fi
pass "app registered (app_id=$APP_ID)"

# --- Step 3: Create launch token ---
LAUNCH_TOKEN=$(curl -sf -X POST "$BROKER_URL/v1/admin/launch-tokens" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"app_id\":\"$APP_ID\",\"scope\":$REQUESTED_SCOPE}" \
  | jq -r '.launch_token // empty')
if [[ -z "$LAUNCH_TOKEN" ]]; then
  fail "launch token" "no launch_token in response"
fi
pass "launch token issued"

# --- Step 4: Exchange for agent token ---
AGENT_TOKEN=$(curl -sf -X POST "$BROKER_URL/v1/app/tokens" \
  -H "Authorization: Bearer $LAUNCH_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"scope\":$REQUESTED_SCOPE}" \
  | jq -r '.token // empty')
if [[ -z "$AGENT_TOKEN" ]]; then
  fail "token exchange" "no token in response"
fi
pass "agent token exchanged"

# --- Step 5: Decode and verify JWT structure ---
# JWT is base64url.base64url.base64url — decode header and payload
decode_jwt_part() {
  local part=$1
  # base64url -> base64, pad, decode
  local padded=$(echo -n "$part" | tr '_-' '/+')
  while [ $((${#padded} % 4)) -ne 0 ]; do padded="${padded}="; done
  echo "$padded" | base64 -d 2>/dev/null
}

HEADER_B64=$(echo "$AGENT_TOKEN" | cut -d'.' -f1)
PAYLOAD_B64=$(echo "$AGENT_TOKEN" | cut -d'.' -f2)
HEADER=$(decode_jwt_part "$HEADER_B64")
PAYLOAD=$(decode_jwt_part "$PAYLOAD_B64")

ALG=$(echo "$HEADER" | jq -r '.alg // empty')
KID=$(echo "$HEADER" | jq -r '.kid // empty')
ISS=$(echo "$PAYLOAD" | jq -r '.iss // empty')
EXP=$(echo "$PAYLOAD" | jq -r '.exp // empty')
IAT=$(echo "$PAYLOAD" | jq -r '.iat // empty')
SCOPE_CLAIM=$(echo "$PAYLOAD" | jq -c '.scope // empty')

[[ "$ALG" == "EdDSA" ]] || fail "jwt alg" "expected EdDSA, got $ALG"
[[ -n "$KID" ]] || fail "jwt kid" "kid missing"
[[ -n "$ISS" ]] || fail "jwt iss" "iss missing"
[[ $EXP -gt $IAT ]] || fail "jwt exp" "exp ($EXP) not greater than iat ($IAT)"
[[ "$SCOPE_CLAIM" == "$REQUESTED_SCOPE" ]] || fail "jwt scope" "expected $REQUESTED_SCOPE, got $SCOPE_CLAIM"
pass "JWT structure valid (alg=EdDSA, kid present, scope matches)"

# --- Step 6: Verify token is accepted ---
VERIFY_STATUS=$(curl -so /dev/null -w "%{http_code}" -X POST "$BROKER_URL/v1/agent/verify" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$AGENT_TOKEN\"}")
[[ "$VERIFY_STATUS" == "200" ]] || fail "verify accepted" "expected 200, got $VERIFY_STATUS"
pass "token verified (200 OK)"

# --- Step 7: Revoke the token ---
JTI=$(echo "$PAYLOAD" | jq -r '.jti // empty')
[[ -n "$JTI" ]] || fail "jwt jti" "jti missing for revocation"
REVOKE_STATUS=$(curl -so /dev/null -w "%{http_code}" -X POST "$BROKER_URL/v1/admin/revocations" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"jti\":\"$JTI\"}")
[[ "$REVOKE_STATUS" == "200" ]] || fail "revocation" "expected 200, got $REVOKE_STATUS"
pass "token revoked (jti=$JTI)"

# --- Step 8: Verify revoked token is rejected ---
REVOKED_VERIFY_STATUS=$(curl -so /dev/null -w "%{http_code}" -X POST "$BROKER_URL/v1/agent/verify" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$AGENT_TOKEN\"}")
if [[ "$REVOKED_VERIFY_STATUS" != "401" && "$REVOKED_VERIFY_STATUS" != "403" ]]; then
  fail "revocation enforced" "expected 401/403, got $REVOKED_VERIFY_STATUS"
fi
pass "revoked token rejected ($REVOKED_VERIFY_STATUS)"

# --- Step 9: Out-of-scope request denied ---
OOS_STATUS=$(curl -so /dev/null -w "%{http_code}" -X POST "$BROKER_URL/v1/app/tokens" \
  -H "Authorization: Bearer $LAUNCH_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"scope\":$OUT_OF_SCOPE}")
[[ "$OOS_STATUS" == "403" ]] || fail "out-of-scope denied" "expected 403, got $OOS_STATUS"
pass "out-of-scope request denied (403)"

echo ""
echo "L2.5 SMOKE: PASS"
exit 0
```

- [ ] **Step 3: Make it executable**

```bash
chmod +x scripts/smoke/core-contract.sh
```

- [ ] **Step 4: Test against a running broker**

```bash
# Start the broker if not already running
export AA_ADMIN_SECRET="live-test-secret-32bytes-long-ok"
./scripts/stack_up.sh

# Wait for health
sleep 2
curl -sf http://localhost:8080/v1/health

# Run the smoke
./scripts/smoke/core-contract.sh
```

Expected output ends with `L2.5 SMOKE: PASS`. If any step fails, debug by:
- Check broker logs (`docker compose logs broker`)
- Check the exact curl command that failed (set `set -x` at top of script)
- Verify the endpoint paths against `docs/api.md`

**IMPORTANT:** If any endpoint path is wrong (e.g., `/v1/admin/login` doesn't exist), check `docs/api.md` and update the smoke script to match. The smoke script MUST reflect reality, not the plan's assumptions.

- [ ] **Step 5: Tear down the broker**

```bash
./scripts/stack_down.sh
```

- [ ] **Step 6: Commit**

```bash
git add scripts/smoke/core-contract.sh
git commit -m "feat(gates): add L2.5 core contract smoke script

Nine-step smoke verifying the credential broker's contract:
admin auth, app registration, launch token, agent token exchange,
JWT structure validation, verify accepted, revocation, revoke
enforcement, out-of-scope denial.

Used by scripts/gates.sh full (locally) and ci.yml smoke-l2.5 job
(in CI). Caller is responsible for starting/stopping the broker.

Deterministic: fixed fixtures, no sleeps, no retries."
```

---

### Task 7: Create `scripts/test-gate-parity.sh` — parity enforcement

**Files:**
- Create: `scripts/test-gate-parity.sh`

- [ ] **Step 1: Create the parity test**

```bash
#!/usr/bin/env bash
set -euo pipefail

# test-gate-parity.sh — enforce local/CI gate list alignment
#
# Reads gate IDs from:
#   (a) scripts/gates.sh --list-gates
#   (b) .github/workflows/ci.yml via grep of job IDs under `jobs:`
#
# Fails if the two lists differ.
#
# Used by: scripts/gates.sh full (indirectly) and ci.yml gate-parity job.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

GATES_SH="$SCRIPT_DIR/gates.sh"
CI_YML="$REPO_ROOT/.github/workflows/ci.yml"

if [[ ! -x "$GATES_SH" ]]; then
  echo "FAIL: $GATES_SH not found or not executable"
  exit 1
fi

if [[ ! -f "$CI_YML" ]]; then
  echo "FAIL: $CI_YML not found"
  exit 1
fi

# Source of truth A: gates.sh
GATES_FROM_SCRIPT=$("$GATES_SH" --list-gates | sort)

# Source of truth B: ci.yml
# Extract the list of gate job IDs. The ci.yml has a comment block listing
# the canonical gate IDs in the format:
#   # GATE_LIST_START
#   # - build
#   # - vet
#   # ...
#   # GATE_LIST_END
# We parse this block so the list is a single source of truth per file.
GATES_FROM_CI=$(awk '
  /# GATE_LIST_START/ { in_block=1; next }
  /# GATE_LIST_END/   { in_block=0; next }
  in_block && /^# - / { sub(/^# - /, ""); print }
' "$CI_YML" | sort)

if [[ -z "$GATES_FROM_CI" ]]; then
  echo "FAIL: no GATE_LIST_START/END block found in $CI_YML"
  exit 1
fi

# Diff
if diff <(echo "$GATES_FROM_SCRIPT") <(echo "$GATES_FROM_CI") >/dev/null; then
  echo "PASS: gate lists match ($(echo "$GATES_FROM_SCRIPT" | wc -l | tr -d ' ') gates)"
  exit 0
else
  echo "FAIL: gates.sh and ci.yml disagree on the gate list"
  echo ""
  echo "--- gates.sh --list-gates ---"
  echo "$GATES_FROM_SCRIPT"
  echo ""
  echo "--- ci.yml GATE_LIST block ---"
  echo "$GATES_FROM_CI"
  echo ""
  echo "Diff:"
  diff <(echo "$GATES_FROM_SCRIPT") <(echo "$GATES_FROM_CI") || true
  exit 1
fi
```

- [ ] **Step 2: Make it executable**

```bash
chmod +x scripts/test-gate-parity.sh
```

- [ ] **Step 3: Do NOT run it yet** — ci.yml doesn't exist yet, so this will fail. That's expected. The script is ready; it'll be exercised starting Task 12.

- [ ] **Step 4: Commit**

```bash
git add scripts/test-gate-parity.sh
git commit -m "chore(gates): add gate-parity enforcement script

Reads gate IDs from gates.sh --list-gates and from ci.yml's
GATE_LIST_START/END comment block. Fails if they differ.

Prevents local and CI gate definitions from silently drifting.
Runs as its own gate both locally (in 'full' mode once ci.yml
exists) and in CI (as the gate-parity job)."
```

---

### Task 8: Install `syft` locally and baseline SBOM

**Files:** none (tool install + baseline SBOM generation)

- [ ] **Step 1: Install syft**

```bash
# macOS
brew install syft

# Or direct install
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
syft version
```

Expected: syft version printed.

- [ ] **Step 2: Generate a baseline SBOM**

```bash
syft packages dir:. -o spdx-json=/tmp/sbom-baseline.spdx.json --quiet
jq '.packages | length' /tmp/sbom-baseline.spdx.json
```

Expected: a number (count of packages found).

- [ ] **Step 3: Verify SBOM is well-formed**

```bash
jq '.spdxVersion, .dataLicense, (.packages | length)' /tmp/sbom-baseline.spdx.json
```

Expected: `"SPDX-2.3"`, `"CC0-1.0"`, and a package count.

No commit — syft install is environmental, not repo content.

---

### Task 9: Run `./scripts/gates.sh full` locally

**Files:** none (verification only)

- [ ] **Step 1: Start the broker**

```bash
export AA_ADMIN_SECRET="live-test-secret-32bytes-long-ok"
./scripts/stack_up.sh
sleep 3
curl -sf http://localhost:8080/v1/health
```

Expected: health endpoint returns.

- [ ] **Step 2: Run full gates**

```bash
./scripts/gates.sh full
```

Expected: all 13 gates (task + unit-tests-race + docker-build + smoke-l2.5 + sbom) pass. The parity test gate is NOT in gates.sh (it only runs in CI and as a separate script). Final line: `RESULT: PASSED`.

If any gate fails:
- **build/vet/lint/format:** fix source code
- **contamination:** CRITICAL — investigate immediately, should never fail on clean develop
- **unit-tests / unit-tests-race:** fix failing tests
- **gosec:** triage findings per Task 2 Step 4 process
- **govulncheck:** update deps or add tech debt entry
- **go-mod-verify:** run `go mod tidy` and commit the result
- **docker-build:** check Dockerfile
- **smoke-l2.5:** broker must be running, check endpoint paths
- **sbom:** syft must be installed

- [ ] **Step 3: Tear down**

```bash
./scripts/stack_down.sh
```

- [ ] **Step 4: If there were any fixes in Step 2**, commit them with an appropriate message:

```bash
git add -p  # review each fix
git commit -m "fix(gates): address findings from first full gates.sh run"
```

---

### Task 10: Update CHANGELOG.md with M-sec entry

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Read current `CHANGELOG.md`**

```bash
head -40 CHANGELOG.md
```

Note the existing format and most recent Unreleased section (if any).

- [ ] **Step 2: Add/extend the Unreleased section**

Add an entry under `## [Unreleased]` (create the section if missing):

```markdown
### Added — CI/build/gates (M-sec v1)

- `.gosec.yml` — explicit gosec configuration with documented suppressions policy.
- `.golangci.yml` — security-aware golangci-lint config (errcheck, gosec, govet,
  ineffassign, staticcheck, unused, gosimple, bodyclose, misspell, gofmt,
  goimports).
- `scripts/smoke/core-contract.sh` — L2.5 core contract smoke test
  (issue/verify/revoke/deny out-of-scope) against a running broker.
- `scripts/test-gate-parity.sh` — enforces gate list alignment between
  `scripts/gates.sh` and `.github/workflows/ci.yml`.
- `.github/workflows/ci.yml` — parallel per-gate CI pipeline on PR and push to
  `develop`/`main`.
- `.github/workflows/codeql.yml` — CodeQL SAST (PR + push + weekly).
- `.github/workflows/scorecard.yml` — OpenSSF Scorecard (weekly + push to main).
- `.github/workflows/nightly.yml` — L4 full regression suite (scheduled).
- `.github/workflows/contribution-policy.yml` — auto-closes external PRs per
  Decision 014 (`pull_request_target`, no PR-branch checkout).
- `.github/dependabot.yml` — weekly dependency updates for github-actions,
  gomod, docker ecosystems with pinned-SHA maintenance.
- `.github/CODEOWNERS` and `.github/MAINTAINERS` — ownership and contribution
  allowlist.

### Changed — CI/build/gates (M-sec v1)

- `scripts/gates.sh` — extended with contamination grep, `govulncheck`,
  `go mod verify`, docker-build, L2.5 smoke, SBOM generation. `gosec` flipped
  from warn-only to blocking. `module` mode renamed to `full` (deprecated
  alias retained). Dead references to `live_test.sh` and `live_test_docker.sh`
  removed. `--list-gates` flag added for parity test.
- `README.md` — added M-sec badges (build, CodeQL, Scorecard, license, Go
  version, security policy).

### Security — M-sec rationale

Per Obsidian KB Decision 015, the M-sec scope treats CI as security evidence,
not just build verification. `govulncheck` and `gosec` are blocking gates.
Pinned action SHAs protect against action hijacking between Dependabot
rotations. The L2.5 core contract smoke verifies the product's actual
contract (issue/verify/revoke/deny) on every PR rather than a generic health
check.
```

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): add M-sec CI/build/gates v1 entry"
```

---

### Task 11: Phase A complete — confirm clean state

**Files:** none (verification)

- [ ] **Step 1: Verify all Phase A commits are in place**

```bash
git log --oneline develop..HEAD
```

Expected: ~7-9 commits (Tasks 2, 3, 5, 6, 7, 10 each produced one commit; Tasks 4 and 9 may have produced commits if there were fixes).

- [ ] **Step 2: Run `./scripts/gates.sh task` one more time to confirm clean state**

```bash
./scripts/gates.sh task
```

Expected: `RESULT: PASSED`.

- [ ] **Step 3: No commit** — this is a confirmation checkpoint.

---

## Phase B — GitHub Actions workflows (Tasks 12–21)

Phase B creates all five workflow files plus Dependabot config, CODEOWNERS, MAINTAINERS. Still on `feature/ci-msec`, still not pushed.

---

### Task 12: Create `.github/dependabot.yml`

**Files:**
- Create: `.github/dependabot.yml`

- [ ] **Step 1: Create the `.github/` directory**

```bash
mkdir -p .github/workflows
```

- [ ] **Step 2: Write `.github/dependabot.yml`**

```yaml
# Dependabot config for agentauth-core
# https://docs.github.com/en/code-security/dependabot/dependabot-version-updates/configuration-options-for-the-dependabot.yml-file

version: 2
updates:
  # GitHub Actions — SHA maintenance for pinned workflow steps
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
      time: "06:00"
      timezone: "UTC"
    open-pull-requests-limit: 3
    commit-message:
      prefix: "chore(deps)"
      include: "scope"
    groups:
      github-actions:
        patterns:
          - "*"
    labels:
      - "dependencies"
      - "github-actions"

  # Go modules — direct and indirect dependencies
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
      time: "06:00"
      timezone: "UTC"
    open-pull-requests-limit: 3
    commit-message:
      prefix: "chore(deps)"
      include: "scope"
    groups:
      go-direct:
        dependency-type: "direct"
      go-indirect:
        dependency-type: "indirect"
    labels:
      - "dependencies"
      - "go"

  # Docker base images
  - package-ecosystem: "docker"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
      time: "06:00"
      timezone: "UTC"
    open-pull-requests-limit: 2
    commit-message:
      prefix: "chore(deps)"
      include: "scope"
    labels:
      - "dependencies"
      - "docker"
```

- [ ] **Step 3: Commit**

```bash
git add .github/dependabot.yml
git commit -m "chore(deps): add Dependabot config for github-actions, gomod, docker"
```

---

### Task 13: Create `.github/CODEOWNERS` and `.github/MAINTAINERS`

**Files:**
- Create: `.github/CODEOWNERS`
- Create: `.github/MAINTAINERS`

- [ ] **Step 1: Create `.github/CODEOWNERS`**

```
# Global ownership — all paths default to the maintainer.
# Per Decision 014, external contributions are not accepted, so CODEOWNERS
# primarily serves as documentation and branch-protection review enforcement.

* @devonartis
```

- [ ] **Step 2: Create `.github/MAINTAINERS`**

```
# MAINTAINERS — allowlist for contribution-policy.yml
#
# Users listed here bypass the auto-close policy in
# .github/workflows/contribution-policy.yml. Anyone else opening a PR
# (except Dependabot and github-actions bot) gets auto-closed with a
# templated comment pointing to the issues-only contribution policy
# (Decision 014).
#
# Format: one GitHub username per line, no @ prefix, no comments inline.

devonartis
```

- [ ] **Step 3: Commit**

```bash
git add .github/CODEOWNERS .github/MAINTAINERS
git commit -m "chore(github): add CODEOWNERS and MAINTAINERS allowlist"
```

---

### Task 14: Create `.github/workflows/ci.yml` — main gate pipeline

**Files:**
- Create: `.github/workflows/ci.yml`

This is the largest single file in the plan. It has 13 parallel jobs plus a `gates-passed` aggregator.

- [ ] **Step 1: Create `.github/workflows/ci.yml`**

```yaml
name: CI

# GATE_LIST_START
# - build
# - vet
# - lint
# - format
# - contamination
# - unit-tests
# - gosec
# - govulncheck
# - go-mod-verify
# - unit-tests-race
# - docker-build
# - smoke-l2.5
# - sbom
# GATE_LIST_END

on:
  pull_request:
    branches: [develop]
  push:
    branches: [develop, main]

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

# Parameterized — no hardcoded owner/repo names. Rebrand-resilient per Decision 015.
permissions:
  contents: read

jobs:
  build:
    name: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - run: go build ./cmd/broker ./cmd/aactl

  vet:
    name: vet
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - run: go vet ./...

  lint:
    name: lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest
          args: --config .golangci.yml

  format:
    name: format
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: |
          unformatted=$(gofmt -l .)
          if [[ -n "$unformatted" ]]; then
            echo "The following files are not gofmt'd:"
            echo "$unformatted"
            exit 1
          fi

  contamination:
    name: contamination
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          if grep -ri 'hitl\|approval\|oidc\|federation\|cloud\|sidecar' internal/ cmd/ 2>/dev/null; then
            echo "FAIL: enterprise references found in core code"
            exit 1
          fi
          echo "PASS: no enterprise contamination"

  unit-tests:
    name: unit-tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - run: go test -short -count=1 ./...

  unit-tests-race:
    name: unit-tests-race
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - run: go test -race -count=1 -coverprofile=coverage.out ./...
      - uses: codecov/codecov-action@v4
        with:
          files: ./coverage.out
          fail_ci_if_error: false
          verbose: true

  gosec:
    name: gosec
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: securego/gosec@master
        with:
          args: '-conf .gosec.yml ./...'

  govulncheck:
    name: govulncheck
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...

  go-mod-verify:
    name: go-mod-verify
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: |
          go mod verify
          go mod tidy
          if ! git diff --exit-code go.mod go.sum; then
            echo "FAIL: go.mod or go.sum changed after 'go mod tidy'"
            exit 1
          fi

  docker-build:
    name: docker-build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build image
        run: docker build -t agentauth-ci:${{ github.sha }} .

  smoke-l2.5:
    name: smoke-l2.5
    runs-on: ubuntu-latest
    needs: [docker-build]  # depends on the image existing
    env:
      AA_ADMIN_SECRET: live-test-secret-32bytes-long-ok  # known test fixture per MEMORY.md
    steps:
      - uses: actions/checkout@v4
      - name: Start broker
        run: |
          export AA_ADMIN_SECRET="$AA_ADMIN_SECRET"
          ./scripts/stack_up.sh
          # Wait for health endpoint
          for i in {1..30}; do
            if curl -sf http://localhost:8080/v1/health >/dev/null 2>&1; then
              echo "Broker up after $i seconds"
              break
            fi
            sleep 1
          done
      - name: Run L2.5 core contract smoke
        run: ./scripts/smoke/core-contract.sh
      - name: Teardown
        if: always()
        run: ./scripts/stack_down.sh

  sbom:
    name: sbom
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: anchore/sbom-action@v0
        with:
          path: .
          format: spdx-json
          output-file: sbom.spdx.json
      - uses: actions/upload-artifact@v4
        with:
          name: sbom
          path: sbom.spdx.json
          retention-days: 30

  dep-review:
    name: dep-review
    runs-on: ubuntu-latest
    if: github.event_name == 'pull_request'
    steps:
      - uses: actions/checkout@v4
      - uses: actions/dependency-review-action@v4
        with:
          fail-on-severity: moderate

  changelog:
    name: changelog
    runs-on: ubuntu-latest
    if: github.event_name == 'pull_request'
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Check CHANGELOG.md diff
        run: |
          # Skip if the PR has the 'skip-changelog' label
          if echo '${{ toJson(github.event.pull_request.labels) }}' | grep -q '"skip-changelog"'; then
            echo "Label 'skip-changelog' present — bypassing CHANGELOG check"
            exit 0
          fi
          BASE_SHA='${{ github.event.pull_request.base.sha }}'
          if git diff --name-only "$BASE_SHA" HEAD | grep -q '^CHANGELOG.md$'; then
            echo "PASS: CHANGELOG.md touched in this PR"
          else
            echo "FAIL: This PR does not touch CHANGELOG.md"
            echo "Add a CHANGELOG entry or apply the 'skip-changelog' label if the PR is docs/tests-only."
            exit 1
          fi

  gate-parity:
    name: gate-parity
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: ./scripts/test-gate-parity.sh

  gates-passed:
    name: gates-passed
    runs-on: ubuntu-latest
    needs:
      - build
      - vet
      - lint
      - format
      - contamination
      - unit-tests
      - unit-tests-race
      - gosec
      - govulncheck
      - go-mod-verify
      - docker-build
      - smoke-l2.5
      - sbom
      - gate-parity
    if: always()
    steps:
      - name: Check all gates passed
        run: |
          if [[ "${{ contains(needs.*.result, 'failure') }}" == "true" ]]; then
            echo "One or more gates failed"
            exit 1
          fi
          if [[ "${{ contains(needs.*.result, 'cancelled') }}" == "true" ]]; then
            echo "One or more gates were cancelled"
            exit 1
          fi
          echo "All gates passed"
```

- [ ] **Step 2: Lint the workflow locally with `actionlint` (if available)**

```bash
if command -v actionlint &>/dev/null; then
  actionlint .github/workflows/ci.yml
else
  echo "actionlint not installed — skipping (install: brew install actionlint)"
fi
```

If `actionlint` finds issues, fix them before committing.

- [ ] **Step 3: Run the parity test locally — it should now pass**

```bash
./scripts/test-gate-parity.sh
```

Expected: `PASS: gate lists match (13 gates)`.

If it fails, the GATE_LIST comment block in `ci.yml` and the `GATES_FULL` array in `gates.sh` must match exactly.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "feat(ci): add main CI workflow with 13 parallel gates

Parallel per-gate jobs: build, vet, lint, format, contamination,
unit-tests, unit-tests-race, gosec, govulncheck, go-mod-verify,
docker-build, smoke-l2.5, sbom. Plus dep-review and changelog
on PR events, gate-parity enforcement, and a gates-passed
aggregator for branch protection.

Triggers: pull_request and push to develop/main.

All gates blocking. Coverage is uploaded informationally
(codecov, non-blocking).

Per Obsidian KB Decision 015."
```

---

### Task 15: Create `.github/workflows/codeql.yml`

**Files:**
- Create: `.github/workflows/codeql.yml`

- [ ] **Step 1: Write `.github/workflows/codeql.yml`**

```yaml
name: CodeQL

on:
  pull_request:
    branches: [develop]
  push:
    branches: [develop, main]
  schedule:
    - cron: '31 7 * * 1'  # weekly Monday 07:31 UTC

permissions:
  actions: read
  contents: read
  security-events: write

jobs:
  analyze:
    name: analyze
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        language: [go]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: github/codeql-action/init@v3
        with:
          languages: ${{ matrix.language }}
          queries: security-extended,security-and-quality
      - uses: github/codeql-action/autobuild@v3
      - uses: github/codeql-action/analyze@v3
        with:
          category: "/language:${{ matrix.language }}"
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/codeql.yml
git commit -m "feat(ci): add CodeQL SAST workflow

Runs on PR and push to develop/main, weekly scheduled scan.
Uses security-extended + security-and-quality query suites.
Results populate the Security tab and the CodeQL badge."
```

---

### Task 16: Create `.github/workflows/scorecard.yml`

**Files:**
- Create: `.github/workflows/scorecard.yml`

- [ ] **Step 1: Write `.github/workflows/scorecard.yml`**

```yaml
name: Scorecard supply-chain security

on:
  branch_protection_rule:
  schedule:
    - cron: '25 3 * * 2'  # weekly Tuesday 03:25 UTC
  push:
    branches: [main]

permissions: read-all

jobs:
  analysis:
    name: Scorecard analysis
    runs-on: ubuntu-latest
    permissions:
      security-events: write
      id-token: write
      contents: read
      actions: read
    steps:
      - uses: actions/checkout@v4
        with:
          persist-credentials: false
      - uses: ossf/scorecard-action@v2
        with:
          results_file: results.sarif
          results_format: sarif
          publish_results: true
      - uses: actions/upload-artifact@v4
        with:
          name: SARIF file
          path: results.sarif
          retention-days: 5
      - uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/scorecard.yml
git commit -m "feat(ci): add OpenSSF Scorecard workflow

Runs on push to main, weekly schedule, and branch protection changes.
Publishes results to the OpenSSF Scorecard badge and uploads SARIF
to the Security tab.

Informational only — not a required check. Scorecard's value signal
appears when the repo flips public."
```

---

### Task 17: Create `.github/workflows/nightly.yml` — L4 full regression

**Files:**
- Create: `.github/workflows/nightly.yml`

- [ ] **Step 1: Write `.github/workflows/nightly.yml`**

```yaml
name: Nightly full regression

on:
  schedule:
    - cron: '17 5 * * *'  # daily 05:17 UTC
  workflow_dispatch:        # manual trigger for ad-hoc runs

permissions:
  contents: read
  issues: write  # to open issues on failure

jobs:
  regression:
    name: L4 full regression
    runs-on: ubuntu-latest
    env:
      AA_ADMIN_SECRET: live-test-secret-32bytes-long-ok
    steps:
      - uses: actions/checkout@v4
        with:
          ref: develop
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build broker image
        run: |
          export AA_ADMIN_SECRET="$AA_ADMIN_SECRET"
          docker compose build

      - name: Run full regression suite
        id: regression
        run: ./scripts/gates.sh regression
        continue-on-error: true

      - name: Upload evidence on failure
        if: steps.regression.outcome == 'failure'
        uses: actions/upload-artifact@v4
        with:
          name: regression-evidence-${{ github.run_id }}
          path: tests/**/evidence/
          retention-days: 14

      - name: Open issue on failure
        if: steps.regression.outcome == 'failure'
        uses: actions/github-script@v7
        with:
          script: |
            const { owner, repo } = context.repo;
            const run_url = `https://github.com/${owner}/${repo}/actions/runs/${context.runId}`;
            const short_sha = context.sha.substring(0, 7);
            await github.rest.issues.create({
              owner,
              repo,
              title: `Nightly regression failed — ${short_sha}`,
              body: [
                '# Nightly L4 regression failure',
                '',
                `**Commit:** \`${short_sha}\``,
                `**Branch:** develop`,
                `**Workflow run:** ${run_url}`,
                '',
                'The nightly full regression suite failed. Evidence uploaded as workflow artifact.',
                '',
                'Triage steps:',
                '1. Download the `regression-evidence-${{ github.run_id }}` artifact',
                '2. Identify which batch failed (`scripts/gates.sh regression` output)',
                '3. Reproduce locally: `./scripts/gates.sh regression`',
                '4. Open a fix branch if the failure is real, or close this issue if flaky',
                '',
                '_Auto-created by `.github/workflows/nightly.yml`_',
              ].join('\n'),
              labels: ['regression', 'nightly', 'needs-triage'],
            });

      - name: Fail workflow if regression failed
        if: steps.regression.outcome == 'failure'
        run: exit 1
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/nightly.yml
git commit -m "feat(ci): add nightly L4 full regression workflow

Runs ./scripts/gates.sh regression nightly against develop.
On failure: uploads evidence artifacts, opens a GitHub issue
tagged 'regression/nightly/needs-triage', fails the workflow.

Informational only — does not block in-flight PRs per Decision
015. The 24-hour lag is acceptable because L2.5 smoke catches
the core contract regressions on every PR."
```

---

### Task 18: Create `.github/workflows/contribution-policy.yml`

**Files:**
- Create: `.github/workflows/contribution-policy.yml`

**CRITICAL:** this workflow uses `pull_request_target`. It MUST NEVER check out the PR branch. See Decision 015 for the security rationale.

- [ ] **Step 1: Write `.github/workflows/contribution-policy.yml`**

```yaml
name: Contribution Policy

# SECURITY: This workflow uses pull_request_target, which runs in the base
# branch context with write permissions. This is required to close PRs. The
# workflow MUST NEVER check out the PR branch — checking out untrusted PR code
# with write tokens is a supply-chain compromise vector.
#
# This workflow only reads metadata (PR author, PR number) via the GitHub API.
# It does NOT run actions/checkout.

on:
  pull_request_target:
    types: [opened, reopened]

permissions:
  pull-requests: write
  issues: write
  contents: read

jobs:
  check-author:
    name: Enforce contribution policy
    runs-on: ubuntu-latest
    steps:
      - name: Check PR author against MAINTAINERS
        uses: actions/github-script@v7
        with:
          script: |
            const { owner, repo } = context.repo;
            const pr = context.payload.pull_request;
            const author = pr.user.login;
            const pr_number = pr.number;

            // Always-exempt bots
            const bot_exempt = ['dependabot[bot]', 'github-actions[bot]', 'renovate[bot]'];
            if (bot_exempt.includes(author)) {
              core.info(`Bot author ${author} exempt — no action`);
              return;
            }

            // Check if author has write access to the repo
            try {
              const { data: perms } = await github.rest.repos.getCollaboratorPermissionLevel({
                owner,
                repo,
                username: author,
              });
              if (['admin', 'maintain', 'write'].includes(perms.permission)) {
                core.info(`Author ${author} has ${perms.permission} access — exempt`);
                return;
              }
            } catch (e) {
              // Not a collaborator — continue to MAINTAINERS check
            }

            // Check MAINTAINERS file via GitHub API (not via checkout)
            let maintainers = [];
            try {
              const { data: file } = await github.rest.repos.getContent({
                owner,
                repo,
                path: '.github/MAINTAINERS',
                ref: context.payload.pull_request.base.ref,
              });
              const content = Buffer.from(file.content, 'base64').toString('utf-8');
              maintainers = content
                .split('\n')
                .map(l => l.trim())
                .filter(l => l && !l.startsWith('#'));
            } catch (e) {
              core.warning(`Could not read MAINTAINERS file: ${e.message}`);
            }

            if (maintainers.includes(author)) {
              core.info(`Author ${author} in MAINTAINERS — exempt`);
              return;
            }

            // Not exempt — enforce policy
            core.info(`Author ${author} not exempt — closing PR per Decision 014`);

            const policy_comment = [
              `Hi @${author}, thank you for your interest in AgentAuth!`,
              '',
              'Per our contribution policy ([Decision 014](https://github.com/' + owner + '/' + repo + '/blob/develop/CONTRIBUTING.md)), AgentAuth does not accept external code contributions at this time — including bug fixes.',
              '',
              'We actively welcome:',
              '- **Bug reports** — please [open an issue](https://github.com/' + owner + '/' + repo + '/issues/new)',
              '- **Feature requests** — same place',
              '- **Security vulnerabilities** — please see [SECURITY.md](https://github.com/' + owner + '/' + repo + '/blob/develop/SECURITY.md) for the responsible disclosure process',
              '',
              'This policy exists because we\'re still defining our contribution workflow (test plan, merge process, review gates). Opening to PRs before that\'s ready would mean every PR becomes a coaching session, which wouldn\'t be fair to you or to us. The policy will be revisited once the workflow is documented and tested.',
              '',
              'This PR will be closed automatically. Please don\'t take it personally — the bot is enforcing policy, not judging your work. We genuinely appreciate the interest.',
              '',
              '_Auto-enforced by `.github/workflows/contribution-policy.yml`_',
            ].join('\n');

            await github.rest.issues.createComment({
              owner,
              repo,
              issue_number: pr_number,
              body: policy_comment,
            });

            await github.rest.pulls.update({
              owner,
              repo,
              pull_number: pr_number,
              state: 'closed',
            });
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/contribution-policy.yml
git commit -m "feat(ci): add contribution policy enforcement workflow

Auto-closes PRs from non-maintainers with a templated comment
pointing to the issues-only contribution policy (Decision 014).

Security: uses pull_request_target for write permissions but
NEVER checks out the PR branch. Reads MAINTAINERS file via
GitHub API only. Exempts dependabot, github-actions bot, and
users with write access or listed in .github/MAINTAINERS."
```

---

### Task 19: actionlint all workflows (if available)

**Files:** none (verification)

- [ ] **Step 1: Run actionlint on all workflows**

```bash
if command -v actionlint &>/dev/null; then
  actionlint .github/workflows/*.yml
else
  echo "actionlint not installed — skipping. Install: brew install actionlint"
fi
```

If actionlint is installed, fix any reported issues before moving on.

- [ ] **Step 2: Run the parity test one more time**

```bash
./scripts/test-gate-parity.sh
```

Expected: `PASS: gate lists match (13 gates)`.

- [ ] **Step 3: Commit any fixes**

Skip if no issues were found.

---

### Task 20: Phase B checkpoint — verify all files present

**Files:** none (verification)

- [ ] **Step 1: Confirm all Phase B files exist**

```bash
ls -la .github/workflows/
ls -la .github/dependabot.yml .github/CODEOWNERS .github/MAINTAINERS
ls -la .gosec.yml .golangci.yml
ls -la scripts/smoke/core-contract.sh scripts/test-gate-parity.sh
```

Expected: all files present, scripts executable.

- [ ] **Step 2: Review commit history for Phase B**

```bash
git log --oneline develop..HEAD
```

Expected: clean, logical commit progression through Phase A + Phase B.

---

## Phase C — Pin SHAs, push, iterate (Tasks 21–25)

---

### Task 21: Install actionlint if not already (recommended)

**Files:** none (tool install)

- [ ] **Step 1: Install actionlint**

```bash
# macOS
brew install actionlint

# Or direct
bash <(curl -sSfL https://raw.githubusercontent.com/rhysd/actionlint/main/scripts/download-actionlint.bash)
actionlint -version
```

- [ ] **Step 2: Run against all workflows**

```bash
actionlint .github/workflows/*.yml
```

Expected: no errors. Fix any reported issues and commit with `chore(ci): fix actionlint findings`.

---

### Task 22: Pin all action SHAs

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/codeql.yml`
- Modify: `.github/workflows/scorecard.yml`
- Modify: `.github/workflows/nightly.yml`
- Modify: `.github/workflows/contribution-policy.yml`

**Why:** Tags are mutable; a compromised action can change what its tag points to. Pinning to a 40-char SHA means Dependabot controls when SHAs rotate, and the rotation produces a visible PR.

- [ ] **Step 1: Collect the SHAs**

For each action below, visit the releases page, find the latest stable release, and record the full commit SHA. Commands to help (requires `gh` CLI):

```bash
get_latest_sha() {
  local action=$1
  gh api "repos/$action/releases/latest" --jq '.target_commitish' 2>/dev/null || \
  gh api "repos/$action/git/refs/tags/$(gh api repos/$action/releases/latest --jq .tag_name)" --jq '.object.sha'
}

# Actions used in this plan:
for action in \
  actions/checkout \
  actions/setup-go \
  actions/upload-artifact \
  actions/github-script \
  actions/dependency-review-action \
  golangci/golangci-lint-action \
  securego/gosec \
  github/codeql-action \
  ossf/scorecard-action \
  anchore/sbom-action \
  codecov/codecov-action ; do
  sha=$(get_latest_sha "$action")
  echo "$action@$sha"
done
```

Record the SHAs in a temp file for reference.

- [ ] **Step 2: Replace tags with SHAs in every workflow file**

For each `uses:` line, replace the tag reference with the SHA and add a `# v<version>` comment. Example:

```yaml
# Before:
- uses: actions/checkout@v4

# After:
- uses: actions/checkout@<40-char-sha> # v4.1.7
```

Apply this to **every** `uses:` line across all five workflow files.

- [ ] **Step 3: Re-run actionlint to confirm nothing broke**

```bash
actionlint .github/workflows/*.yml
```

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/
git commit -m "chore(ci): pin all action SHAs for supply chain safety

All actions referenced by their 40-char commit SHA with a
version comment. Dependabot will maintain these weekly per
.github/dependabot.yml.

Protects against action tag hijacking between Dependabot
rotations. Standard discipline for security-adjacent repos
per Obsidian KB Decision 015."
```

---

### Task 23: Push `feature/ci-msec` and observe the first CI run

**Files:** none (git push + observation)

- [ ] **Step 1: Confirm clean local state**

```bash
git status --short
./scripts/gates.sh task
```

Expected: working tree clean, all task gates pass.

- [ ] **Step 2: Push the branch**

```bash
git push -u origin feature/ci-msec
```

- [ ] **Step 3: Watch the CI run**

```bash
gh run watch
# or
gh run list --branch feature/ci-msec --limit 5
```

- [ ] **Step 4: Expect failures on the first run**

Common first-run issues:
- **`smoke-l2.5` fails because Docker takes longer to start than the 30s wait** → extend the wait loop in ci.yml
- **`docker-build` fails because Dockerfile references cache mounts the runner doesn't have** → adjust Dockerfile or CI step
- **`golangci-lint-action` version mismatch** → update the `version:` field
- **`gosec` action complains about config path** → check the `args:` path is relative to repo root
- **`codecov` upload fails on private repo** → either add `CODECOV_TOKEN` secret or set `fail_ci_if_error: false` (already set in the plan)
- **`go-mod-verify` fails because CI's Go toolchain is slightly different** → may need to commit `go.mod` changes
- **`contribution-policy` fires on our own PR when we open it in Task 25** → add the branch author to MAINTAINERS before opening the PR (already done in Task 13)
- **`smoke-l2.5` fails because the endpoint paths don't match** → update `scripts/smoke/core-contract.sh` to match `docs/api.md`

Iterate until CI is green on `feature/ci-msec`. Each fix is a new commit on the branch.

- [ ] **Step 5: Do not squash** — keep the iteration history for now. It'll be squashed in Task 25 when opening the PR.

---

### Task 24: Triage and fix CI-only issues

**Files:** whichever files need fixing

- [ ] **Step 1: For each failing job, read the logs**

```bash
gh run view --log-failed
```

- [ ] **Step 2: Fix the root cause**

Do NOT paper over issues:
- If `gosec` finds a real issue, fix the code or suppress it with justification
- If `smoke-l2.5` endpoint path is wrong, update the smoke script to match actual API
- If `docker-build` fails, fix the Dockerfile
- If `codecov` is spammy, tighten its config

- [ ] **Step 3: Commit fix, push, re-run**

```bash
git add <fixed files>
git commit -m "fix(ci): <what was wrong and how it's fixed>"
git push
gh run watch
```

- [ ] **Step 4: Repeat until all jobs green**

Be patient. Expect 3-8 iterations on first rollout.

---

### Task 25: Open PR from `feature/ci-msec` to `develop`

**Files:** none (PR creation)

- [ ] **Step 1: Confirm CI is green on the feature branch**

```bash
gh run list --branch feature/ci-msec --limit 3
```

Expected: latest run is green (all gates passed).

- [ ] **Step 2: Create the PR**

```bash
gh pr create --base develop --head feature/ci-msec \
  --title "feat(ci): M-sec CI/build/gates v1" \
  --body "$(cat <<'EOF'
## Summary

Implements the M-sec CI/build/gates pipeline per Obsidian KB Decision 015 and the design doc at `.plans/designs/2026-04-10-ci-build-gates-msec-design.md`.

**What changes:**
- Five GitHub Actions workflows: `ci.yml` (13 parallel gates), `codeql.yml`, `scorecard.yml`, `nightly.yml`, `contribution-policy.yml`
- `scripts/gates.sh` extended with M-sec gates; `gosec` flipped from warn to blocking
- New: `scripts/smoke/core-contract.sh` (L2.5 core contract smoke), `scripts/test-gate-parity.sh`
- New: `.gosec.yml`, `.golangci.yml`, `.github/dependabot.yml`, `.github/CODEOWNERS`, `.github/MAINTAINERS`
- `CHANGELOG.md` updated under Unreleased
- All action references pinned to 40-char SHAs per Decision 015 supply-chain discipline

**What this is NOT:**
- No release automation, no GHCR publish, no SLSA provenance (deferred to a later cycle)
- No pre-commit hooks (separate smaller cycle)
- No README badge updates (Task 26 post-merge)

**Rationale:** Decision 015 lays out why M-sec (not generic M) is the right scope for a credential broker, and why CI must exist before the AgentWrit rebrand lands.

## Test plan

- [x] `./scripts/gates.sh task` passes locally
- [x] `./scripts/gates.sh full` passes locally against a `stack_up.sh` broker
- [x] `./scripts/test-gate-parity.sh` passes
- [x] `./scripts/smoke/core-contract.sh` passes against a live broker (9/9 steps)
- [x] All CI gates green on `feature/ci-msec` before opening this PR
- [ ] Merge this PR
- [ ] Configure branch protection on `develop` (Task 27)
- [ ] Merge `develop` → `main` via `strip_for_main.sh`
- [ ] Configure branch protection on `main` (Task 30)
- [ ] 7-day observation: first Dependabot PR, first nightly run

## Related

- Obsidian KB Decision 015: CI/Gates Strategy — Security-First, Rebrand-Resilient
- Obsidian KB Decision 014: No External Contributions (enforced by `contribution-policy.yml`)
- Obsidian KB Decision 013: AgentWrit Rebrand (this CI unblocks the rebrand PR)
- Design doc: `.plans/designs/2026-04-10-ci-build-gates-msec-design.md`
EOF
)"
```

- [ ] **Step 3: Verify CI runs on the PR**

```bash
gh pr checks
```

Expected: all checks pending, then green within ~5-10 minutes.

---

## Phase D — Merge, protect, observe (Tasks 26–31)

---

### Task 26: Merge the PR to develop

**Files:** none (merge operation)

- [ ] **Step 1: Verify all checks green**

```bash
gh pr checks
```

- [ ] **Step 2: Merge**

Choose squash merge to collapse the iteration history into one clean commit:

```bash
gh pr merge --squash --delete-branch
```

Expected: PR closed, branch deleted locally and remotely.

- [ ] **Step 3: Pull latest develop**

```bash
git checkout develop
git pull origin develop
```

- [ ] **Step 4: Verify CI runs on push to develop**

```bash
gh run list --branch develop --limit 3
```

Expected: new run started by the merge commit.

---

### Task 27: Configure branch protection on `develop`

**Files:** none (GitHub API calls via `gh`)

- [ ] **Step 1: Wait for the develop CI run to complete green**

```bash
gh run watch
```

- [ ] **Step 2: Apply branch protection via `gh api`**

```bash
OWNER=$(gh repo view --json owner --jq .owner.login)
REPO=$(gh repo view --json name --jq .name)

gh api -X PUT "repos/$OWNER/$REPO/branches/develop/protection" \
  --input - <<'EOF'
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["gates-passed", "analyze"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": false,
    "required_approving_review_count": 0
  },
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
EOF
```

Note: `required_approving_review_count: 0` because the repo has a single maintainer. Increase if the team grows.

- [ ] **Step 3: Verify protection is active**

```bash
gh api "repos/$OWNER/$REPO/branches/develop/protection" | jq '.required_status_checks.contexts'
```

Expected: `["gates-passed", "analyze"]`.

---

### Task 28: Merge `develop` → `main` via strip script

**Files:** `strip_for_main.sh` runs

- [ ] **Step 1: Check out main and fast-forward**

```bash
git checkout main
git pull --ff-only origin main
git merge --no-ff develop -m "Merge develop → main: M-sec CI/build/gates v1"
```

- [ ] **Step 2: Run strip_for_main.sh**

```bash
./scripts/strip_for_main.sh
```

Expected: files in FLOW.md, MEMORY.md, .plans/, adr/, tests/, TECH-DEBT.md, etc. are removed from the working tree on main.

- [ ] **Step 3: Review what was stripped**

```bash
git status
```

Ensure the strip removed the expected files and didn't touch anything in `.github/`, `scripts/`, `.gosec.yml`, `.golangci.yml`, or `internal/`/`cmd/`.

- [ ] **Step 4: Commit the strip**

```bash
git add -A
git commit -m "chore: strip dev files for main merge"
```

- [ ] **Step 5: Push main**

```bash
git push origin main
```

- [ ] **Step 6: Verify CI runs on push to main**

```bash
gh run list --branch main --limit 3
```

Expected: new run on main. Wait for it to go green.

---

### Task 29: Configure branch protection on `main`

**Files:** none

- [ ] **Step 1: Wait for the main CI run to complete green**

```bash
gh run watch
```

- [ ] **Step 2: Apply branch protection**

```bash
gh api -X PUT "repos/$OWNER/$REPO/branches/main/protection" \
  --input - <<'EOF'
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["gates-passed", "analyze"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": false,
    "required_approving_review_count": 0
  },
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
EOF
```

- [ ] **Step 3: Verify**

```bash
gh api "repos/$OWNER/$REPO/branches/main/protection" | jq '.required_status_checks.contexts'
```

---

### Task 30: Add badges to README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Read current README**

```bash
head -30 README.md
```

Identify where badges should go (usually right after the title / description).

- [ ] **Step 2: Add the six M-sec badges**

Insert this block after the README title / intro:

```markdown
[![Build](https://github.com/devonartis/agentauth/actions/workflows/ci.yml/badge.svg)](https://github.com/devonartis/agentauth/actions/workflows/ci.yml)
[![CodeQL](https://github.com/devonartis/agentauth/actions/workflows/codeql.yml/badge.svg)](https://github.com/devonartis/agentauth/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/devonartis/agentauth/badge)](https://securityscorecards.dev/viewer/?uri=github.com/devonartis/agentauth)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/devonartis/agentauth)](go.mod)
[![Security Policy](https://img.shields.io/badge/security-policy-brightgreen)](SECURITY.md)
```

**NOTE on badge URLs:** These are the only places where the rebrand will need to sed — badge URLs contain `devonartis/agentauth` literals because markdown doesn't interpolate. Acceptable — documented in Decision 015.

- [ ] **Step 3: Commit**

```bash
git checkout develop
git add README.md
git commit -m "docs(readme): add M-sec badges (build, CodeQL, Scorecard, license, Go, security)"
git push origin develop
```

- [ ] **Step 4: Verify the CI runs triggered by this push stay green**

```bash
gh run watch
```

---

### Task 31: Observation period — confirm nightly + Dependabot fire

**Files:** none (monitoring)

- [ ] **Step 1: Wait for the next scheduled nightly run**

Next scheduled: 05:17 UTC the next day. Check:

```bash
gh run list --workflow=nightly.yml --limit 3
```

Expected: nightly run completes (green or red doesn't matter — the workflow should fire).

- [ ] **Step 2: Wait for the first Dependabot PR**

Next scheduled: Monday 06:00 UTC. Check:

```bash
gh pr list --label dependencies
```

Expected: Dependabot has opened PRs for any available github-actions / gomod / docker updates.

- [ ] **Step 3: Review and merge any Dependabot PRs that pass CI**

For each Dependabot PR:
```bash
gh pr checks <pr-number>
gh pr merge <pr-number> --squash
```

- [ ] **Step 4: Confirm the contribution-policy workflow is active**

No external PRs are likely during the observation window, so this is best-effort. If a test is desired, ask a maintainer collaborator to open a throwaway PR from a non-maintainer account and confirm it gets auto-closed with the templated comment.

---

## Post-rollout — Wrap-up

After Task 31, the M-sec CI/build/gates v1 is complete. Final handoff:

- [ ] Update `FLOW.md` with a `## 2026-04-XX — M-sec CI/build/gates v1 MERGED` entry (decision + what shipped + next priority)
- [ ] Update `MEMORY.md` with any lessons learned from the rollout (what broke, what surprised, what the user corrected)
- [ ] Consider writing an ADR in `adr/` for the specific technical architecture (referencing Decision 015). Suggested: `adr/015-ci-gates-architecture.md` — captures Option B, the parity test mechanism, the L2.5 contract, and the pinned-SHA discipline. This is the "how" ADR that complements Decision 015's "why."
- [ ] Close out this devflow cycle — mark complete in any tracker, and identify the next priority (likely the AgentWrit rebrand cycle, now unblocked by CI existing)

---

## Self-review checklist (run before handing off)

- [ ] Every task has exact file paths
- [ ] Every code block is complete — no "TBD", no "similar to above"
- [ ] Every command has an expected output or success criterion
- [ ] No hardcoded owner/repo strings in workflow files (parameterized via `${{ github.repository }}` or `${{ github.repository_owner }}`)
- [ ] All action references use `@v<N>` tags AT PLAN TIME but are pinned to SHAs in Task 22 BEFORE first push
- [ ] CHANGELOG.md entry exists and names all files/changes
- [ ] Decision 015 is referenced in the PR body
- [ ] Branch protection references the `gates-passed` aggregator (not individual job names) to survive gate list changes

---

**End of plan.** Total: 31 tasks across 4 phases. Estimated implementation time is deliberately not included — this is a plan, not a schedule.
