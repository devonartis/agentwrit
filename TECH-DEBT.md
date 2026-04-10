# TECH-DEBT.md

Active tech debt. Append new entries as debt is taken. Never remove — mark as RESOLVED with date.

Full details for each item live in the referenced file. This is the index.

---

## Public release / licensing

| ID | What | Severity | When to Fix | Reference |
|----|------|----------|-------------|-----------|
| TD-PUB-001 | ~~Apache 2.0 insufficient for SaaS/resale restrictions~~ | ~~Medium~~ | **RESOLVED 2026-04-08** — Switched to AGPL-3.0 + CLA + enterprise license summary | `LICENSE`, `CLA.md`, `ENTERPRISE_LICENSE.md` |
| TD-019 | **Domain name decision needed** — all legal docs use placeholder `devon@agentauthdev._`. Also affects `security@agentauth.dev` (SECURITY.md), `conduct@agentauth.dev` (CODE_OF_CONDUCT.md). Pick a domain, register it, update all contact emails. | **HIGH** | Before going public | `CLA.md`, `ENTERPRISE_LICENSE.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md` |

## Carried Forward (from agentauth-internal)

| ID | What | Severity | When to Fix | Reference |
|----|------|----------|-------------|-----------|
| TD-001 | `app_rate_limited` audit event not emitted — rate limiter fires before handler audit call | Low | Future | `internal/admin/admin_hdl.go` |
| TD-007 | Resilient logging — audit writes inline, no fallback on store failure | Medium | Future | `internal/audit/audit_log.go` |
| TD-008 | Token predecessor not invalidated on renewal — two valid tokens exist | Medium | B1 (P0) may fix | `internal/token/tkn_svc.go` |
| TD-009 | JTI blocklist never pruned — memory grows indefinitely | Medium | B1 (P0) may fix | `internal/store/sql_store.go` |
| TD-010 | Admin JWT TTL hardcoded (`const adminTTL = 300`) — should be operator-configurable via `AA_ADMIN_TOKEN_TTL` | Low | Future | `internal/admin/admin_svc.go` |

## New — Documentation Drift (B0 Sidecar Removal)

B0 removed all sidecar Go code and infrastructure but did NOT rewrite the user-facing docs. These files still reference the sidecar architecture, `cmd/sidecar`, `docker-compose.uds.yml`, and token exchange flows that no longer exist.

| ID | What | Severity | Files Affected | Notes |
|----|------|----------|----------------|-------|
| TD-D01 | `docs/sidecar-deployment.md` — entire file is about sidecar deployment | High | `docs/sidecar-deployment.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D02 | `docs/getting-started-user.md` — sidecar path, port 8081, `go run ./cmd/sidecar`, `docker-compose.uds.yml` | High | `docs/getting-started-user.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D03 | `docs/getting-started-developer.md` — sidecar SDK integration, token exchange flow | High | `docs/getting-started-developer.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D04 | `docs/getting-started-operator.md` — sidecar configuration, env vars, deployment topology | High | `docs/getting-started-operator.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D05 | `docs/architecture.md` — sidecar in architecture diagrams, `docker-compose.uds.yml`, token exchange | Medium | `docs/architecture.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D06 | `docs/api.md` — token exchange endpoint documentation | Medium | `docs/api.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D07 | `docs/api/openapi.yaml` — token exchange route in OpenAPI spec | Medium | `docs/api/openapi.yaml` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D08 | `docs/concepts.md` — sidecar in conceptual model | Medium | `docs/concepts.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D09 | `docs/troubleshooting.md` — sidecar troubleshooting section, UDS refs | Medium | `docs/troubleshooting.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D10 | `docs/common-tasks.md` — sidecar operations tasks | Low | `docs/common-tasks.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D11 | `docs/integration-patterns.md` — sidecar integration pattern | Low | `docs/integration-patterns.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D12 | `docs/examples/*.md` — 4 example docs reference sidecar flows | Low | `docs/examples/customer-support.md`, `data-pipeline.md`, `devops-automation.md`, `code-generation.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D13 | `docs/examples/README.md` — sidecar in examples overview | Low | `docs/examples/README.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D14 | `docs/aactl-reference.md` — sidecar aactl commands if any | Low | `docs/aactl-reference.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D15 | `docs/RECOMMENDATIONS.md` — sidecar recommendations | Low | `docs/RECOMMENDATIONS.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D16 | `README.md` — sidecar in project overview | Medium | `README.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D17 | `CHANGELOG.md` — historical sidecar entries (leave as-is, they're history) | None | `CHANGELOG.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D18 | `KNOWN-ISSUES.md` — sidecar-related known issues | Low | `KNOWN-ISSUES.md` | RESOLVED — fix/docs-overhaul branch (March 2026 documentation overhaul) |
| TD-D19 | Post-docs-overhaul sidecar grep verification | Low | All tracked files | New entry post-merge: run final grep for remaining sidecar/UDS/token-exchange references to catch any stragglers missed during overhaul |

