# AgentAuth Project Roadmap (Git-Mapped)

Ephemeral Credentialing for AI Agents | 55,550+ Lines of Code | 7 Phases

Updated: February 19, 2026

---

## Repository Map

| Repo | Path | Description |
|------|------|-------------|
| **agentAuth** (this repo) | `/Users/divineartis/proj/agentAuth` | Go broker, sidecar proxy, Ed25519 challenge-response, audit trail |
| **agentauth-app** | `/Users/divineartis/proj/agentauth-app` | Python showcase app — SDK, CLI, dashboard, demo agents, attack simulator |

> The Python showcase app was originally developed in this repo (M11–M14 milestones) and later extracted to `agentauth-app` as a standalone project. The `agentauth-app` repo has `upstream` set to this repo. Commits from both repos are tracked below.

---

## Phase 0: Architecture & Design

**COMPLETED** | Feb 15, 2026

| # | Feature | Status | Git Evidence (agentAuth) |
|---|---------|--------|--------------------------|
| 0.1 | Ephemeral Agent Credentialing Pattern (v1.2) | DONE | `e129a3c` initial AgentAuth M00-M03 scaffold |
| 0.2 | Threat Model & Trust Boundaries | DONE | `e129a3c` initial AgentAuth M00-M03 scaffold |
| 0.3 | Tool-Based Enforcement Architecture | DONE | `e129a3c` initial scaffold; `f7ce4e0` integrate revocation check into validation middleware |
| 0.4 | Cloud IAM Research (AWS / GCP / Azure) | DONE | `2270c9e` ADR-003: SPIFFE-compatible ID decision; `9a86fe3` ADR-002: prioritize MVP requirements |

**4/4 complete**

---

## Phase 1: Go Broker & Sidecar

**COMPLETED** | Feb 15-16, 2026

| # | Feature | Status | LOC | Git Evidence (agentAuth) |
|---|---------|--------|-----|--------------------------|
| 1.1 | JWT Token Service | DONE | 301 | `e129a3c` M00-M03 scaffold; `bbb5aba` sid claim support; `407272c` sidecar token exchange service |
| 1.2 | SPIFFE Identity Service | DONE | 308 | `e129a3c` M00-M03 scaffold; `2270c9e` ADR-003: SPIFFE-compatible ID |
| 1.3 | Scope Engine & Wildcard Matching | DONE | 353 | `e129a3c` M00-M03 scaffold; `fb5d020` scope attenuation for delegation (M07-T01) |
| 1.4 | Admin Service & Ceiling CRUD | DONE | 804 | `e129a3c` M00-M03 scaffold; `8970a02` multi-scope sidecar activation fix; `c024cad` dynamic scope ceiling management |
| 1.5 | SHA-256 Hash-Chained Audit Log | DONE | 225 | `cd6f3e9` audit core package; `c662bac` AuditHdl wired into broker; `692b333` audit events from register/authz/revoke; `7649326` audit integration tests |
| 1.6 | Scope-Attenuated Delegation | DONE | 211 | `fb5d020` M07-T01 scope attenuation; `2b565bc` M07-T02 DelegSvc; `e7bffe8` M07-T03 chain verification; `7ceac9a` M07-T04 POST /v1/delegate; `0fbb3c6` M07-T05 wire into main.go |
| 1.7 | Multi-Level Revocation | DONE | 113 | `4c69108` revocation service (4-level); `e48cc8a` POST /v1/revoke handler; `f7ce4e0` revocation check in middleware; `dd145b8` wire into broker |
| 1.8 | Mutual TLS & Heartbeating | DONE | 434 | `c2eaf55` M06 mutual auth with ADR-001; `e5454b9` peer/initiator identity checks; `365335a` responder binding + discovery |
| 1.9 | Sidecar Proxy | DONE | 2,516 | `4239a23` config loader; `951c835` broker HTTP client; `4aed8e5` auto-bootstrap; `dfd7cb8` developer-facing handlers; `abc6d9b` main entrypoint; `8ffb4bd` Phase 1 binary; `4a4f932` thread-safe state; `914e72b` background renewal; `a908b3b` agent registry; `bbff0fe` challenge/register client; `1b75ad5` lazy registration; `2cec226` BYOK handler; `21c591b` integration tests |
| 1.10 | SQLite Persistence | DONE | 254 | `dac4c29` activation token replay protection; `4c2733d` SQLite audit persistence |
| 1.11 | Go Test Suite (147% coverage) | DONE | 9,607 | `1f17def` revocation tests; `7649326` audit tests; `104c0da` sidecar E2E test; `c837559` Go smoketest (12 steps); `21401e6` containerized broker smoke test |

