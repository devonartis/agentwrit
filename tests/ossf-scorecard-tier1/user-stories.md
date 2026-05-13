# OSSF Scorecard Tier-1 Hardening — Acceptance Stories

Stories derived from `~/proj/devflow/agentwrit/.plans/specs/2026-05-13-ossf-scorecard-tier1-spec.md` (council-approved + owner-revised). These are the acceptance criteria for PR opened against `fix/ossf-scorecard-tier1` → `develop`.

## Audience

| Reader | What they need from this file |
|--------|--------------------------------|
| **Executive** | Plain-language confirmation that public security signaling (OSSF Scorecard score) improves without breaking the broker. |
| **QA tester** | Step-by-step commands to verify each story, with copy-pastable expected output. |
| **Security reviewer** | Evidence that supply-chain hygiene improved (workflow permissions narrowed, base images and CLI installs hash-pinned, Python eliminated from the repo). |
| **Engineer** | Last. Stories are not Go test code — they are operator-level acceptance checks against a live broker and against GitHub's CI/Scorecard surfaces. |

## Precondition

- Branch `fix/ossf-scorecard-tier1` exists and is pushed to `origin`.
- Issue `#81` is open on `devonartis/agentwrit`.
- Baseline Scorecard score captured: `6.2` on commit `59acce7` at `2026-05-13T20:58:14Z` (recorded in spec).

---

## P1 — `[PRECONDITION]` Workflow `uses:` entries are SHA-pinned

| Field | Value |
|-------|-------|
| **Who** | Engineer (operator-equivalent) on the feature branch |
| **What** | Confirm every `uses:` reference in every workflow file is pinned to a 40-character commit SHA |
| **Why** | If a workflow line references a third-party action by a moving tag (like `@v6`), a malicious update to that tag could silently run code in our CI on every commit. Pinning by SHA freezes the exact bytes. If this fails, our public security signal (OSSF Scorecard) cannot reach the target score and our CI is also genuinely less safe. |
| **How** | Run from repo root:<br>`grep -rnE "^\s*-?\s*uses:" .github/workflows/ \| grep -vE "@[0-9a-f]{40}\b" \|\| echo "ALL_SHA_PINNED"` |
| **Expected** | Prints exactly `ALL_SHA_PINNED` and nothing else. (Audited 2026-05-13: clean.) |
| **If fails** | Surface to owner. Do NOT continue Tasks 1-6 until every `uses:` is SHA-pinned. |

---

## P3 — `[PRECONDITION]` `release.yml` is structurally valid (actionlint)