## New — Script Drift (B0 Sidecar Removal)

| ID | What | Severity | Files Affected | Notes |
|----|------|----------|----------------|-------|
| TD-S01 | `scripts/live_test.sh` — sidecar test flows, `cmd/smoketest` reference | High | `scripts/live_test.sh` | Verify against agentauth's clean version post-cherry-pick |
| TD-S02 | `scripts/verify_compose.sh` — checks for sidecar service in compose | Medium | `scripts/verify_compose.sh` | Update to check broker-only |
| TD-S03 | `scripts/live_test_docker.sh` — sidecar Docker test flows (`docker compose ... build --no-cache broker sidecar`) | High | `scripts/live_test_docker.sh` | **Decision needed:** delete (test_batch.sh replaces it) or rewrite for broker-only. Hardcodes sidecar in build+up commands. |
| TD-S04 | Raw `docker compose` vs stack scripts — inconsistent Docker lifecycle | Medium | `scripts/test_batch.sh`, `scripts/live_test_docker.sh` | Standard: use `stack_up.sh` / `stack_down.sh` for Docker lifecycle. Raw `docker compose` only for `docker compose build` (no stack script for build-only). See cfg.go for env var flow. |
| TD-S05 | G6 smoke test payloads don't match current API contract | Medium | `scripts/test_batch.sh` | Launch token (missing `agent_name`), register, validate, renew curls need correct field names and required fields. 3/7 pass (health, admin auth, audit). Unit tests (G2) cover endpoint behavior. Fix after B0 merge. |
| TD-S06 | Rate limiting on admin auth endpoint (bcrypt brute force) | Medium | `internal/admin/admin_hdl.go` | Source: B2 security review finding I-5. Bcrypt is slow by design but without rate limiting an attacker can still attempt brute force. Add token bucket or sliding window rate limiter to POST /v1/admin/auth. Phase: B3 (SEC-L1) or later. |
| TD-S07 | Post-migration documentation refresh | Low | `docs/`, `README.md`, `MEMORY.md` | Source: B2 review. Docs reference old AA_ADMIN_SECRET direct flow, no mention of aactl init or config files. Update all docs/diagrams/README after B6 to reflect new architecture. Phase: Post-B6. |
| TD-S08 | `docs/api.md` + `docs/getting-started-operator.md` — wrong auth field names + OIDC refs | **CRITICAL** | `docs/api.md` (lines 52, 248, 255), `docs/getting-started-operator.md` (lines 467, 489, 604) | 5 instances of `client_id`/`client_secret` — API now uses `{"secret":"..."}`. Code has `legacyAuthReq` that returns migration error. Also OIDC/JWKS endpoint docs that don't exist in core. |
| TD-S09 | `README.md` + operator docs — `change-me-in-production` shown as valid example | **CRITICAL** | `README.md` (line 185), `docs/getting-started-operator.md` (line 76), `docs/common-tasks.md` (line 1145), `scripts/stack_up.sh` (line 9) | 6 instances. This secret is now **rejected at startup** by the B3 weak secret denylist. Examples will cause broker FATAL on first try. Must use `aactl init` or `live-test-secret-32bytes-long-ok`. |
| TD-S10 | `.claude/skills/broker-up/SKILL.md` — wrong field names + old secrets | Medium | `.claude/skills/broker-up/SKILL.md` (lines 205, 256, 321) | Shows `{"client_id":"admin","client_secret":"..."}` (wrong), `test-secret-minimum-32-characters-long` (rejected). Should use `{"secret":"..."}` and `live-test-secret-32bytes-long-ok`. |
| TD-S11 | `docker-compose.mtls.yml` / `docker-compose.tls.yml` — VERIFIED CLEAN | Low | `docker-compose.mtls.yml`, `docker-compose.tls.yml` | Audit confirmed: no sidecar/OIDC refs. These are clean. Can close after verification. |
| TD-S12 | `scripts/gen_test_certs.sh` — generates sidecar client certs | Medium | `scripts/gen_test_certs.sh` | Still generates certs for sidecar mTLS. Remove sidecar cert generation, keep broker certs only. |
| TD-S13 | `scripts/verify_compose.sh` / `scripts/gates.sh` — stale sidecar references | Medium | `scripts/verify_compose.sh`, `scripts/gates.sh` | verify_compose checks for sidecar service, gates.sh references live_test_sidecar.sh. Both need updating. |
| TD-S14 | `docs/api/openapi.yaml` — 51 sidecar endpoint references | High | `docs/api/openapi.yaml` | OpenAPI spec still has all sidecar endpoints. Needs full rewrite to match core's broker-only API. |
| TD-S15 | `.plans/cherry-pick/TESTING.md` — old secret `test-secret-minimum-32-characters-long` | Low | `.plans/cherry-pick/TESTING.md` (line 101) | Rejected at startup. Update to `live-test-secret-32bytes-long-ok` or remove. |

