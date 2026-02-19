# AgentAuth Project Roadmap (Git-Mapped)

Ephemeral Credentialing for AI Agents | 55,550+ Lines of Code | 7 Phases

Updated: February 18, 2026

---

## Phase 0: Architecture & Design

**COMPLETED** | Feb 15, 2026

| # | Feature | Status | Git Evidence |
|---|---------|--------|--------------|
| 0.1 | Ephemeral Agent Credentialing Pattern (v1.2) | DONE | `e129a3c` feat(scaffold): initial AgentAuth M00-M03 implementation |
| 0.2 | Threat Model & Trust Boundaries | DONE | `e129a3c` feat(scaffold): initial AgentAuth M00-M03 implementation |
| 0.3 | Tool-Based Enforcement Architecture | DONE | `e129a3c` feat(scaffold): initial AgentAuth M00-M03 implementation; `f7ce4e0` feat(authz): integrate revocation check into validation middleware |
| 0.4 | Cloud IAM Research (AWS / GCP / Azure) | DONE | `2270c9e` docs(plans): document SPIFFE-compatible ID decision (ADR-003); `9a86fe3` docs(plans): prioritize MVP requirements (ADR-002) |

**4/4 complete**

---

## Phase 1: Go Broker & Sidecar

**COMPLETED** | Feb 15-16, 2026

| # | Feature | Status | LOC | Git Evidence |
|---|---------|--------|-----|--------------|
| 1.1 | JWT Token Service | DONE | 301 | `e129a3c` feat(scaffold): initial AgentAuth M00-M03 implementation; `bbb5aba` feat(model): Add sid claim support to token service; `407272c` feat(token): Implement Sidecar Token Exchange service logic |
| 1.2 | SPIFFE Identity Service | DONE | 308 | `e129a3c` feat(scaffold): initial AgentAuth M00-M03 implementation; `2270c9e` docs(plans): document SPIFFE-compatible ID decision (ADR-003) |
| 1.3 | Scope Engine & Wildcard Matching | DONE | 353 | `e129a3c` feat(scaffold): initial AgentAuth M00-M03 implementation; `fb5d020` feat(deleg): implement scope attenuation for delegation chain (M07-T01) |
| 1.4 | Admin Service & Ceiling CRUD | DONE | 804 | `e129a3c` feat(scaffold): initial AgentAuth M00-M03 implementation; `8970a02` fix(admin): multi-scope sidecar activation; `c024cad` feat: dynamic scope ceiling management with enhanced audit trail |
| 1.5 | SHA-256 Hash-Chained Audit Log | DONE | 225 | `cd6f3e9` feat(audit): port audit core package from m05-audit-qa; `c662bac` feat(audit): add AuditHdl and wire audit into broker; `692b333` feat(audit): emit audit events from register, authz, and revoke; `7649326` test(audit): add integration tests for audit trail |
| 1.6 | Scope-Attenuated Delegation | DONE | 211 | `fb5d020` feat(deleg): scope attenuation (M07-T01); `2b565bc` feat(deleg): DelegSvc delegation token creation (M07-T02); `e7bffe8` feat(deleg): delegation chain verification (M07-T03); `7ceac9a` feat(deleg): POST /v1/delegate handler (M07-T04); `0fbb3c6` feat(deleg): wire delegation into main.go (M07-T05) |
| 1.7 | Multi-Level Revocation | DONE | 113 | `4c69108` feat(revoke): add revocation service with 4-level support; `e48cc8a` feat(revoke): add POST /v1/revoke handler; `f7ce4e0` feat(authz): integrate revocation check into validation middleware; `dd145b8` feat(broker): wire revocation service and handler |
| 1.8 | Mutual TLS & Heartbeating | DONE | 434 | `c2eaf55` feat(mutauth): implement M06 mutual authentication with ADR-001 live testing; `e5454b9` fix(mutauth): add peer and initiator identity checks; `365335a` fix(mutauth): enforce responder binding and wire discovery runtime |
| 1.9 | Sidecar Proxy | DONE | 2,516 | `4239a23` feat(sidecar): add configuration loader with env vars; `951c835` feat(sidecar): add broker HTTP client; `4aed8e5` feat(sidecar): add auto-bootstrap sequence; `dfd7cb8` feat(sidecar): add /v1/token, /v1/token/renew, /v1/health handlers; `abc6d9b` feat(sidecar): add main entrypoint; `8ffb4bd` feat(sidecar): Phase 1 Go sidecar binary; `4a4f932` feat(sidecar): thread-safe sidecarState; `914e72b` feat(sidecar): background token renewal goroutine; `a908b3b` feat(sidecar): in-memory ephemeral agent registry; `bbff0fe` feat(sidecar): challenge, launch-token, and register broker client methods; `1b75ad5` feat(sidecar): lazy agent registration; `2cec226` feat(sidecar): BYOK registration handler and challenge proxy; `21c591b` test(sidecar): integration tests for lazy registration and BYOK |
| 1.10 | SQLite Persistence | DONE | 254 | `dac4c29` feat(store): Implement activation token replay protection; `4c2733d` feat(p0): SQLite audit persistence, health enhancements, and observability |
| 1.11 | Go Test Suite (147% coverage) | DONE | 9,607 | `1f17def` test(revoke): add integration and live tests for revocation; `7649326` test(audit): add integration tests for audit trail; `104c0da` test(sidecar): add end-to-end integration test; `c837559` feat(live-test): add Go-based smoketest with full sidecar lifecycle (12 steps); `21401e6` test(deploy): Add smoke test script for containerized broker |

