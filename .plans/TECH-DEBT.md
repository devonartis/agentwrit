# TECH-DEBT.md

Active tech debt. Append new entries as debt is taken. Never remove — mark as RESOLVED with date.

Full details for each item live in the referenced file. This is the index.

---

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
| TD-D01 | `docs/sidecar-deployment.md` — entire file is about sidecar deployment | High | `docs/sidecar-deployment.md` | DELETE or rewrite as broker-only deployment |
| TD-D02 | `docs/getting-started-user.md` — sidecar path, port 8081, `go run ./cmd/sidecar`, `docker-compose.uds.yml` | High | `docs/getting-started-user.md` | Rewrite: remove sidecar path, update Docker instructions, keep direct broker path |
| TD-D03 | `docs/getting-started-developer.md` — sidecar SDK integration, token exchange flow | High | `docs/getting-started-developer.md` | Rewrite: developer flow is now direct broker registration |
| TD-D04 | `docs/getting-started-operator.md` — sidecar configuration, env vars, deployment topology | High | `docs/getting-started-operator.md` | Rewrite: operator manages broker only |
| TD-D05 | `docs/architecture.md` — sidecar in architecture diagrams, `docker-compose.uds.yml`, token exchange | Medium | `docs/architecture.md` | Partially cleaned in B0 (items 8-10 removed). Still has sidecar in overview sections |
| TD-D06 | `docs/api.md` — token exchange endpoint documentation | Medium | `docs/api.md` | Remove token exchange endpoint, verify remaining endpoints |
| TD-D07 | `docs/api/openapi.yaml` — token exchange route in OpenAPI spec | Medium | `docs/api/openapi.yaml` | Remove `/v1/token/exchange` path and related schemas |
| TD-D08 | `docs/concepts.md` — sidecar in conceptual model | Medium | `docs/concepts.md` | Update agent registration model (no sidecar) |
| TD-D09 | `docs/troubleshooting.md` — sidecar troubleshooting section, UDS refs | Medium | `docs/troubleshooting.md` | Remove sidecar troubleshooting, update for broker-only |
| TD-D10 | `docs/common-tasks.md` — sidecar operations tasks | Low | `docs/common-tasks.md` | Update for broker-only operations |
| TD-D11 | `docs/integration-patterns.md` — sidecar integration pattern | Low | `docs/integration-patterns.md` | Rewrite patterns for direct broker integration |
| TD-D12 | `docs/examples/*.md` — 4 example docs reference sidecar flows | Low | `docs/examples/customer-support.md`, `data-pipeline.md`, `devops-automation.md`, `code-generation.md` | Update examples to broker-direct flow |
| TD-D13 | `docs/examples/README.md` — sidecar in examples overview | Low | `docs/examples/README.md` | Update overview |
| TD-D14 | `docs/aactl-reference.md` — sidecar aactl commands if any | Low | `docs/aactl-reference.md` | Verify and remove sidecar CLI refs |
| TD-D15 | `docs/RECOMMENDATIONS.md` — sidecar recommendations | Low | `docs/RECOMMENDATIONS.md` | Update recommendations |
| TD-D16 | `README.md` — sidecar in project overview | Medium | `README.md` | Update for open-core, broker-only |
| TD-D17 | `CHANGELOG.md` — historical sidecar entries (leave as-is, they're history) | None | `CHANGELOG.md` | No action needed — historical record |
| TD-D18 | `KNOWN-ISSUES.md` — sidecar-related known issues | Low | `KNOWN-ISSUES.md` | Remove sidecar issues, add any new core-only issues |

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

---

## When to Fix

Documentation and script drift items (TD-D*, TD-S*) should be resolved **after all cherry-pick batches are complete** (B0-B6). Doing them now risks conflicts with incoming commits. Schedule as a dedicated docs refresh phase post-migration.

Exception: TD-S01/S02/S03 may need partial fixes during migration if they block Docker testing for a batch.