## New — B6 (SEC-A1 + Gates)

| ID | What | Severity | Files Affected | Notes |
|----|------|----------|----------------|-------|
| TD-011 | App launch-token endpoint registered by `AdminHdl`, not `AppHdl` — handler ownership mismatch | Medium | `internal/admin/admin_hdl.go:61-63` | App endpoint `POST /v1/app/launch-tokens` is wired in `AdminHdl.RegisterRoutes()`. Fix: move to `AppHdl.RegisterRoutes()` or extract shared `LaunchTokenHdl`. |
| TD-012 | **MISSING: Role model documentation — who does what, which scopes, and why** | **CRITICAL** | `docs/` — new file needed | See detail below. Without this document, every agent that touches the code misunderstands the system. |
| TD-013 | `POST /v1/admin/launch-tokens` lets admin CREATE agents — admin should only LIST/REVOKE launch tokens | High | `cmd/broker/main.go:257`, `internal/admin/admin_hdl.go:58-60` | See detail below. |
| TD-014 | **Code comments audit — all handlers, services, and types need role/scope/boundary comments** | **CRITICAL** | All `internal/` packages | Every function, handler, and type must document: what it does, who can call it (role/scope), why it exists, and its boundaries. Current code has minimal Go doc comments that describe mechanics but not roles, scopes, or security boundaries. An agent reading only the source file cannot understand who calls what or why. Standard defined in `.claude/rules/golang.md`. |

### TD-012 Detail: Missing Role Model Document

**The problem:** There is no document that explains the roles, their scopes, and WHY each role has each capability. Multiple AI agent sessions have written code, reviewed code, and updated docs — and none flagged this gap. The result:

- Acceptance tests get written against the wrong flow (admin creating agents instead of apps creating agents)
- Code reviewers approve shared handlers between admin and app without questioning the boundary
- `admin:launch-tokens:*` gets interpreted as "admin can create launch tokens" when it should mean "admin can oversee and revoke launch tokens"
- Nobody understands the default roles of the apps

