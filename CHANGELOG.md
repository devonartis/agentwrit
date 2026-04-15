# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed — Delegation framing aligned with non-strict subset behavior (2026-04-15)

- Comments and docs in 9 places claimed delegation enforces strict narrowing ("strict subset", "only narrow", "narrower-scoped"). The actual `authz.ScopeIsSubset` is a non-strict containment check — equal scopes pass, and same-scope delegation is a deliberate pattern (e.g., fan-out to workers carrying the parent's full authority, verified by SDK acceptance Story 8). Wording corrected across `internal/deleg/deleg_svc.go`, `internal/authz/scope.go`, `README.md`, `docs/security-topology.md`, `docs/architecture.md`, `docs/roles.md`, `docs/common-tasks.md`, `docs/integration-patterns.md`, and the `docs/diagrams/security-topology.svg` callout label. Two source-file docstrings now carry a back-reference to issue #41 explaining why this is not a strict-subset check. Closes #41.

### Added — FAQ from community feedback (2026-04-13)

- New `docs/faq.md` — real questions from practitioners evaluating AgentWrit. Covers identity vs credential exchange, OIDC (enterprise), scope model, scope drift detection (roadmap), SDK status, restart/HA behavior, demo status, and licensing.
- FAQ added to README documentation table.

### Added — Diagram documentation pages + README fix (2026-04-13)

- Token lifecycle and security topology diagrams now have dedicated markdown pages (`docs/token-lifecycle.md`, `docs/security-topology.md`) with context tables and navigation. Architecture doc links updated.
- README: fixed "Key works for everything, forever" — IAM keys don't work like that. Changed to "Key is over-permissioned and long-lived."

### Added — AgentWrit logo (2026-04-13)

- README hero image added — centered logo at `docs/diagrams/agentwrit-logo.png`.

### Fixed — Architecture component mapping + signing key documentation (2026-04-13)

- Pattern component table now maps all 8 EAC v1.3 components with correct numbering. Component 6 (Mutual Authentication) documented as present but not wired. Component 7 (Delegation) and 8 (Observability) added.
- Package count corrected (14→15) and `mutauth` added to directory layout. Clarified distinction: `mutauth` package is agent-to-agent auth (Component 6, planned); server-side mTLS transport (`AA_TLS_MODE=mtls` in `cmd/broker/serve.go`) is separate and working.
- Security Assumptions: removed false claim that "all previously issued tokens become unverifiable after restart (new signing keys)." The signing key is persistent via `internal/keystore` — tokens survive restarts. Single-broker note corrected to describe the actual split-brain risk (in-memory state, not signing keys).

### Fixed — SPDX identifier + gate + middleware diagram (2026-04-13)

- SPDX identifier corrected from `PolyForm-Internal-Use-1.0.0` to `LicenseRef-PolyForm-Internal-Use-1.0.0` across LICENSE and all 77 Go files. PolyForm Internal Use is not on the SPDX License List — `LicenseRef-` prefix is required by spec for unlisted licenses.
- CI SPDX gate now checks the exact expected identifier, not just presence of `SPDX-License-Identifier:`.
- Middleware stack diagram in `docs/architecture.md` corrected to match actual wrapping order in `cmd/broker/main.go`: `RequestID → Logging → MaxBytesBody → SecurityHeaders → mux`.

### Fixed — Architecture doc accuracy (2026-04-13)

- Corrected SQLite version in External Dependencies table (v1.35.0 → v1.46.1).
- Added missing `cobra` CLI dependency to table.
- Removed Prometheus branding from `obs` package descriptions — implementation detail, not a feature.
- Fixed "Open Source" label in token-lifecycle SVG → "Free for internal use".

### Added — SPDX headers on all Go source files (2026-04-13)

- Every `.go` file in `cmd/` and `internal/` (77 files) now carries `// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0` as the first line.
- CI `contamination` gate extended with an SPDX header check — new Go files without the header will fail the gate.

### Changed — License: AGPL-3.0 → PolyForm Internal Use 1.0.0 (2026-04-13)

- **`LICENSE`** — replaced AGPL-3.0 text with PolyForm Internal Use License 1.0.0 (source-available, permanent, no sunset). SPDX identifier: `LicenseRef-PolyForm-Internal-Use-1.0.0`. Dual-license header added for commercial-use contact path.
- **`README.md`** — license badge updated; License section rewritten with the free / paid dual-license split and the `licensing@agentwrit.com` contact line.
- **`Dockerfile`** — `org.opencontainers.image.licenses` OCI label updated from `AGPL-3.0-only` to `LicenseRef-PolyForm-Internal-Use-1.0.0`.
- **`docs/api/openapi.yaml`** — OpenAPI `info.license` block updated to reference PolyForm Internal Use 1.0.0.
- **`docs/getting-started-operator.md`** — OCI labels reference updated to match the new Dockerfile label.
- **What this means for users:**
  - **Internal business use — including by for-profit companies — remains free and unchanged.** Any individual, business, or organization may use and modify AgentWrit for their own internal operations at no cost and without contacting the author. Contractors acting on behalf of a permitted user are covered for the duration of their engagement.
  - **New restriction:** Offering AgentWrit as a hosted or managed service to third parties (paid or free), redistributing original or modified versions, reselling, or embedding AgentWrit in a product you sell now requires a commercial license. Email `licensing@agentwrit.com` with your use case.
  - **SemVer note:** this is a license change, not a behavior change. No wire-format changes. No API changes. Deployed brokers continue to run unchanged. Anyone pinning the prior AGPL-3.0 versions (`v2.0.0` and earlier) should be aware subsequent releases ship under PolyForm Internal Use 1.0.0.

### Added — Architecture diagrams + splash pages (2026-04-13)

- **`docs/diagrams/`** — 3 SVG architecture diagrams (architecture overview, token lifecycle, security topology) replacing the inline mermaid block. Built from code review — only components that exist in the codebase. No HITL, no resource server, no monitoring boxes.
- **`docs/python-sdk.md`** — splash page for the Python SDK (private repo). Shows status, code sample, and links to raw HTTP alternative.
- **`docs/demos.md`** — splash page for MedAssist AI and Support Ticket demos (ship with Python SDK).
- **`README.md`** — added Ephemeral Agent Credentialing v1.3 pattern lineage in "How it works". All private-repo links now point to splash pages instead of 404s.
- **`docs/`** — 5 doc files updated: private `agentwrit-python` links replaced with splash page links.

### Changed — README overhaul for scannability and wayfinding (2026-04-13)

- **`README.md`** — full restructure for newcomers who scan, not read:
  - Quick Start collapsed to one path (Docker Hub), 5 commands, no inline explanations. "Next steps" decision table immediately after.
  - "Why this matters" side-by-side comparison table replaces prose bullet list.
  - "How it works" simplified mermaid flow with numbered steps. Component table links to source packages.
  - API table adds "Who uses it" column so each role finds their endpoints fast.
  - Documentation section reframed as "I want to..." intent-based navigation.
  - Configuration collapsed to one essential table + link to operator guide.
  - TLS/mTLS/nginx config, cosign verification, image tag details moved to docs (not README).
  - Audit trail wording clarified: broker logs credential lifecycle events, not agent activity at the resource server.
  - Net: 169 insertions, 469 deletions — README is now a landing page, not a manual.

### Fixed — stale agentauth brand refs missed in rebrand (2026-04-13)

- **`SECURITY.md`** — contact emails `security@agentauth.dev` → `security@agentwrit.com`; all "AgentAuth" brand references → "AgentWrit".
- **`CODE_OF_CONDUCT.md`** — contact email `conduct@agentauth.dev` → `conduct@agentwrit.com`.
- **`docs/scenarios.md`**, **`docs/agentwrit-explained.md`**, **`docs/README.md`**, **`docs/getting-started-developer.md`**, **`docs/getting-started-user.md`** — 11 Python SDK links still pointing to pre-rename `agentauth-python` → `agentwrit-python`.

### Changed — README rewrite for newcomers + build-in-public banner (2026-04-12)

- **`README.md`** — significant rewrite of the top section for people who are not already familiar with the product:
  - Added a GitHub `[!IMPORTANT]` **build-in-public status banner** right under the badges: broker is stable, SDK and demo still landing, pin to `v<semver>` or `main-<sha>` for anything non-lab, issues welcome / external PRs paused.
  - Replaced the jargon-heavy hero paragraph with a plain-English "What is AgentWrit?" section that leads with the writ metaphor — narrow authority, time-limited, revocable at the source.
  - Added a new **"The problem AgentWrit solves"** section that frames the pain (long-lived API keys + prompt-injected agents = full blast radius) before introducing the solution.
  - Rewrote the **Quick Start** as a 5-minute zero-to-first-agent-token walkthrough: Docker Hub pull → admin auth → launch token → Python SDK registration. Every step explains what the command does and why you're running it.
  - Fixed 5 remaining prose references to `AgentAuth` missed in the earlier brand sweep.
  - Updated Python SDK links from `github.com/devonartis/agentauth-python` → `github.com/devonartis/agentwrit-python` after the sister repo rename.

### Changed — CONTRIBUTING.md policy rewrite (Decision 014, 2026-04-12)

- **`CONTRIBUTING.md`** — restructured to match Decision 014 (build-in-public, external PRs paused):
  - New top section "Contribution Policy (READ FIRST)" with a table of what's accepted (issues, security reports, feature requests) and what's not (external PRs of any kind). Explains why PRs are paused and lists 5-item exit criteria for reopening.
  - Deleted the "Pull Request Process" section and PR checklist (misleading given the current policy).
  - New "Filing a good Issue" section with concrete templates for bug reports and feature requests.
  - New "If you're reading this to understand the code" section — preserves dev setup / code style / testing sections as valuable for readers who want to trace behavior without submitting code.
  - Fixed `agentauth/` project tree root → `agentwrit/` and remaining prose brand references.

### Added — Docker Hub image publishing + cosign signing (TD-CI-002, 2026-04-12)

- **`.github/workflows/release.yml`** — new workflow that publishes multi-arch (`linux/amd64` + `linux/arm64`) broker images to Docker Hub on every push to `main` and on `v*` release tags. Tags produced: `latest` (tracks main), `main-<sha>` (per-commit traceability), `v<major>.<minor>.<patch>` / `v<major>.<minor>` / `v<major>` (semver releases). Builds with buildx + QEMU emulation on the hosted x86 runner; cached via `type=gha` so repeat builds are fast.
- **Cosign keyless signing** — every published image is signed with Sigstore via GitHub Actions OIDC. No long-lived signing key to rotate. Verification command published in the README and `docs/getting-started-operator.md`.
- **SLSA provenance + SBOM attestation** — `docker/build-push-action` emits provenance (`mode=max`) and SBOM alongside the image.
- **`Dockerfile`** — added OCI image labels (`org.opencontainers.image.title/description/vendor/licenses/source/url/documentation`) as bake-time defaults, plus `-trimpath` on the Go build for reproducibility. The release workflow overrides `revision`, `version`, and `created` labels at publish time via `docker/metadata-action`.
- **`README.md`** — new "Option A: Pre-built image from Docker Hub" section in Quick Start, now the recommended onboarding path ahead of source builds. Includes signature verification command.
- **`docs/getting-started-operator.md`** — restructured "Quick Start" into three deployment options (pre-built image / Docker Compose local build / native binary). Pre-built image is the new default with full pull/run/verify walkthrough and OCI-image inventory.
- **`docker-compose.yml`** — added commented-out `image: devonartis/agentwrit:latest` as an alternative to `build: .` so operators can swap without editing the compose file structurally. Also fixed stale `AA_DB_PATH` default (`agentauth.db` → `data.db`) missed in the earlier brand sweep.
- **`TECH-DEBT.md`** — TD-CI-002 marked RESOLVED with link to this branch.

**Required manual setup before first release:**
1. Docker Hub repo `devonartis/agentwrit` must exist (public)
2. Docker Hub PAT with read/write scope to `agentwrit` only
3. GitHub repo secrets: `DOCKERHUB_USERNAME` + `DOCKERHUB_TOKEN`

### Added — `main-hygiene` CI gate (2026-04-12)

- **`.github/workflows/ci.yml`** — new `main-hygiene` job greps for strip-target paths on pushes to `main`. Runs only when `github.ref == 'refs/heads/main'` so PR runs against `develop` are untouched (develop legitimately carries these files). Fails the `gates-passed` aggregator if any dev-only file slips onto `main` via a missed `strip_for_main.sh` invocation — an automated safety net on top of the existing manual strip script.
- **`scripts/gates.sh`** — mirror of the new gate in `GATES_FULL` so `scripts/test-gate-parity.sh` passes. Local invocation skips with `skip_gate` unless the current branch is `main`, since a developer running `gates.sh full` on `develop` would otherwise get a spurious failure.
- **`TECH-DEBT.md`** — TD-CI-003 (automated develop→main PR workflow) added as follow-on work.

### Fixed — Runtime rebrand alignment (2026-04-12)

- **`internal/problemdetails/problemdetails.go`** — RFC 7807 problem `type` URNs now use `urn:agentwrit:error:{errType}` instead of the former `urn:agentauth:error:{errType}` namespace, matching the published API docs.
- **`internal/obs/obs.go`** — Prometheus metric names now use the `agentwrit_` namespace. Updated the metrics test and troubleshooting/common-tasks docs so runtime output and docs agree.
- **`cmd/awrit/init_cmd.go`** — `awrit init` now writes to the same default paths the broker auto-loads (`~/.broker/config`, fallback `/etc/broker/config`). Added a unit test for the default user config path.
- **`docs/common-tasks.md`** — JWT `iss` claim text now matches the actual `AA_ISSUER` behavior, and the alert table no longer lists non-existent broker/app-auth/expired-token metrics.

### Fixed — Docs audit P1/P2 corrections (2026-04-12)

- **`docs/getting-started-user.md`** — admin auth examples used the literal `"my-secret-key-change-in-production"` while the guide starts the broker with a randomly generated `$AA_ADMIN_SECRET`. Examples now reference `$AA_ADMIN_SECRET` so the first-run path works without 401s.
- **`docs/awrit-reference.md`** — `awrit init` sample output showed `~/.agentwrit/config`; corrected to `~/.broker/config`, matching the broker's read path and the CLI default after TD-CLI-002.
- **`docs/api.md`** — JWT claims table corrected: `iss` is driven by `AA_ISSUER` (empty by default, issuer validation skipped); app token subject is `app:{internal_app_id}` not `app:{client_id}`; `aud` is driven by `AA_AUDIENCE` (omitted if unset, audience validation skipped).
- **`docs/getting-started-operator.md`** — `AA_AUDIENCE` default corrected from `"agentwrit"` to *(empty)*. SQLite persistence note corrected: setting `AA_DB_PATH=""` does not enable memory-only mode — unset uses the `./data.db` default.
- **`docs/api/openapi.yaml`** — `info.license` corrected from `Apache-2.0` to `AGPL-3.0` with the correct license URL. Matches `LICENSE`, `README.md`, and `CLA.md`.
- **`docker-compose.yml`** — Docker bridge network renamed `agentauth-net` → `agentwrit-net` to match brand sweep and operator docs.
- **`docs/README.md`** — API reference entry corrected from "22 HTTP endpoints" to "19". Concepts entry corrected from "seven components" to "eight".
- **`docs/concepts.md`** — intro sentence corrected from "seven components" to "eight".
- **`TECH-DEBT.md`** — added TD-CLI-002 (HIGH: `awrit init` writes to `~/.agentauth/config`, broker reads `~/.broker/config` — broken first-run path introduced in commit `4e197a5`) and TD-CLI-003 (Low: docker-compose.yml network name lag). Bug report at `.plans/bugs/BUG-CLI-002-awrit-init-config-path.md`; TD-CLI-002 is resolved by the runtime rebrand alignment fix above.

### Renamed CLI binary `aactl` → `awrit` (TD-CLI-001)

- **`cmd/aactl/` → `cmd/awrit/`** — directory renamed. Cobra command name changed (`Use: "aactl"` → `Use: "awrit"`). All internal CLI output, help text, and error messages updated.
- **`docs/aactl-reference.md` → `docs/awrit-reference.md`** — reference doc renamed. All example commands in the doc rewritten to use `awrit`.
- **Docs, scripts, tests, README, CONTRIBUTING, docker-compose.yml, .github/workflows/ci.yml, .gitignore** — every `aactl` reference in ship-to-main files rewritten to `awrit`. Evidence files under `tests/*/evidence/*.md` intentionally preserved as-is because they are historical records of past test runs (rewriting history would misrepresent what happened at the time).
- **`cmd/broker/main.go`** — error message `"Run 'aactl init'..."` → `"Run 'awrit init'..."`.
- **`.gitignore`** — both `/awrit` and `/aactl` listed so accidentally-built binaries under either name stay untracked during the transition.
- **`internal/cfg/configfile.go`** — user-visible references in the env var comment block updated to `awrit`.

Scope: ~36 files touched plus directory + file renames. No production logic changes — pure mechanical rename. The `github.com/devonartis/agentauth` Go module path is NOT changed (that's gated on the GitHub repo rename, separate work).

### Promoted `adminTTL` const to configurable `cfg.AdminTokenTTL` (TD-010)

- **`internal/admin/admin_svc.go`** — deleted the magic-number const `adminTTL = 300`. Admin JWT TTL is now driven by `cfg.AdminTokenTTL` (seconds), wired through a new `tokenTTL` parameter on `NewAdminSvc`. Operators tune via `AA_ADMIN_TOKEN_TTL` (default 300 / 5 min).
- **`internal/cfg/cfg.go`** — added `AdminTokenTTL int` field and a named const `defaultAdminTokenTTL = 300` (seconds; matches existing int-seconds convention for DefaultTTL, MaxTTL, AppTokenTTL so the cfg package stays internally consistent). Env var `AA_ADMIN_TOKEN_TTL` added to the inline doc comment.
- **`cmd/broker/main.go`** — `NewAdminSvc` wiring updated to pass `c.AdminTokenTTL`.
- **Tests** — `newTestAdminSvc` helpers and direct `NewAdminSvc` calls across `admin_svc_test.go`, `admin_hdl_test.go`, `app_hdl_test.go`, `handler/handler_test.go` now pass an explicit `testAdminTokenTTL = 300` fixture. Assertions that checked `resp.ExpiresIn != adminTTL` now check against the fixture value — the test drives cfg-to-claim TTL flow end-to-end inside the admin package, which is the unit-level equivalent of a config-matrix behavioral test for this field.
- **Rationale for int seconds (not `time.Duration`)** — the existing TTL fields (`DefaultTTL`, `MaxTTL`, `AppTokenTTL`) all use int seconds. Adding one `time.Duration` field would create two conventions in the same cfg package and leak into every caller that passes the field through. A future cleanup can migrate all TTL fields to `time.Duration` together (proposed TD-CFG-003) — but mixing conventions in this PR would be worse than preserving the existing one.

### Removed hardcoded identity literals from cfg + token packages (TD-TOKEN-001, TD-TOKEN-002, TD-CFG-001, TD-CFG-002)

- **`internal/token/tkn_svc.go`** — JWT `iss` claim is now driven by `cfg.Issuer` instead of the hardcoded literal `"agentauth"`. Issuer enforcement moved from `TknClaims.Validate()` (pure structural check) into `TknSvc.Verify()` where config is available. Empty `cfg.Issuer` skips the issuer check (mirrors the Audience contract — operator opt-in).
- **`internal/cfg/cfg.go`** — added `Issuer string` field, env var `AA_ISSUER`. No default; empty value means "skip issuer enforcement at verify time," matching the documented Audience pattern.
- **`internal/cfg/cfg.go`** — `TrustDomain` default literal `"agentauth.local"` → `"agentwrit.local"` (no longer leaks the prior brand into source).
- **`internal/cfg/cfg.go`** — `DBPath` default literal `"./agentauth.db"` → `"./data.db"` (neutral, no brand in source).
- **`internal/cfg/cfg.go`** — `Audience` default override at line 96 deleted. The `cfg.go:22` doc comment said `"empty = skip"` but the code overrode unset → `"agentauth"`. Now `Audience` honors its documented contract: unset OR explicitly empty both skip audience validation. No brand-coupled default.
- **`internal/cfg/configfile.go`** — config search paths `/etc/agentauth/config` → `/etc/broker/config` and `~/.agentauth/config` → `~/.broker/config`. Filesystem layout no longer encodes the brand. Header comment in generated config files updated from `# AgentAuth Configuration` → `# Broker Configuration`.
- **`internal/token/tkn_claims.go`** — package doc comment updated to reflect that `iss` is operator-configured via `cfg.Issuer`, not "always 'agentauth'". `Validate()` is now a pure structural check (sub, jti, exp, nbf) — issuer enforcement is the service layer's job.
- **Test surface** — test fixtures across `cfg/`, `token/`, `authz/`, `deleg/`, `admin/`, `identity/`, `mutauth/` updated to use brand-neutral test values (`test-issuer`, `test.local`, `spiffe://test.local/...`) instead of leaked `"agentauth"` and `agentauth.local` literals. Tests now drive issuer/audience expectations from fixture cfg, not hardcoded constants.
- **Root cause:** `IssuerURL` was an OIDC-coupled config field stripped during the open-core split. The strip removed the field, the validation, AND the tests (tombstone preserved at `internal/token/tkn_svc_test.go:521`), but the validation was replaced with a hardcoded literal `"agentauth"` rather than left as configurable. The general JWT `iss` claim is independent of OIDC and core still needs it — this PR restores configurability without re-coupling to OIDC. Full audit at `.plans/reviews/2026-04-10-hardcoded-identity-audit.md`.
- **Standing rule added:** `~/.claude/CLAUDE.md` now contains "No Hardcoded Identity Values — Universal, Non-Negotiable" as a global rule. Identity-shaped string literals in source code (brand names, issuers, trust domains, search paths) are non-negotiable findings going forward.

### Added — M-sec README badges (Task 30)

- **`README.md`** — added three CI-health badges ahead of the existing
  language/license/tech row:
  - **CI** — `ci.yml` workflow status on `main`
  - **CodeQL** — `codeql.yml` SAST status on `main`
  - **OpenSSF Scorecard** — supply-chain posture score
  Badges will show as "not found" or broken while the repo is private
  (CI badge requires viewer auth; CodeQL and Scorecard require public
  repo access). They're added now so the moment the repo flips public
  they light up without a README update — fire-and-forget. CodeQL
  and Scorecard will ALSO need their workflow triggers re-enabled
  per TD-VUL-006 fix sequence. A comment in the README notes this.

### Fixed — `.vscode/` removed from tree and gitignored

- **`.vscode/settings.json`** — was tracked on develop but carries
  per-user editor settings (e.g. Snyk IDE prefs). Untracked via
  `git rm` and added to `.gitignore` so it stays out of every branch.
  This closes the loop on the leak that happened during the first
  develop → main strip merge attempt: VSCode recreated the file between
  `rm -rf` and `git commit`, so it landed in the merge commit. The
  commit was amended to remove it (see `a72a959`), but the root cause
  was that the file was tracked on develop at all. Now both the strip
  script and .gitignore cooperate to keep it out.

### Fixed — strip_for_main.sh mid-merge support + two drift fixes

- **`scripts/strip_for_main.sh`** — the documented `git merge develop
  --no-commit` → strip flow could never actually work because the
  script's dirty-tree guard refused to run mid-merge. Added merge-state
  detection (`$GIT_DIR/MERGE_HEAD` presence); when mid-merge the strip
  uses `git rm -rf --ignore-unmatch` so modify/delete conflicts get
  deleted AND staged as resolved in one step. The "absolute refusal to
  run on develop" guard is preserved regardless of merge state.
- **`scripts/strip_for_main.sh` + `.githooks/pre-commit`** — added
  `.vscode/` (editor settings, often carry per-user Snyk / IDE prefs)
  to both strip lists, and `adr/` to the pre-commit FORBIDDEN list
  (it was already in the strip script). The two defense layers now
  agree. A note in pre-commit tells future editors to keep both lists
  in sync.

### Added — CI/build/gates (M-sec v1)

- **`.gosec.yml`** — explicit gosec configuration with documented rule
  exclusions (G117, G304, G101) rationalized for a credential broker's API
  surface. Every excluded rule carries a reviewer-auditable rationale.
- **`.golangci.yml`** — security-aware `golangci-lint` config (errcheck,
  gosec, govet, ineffassign, staticcheck, unused, gosimple, bodyclose,
  misspell, gofmt, goimports) with tuned govet subchecks (fieldalignment
  and shadow disabled with rationale) and mirrored gosec excludes.
- **`scripts/smoke/core-contract.sh`** — L2.5 core contract smoke test.
  10-step verification (health, admin auth, launch token, challenge,
  Ed25519 challenge-response register, JWT structure, validate-accepted,
  revoke, validate-rejected, out-of-scope denied) against a running
  broker. Uses `python3 + cryptography` for the Ed25519 signing step.
- **`scripts/test-gate-parity.sh`** — enforces gate list alignment
  between `scripts/gates.sh --list-gates` and `.github/workflows/ci.yml`
  `GATE_LIST_START/END` block. Prevents silent drift.
- **`syft scan` baseline** — SBOM generation integrated into the local
  `gates.sh full` pipeline (SPDX-2.3, 27 packages at baseline).

### Changed — CI/build/gates (M-sec v1)

- **`scripts/gates.sh`** — extended from 4 gates to 13. New blocking
  gates: `contamination` (enterprise refs grep), `govulncheck` (stdlib
  and dependency vulnerabilities), `go-mod-verify` (module integrity +
  tidy drift), `vet`, `format`, plus `full`-mode-only: `unit-tests-race`,
  `docker-build`, `smoke-l2.5`, `sbom`. `gosec` flipped from warn-only
  to blocking. `module` renamed to `full` (deprecated alias retained).
  Dead references to `live_test.sh`/`live_test_docker.sh` removed.
  `golangci-lint` and `gosec` are now required (no fallback). Added
  `--list-gates` for parity enforcement. Honors `BROKER_URL` for
  smoke-l2.5 on non-default ports.
- **`TECH-DEBT.md`** — recorded TD-VUL-001..004 (four Go stdlib CVEs
  fixed by bumping `go.mod` toolchain from `go1.25.7` to `go1.25.9`,
  scheduled for landing at the first CI push).

### Fixed — CI/build/gates (M-sec v1)

- **gofmt drift** — 24 pre-existing gofmt-dirty files normalized in a
  single style commit. No behavior change. Surfaced by adding `format`
  as a blocking gate.
- **`internal/keystore/parseKey`** — defensive type-assertion on
  `priv.Public().(ed25519.PublicKey)` to satisfy `errcheck
  check-type-assertions`. Unreachable on the happy path.
- **`internal/mutauth/heartbeat.sweep`** — heartbeat auto-revoke
  failures are now logged via `obs.Warn` instead of being silently
  dropped. Previously `_, _ = h.revSvc.Revoke(...)` was followed by an
  unconditional "agent auto-revoked" log line, even when the revocation
  actually failed.
- **`cmd/aactl/client`** — `json.Marshal` and `io.ReadAll` errors are
  now propagated as wrapped errors instead of being discarded. Affects
  `authenticate()` (two sites) and `doPostWithToken()`.
- **`internal/store/sql_store.QueryAuditEvents`** — documented `#nosec
  G202` on the audit query SELECT, explaining why the fragment
  concatenation is safe (fixed template, parameterized values).
- **`internal/admin/admin_svc_test.TestLaunchTokenRecord_SpecCompliance`** —
  clarified the exhaustive-literal intent in a doc comment and silenced
  `govet unusedwrite` with `_ = rec`.

### Added

**Security hardening**

- **Token TTL enforcement** — `AA_MAX_TTL` configuration sets the maximum token lifetime ceiling (default 86400s, set to 0 to disable). The broker clamps any requested TTL to this ceiling.
- **TTL carry-forward on renewal** — Renewed tokens preserve the original token's TTL instead of falling back to the default. Closes a privilege escalation path where a short-lived token could be renewed to the broker default.
- **JWT algorithm validation** — The broker rejects tokens with `alg != EdDSA`, preventing the `alg:none` and HS256/RS256 algorithm confusion attacks.
- **JWT key ID validation** — The broker rejects tokens with a mismatched `kid`, preventing cross-broker token replay.
- **Revocation check in Verify()** — Every token verification path checks the revocation list. Defense in depth.
- **Transactional renewal** — Predecessor token is revoked before the new token is issued. If revocation fails, renewal fails.
- **Startup warning when DefaultTTL > MaxTTL** — Surfaces silent clamping at startup.
- **Token expiry required** — Tokens with `exp=0` or missing `exp` are rejected.

**HTTP hardening**

- **SecurityHeaders middleware** — All responses carry `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `X-Frame-Options: DENY`. HSTS added on TLS/mTLS deployments.
- **Request body size limit** — Global 1MB limit on all endpoints, enforced by eager buffering so streaming decoders can't bypass it. Returns 413 on oversized bodies.
- **Error sanitization** — Token validation, renewal, and auth middleware errors return generic messages to the client. Full errors are recorded in the audit trail with a correlation `request_id`.
- **Bind address safety** — Broker defaults to `127.0.0.1`; warns at startup when binding to `0.0.0.0` without TLS.
- **HTTP server timeouts** — Read, write, and idle timeouts prevent slowloris-style attacks.
- **TLS 1.2 minimum + AEAD-only ciphers** — Enforced when TLS is enabled.
- **Weak secret denylist** — The broker refuses to start with a known-weak admin secret (empty, `change-me-in-production`, etc.). Use `aactl init` or generate a strong value.

**Operator tooling**

- **`aactl init` command** — Generates a secure admin secret and config file in dev or prod mode. Atomic file creation with `O_EXCL`, rejects symlinks, enforces 0600 file / 0700 directory permissions.
- **Config file support** — KEY=VALUE format at `AA_CONFIG_PATH` > `/etc/agentauth/config` > `~/.agentauth/config`. Rejects insecure permissions like SSH/GPG does.
- **Bcrypt admin authentication** — Admin secret stored as a bcrypt hash; plaintext only shown once at init. Dev mode supports plaintext config for convenience, bcrypt is derived at startup.
- **`gates.sh` developer tool** — Build + lint + unit tests + gosec in one command (`./scripts/gates.sh task`). Module mode adds full tests and Docker E2E. Regression mode runs all phase regression suites.

**App credential lifecycle**

- **`POST /v1/app/launch-tokens`** — App-facing endpoint for creating launch tokens within the app's scope ceiling. Scope ceiling enforcement prevents apps from escalating beyond what the operator granted at registration.
- **`POST /v1/admin/launch-tokens`** — Admin-facing endpoint for bootstrapping and break-glass scenarios. No ceiling enforcement (admin is the root of trust).
- **App scope ceiling** — Operators set a scope ceiling when registering an app; the broker enforces it on every `POST /v1/app/launch-tokens` call.
- **App traceability** — `app_id`, `app_name`, and `original_principal` claims flow through launch tokens into agent JWTs, preserved through delegation.

**Production foundations**

- **Persistent signing key** — Ed25519 signing key loaded from disk at startup (`AA_SIGNING_KEY_PATH`), generated with 0600 permissions on first start. Agent tokens survive broker restart.
- **Graceful shutdown** — SIGINT/SIGTERM triggers clean shutdown: HTTP server drains, SQLite closed.
- **Corrupt key fails fast** — Broker refuses to start with a malformed signing key, surfacing the problem at deploy time.
- **Token predecessor revocation on renewal** — Prevents two valid tokens existing for the same agent.
- **JTI blocklist pruning** — Background goroutine removes expired revocation entries so memory doesn't grow unbounded.
- **Agent record expiry** — Agent records marked expired when their token TTL elapses.

**Audit and observability**

- **Structured audit fields** — Audit events carry `resource`, `outcome`, `deleg_depth`, `deleg_chain_hash`, and `bytes_transferred` via a backward-compatible options pattern. Hash chain tamper evidence covers all structured fields.
- **Outcome filtering** — Query the audit trail by outcome via `--outcome` on `aactl audit events` or `?outcome=` on `GET /v1/audit/events`.
- **Enforcement event coverage** — Audit events emitted for every denial path: missing auth, invalid scheme, token verification failure, revoked token access, scope violations, delegation attenuation violations, scope ceiling exceeded.
- **New Prometheus metrics** — `agentauth_audit_events_total`, `agentauth_audit_write_duration_seconds`, `agentauth_db_errors_total`, `agentauth_audit_events_loaded`, `agentauth_admin_auth_total`.

**Persistence**

- **SQLite audit persistence** — Audit events persist to SQLite via `modernc.org/sqlite` (pure Go, no CGo). Hash chain is rebuilt from disk on startup. Configurable via `AA_DB_PATH` (default `./agentauth.db`).
- **Revocation persistence** — Revocations stored in SQLite so tokens stay revoked across broker restarts.

### Changed

- **Documentation accuracy** — Corrected public documentation to match the current broker contract for agent registration, renewal, release, app authentication, launch token creation, and health responses. Fixed copy/paste examples with stale payload shapes and outdated event names.
- **Direct HTTP integration** — Go developers get explicit pre-SDK guidance with end-to-end examples for registration, renewal, and release in `docs/getting-started-developer.md`.
- **Authorization middleware** — `WithRequiredScope()` standalone function replaced by `ValMw.RequireScope()` method. Scope checking now emits `scope_violation` audit events on denial.

---

## [2.0.0] — 2026-02-09

Complete rewrite implementing the Ephemeral Agent Credentialing security pattern.

### Added

**Identity and authentication**

- Challenge-response agent registration with Ed25519 cryptographic verification
- SPIFFE-format agent IDs (`spiffe://{domain}/agent/{orch}/{task}/{instance}`)
- EdDSA-signed JWT tokens with configurable TTL (default 5 minutes)
- Token verification endpoint returning decoded claims
- Token renewal with fresh timestamps and new JTI

**Authorization**

- `ValMw` middleware enforcing Bearer token + scope on every request
- Scope format `action:resource:identifier` with wildcard support

**Revocation**

- 4-level token revocation (token/JTI, agent/SPIFFE ID, task, delegation chain)

**Audit**

- Hash-chain tamper-evident audit trail with SHA-256 linking
- Automatic PII sanitization (secrets, passwords, private keys, token values)
- 12 event types covering admin auth, registration, token lifecycle, delegation, resource access
- Query endpoint with filtering (agent, task, event type, time range) and pagination

**Delegation**

- Scope-attenuated token delegation with chain verification
- Maximum delegation depth of 5 hops
- Cryptographic delegation chain embedded in token claims

**Admin**

- Admin authentication via shared secret with constant-time comparison
- Launch token creation with policy (allowed scope, max TTL, single-use flag)
- Admin bootstrap flow for initial system setup

**Observability**

- Prometheus metrics (registrations, revocations, active agents)
- Structured logging via `obs` package
- RFC 7807 `application/problem+json` error responses on all endpoints
- Health check endpoint reporting status, version, and uptime
- Prometheus exposition format at `/v1/metrics`

**Configuration**

- `AA_*` environment variable configuration with sensible defaults