**11/11 complete**

---

## Phase 2: Python Showcase App

**COMPLETED** | Feb 15-16, 2026

> **Repo note:** Phase 2 was originally prototyped in agentAuth (milestones M11–M14) and then extracted to **agentauth-app** as a standalone Python project. The table below shows commits from both repos. The agentAuth commits are historical — that code no longer exists in this repo.

| # | Feature | Status | LOC | agentAuth (historical) | agentauth-app (current) |
|---|---------|--------|-----|------------------------|-------------------------|
| 2.1 | Python SDK (5 Sub-Clients) | DONE | 439 | `00b1a81` M12-T01: BrokerClient + AgentBase | `faafb7b` base HTTP client; `e7454b5` operator/developer/security modules; `e2bb7a6` facade properties |
| 2.2 | Typer CLI (Full Command Suite) | DONE | 353 | `af8364b` M13-T06: simulator CLI | `5802991` operator commands; `5a2de83` developer commands; `1fd139d` security commands; `9b471da` demo commands |
| 2.3 | Identity-First Pipeline | DONE | 1,019 | `4c2f173` M12-T02 Agent A; `cb8f9c2` M12-T03 Agent B; `20d8648` M12-T04 Agent C; `176e366` M12-T05 orchestrator | `d20df9f` demo data pipeline; `56c7960` identity-first pipeline + UI overhaul; `c79df77` runtime scope narrowing |
| 2.4 | Tool System (8 Tools) | DONE | 329 | `0b49fa4` M11-T01 FastAPI resource server; `44fe364` M11-T02 token validation | `c37ae0a` tool-based scope enforcement with mock data |
| 2.5 | HTMX Dashboard (3 Personas) | DONE | 451 | `95444c7` M14-T02 HTMX frontend | `049c852` FastAPI app + tab routing; `afc9ad7` HTMX endpoints for 3 personas |
| 2.6 | SSE Real-Time Web UI | DONE | 1,035 | `1ef0a3d` M14-T01 dashboard backend; `95444c7` M14-T02 HTMX frontend; `fd807e2` M14-T03 tests | `34c31a1` web UI with SSE streaming + scope enforcement |
| 2.7 | Docker Compose Full Stack | DONE | -- | `55c29bb` multi-stage Dockerfile; `213de97` broker+sidecar build; `884f493` compose config | `18e4acb` Docker Compose full stack (broker + sidecar + app) |
| 2.8 | Python Test Suite (71% coverage) | DONE | 2,652 | `6e2a514` M11-T03 resource server tests; `3fa3ff4` M12-T06 demo-agent tests; `fd807e2` M14-T03 dashboard tests | `06b269d` integration smoke test; `78dbd71` identity + escalation tests |

**8/8 complete**

---

## Phase 3: Runtime Hardening

**COMPLETED** | Feb 16-17, 2026

> **Repo note:** Runtime hardening spans both repos. The Go broker/sidecar enforcement is in agentAuth; the Python-side scope narrowing and CLI commands are in agentauth-app.