**11/11 complete**

---

## Phase 2: Python Showcase App

**COMPLETED** | Feb 16, 2026

| # | Feature | Status | LOC | Git Evidence |
|---|---------|--------|-----|--------------|
| 2.1 | Python SDK (5 Sub-Clients) | DONE | 439 | `00b1a81` feat(demo-agents): add BrokerClient and AgentBase with Ed25519 registration (M12-T01) |
| 2.2 | Typer CLI (Full Command Suite) | DONE | 353 | `af8364b` feat(attacks): simulator CLI, integration tests, and docs (M13-T06) |
| 2.3 | Identity-First Pipeline | DONE | 1,019 | `4c2f173` feat(demo-agents): add Agent A DataRetriever with customer fetch workflow (M12-T02); `cb8f9c2` feat(demo-agents): add Agent B Analyzer with order analysis and delegation (M12-T03); `20d8648` feat(demo-agents): add Agent C ActionTaker with delegated ticket close (M12-T04); `176e366` feat(demo-agents): add orchestrator driving Agent A->B->C workflow (M12-T05) |
| 2.4 | Tool System (8 Tools) | DONE | 329 | `0b49fa4` feat(resource-server): add FastAPI app with seed data, routes, and unit tests (M11-T01); `44fe364` feat(resource-server): add token validation middleware (M11-T02) |
| 2.5 | HTMX Dashboard (3 Personas) | DONE | 451 | `95444c7` feat(dashboard): add HTMX frontend with dark theme and SSE (M14-T02) |
| 2.6 | SSE Real-Time Web UI | DONE | 1,035 | `1ef0a3d` feat(dashboard): add dashboard backend with SSE and demo control (M14-T01); `95444c7` feat(dashboard): add HTMX frontend with dark theme and SSE (M14-T02); `fd807e2` feat(dashboard): add integration tests, docs, and changelog (M14-T03) |
| 2.7 | Docker Compose Full Stack | DONE | -- | `55c29bb` feat(docker): add multi-stage Dockerfile and configure compose for dev; `213de97` feat(docker): multi-stage build for broker and sidecar targets; `884f493` feat(deploy): Add Docker Compose configuration for local development; `55f0e8b` feat(deploy): Create multi-stage Dockerfile for AgentAuth broker |
| 2.8 | Python Test Suite (71% coverage) | DONE | 2,652 | `6e2a514` feat(resource-server): add integration tests and module docs (M11-T03); `3fa3ff4` docs(demo-agents): add integration tests and M12 documentation (M12-T06); `fd807e2` feat(dashboard): add integration tests, docs, and changelog (M14-T03) |

**8/8 complete**

---

## Phase 3: Runtime Hardening

**COMPLETED** | Feb 16-17, 2026