**What the document must define:**

| Role | Purpose | Scopes | CAN do | CANNOT do |
|------|---------|--------|--------|-----------|
| **Admin (Operator)** | Manages the system. Registers apps, revokes tokens, audits activity. | `admin:apps:*`, `admin:revoke:*`, `admin:audit:*`, `admin:launch-tokens:read` | Register/deregister apps, set scope ceilings, revoke any token/agent/task/chain, view audit trail, list/inspect launch tokens | Create launch tokens, launch agents, act as an app |
| **App** | Registered software that manages its own agents within its scope ceiling. | `app:launch-tokens:*`, `app:agents:*`, `app:audit:read` | Create launch tokens (within ceiling), manage its own agents, read its own audit events | Revoke other apps' agents, register other apps, exceed its scope ceiling |
| **Agent** | Does actual work with a short-lived, scope-attenuated token. | Task-specific (e.g., `read:data:*`) | Validate tokens, renew own token, delegate (scope attenuation only) | Escalate scope, extend TTL beyond original, access admin/app endpoints |

**The production flow:**
1. Admin registers an App (sets name, scope ceiling, max TTL)
2. App authenticates via `POST /v1/app/auth` → gets app token
3. App creates launch tokens via `POST /v1/app/launch-tokens` → scope must be within ceiling
4. Agent registers using the launch token → gets short-lived scoped token
5. Agent works, renews as needed (TTL preserved), delegates if needed
6. Admin revokes if something goes wrong — can kill at token, agent, task, or chain level

**Where this doc should live:** `docs/roles.md` or a new section in `docs/concepts.md`.

### TD-013 Detail: Admin Should Not Create Agents

The admin token (issued at `cmd/broker/main.go:255-258`) gets scope `admin:launch-tokens:*`. The handler at `POST /v1/admin/launch-tokens` uses `RequireScope("admin:launch-tokens:*")` and calls `handleCreateLaunchToken` — which CREATES a launch token with NO scope ceiling check (the ceiling check at line 152-170 only fires for `app:` subjects).

This means admin can create launch tokens with ANY scopes, no ceiling, no AppID. Agents created from these tokens:
- Have no app association (empty AppID)
- Had no scope ceiling enforcement
- Are not traceable to any app

`admin:launch-tokens:*` should mean oversight (list, inspect, revoke launch tokens), NOT creation. Creating agents is the app's responsibility — that's where scope ceiling enforcement happens.

**Fix options:**
1. Remove `POST /v1/admin/launch-tokens` entirely — admin manages apps, apps manage agents
2. Restrict to dev mode only — useful for bootstrapping/testing, blocked in production
3. Require an app_id parameter — admin creates on behalf of an app, ceiling still enforced

---

## New — Post-Migration Repo Cleanup

### TD-015: renew/release/delegate endpoints have no scope restriction

**Severity: HIGH** — Security design question.

`POST /v1/token/renew`, `POST /v1/token/release`, and `POST /v1/delegate` use `valMw.Wrap()` with no `RequireScope`. Any valid Bearer token (admin, app, or agent) can call them. Found by tracing `cmd/broker/main.go` lines 181-183.

Question: is this intentional (any token holder should be able to self-manage) or should these require agent-specific scopes? Need to test and decide.

### TD-016: Docker Compose file audit

**Severity: MEDIUM** — Repo cleanup.

Three docker-compose files exist. Need to verify which are still valid, which are used by tests, and which should ship with open-source:

| File | Size | Last modified | Status |
|------|------|---------------|--------|
| `docker-compose.yml` | 1.3 KB | Mar 29 | Primary — used by `stack_up.sh` |
| `docker-compose.tls.yml` | 520 B | Mar 29 (fork point) | Unknown — TLS mode. Test? Ship? |
| `docker-compose.mtls.yml` | 634 B | Mar 29 (fork point) | Unknown — mTLS mode. Test? Ship? |