| # | Feature | Status | agentAuth | agentauth-app |
|---|---------|--------|-----------|---------------|
| 3.1 | Runtime Scope Narrowing | DONE | `c9e3eae` phase4 token exchange with scope attenuation; `6f37229` scope format pre-validation; `c024cad` dynamic scope ceiling management | `c79df77` runtime scope narrowing — broker-native data boundary enforcement |
| 3.2 | Immediate Revocation on Policy Violation | DONE | `4c69108` revocation service; `f7ce4e0` revocation check in middleware; `12a5af6` audit recording on all denial paths | `0d2b58a` immediate revocation on policy violation — zero tool calls permitted |
| 3.3 | CLI Ceiling Management (4 Commands) | DONE | `c024cad` dynamic scope ceiling management with audit trail | `33fb0b2` CLI ceiling management, wildcard ceiling fix, user stories |
| 3.4 | FIX-001: Wildcard Ceiling for Compound Scopes | DONE | `8970a02` multi-scope sidecar activation fix; `c024cad` dynamic scope ceiling management | `33fb0b2` wildcard ceiling fix |
| 3.5 | FIX-002: Broad + Narrowed Scopes at Registration | DONE | `c9e3eae` token exchange with scope attenuation; `5921750` empty sidecar_id derivation guard | `0974f02` broad + narrowed scopes at registration fix |

**5/5 complete**

---

## Phase 4: Demo Polish & Data Expansion

**ACTIVE** | Feb 17+, 2026

| # | Feature | Status | Notes | Git Evidence |
|---|---------|--------|-------|--------------|
| 4.1 | Persist Audit Log to SQLite | **DONE** | Audit events survive broker restarts. `AA_DB_PATH` configurable. Health endpoint returns `db_connected` and `audit_events_count`. | agentAuth: `4c2733d` SQLite audit persistence; `9290e9d` merge to develop |
| 4.2 | Sidecar ID Auto-Discovery | **DONE** | Sidecar health returns `sidecar_id`. Integration tests passing. | agentAuth: `50a2809` exchange/denial metrics + health endpoint; `9eaf773` lastRenewal/startTime state; `3a0677a` structured logging + Prometheus |
| 4.3 | Docker Compose pulls from GitHub repo | **DONE** | Broker/sidecar build from `devonartis/agentAuth` develop branch. No local broker code in app repo. | agentAuth: `213de97` multi-stage build; `55c29bb` Dockerfile + compose |
| 4.4 | Orders / Transactions Database | PLANNED | Second data source for cross-database scoping. | -- |
| 4.5 | 4 New Order Tools | PLANNED | `get_customer_orders`, `get_order_detail`, `get_invoice`, `issue_refund`. | -- |
| 4.6 | 4 New Admin/Internal Tools | PLANNED | `search_audit_log`, `get_system_metrics`, `export_customer_data`, `flag_for_review`. | -- |
| 4.7 | Run All 18 User Stories | PLANNED | 6 legitimate + 8 attack + 4 operator workflows. | -- |
| 4.8 | Decompose pipeline.py (1,019 -> 6 modules) | PLANNED | Refactor for maintainability. Target repo: agentauth-app. | -- |
| 4.9 | Automated Scope Narrowing Tests | PLANNED | Unit tests for `_scope_matches_any()`, registration scope building, `_enforce_tool_call()`. Target repo: agentauth-app. | -- |
| 4.10 | Attack Simulation Test Suite (12 scenarios) | PLANNED | Real adversarial tests. | -- |

**3/10 complete**

---

## Phase 5: Production Readiness

**UPCOMING** | Target: Q2 2026

| # | Feature | Status | Backlog Ref | Git Evidence |
|---|---------|--------|-------------|--------------|
| 5.1 | Real Authentication (Session/JWT/SSO) | PLANNED | Backlog #8. Current `_authenticate_user()` is a mock. | -- |
| 5.2 | Ceiling Request Workflow | PLANNED | Backlog #4. Developer requests scopes, operator approves/denies. | -- |
| 5.3 | List Active Sidecars & Agents | **PARTIAL** | Backlog #5 (sidecars) DONE — endpoint built, tested, live. Backlog #6 (agents) still planned. | agentAuth: feature/list-sidecars-endpoint branch (10 commits) |
| 5.3a | Go CLI for Admin Endpoints (`cmd/cli/`) | **PLANNED** | Backlog #16. Operator tooling — without this, admin endpoints are unusable. CLI belongs in Go repo, NOT agentauth-app. Third binary alongside broker and sidecar. | -- |
| 5.4 | RBAC on Admin API | PLANNED | Separate operator, security, and compliance roles. | -- |
| 5.5 | HA / Clustering | PLANNED | No single point of failure for the broker. | -- |
| 5.6 | Real Database Adapter | PLANNED | Replace in-memory data with PostgreSQL or MongoDB. | -- |
| 5.7 | CI/CD Pipeline | PLANNED | Automated build, test, deploy. | -- |
| 5.8 | External Security Audit | PLANNED | Third-party review of credential system. | -- |
| 5.9 | Rate Limiting on Ceiling Updates | PLANNED | Backlog #11. | -- |
| 5.10 | Dashboard: Show Scope Narrowing | PLANNED | Backlog #10. Target repo: agentauth-app. | -- |
| 5.11 | Ceiling Change Audit History in CLI | PLANNED | Backlog #9. | -- |
| 5.12 | Operator Docs: Runtime Ceiling Management | PLANNED | Backlog #3. | -- |