| Field | Value |
|-------|-------|
| **Who** | Engineer on the feature branch, after Task 2 commit lands |
| **What** | Run `actionlint` against the release workflow file (and the others, while we're at it) to confirm the YAML still parses, the permission block is correctly placed, and no expression syntax broke when permissions moved from top-level to job-level |
| **Why** | `release.yml` only runs when a `v*` tag is pushed to `main`. Between merging this PR and the next release, a typo in that file would be invisible to CI. If this fails, the next release would break and we wouldn't know until we tried to ship. |
| **How** | From repo root:<br>`actionlint .github/workflows/release.yml .github/workflows/codeql.yml .github/workflows/ci.yml`<br>If `actionlint` not installed locally: `bash <(curl -fsSL https://raw.githubusercontent.com/rhysd/actionlint/main/scripts/download-actionlint.bash) && ./actionlint .github/workflows/release.yml .github/workflows/codeql.yml .github/workflows/ci.yml` |
| **Expected** | Exit 0, no errors or warnings. |
| **If fails** | Surface the actionlint diagnostic. Most likely cause: misplaced `permissions:` key inside a job (wrong indentation) or wrong key name. Fix before the PR merges. |

## P2 — `[PRECONDITION]` Go Ed25519 helper builds and signs

| Field | Value |
|-------|-------|
| **Who** | Engineer on the feature branch, after Task 5's source files are written |
| **What** | Build the new helper at `tests/sec-l2b/edsign` and confirm it produces a valid base64 public key + signature given a hex nonce |
| **Why** | The bash integration script depends on this helper to run. If the helper doesn't build or returns malformed JSON, `smoke-l25` will fail in CI without revealing the cause. |
| **How** | From repo root:<br>`go build -o /tmp/sec-l2b-edsign ./tests/sec-l2b/edsign`<br>`/tmp/sec-l2b-edsign $(openssl rand -hex 32) \| jq -e '.public_key \| length > 0' >/dev/null && /tmp/sec-l2b-edsign $(openssl rand -hex 32) \| jq -e '.signature \| length > 0' >/dev/null && echo "helper works"` |
| **Expected** | Prints `helper works` and exit code 0. |
| **If fails** | Helper source needs fixing before any commit lands. Pure stdlib — no module fetches, no external deps. |

---

## A1 — `[ACCEPTANCE]` sec-l2b live broker run completes with the Go helper

| Field | Value |
|-------|-------|
| **Who** | Operator running the live sec-l2b acceptance test against a real broker |
| **What** | Spin up the broker, run `tests/sec-l2b/integration.sh`, observe Story S2 and S3 verdicts |
| **Why** | The Go helper replaces a long-standing Python implementation. If the agent-registration flow (challenge → sign → register → token) breaks, every downstream story breaks. This is the end-to-end smoke test for the helper swap. |
| **How** | Always-teardown wrapper so a broker process never leaks on failure:<br>```bash<br>export AA_ADMIN_SECRET="$(openssl rand -base64 32)"<br>trap './scripts/stack_down.sh' EXIT<br>./scripts/stack_up.sh<br>for i in {1..30}; do curl -sf http://localhost:8080/v1/health >/dev/null && break; sleep 1; done<br>./tests/sec-l2b/integration.sh<br>``` |
| **Expected** | Script exits 0. Stdout shows Story S2 PASS and Story S3 PASS. Output for `Agent token:` and `Agent ID:` fields populated (proves the helper produced a working signature that the broker accepted). Trap ensures `stack_down.sh` runs on success AND failure — no leaked containers either way. |
| **If fails** | Capture the stderr from the `go build`, the JSON from the helper, and the `curl` response from `/v1/register`. Surface to owner. |

---

## A2 — `[ACCEPTANCE]` All 20 CI gates pass on the feature PR

| Field | Value |
|-------|-------|
| **Who** | Anyone watching the PR after `git push` |
| **What** | All gates green; SARIF upload (CodeQL) and Dockerfile-based `docker-build` + `smoke-l25` succeed under the new permission scopes and SHA-pinned bases |
| **Why** | The whole point of this PR is to narrow permissions and pin dependencies WITHOUT breaking anything. If any CI gate goes red, we have broken a build path used by every future commit. Job-level permissions and SHA-pinned base images both need a real CI run to prove they work — this is that run. |
| **How** | Resolve the PR number first, then watch:<br>`PR=$(gh pr view --json number -q .number)`<br>`gh pr checks "$PR" --watch --interval 15` |
| **Expected** | All 20 gates `pass`: build, vet, lint, format, contamination, unit-tests, unit-tests-race, gosec, govulncheck, go-mod-verify, docker-build, sbom, dep-review, changelog, gate-parity, no-tracked-ignored, smoke-l25, gates-passed, analyze (go), CodeQL. |
| **If fails** | The specific failing gate is the diagnostic. Stop, fix root cause, push again. Never `--no-verify` past a failure. |

---

## A3 — `[ACCEPTANCE]` Python is fully removed from the repo

| Field | Value |
|-------|-------|
| **Who** | Security reviewer, post-merge |
| **What** | No `python3` invocations or `pip install` lines anywhere in the workflow or sec-l2b test surfaces |
| **Why** | The whole reason for picking Go over hash-pinning Python: align with the project rule "all crypto is Go stdlib." If a remnant `pip install` survives, the rule is violated and the Pinned-Dependencies finding may recur. |
| **How** | From repo root, after Task 5 commit lands:<br>`grep -rEw "(python3\|pip install\|cryptography)" .github/ tests/sec-l2b/ \|\| echo "PYTHON REMOVED"` (word-boundary form to avoid false positives on Go identifiers / comments containing `cryptography` as a substring) |
| **Expected** | Prints `PYTHON REMOVED` (no other output). |
| **If fails** | Either the workflow edit was incomplete or the bash script still references python. Find the offending line and remove. |

---

## A4 — `[ACCEPTANCE]` Token-Permissions check reports 10 on the next Scorecard run

| Field | Value |
|-------|-------|
| **Who** | Operator triggering the Scorecard workflow post-merge to `main` |
| **What** | OSSF Scorecard's `Token-Permissions` check moves from 0 (baseline `59acce7`) to 10 |
| **Why** | This is the headline outcome of Task 1 + Task 2. Per Scorecard v5.3.0 docs, job-level writes are NOT penalized when (a) the top level is read-only and (b) the writes are consumed by recognized actions (`codeql-action/analyze` and `cosign-installer`). |
| **How** | After PR merges to `develop` AND the develop→main promotion PR merges:<br>`gh workflow run scorecard.yml --ref main`<br>Wait for the Scorecard run to complete. Scorecard's public API is refreshed by an external BigQuery cron, so the API may lag the workflow run by **4-6 hours** (sometimes longer). Check workflow status with `gh run list --workflow scorecard.yml --limit 1 --json status,conclusion`. Once the run is `completed` AND `~4-6 hours` have passed, query the API:<br>`curl -s "https://api.scorecard.dev/projects/github.com/devonartis/agentwrit" \| jq '.repo.commit, .checks[] \| select(.name == "Token-Permissions") \| {score, reason}'`<br>Confirm `.repo.commit` matches the new `main` HEAD before trusting the score. |
| **Expected** | `score: 10`, reason indicates "Tokens are only granted minimum required permissions" (or equivalent positive language). API `.repo.commit` matches the post-merge `main` HEAD SHA. |
| **If fails** | If `.repo.commit` still shows the pre-merge SHA, the API hasn't refreshed yet — wait longer. If score = 9 with "actions:read at job-level not penalized" still flagging, Scorecard's algorithm may have changed since spec write — surface to owner with the JSON. If score = 0 still, the permission move didn't take effect on `main` — verify the develop→main PR included Task 1 and Task 2 commits. |

---

## A5 — `[ACCEPTANCE]` Pinned-Dependencies check reports 10

| Field | Value |
|-------|-------|
| **Who** | Same operator + same Scorecard run as A4 |
| **What** | OSSF Scorecard's `Pinned-Dependencies` check moves from 8 to 10 |
| **Why** | Task 3 (Docker SHA pins) + Task 4 (govulncheck SHA pin) + Task 5 (Python removed) eliminate all 4 originally-flagged unpinned dependencies. P1 confirms no `uses:` lines re-introduce unpinned references. |
| **How** | `curl -s "https://api.scorecard.dev/projects/github.com/devonartis/agentwrit" \| jq '.checks[] \| select(.name == "Pinned-Dependencies") \| {score, reason, details}'` |
| **Expected** | `score: 10`, no `Warn:` entries in `details`. |
| **If fails** | The `details` field will name the still-unpinned item. Most likely cause: a registry digest changed and the Dockerfile pin is now stale (Dependabot will catch this on next Monday). Less likely: a transitive dep that was pinned at spec time has been unpinned by an intervening merge. Either way, surface to owner with the details JSON. |

---

## A6 — `[ACCEPTANCE]` Overall Scorecard score lifts from 6.2 to ≥ 7.0

| Field | Value |
|-------|-------|
| **Who** | Owner reading the post-merge verdict |
| **What** | Aggregate Scorecard score reflects the two per-check improvements |
| **Why** | This is the visible public number on the OSSF Scorecard badge embedded in `README.md`. Every visitor to the GitHub repo sees it. If this fails, the badge stays low and visitors get a misleading public signal about supply-chain health. |
| **How** | Same `curl` as A4/A5 (mind the 4-6 hour API refresh lag — see A4 timing notes). Capture top-level `.score` field. Compare to `6.2` baseline. |
| **Expected** | **`.score` is at least 7.0; target around 7.3.** (Math detail for the curious: Scorecard uses risk-weighted averaging with High weight 7.5, Medium 5. Moving Token-Permissions 0→10 contributes ≈ +1.0 and Pinned-Deps 8→10 contributes ≈ +0.1 to the aggregate. So 6.2 + ~1.1 ≈ 7.3.) |
| **If fails** | If A4 and A5 are at 10 but overall is < 7.0, something else regressed (see A9). Re-run the breakdown:<br>`curl -s "https://api.scorecard.dev/projects/github.com/devonartis/agentwrit" \| jq '.checks[] \| {name, score}'`<br>and compare to spec baseline. |

---

## A7 — `[ACCEPTANCE — deferred]` cosign keyless signing still works on the next `v*` tag push

| Field | Value |
|-------|-------|
| **Who** | Release manager pushing the next `v*` tag to `main` after this PR ships |
| **What** | `release.yml` `publish` job completes; image gets signed; `cosign verify` validates against Sigstore transparency log |
| **Why** | We narrowed the permission that lets cosign sign images (the `id-token: write` we moved from top-level to the publish job). If we narrowed it wrong, the next release will fail to sign. Anyone pulling our image and asking "is this really from AgentWrit?" gets no answer. Cannot verify on this PR because `release.yml` only fires on `v*` tags — runs on next release. P3 (actionlint) is the best we can do today. |
| **How** | After PR merges and a future `v*` tag is pushed:<br>1. Watch `release.yml` run via `gh run watch --workflow release.yml`<br>2. After success, verify the signed image:<br>`cosign verify devonartis/agentwrit:v<version> --certificate-identity-regexp='^https://github.com/devonartis/agentwrit/.github/workflows/release.yml@' --certificate-oidc-issuer=https://token.actions.githubusercontent.com` |
| **Expected** | `release.yml` run exits 0. `cosign verify` prints `Verification for ...` with the certificate identity matching the GitHub workflow path. |
| **If fails** | OIDC token request inside the cosign step failed. Most likely cause: the job-level `id-token: write` declaration is missing or misplaced. Inspect the active `release.yml` and confirm `permissions:` block on `publish` job. If correct but still failing, revert that specific permission to top-level in a fix-forward PR. |

---

## A8 — `[ACCEPTANCE]` Helper binary does not leak into the repo (Binary-Artifacts protection)

| Field | Value |
|-------|-------|
| **Who** | Engineer running the helper locally, plus a final post-Task-5 check |
| **What** | The compiled `edsign` binary from `go build ./tests/sec-l2b/edsign` lands ONLY in `/tmp/`, never inside the repo tree |
| **Why** | If `edsign` (or any other compiled binary) gets `git add`-ed and committed, OSSF Scorecard's `Binary-Artifacts` check drops from 10 to 0 — we'd lift two scores while sinking a third, net zero. Worst-case visible regression. |
| **How** | After running P2 / A1, from repo root:<br>`git status --porcelain \| grep -E "^\?\? (tests/sec-l2b/edsign/(edsign\|main)\$\|edsign\$)" && echo "BINARY LEAKED — DO NOT COMMIT" \|\| echo "no leaked binary"`<br>Also confirm `.gitignore` has an explicit entry. Add if missing:<br>`grep -qF "tests/sec-l2b/edsign/edsign" .gitignore \|\| echo "tests/sec-l2b/edsign/edsign" >> .gitignore` |
| **Expected** | Prints `no leaked binary` and `.gitignore` already contains the exclusion line (or this story adds it as part of the Task 5 commit). |
| **If fails** | Remove the binary from `git status`: `rm tests/sec-l2b/edsign/edsign` (or whatever path), then add the `.gitignore` entry. Do NOT `git commit` until `git status --porcelain` only shows expected source files. |

---

## A9 — `[ACCEPTANCE]` Scorecard non-regression: other 10-scored checks stay at 10

| Field | Value |
|-------|-------|
| **Who** | Same operator + same post-merge Scorecard run as A4/A5/A6 |
| **What** | The 8 currently-10-scored checks all remain at 10 |
| **Why** | A6 proves overall ≥ 7.0, but doesn't catch a scenario where Token-Permissions/Pinned-Deps jump while one of the other 8 quietly drops. We could end up with the same overall score for the wrong reasons — masking a real regression. |
| **How** | Same `curl` as A4-A6, dump all check scores:<br>`curl -s "https://api.scorecard.dev/projects/github.com/devonartis/agentwrit" \| jq -r '.checks[] \| select(.score == 10) \| .name' \| sort > /tmp/post-merge-tens.txt`<br>Expected names: `Binary-Artifacts`, `CI-Tests`, `Dangerous-Workflow`, `Dependency-Update-Tool`, `Packaging`, `SAST`, `Security-Policy`, `Vulnerabilities`. Plus the two we lifted: `Token-Permissions` (was 0) and `Pinned-Dependencies` (was 8). License is at 9, expected to stay 9. |
| **Expected** | All 8 baseline-10 checks are present in `/tmp/post-merge-tens.txt`. Token-Permissions and Pinned-Dependencies also present. License = 9 (unchanged). |
| **If fails** | Any 10-baseline check missing from the post-merge `10` set means it regressed. Most likely candidate: `Binary-Artifacts` if A8 was skipped. Run `jq '.checks[] | select(.name == "<X>") | {score, reason, details}'` for the missing name. Surface to owner. |

---

## A10 — `[ACCEPTANCE]` Docker image is reproducible across two builds at the same source commit (digest pin works)

| Field | Value |
|-------|-------|
| **Who** | Engineer verifying the SHA pins are doing their job, after Task 3 commit lands |
| **What** | Two `docker build`s of the same source commit produce images with the same content digest |
| **Why** | This is the actual security property the SHA pinning is supposed to give us. If a registry digest changed (the wrong way) between our pin and today, or our pinned SHA is typo'd, builds could still succeed but with different content — defeating the entire pinning exercise. |
| **How** | From repo root, after Task 3 commit lands:<br>```bash<br># 1. Confirm the embedded SHAs match what the registry currently serves:<br>EMBED_G=$(awk '/^FROM golang:.*@sha256:/{print $2}' Dockerfile)<br>EMBED_A=$(awk '/^FROM alpine:.*@sha256:/{print $2}' Dockerfile)<br>LIVE_G=$(docker buildx imagetools inspect golang:1.24-alpine --format '{{.Manifest.Digest}}')<br>LIVE_A=$(docker buildx imagetools inspect alpine:3.21        --format '{{.Manifest.Digest}}')<br>echo "embedded golang base: $EMBED_G"<br>echo "live     golang base: $LIVE_G"<br>echo "embedded alpine base: $EMBED_A"<br>echo "live     alpine base: $LIVE_A"<br># 2. Build twice with no-cache, compare resulting image digests:<br>docker build --no-cache -t agentwrit:repro-a .<br>docker build --no-cache -t agentwrit:repro-b .<br>DIG_A=$(docker inspect --format '{{index .Id}}' agentwrit:repro-a)<br>DIG_B=$(docker inspect --format '{{index .Id}}' agentwrit:repro-b)<br>echo "build A: $DIG_A"<br>echo "build B: $DIG_B"<br>``` |
| **Expected** | The embedded SHA in `Dockerfile` matches the live registry digest (assuming Dependabot hasn't rolled it yet). Build A and Build B produce **the same image content digest** (the IDs may differ in metadata layers but the layer chain matches). |
| **If fails** | If embedded ≠ live: Dependabot likely needs to rotate — let it. If build A ≠ build B at the same source commit: the Dockerfile has a non-deterministic step (rare, surface to owner). |

---

## Story-to-Task Mapping

| Story | Task(s) that satisfy it |
|-------|--------------------------|
| P1 (uses: SHA-pinned) | Preconditions block (verified at plan-write, re-verify before Task 1) |
| P2 (Go helper builds) | Task 5, Step 2 (smoke test the helper) |
| P3 (actionlint release.yml) | After Task 2 commit lands, before opening PR |
| A1 (sec-l2b live with helper) | Task 5, Step 6 (live integration run) |
| A2 (20 CI gates green) | Task 7, Step 2 (`gh pr checks --watch`) |
| A3 (Python removed) | Task 5, Step 7 (`grep` confirms no python remnants) |
| A4 (Token-Permissions = 10) | Task 7, Step 4 (post-merge `workflow_dispatch` + Scorecard API check) |
| A5 (Pinned-Deps = 10) | Task 7, Step 4 (same Scorecard re-run) |
| A6 (overall ≥ 7.0) | Task 7, Step 4 (same Scorecard re-run, top-level `.score`) |
| A7 (cosign keyless on next tag) | Deferred — runs on next `v*` tag push after this PR merges |
| A8 (no helper binary leak) | Task 5, between Step 2 and Step 9 (.gitignore + `git status` check) |
| A9 (Scorecard non-regression) | Task 7, Step 4 (same Scorecard re-run, full check dump vs baseline) |
| A10 (Docker reproducibility) | After Task 3 commit lands, before final PR open |

Every task in the plan maps to at least one acceptance story. A7 is deferred because `release.yml` only fires on `main` push or `v*` tag.

---

## Acceptable-risk gaps (intentionally not covered)

Documented per council reviewer #3 so future readers see what's NOT in the story set:

1. **govulncheck efficacy on stale pinned SHA.** We pin govulncheck v1.1.4 (current at spec time). If `golang/vuln` ships a new release with detection for a CVE that affects our `internal/` code, our stale pin would miss it until Dependabot bumps. Accepted because (a) Dependabot rotates weekly via `gomod` ecosystem and (b) gosec + CodeQL provide secondary CVE coverage.
2. **SARIF upload verification beyond gate-green.** A2 trusts that if `analyze (go)` exits 0, the SARIF actually landed in GitHub code-scanning. Could query the code-scanning API to triple-check, but `codeql-action/analyze` is well-trusted by the OSS community to fail loudly on upload errors.
3. **Local dev `go` prerequisite drift.** Contributors trying to run `tests/sec-l2b/integration.sh` on a workstation without Go installed will see `go: command not found`. The CHANGELOG entry (Task 6) calls out the helper. README isn't being updated as part of this PR because the same prerequisite already existed for the broker build itself.
4. **Dependabot pip ecosystem cleanup.** Earlier spec versions proposed adding a pip ecosystem to `.github/dependabot.yml`. Owner revision dropped Python entirely, so the pip ecosystem was never added. No stale pip references should exist; P1 grep style verifies by absence. If a future drift introduces one, Dependabot's own validator will catch it on next push.