TD-S11 noted these were "verified clean" (no sidecar refs), but never assessed whether they're functional, tested, or needed for open-source.

### TD-017: Full repo artifact inventory — decide what ships open-source

**Severity: MEDIUM** — Must resolve before public release.

The repo has accumulated artifacts from migration, multiple agent sessions, and internal coordination. Need to classify everything as: ships with open-source, internal-only (remove before release), or archive.

**Root markdown files (12):**

| File | Verdict needed |
|------|---------------|
| `README.md` | Ships — needs update for open-source |
| `CHANGELOG.md` | Ships |
| `CONTRIBUTING.md` | Ships — review content |
| `SECURITY.md` | Ships — review content |
| `CLAUDE.md` | Internal — remove or move to `.claude/` |
| `MEMORY.md` | Internal — remove before release |
| `MEMORY_ARCHIVE.md` | Internal — remove before release |
| `FLOW.md` | Internal — remove before release |
| `TECH-DEBT.md` | Internal — remove or sanitize before release |
| `KNOWN-ISSUES.md` | Ships or remove — review content |
| `COWORK_SESSION.md` | Internal — remove before release |
| `COWORK_DOCS_AUDIT.md` | Internal — remove before release |

**docs/ (19 files + 2 subdirs):**

| File | Origin | Verdict needed |
|------|--------|---------------|
| `api.md` | Legacy + updated | Ships — verify against code |
| `api/openapi.yaml` | Legacy | Ships — TD-S14 says 51 stale sidecar refs |
| `architecture.md` | Legacy + updated | Ships — verify |
| `concepts.md` | Legacy + updated | Ships — verify |
| `scenarios.md` | Legacy + updated | Ships — verify |
| `implementation-map.md` | Legacy + updated | Ships — verify |
| `getting-started-operator.md` | Legacy + updated | Ships — verify (TD-S08/S09 issues) |
| `getting-started-developer.md` | Legacy | Ships — verify |
| `getting-started-user.md` | Legacy | Ships — verify |
| `aactl-reference.md` | Legacy | Ships — verify |
| `common-tasks.md` | Legacy | Ships — 72 KB, verify |
| `integration-patterns.md` | Legacy | Ships — 79 KB, verify |
| `troubleshooting.md` | Legacy | Ships — verify |
| `cc-foundations.md` | Today (Claude Code) | Draft — review before shipping |
| `cc-scope-model.md` | Today (Claude Code) | Draft — review before shipping |
| `cc-token-concept.md` | Today (Claude Code) | Draft — review before shipping |
| `cc-design-decisions.md` | Today (Claude Code) | Draft — review before shipping |
| `token-roles.md` | Today (other agent) | Draft — review before shipping |
| `agentauth-explained.md` | Today (other agent) | Draft — review before shipping |
| `diagrams/` | Legacy | Review contents |
| `patent/` | Today | Internal — NEVER ship |

**scripts/ (9 files):**

| File | Verdict needed |
|------|---------------|
| `stack_up.sh` | Ships — primary Docker lifecycle |
| `stack_down.sh` | Ships — primary Docker lifecycle |
| `gates.sh` | Internal — CI/migration tool |
| `test_batch.sh` | Internal — migration tool |
| `live_test.sh` | Unclear — TD-S01 says stale sidecar refs |
| `live_test_docker.sh` | Unclear — TD-S03 says stale sidecar refs |
| `gen_test_certs.sh` | Ships if TLS docs ship — TD-S12 says sidecar cert gen |
| `verify_compose.sh` | Internal — TD-S13 says stale sidecar refs |
| `verify_dockerfile.sh` | Review |

**tests/ (7 dirs + 2 files):**