**0/13 complete (5.3 partial — sidecars endpoint done, agents + CLI pending)**

---

## Phase 6: Market Launch

**FUTURE** | Target: Q3 2026

| # | Feature | Status | Git Evidence |
|---|---------|--------|--------------|
| 6.1 | AgentAuth Cloud (Hosted SaaS) | PLANNED | -- |
| 6.2 | Open Source Core + Enterprise | PLANNED | -- |
| 6.3 | Multi-Framework SDK (Python, Node.js, Go + LangChain, CrewAI, AutoGen) | PLANNED | -- |
| 6.4 | Scope Playground / Simulator | PLANNED | -- |
| 6.5 | Webhooks on Ceiling Change | PLANNED | -- |
| 6.6 | Anomaly Detection | PLANNED | -- |
| 6.7 | Ceiling Diff Preview (dry-run) | PLANNED | -- |
| 6.8 | Multi-Sidecar Management | PLANNED | -- |

**0/8 complete**

---

## Progress Summary

| Metric | Previous (Feb 17) | Current (Feb 18) | Delta |
|--------|-------------------|-------------------|-------|
| Total Features | 50 | 55 | +5 |
| Completed | 28 (56%) | 31 (56%) | +3 |
| Blocked | 1 | 0 | -1 (P0 audit persistence resolved) |
| Planned | 21 | 24 | +3 (new items surfaced) |
| Phases Completed | 3 of 7 | 3 of 7 | -- |
| Total LOC | 55,550 | 55,550+ | -- |
| Git Commits Mapped | -- | 80+ (across both repos) | -- |

### What changed since Feb 17

**Resolved:**
- P0 blocker: Audit log persistence to SQLite (was BLOCKED, now DONE)
- Sidecar ID auto-discovery (was PLANNED, now DONE)
- Docker Compose builds from GitHub repo (no local broker snapshot)

**New items added:**
- Attack simulation test suite (12 adversarial scenarios)
- Rate limiting on ceiling updates (surfaced from backlog)
- Dashboard scope narrowing visualization (surfaced from backlog)
- Ceiling change audit history in CLI (surfaced from backlog)
- Operator docs for runtime ceiling management (surfaced from backlog)

**Key architecture change:**
- Python showcase app extracted from agentAuth to standalone `agentauth-app` repo. Broker builds from `devonartis/agentAuth` GitHub repo directly. Clean separation between the Go security engine and the Python showcase app.

---

## Git Commit Summary by Phase

### Phase 0 — Architecture & Design (agentAuth)
| Commit | Summary |
|--------|---------|
| `e129a3c` | Initial AgentAuth M00-M03 scaffold |
| `9a86fe3` | ADR-002: prioritize MVP requirements |
| `2270c9e` | ADR-003: SPIFFE-compatible ID decision |