| # | Feature | Status | Git Evidence |
|---|---------|--------|--------------|
| 3.1 | Runtime Scope Narrowing | DONE | `c9e3eae` feat(sidecar): implement phase4 token exchange with scope attenuation and lineage injection; `6f37229` feat(exchange): add scope format pre-validation and edge case tests; `c024cad` feat: dynamic scope ceiling management with enhanced audit trail |
| 3.2 | Immediate Revocation on Policy Violation | DONE | `4c69108` feat(revoke): add revocation service with 4-level support; `f7ce4e0` feat(authz): integrate revocation check into validation middleware; `12a5af6` feat(authz): wire audit recording into all middleware denial paths |
| 3.3 | CLI Ceiling Management (4 Commands) | DONE | `c024cad` feat: dynamic scope ceiling management with enhanced audit trail |
| 3.4 | FIX-001: Wildcard Ceiling for Compound Scopes | DONE | `8970a02` fix(admin): multi-scope sidecar activation -- AllowedScopePrefix to AllowedScopes; `c024cad` feat: dynamic scope ceiling management with enhanced audit trail |
| 3.5 | FIX-002: Broad + Narrowed Scopes at Registration | DONE | `c9e3eae` feat(sidecar): implement phase4 token exchange with scope attenuation and lineage injection; `5921750` fix(exchange): add empty sidecar_id derivation guard (defense-in-depth) |

**5/5 complete**

---

## Phase 4: Demo Polish & Data Expansion

**ACTIVE** | Feb 17+, 2026

| # | Feature | Status | Notes | Git Evidence |
|---|---------|--------|-------|--------------|
| 4.1 | Persist Audit Log to SQLite | **DONE** | Merged via `feature/p0-audit-persistence-and-fixes`. Audit events survive broker restarts. `AA_DB_PATH` configurable. Health endpoint returns `db_connected` and `audit_events_count`. | `4c2733d` feat(p0): SQLite audit persistence, health enhancements, and observability; `9290e9d` Merge feature/p0-audit-persistence-and-fixes into develop |
| 4.2 | Sidecar ID Auto-Discovery | **DONE** | Sidecar health now returns `sidecar_id`. SDK has `get_sidecar_id()` and `get_broker_db_status()`. Integration tests passing. | `50a2809` feat(sidecar): add exchange/denial metrics + enhance health endpoint; `9eaf773` feat(sidecar): add lastRenewal and startTime to sidecar state; `3a0677a` feat(sidecar): add observability -- structured logging + Prometheus metrics |
| 4.3 | Docker Compose pulls from GitHub repo | **DONE** | Broker/sidecar build from `devonartis/agentAuth` develop branch via `gh repo clone`. No local broker code in app repo. | `213de97` feat(docker): multi-stage build for broker and sidecar targets; `55c29bb` feat(docker): add multi-stage Dockerfile and configure compose for dev |
| 4.4 | Orders / Transactions Database | PLANNED | Second data source for cross-database scoping. | -- |
| 4.5 | 4 New Order Tools | PLANNED | `get_customer_orders`, `get_order_detail`, `get_invoice`, `issue_refund`. | -- |
| 4.6 | 4 New Admin/Internal Tools | PLANNED | `search_audit_log`, `get_system_metrics`, `export_customer_data`, `flag_for_review`. | -- |
| 4.7 | Run All 18 User Stories | PLANNED | 6 legitimate + 8 attack + 4 operator workflows. | -- |
| 4.8 | Decompose pipeline.py (1,019 -> 6 modules) | PLANNED | Refactor for maintainability. | -- |
| 4.9 | Automated Scope Narrowing Tests | PLANNED | Unit tests for `_scope_matches_any()`, registration scope building, `_enforce_tool_call()`. | -- |
| 4.10 | Attack Simulation Test Suite (12 scenarios) | PLANNED | Real adversarial tests. | -- |

**3/10 complete**

---

## Phase 5: Production Readiness

**UPCOMING** | Target: Q2 2026