| Item | Verdict needed |
|------|---------------|
| `LIVE-TEST-TEMPLATE.md` | Ships — acceptance test methodology |
| `FUCKING QUETIONS.MD` | Internal — remove before release |
| `p0-production-foundations/` | Ships — acceptance evidence |
| `p1-admin-secret/` | Ships — acceptance evidence |
| `sec-l1/` | Ships — acceptance evidence |
| `sec-l2a/` | Ships — acceptance evidence |
| `sec-l2b/` | Ships — acceptance evidence |
| `sec-a1/` | Ships — acceptance evidence |
| `app-launch-tokens/` | Review — may be incomplete |

**.plans/ (internal — entire directory should NOT ship):**

| Item | Verdict |
|------|---------|
| `tracker.jsonl` | Internal |
| `code-comments-audit.md` | Internal |
| `cherry-pick/` | Internal |
| `designs/` | Internal |

---

### TD-018: License decision — RESOLVED (2026-04-08)

**RESOLVED:** Switched from Apache 2.0 to AGPL-3.0 + CLA + enterprise license summary.

- `LICENSE` — replaced with AGPL-3.0 full text + copyright preamble
- `CLA.md` — created (dual-license grant: AGPL-3.0 open-source + commercial rights to maintainer)
- `ENTERPRISE_LICENSE.md` — created (non-binding commercial license summary)
- `CONTRIBUTING.md` — updated (references AGPL-3.0 and CLA)
- `README.md` — updated (AGPL-3.0 badge + license section)

**Remaining:** `docs/api/openapi.yaml` still references Apache 2.0 in `info.license.name` — update when touching OpenAPI spec (TD-S14).

---

## TD-VUL-005/006 — GHAS-gated workflows disabled (M-sec, 2026-04-10)

Three GitHub security features require GitHub Advanced Security (GHAS)
on private repos. `devonartis/agentauth` is currently private without
GHAS, so all three fail on first run. All three become FREE when the
repo flips public (Phase 4 of release strategy).

| ID | Workflow / Feature | What it gives | Status |
|----|-------------------|---------------|--------|
| TD-VUL-005 | `dep-review` job in `ci.yml` | Dependency graph + license policy scanning on every PR | Job commented out |
| TD-VUL-006a | `codeql.yml` (Go SAST) | Static analysis findings in Security tab, weekly scan | Workflow trigger changed to `workflow_dispatch` only |
| TD-VUL-006b | `scorecard.yml` (OpenSSF Scorecard) | Supply-chain posture score (badge on README) | Workflow trigger changed to `workflow_dispatch` only |

Remaining security coverage while these are disabled:
  - `govulncheck` — stdlib + Go module CVEs (live, blocking)
  - `gosec` — application-layer static analysis (live, blocking)
  - `contamination` grep — enterprise-module references (live, blocking)

**Fix sequence** when the repo flips public (no GHAS purchase needed):
  1. `dep-review`: uncomment the job block in `.github/workflows/ci.yml`
     and restore it to the `gates-passed` needs list if branch protection
     requires it.
  2. `codeql.yml`: revert the `on:` block header to `pull_request` +
     `push` + `schedule` (see original block preserved in the comment).
  3. `scorecard.yml`: same — restore the original `on:` block.
  4. Add badges to `README.md` (Task 30 in the M-sec plan) — CodeQL
     badge and Scorecard badge URLs are already in the plan draft.

---

## Go stdlib vulnerabilities (M-sec CI baseline, 2026-04-10) — RESOLVED

`govulncheck ./...` run on the M-sec baseline surfaced 4 stdlib CVEs, all
fixable by bumping the `toolchain` directive in `go.mod` from `go1.25.7`
to `go1.25.9`. **Resolved 2026-04-10 in Task 23** — toolchain bumped,
`govulncheck ./...` now returns "No vulnerabilities found."

| ID | Advisory | Package | Fixed in | Status |
|----|----------|---------|----------|--------|
| TD-VUL-001 | GO-2026-4947 | `crypto/x509` | `go1.25.9` | RESOLVED |
| TD-VUL-002 | GO-2026-4946 | `crypto/x509` | `go1.25.9` | RESOLVED |
| TD-VUL-003 | GO-2026-4870 (TLS 1.3 KeyUpdate DoS) | `crypto/tls` | `go1.25.9` | RESOLVED |
| TD-VUL-004 | GO-2026-4601 (IPv6 host literal parsing) | `net/url` | `go1.25.8` | RESOLVED |