### Phase 1 — Go Broker & Sidecar (agentAuth)
| Commit | Summary |
|--------|---------|
| `e129a3c` | M00-M03 core: token, SPIFFE, scope, admin |
| `4c69108` | M04: revocation service with 4-level support |
| `e48cc8a` | M04: POST /v1/revoke handler |
| `f7ce4e0` | M04: revocation check in validation middleware |
| `dd145b8` | M04: wire revocation service into broker |
| `c2eaf55` | M06: mutual authentication with ADR-001 |
| `fb5d020` | M07-T01: scope attenuation for delegation |
| `2b565bc` | M07-T02: DelegSvc delegation token creation |
| `e7bffe8` | M07-T03: delegation chain verification |
| `7ceac9a` | M07-T04: POST /v1/delegate handler |
| `0fbb3c6` | M07-T05: wire delegation, live tests, docs |
| `45378c8` | M08: RFC7807 error factory |
| `04dfbb0` | M08: Prometheus metrics primitives |
| `863f95b` | M08: health and metrics endpoints |
| `cd6f3e9` | M05: audit core package |
| `c662bac` | M05: AuditHdl wired into broker |
| `692b333` | M05: audit events from register, authz, revoke |
| `4239a23` | Sidecar P1: configuration loader |
| `951c835` | Sidecar P1: broker HTTP client |
| `4aed8e5` | Sidecar P1: auto-bootstrap sequence |
| `dfd7cb8` | Sidecar P1: developer-facing handlers |
| `abc6d9b` | Sidecar P1: main entrypoint |
| `4a4f932` | Sidecar P2: thread-safe state + renewal config |
| `914e72b` | Sidecar P2: background token renewal |
| `a908b3b` | Sidecar P2: ephemeral agent registry |
| `1b75ad5` | Sidecar P2: lazy agent registration |
| `2cec226` | Sidecar P2: BYOK registration handler |
| `1bcca60` | Sidecar Obs: Prometheus metrics |
| `fa62c3d` | Sidecar Obs: register handler metrics + agent gauge |
| `fa975d9` | Sidecar Resilience: circuit breaker config |
| `edd6aa0` | Sidecar Resilience: circuit breaker with sliding-window |
| `cc59f0e` | Sidecar Resilience: wire circuit breaker into token handler |
| `6cc7110` | Sidecar Resilience: bootstrap retry with backoff |
| `bbb5aba` | Token: sid claim support |
| `dac4c29` | Store: activation token replay protection |
| `407272c` | Token: sidecar token exchange service |
| `c9e3eae` | Sidecar P4: token exchange with scope attenuation |
| `4c2733d` | P0: SQLite audit persistence |

### Phase 2 — Python Showcase App

**agentAuth (historical — code moved to agentauth-app):**
| Commit | Summary |
|--------|---------|
| `0b49fa4` | M11-T01: FastAPI resource server with seed data |
| `44fe364` | M11-T02: token validation middleware |
| `6e2a514` | M11-T03: resource server integration tests |
| `00b1a81` | M12-T01: BrokerClient + AgentBase with Ed25519 |
| `4c2f173` | M12-T02: Agent A DataRetriever |
| `cb8f9c2` | M12-T03: Agent B Analyzer with delegation |
| `20d8648` | M12-T04: Agent C ActionTaker |
| `176e366` | M12-T05: orchestrator A->B->C workflow |
| `3fa3ff4` | M12-T06: demo-agents integration tests |
| `de3cbe4` | M13-T01: credential theft attack |
| `d7b95d3` | M13-T02: lateral movement attack |
| `8b39fb1` | M13-T03: agent impersonation attack |
| `06dc1bd` | M13-T04: privilege escalation attack |
| `f973adf` | M13-T05: accountability check |
| `af8364b` | M13-T06: simulator CLI and docs |
| `1ef0a3d` | M14-T01: dashboard backend with SSE |
| `95444c7` | M14-T02: HTMX frontend with dark theme |
| `fd807e2` | M14-T03: dashboard integration tests |