| # | Feature | Status | Backlog Ref | Git Evidence |
|---|---------|--------|-------------|--------------|
| 5.1 | Real Authentication (Session/JWT/SSO) | PLANNED | Backlog #8. Current `_authenticate_user()` is a mock. | -- |
| 5.2 | Ceiling Request Workflow | PLANNED | Backlog #4. Developer requests scopes, operator approves/denies. | -- |
| 5.3 | List Active Sidecars & Agents | PLANNED | Backlog #5, #6. `GET /v1/admin/sidecars` and `GET /v1/agents`. | -- |
| 5.4 | RBAC on Admin API | PLANNED | Separate operator, security, and compliance roles. | -- |
| 5.5 | HA / Clustering | PLANNED | No single point of failure for the broker. | -- |
| 5.6 | Real Database Adapter | PLANNED | Replace in-memory data with PostgreSQL or MongoDB. | -- |
| 5.7 | CI/CD Pipeline | PLANNED | Automated build, test, deploy. | -- |
| 5.8 | External Security Audit | PLANNED | Third-party review of credential system. | -- |
| 5.9 | Rate Limiting on Ceiling Updates | PLANNED | Backlog #11. | -- |
| 5.10 | Dashboard: Show Scope Narrowing | PLANNED | Backlog #10. | -- |
| 5.11 | Ceiling Change Audit History in CLI | PLANNED | Backlog #9. | -- |
| 5.12 | Operator Docs: Runtime Ceiling Management | PLANNED | Backlog #3. | -- |

**0/12 complete**

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
| Git Commits Mapped | -- | 80+ | (new in this version) |

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
- `broker/` directory removed from app repo. Broker now builds from `devonartis/agentAuth` GitHub repo directly. Clean separation between the Go security engine and the Python showcase app.

---

## Git Commit Summary by Phase

### Phase 0 — Architecture & Design
| Commit | Summary |
|--------|---------|
| `e129a3c` | Initial AgentAuth M00-M03 scaffold |
| `9a86fe3` | ADR-002: prioritize MVP requirements |
| `2270c9e` | ADR-003: SPIFFE-compatible ID decision |

### Phase 1 — Go Broker & Sidecar
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
| `55c29bb` | Docker: multi-stage Dockerfile + compose |
| `213de97` | Docker: multi-stage build for broker + sidecar |
| `884f493` | Docker Compose for local development |

### Phase 3 — Runtime Hardening
| Commit | Summary |
|--------|---------|
| `c024cad` | Dynamic scope ceiling management with audit trail |
| `c9e3eae` | Token exchange with scope attenuation + lineage |
| `8970a02` | Fix: multi-scope sidecar activation |
| `12a5af6` | Wire audit recording into all middleware denial paths |
| `204787b` | Audit recording on delegation attenuation violation |
| `3b9feb3` | Enrich scope ceiling denial log with audit fields |

### Phase 4 — Demo Polish (completed items only)
| Commit | Summary |
|--------|---------|
| `4c2733d` | P0: SQLite audit persistence, health, observability |
| `9290e9d` | Merge p0-audit-persistence-and-fixes into develop |
| `50a2809` | Sidecar: exchange/denial metrics + health endpoint |
| `3a0677a` | Sidecar: observability -- structured logging + Prometheus |

---

## Remaining Gaps (Prioritized)

### Must-Have for Demo (Phase 4 remaining)

1. **Orders database + 4 order tools** -- Without a second data source, the cross-database scoping story is incomplete. This is the strongest demo differentiator.
2. **18 user stories end-to-end** -- 6 happy path + 8 attacks + 4 operator workflows. Proves the system works under both normal and adversarial conditions.
3. **Attack simulation tests** -- Automated adversarial tests that validate the audit trail catches every intrusion attempt.
4. **Scope narrowing unit tests** -- Zero automated tests exist for the core security logic.

### Must-Have for Production (Phase 5 critical path)

5. **Real authentication** -- The mock `_authenticate_user()` is a security gap. Identity must come from session/JWT, not ticket text.
6. **Ceiling request workflow** -- Without this, scope management is ad-hoc and unaudited.
7. **List sidecars/agents** -- Operators can't manage what they can't see.
8. **CI/CD pipeline** -- No automated quality gate today.

### Nice-to-Have (Phase 5/6)

9. Pipeline refactor (1,019 LOC -> 6 modules)
10. RBAC on admin API
11. HA / Clustering
12. External security audit