## TD-CI-001 — Path-filter ci.yml for docs-only PRs (M-sec, 2026-04-10)

Full 13-gate pipeline runs on every PR regardless of whether any Go code
changed. A pure docs PR still triggers build, vet, lint, unit-tests,
unit-tests-race, gosec, govulncheck, go-mod-verify, docker-build, smoke-l25,
sbom — ~6-8 minutes of wasted compute per docs-only PR.

Maintainers (admins) can direct-push dev-file-only changes to develop today via
the "Direct push vs PR — strip-list test" Standing Rule in MEMORY.md. Non-admin
contributors will have no such escape hatch once Decision 016's exit criteria
are met and contributions open.

**Fix:** add `paths:` / `paths-ignore:` to `ci.yml`'s `on: pull_request` and
`on: push` triggers so the workflow doesn't fire on `**.md`, `docs/**`,
`.plans/**`, `adr/**`, or the stripped top-level markdown files. Keep the small
fast gates (`format`, `contamination`, `changelog`, `gate-parity`, `gates-passed`)
running in a lightweight `ci-docs.yml` workflow OR use `dorny/paths-filter`
per-job skipping so `gates-passed` still reports `success` on docs-only PRs
(branch protection requires it).

**Validate:** after the change, open a docs-only PR and verify the heavy gates
skip while `gates-passed` still reports success. Do not break the branch-protection
contract.

**When to fix:** before external contributions open. Not urgent while private +
no-contribs, but MUST be done before Decision 016's exit criteria are met.

## TD-DOCS-001 — `docs/` directory refactor (M-sec, 2026-04-10)

Flagged during the M-sec session: `docs/` has no table of contents, files are
scattered, some leak meta-tags / internal tooling artifacts that shouldn't be
public-facing.

**Fix:** audit every file in `docs/`, categorize (getting-started / concepts /
reference / examples / operations), write a `docs/README.md` as the navigational
root, remove or rewrite anything that shouldn't ship publicly, normalize
frontmatter to exclude internal tags. Consider a static-site generator (mdBook,
Docusaurus, Hugo) if the structure warrants it.

**When to fix:** before the repo flips public. Public-facing docs are one of
the first things a skeptical engineer reads; bad navigation is a trust-loss
moment.

## Hardcoded Identity Audit (2026-04-10)

Audit triggered by discovery of hardcoded `iss: "agentauth"` in `internal/token/tkn_claims.go` during the rebrand inventory. Full audit + recommendations at `.plans/reviews/2026-04-10-hardcoded-identity-audit.md`. The root cause — that `IssuerURL` was stripped alongside the OIDC provider removal for the open-core split, treating a general JWT concern as OIDC-specific — is documented in the audit doc. Standing rule added to `~/.claude/CLAUDE.md` ("No Hardcoded Identity Values — Universal, Non-Negotiable") to prevent recurrence.