**agentauth-app (current):**
| Commit | Summary |
|--------|---------|
| `d3cd6dc` | Showcase app design — dashboard + CLI + SDK |
| `9871a7b` | Project scaffolding — pyproject.toml, docs, fixtures |
| `faafb7b` | SDK: base HTTP client with broker/sidecar helpers |
| `e7454b5` | SDK: operator, developer, and security modules |
| `e2bb7a6` | SDK: facade properties |
| `5802991` | CLI: operator commands — auth, launch-token, health |
| `049c852` | Dashboard: FastAPI app, base template, tab routing |
| `18e4acb` | Docker Compose full stack — broker + sidecar + app |
| `5a2de83` | CLI: developer commands — token request, renew, inspect |
| `d20df9f` | Demo data pipeline — 3-agent scenario |
| `1fd139d` | CLI: security commands — audit list, verify-chain, revoke |
| `afc9ad7` | Dashboard: HTMX endpoints for 3 persona tabs |
| `06b269d` | Integration smoke test — full lifecycle |
| `477eb2b` | Code review fixes — error handling, healthcheck |
| `183dedf` | ExampleAgent with real LLM + identity + scope escalation |
| `eda4653` | Customer support example with prompt injection demo |
| `68a47b5` | Data pipeline example with research/writer/review agents |
| `78dbd71` | Scope ceiling for examples + identity tests |
| `9b471da` | CLI demo commands for customer-support and data-pipeline |
| `34c31a1` | Web UI with SSE streaming + scope enforcement |
| `c37ae0a` | Tool-based scope enforcement with mock customer data |

### Phase 3 — Runtime Hardening

**agentAuth:**
| Commit | Summary |
|--------|---------|
| `c024cad` | Dynamic scope ceiling management with audit trail |
| `c9e3eae` | Token exchange with scope attenuation + lineage |
| `8970a02` | Fix: multi-scope sidecar activation |
| `12a5af6` | Wire audit recording into all middleware denial paths |
| `204787b` | Audit recording on delegation attenuation violation |
| `3b9feb3` | Enrich scope ceiling denial log with audit fields |

**agentauth-app:**
| Commit | Summary |
|--------|---------|
| `8c36752` | Dynamic scope ceiling management across broker, sidecar, and Python app |
| `56c7960` | Identity-first pipeline, broker-centric enforcement, UI overhaul |
| `21a4832` | Demo realism fixes (Fixes 1-6) |
| `0d2b58a` | Immediate revocation on policy violation — zero tool calls |
| `c79df77` | Runtime scope narrowing — broker-native data boundary enforcement |
| `33fb0b2` | CLI ceiling management, wildcard ceiling fix, user stories |
| `0974f02` | Broad + narrowed scopes at registration fix |

### Phase 4 — Demo Polish (completed items, agentAuth)
| Commit | Summary |
|--------|---------|
| `4c2733d` | P0: SQLite audit persistence, health, observability |
| `9290e9d` | Merge p0-audit-persistence-and-fixes into develop |
| `50a2809` | Sidecar: exchange/denial metrics + health endpoint |
| `3a0677a` | Sidecar: observability — structured logging + Prometheus |

---

## Remaining Gaps (Prioritized)

### Must-Have for Demo (Phase 4 remaining)

1. **Orders database + 4 order tools** — Without a second data source, the cross-database scoping story is incomplete. Target: agentauth-app.
2. **18 user stories end-to-end** — 6 happy path + 8 attacks + 4 operator workflows. Spans both repos.
3. **Attack simulation tests** — Automated adversarial tests that validate the audit trail catches every intrusion attempt.
4. **Scope narrowing unit tests** — Zero automated tests exist for the core security logic. Target: agentauth-app.

### Must-Have for Production (Phase 5 critical path)

5. **Go CLI for admin endpoints** — Every admin endpoint is unusable without operator tooling. CLI must live in `cmd/cli/` in the Go repo (not the Python demo app). This is the highest priority production gap.
6. **Real authentication** — The mock `_authenticate_user()` is a security gap. Identity must come from session/JWT, not ticket text.
7. **Ceiling request workflow** — Without this, scope management is ad-hoc and unaudited.
8. **List agents endpoint** — Sidecars done (5.3), agents still needed. Operators can't manage what they can't see.
9. **CI/CD pipeline** — No automated quality gate today.

### Nice-to-Have (Phase 5/6)

9. Pipeline refactor (1,019 LOC -> 6 modules) — Target: agentauth-app
10. RBAC on admin API
11. HA / Clustering
12. External security audit