| ID | What | Severity | When to Fix | Files Affected |
|----|------|----------|-------------|----------------|
| TD-TOKEN-001 | ~~**JWT `iss` claim hardcoded as literal `"agentauth"`**~~ | ~~CRITICAL~~ | **RESOLVED 2026-04-10** — `cfg.Issuer` field added (env `AA_ISSUER`), no default; empty = skip enforcement (mirrors Audience contract). Issuer check moved from `Validate()` (pure structural) into `Verify()` where cfg is available. Branch `fix/td-token-001-remove-issuer-hardcode`. | `internal/token/tkn_claims.go`, `internal/token/tkn_svc.go`, `internal/cfg/cfg.go` |
| TD-TOKEN-002 | ~~**JWT `aud` claim default literal `"agentauth"`** violates the `cfg.go:22` contract~~ | ~~HIGH~~ | **RESOLVED 2026-04-10** — override at `cfg.go:96` deleted; `Audience` honors documented `empty = skip` contract. `cfg_test.go` `TestLoad_AudienceDefault` updated to assert empty-when-unset. Same branch as TD-TOKEN-001. | `internal/cfg/cfg.go`, `internal/cfg/cfg_test.go` |
| TD-CFG-001 | ~~**Config defaults bake brand into binary** — `TrustDomain` and `DBPath`~~ | ~~HIGH~~ | **RESOLVED 2026-04-10** — `TrustDomain` default swapped `"agentauth.local"` → `"agentwrit.local"`; `DBPath` default swapped `"./agentauth.db"` → `"./data.db"` (neutral). Same branch as TD-TOKEN-001. | `internal/cfg/cfg.go` |
| TD-CFG-002 | ~~**Hardcoded FHS search path `/etc/agentauth/config`**~~ | ~~CRITICAL~~ | **RESOLVED 2026-04-10** — config search paths updated: `/etc/agentauth/config` → `/etc/broker/config` and `~/.agentauth/config` → `~/.broker/config`. Generated config file header `# AgentAuth Configuration` → `# Broker Configuration`. Same branch as TD-TOKEN-001. | `internal/cfg/configfile.go` |
| TD-TOKEN-003 | ~~**Tests lock the issuer hardcode in place** — 6 assertions across `tkn_svc_test.go` and `val_mw_test.go`~~ | ~~HIGH~~ | **RESOLVED 2026-04-10** — all 6 assertions and 3 `cfg.Cfg{}` literal constructions updated to drive from fixture `Issuer: "test-issuer"`. Same branch as TD-TOKEN-001. | `internal/token/tkn_svc_test.go`, `internal/authz/val_mw_test.go`, `internal/deleg/deleg_svc_test.go`, `internal/admin/admin_svc_test.go` |
| TD-TEST-001 | ~~**Test SPIFFE fixtures leak `agentauth.local`**~~ | ~~MEDIUM~~ | **RESOLVED 2026-04-10** — all `agentauth.local` references in test files swept to `test.local` (mechanical sed across `admin_hdl_test.go`, `identity/id_svc_test.go`, `mutauth/{heartbeat,discovery,mut_auth_hdl}_test.go`, `token/tkn_svc_test.go`). Same branch as TD-TOKEN-001. | `internal/admin/admin_hdl_test.go`, `internal/identity/id_svc_test.go`, `internal/mutauth/heartbeat_test.go`, `internal/mutauth/discovery_test.go`, `internal/mutauth/mut_auth_hdl_test.go`, `internal/token/tkn_svc_test.go` |
| TD-CLI-001 | **Binary name `aactl` → `awrit` rename** — 227 occurrences across `cmd/aactl/`, scripts, docs, tests, CHANGELOG. Mechanical. No logic change. | **MEDIUM** | PR 2 (can parallel PR 1) | `cmd/aactl/` (→ `cmd/awrit/`), `docs/aactl-reference.md`, scripts, tests |

**Not creating a TD for env var prefix** — decided 2026-04-10 to keep `AA_*` indefinitely. Neutral enough (two letters), operator-facing, highest-friction change in the whole rebrand. Re-evaluate at 1.0 if ever.

## When to Fix

Documentation and script drift items (TD-D*, TD-S*) should be resolved **after all cherry-pick batches are complete** (B0-B6). Doing them now risks conflicts with incoming commits. Schedule as a dedicated docs refresh phase post-migration.

Exception: TD-S01/S02/S03 may need partial fixes during migration if they block Docker testing for a batch.

**Hardcoded identity items (TD-TOKEN-*, TD-CFG-*, TD-TEST-001, TD-CLI-001)** — scheduled for PR 1 (correctness) and PR 2 (binary rename). These are prerequisites for the rebrand but also correctness fixes worth doing independently. See `.plans/reviews/2026-04-10-hardcoded-identity-audit.md` for full execution plan.
