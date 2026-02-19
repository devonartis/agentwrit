# AgentAuth — Project Roadmap

**Ephemeral Credentialing for AI Agents** · 55,550 Lines of Code · 7 Phases

| Go Source | Go Tests | Py Source | Py Tests | Cost to Recreate | Team |
|-----------|----------|-----------|----------|-----------------|------|
| 6,531 | 9,607 (147%) | 3,720 | 2,652 (71%) | $493K | 3 FT + 2 PT |

---

## Phase 0: Architecture & Design — COMPLETED

**Feb 15, 2026** · Security pattern, threat model, scope algebra

| Feature | Description | Status | Repo | LOC |
|---------|-------------|--------|------|-----|
| Ephemeral Agent Credentialing Pattern (v1.2) | 7-component security pattern: SPIFFE identity, task-scoped JWT tokens, zero-trust enforcement, auto-expiration, hash-chained audit, mutual auth, delegation chain. | Done | Docs | 1,036 |
| Threat Model & Trust Boundaries | Adversary analysis: credential theft, compromised agents, lateral movement, rogue agents, cross-agent privilege escalation. | Done | Docs | — |
| Tool-Based Enforcement Architecture | Root cause: scope system checked what LLM said, not what it did. Solution: executable tools with scope mapping. | Done | Docs | — |
| Cloud IAM Research (AWS / GCP / Azure) | Evaluated SPIFFE/SPIRE, AWS IAM Roles Anywhere, Azure Managed Identity, GCP Workload Identity. | Done | Docs | — |

**Progress: 4/4 complete**

---

## Phase 1: Go Broker & Sidecar — COMPLETED

**Feb 15–16, 2026** · Zero-dependency security broker — the core product

| Feature | Description | Status | Repo | LOC |
|---------|-------------|--------|------|-----|
| JWT Token Service | Creation, validation, renewal, SPIFFE claims, scope embedding. Ed25519/RSA signing. JTI-based revocation. | Done | Go | 301 |
| SPIFFE Identity Service | Agent registration via challenge-response. SPIFFE ID generation. Scope narrowing at registration. | Done | Go | 308 |
| Scope Engine & Wildcard Matching | 3-segment parser (action:resource:identifier). scopeCovers() with wildcard. Validation middleware. Rate limiting. | Done | Go | 353 |
| Admin Service & Ceiling CRUD | Admin auth, sidecar lifecycle, launch tokens, activation tokens, ceiling GET/PUT endpoints. | Done | Go | 804 |
| SHA-256 Hash-Chained Audit Log | Append-only trail, cryptographic hash chain, PII sanitization, 23 event types. Tamper-evidence. | Done | Go | 225 |
| Scope-Attenuated Delegation | Parent→child token with strictly narrower scopes. Cryptographic delegation chain. | Done | Go | 211 |
| Multi-Level Revocation | Revoke token, agent, or sidecar. Cascading. JTI-based with periodic cleanup. | Done | Go | 113 |
| Mutual TLS & Heartbeating | mTLS discovery, heartbeat monitoring, transport-layer authentication handler. | Done | Go | 434 |
| Sidecar Proxy | Per-app HTTP proxy: ceiling enforcement, circuit breaker, renewal goroutine, atomic cache swap. | Done | Go | 2,516 |
| SQLite Persistence | Schema: sidecars, agents, tokens, JTI revocation. Migrations. Zero external deps. | Done | Go | 254 |
| Go Test Suite (147% coverage) | 9,607 lines of tests. Handler, integration, admin, token, delegation tests. | Done | Go | 9,607 |

**Progress: 11/11 complete**

---

## Phase 2: Python Showcase App — COMPLETED

**Feb 16, 2026** · SDK, CLI, pipeline, dashboard — the integration layer

| Feature | Description | Status | Repo | LOC |
|---------|-------------|--------|------|-----|
| Python SDK (5 Sub-Clients) | AgentAuthClient, DeveloperClient, OperatorClient, SecurityClient, DelegationClient. Typed dicts. | Done | Python | 439 |
| Typer CLI (Full Command Suite) | operator, security, developer, demo commands. Interactive mode. | Done | Python | 353 |
| Identity-First Pipeline | Triage → identity → routing → scope provisioning → LLM loop → policy gate → response. | Done | Python | 1,019 |
| Tool System (8 Tools) | OpenAI function-calling format. Scope mapping, customer_bound flag, three enforcement outcomes. | Done | Python | 329 |
| HTMX Dashboard (3 Personas) | Operator, Security, Developer tabs. Health monitoring, audit table, token inspector. | Done | Python | 451 |
| SSE Real-Time Web UI | Single-page HTMX app. Pipeline visualization, agent streaming, scope chain display. | Done | Python | 1,035 |
| Docker Compose Full Stack | Broker + sidecar + Python app. Health checks, env config. Single docker compose up. | Done | Both | — |
| Python Test Suite (71% coverage) | Pipeline, policy gate, SDK, example, integration tests. respx mocking. | Done | Python | 2,652 |

**Progress: 8/8 complete**

---

## Phase 3: Runtime Hardening — COMPLETED

**Feb 16–17, 2026** · Scope narrowing, ceiling management, immediate revocation

| Feature | Description | Status | Repo |
|---------|-------------|--------|------|
| Runtime Scope Narrowing | Broad ceiling + narrow token. Maps to AWS IAM Role + STS session policy model. | Done | Both |
| Immediate Revocation on Policy Violation | Zero tool calls after detection. Closed execution gap. | Done | Python |
| CLI Ceiling Management (4 Commands) | show-ceiling, get-ceiling, update-ceiling, sidecar health. No restart needed. | Done | Both |
| FIX-001: Wildcard Ceiling for Compound Scopes | SplitN(":", 3) mismatch. Ceiling uses wildcards (read:customer:*). | Done | Both |
| FIX-002: Broad + Narrowed Scopes at Registration | Agent gets BOTH broad (for lookups) and narrowed (for customer-bound) scopes. | Done | Python |

**Progress: 5/5 complete**

---

## Phase 4: Demo Polish & Data Expansion — ACTIVE

**Feb 17+, 2026** · Orders DB, new tools, 18 user stories, audit persistence

| Feature | Description | Status | Repo |
|---------|-------------|--------|------|
| Persist Audit Log to SQLite | In-memory Go slice loses events on restart. Add SaveAuditEvent() to SqlStore. **P0.** | **BLOCKED** | Go |
| Orders / Transactions Database | New data source. Cross-database scoping: read own customer's orders only. | Planned | Python |
| 4 New Order Tools | get_customer_orders, get_order_detail, get_invoice, issue_refund. customer_bound. | Planned | Python |
| 4 New Admin/Internal Tools | search_audit_log, get_system_metrics, export_customer_data, flag_for_review. | Planned | Python |
| Run All 18 User Stories | 6 good customer + 8 hacker + 4 admin. Test live, document results. | Planned | Python |
| Sidecar ID Auto-Discovery | Health endpoint returns sidecar_id. CLI auto-discovers. | Planned | Both |
| Decompose pipeline.py (1,019 → 6 modules) | triage, identity, router, scope_provisioner, llm_loop, policy_gate. | Planned | Python |
| Automated Scope Narrowing Tests | Unit + integration tests for scope narrowing logic. | Planned | Python |

**Progress: 0/8 complete · 1 blocked**

---

## Phase 5: Production Readiness — UPCOMING

**Target: Q2 2026** · Real auth, HA, RBAC, CI/CD — enterprise features

| Feature | Description | Status | Repo |
|---------|-------------|--------|------|
| Real Authentication (Session/JWT/SSO) | Replace mock identity. Customer ID from auth layer, never from text. | Planned | Python |
| Ceiling Request Workflow | Developer requests scopes, operator approves/denies. Audit trail. | Planned | Both |
| List Active Sidecars & Agents | GET /v1/admin/sidecars + agents. Multi-sidecar management. | Planned | Both |
| RBAC on Admin API | Separate operator, security, compliance roles. | Planned | Go |
| HA / Clustering | Leader election or shared state. Eliminate SPOF. | Planned | Go |
| Real Database Adapter | PostgreSQL or MongoDB. Replace mock dicts. | Planned | Python |
| CI/CD Pipeline | GitHub Actions: tests, lint, build, Docker images. | Planned | Both |
| External Security Audit | Third-party review of scope engine, tokens, hash chain. | Planned | Both |

**Progress: 0/8 complete**

---

## Phase 6: Market Launch — FUTURE

**Target: Q3 2026** · Open source or SaaS — packaging for the world

| Feature | Description | Status |
|---------|-------------|--------|
| AgentAuth Cloud (Hosted SaaS) | Managed broker + sidecar. Multi-tenant. $99–$2,499/mo. | Planned |
| Open Source Core + Enterprise | MIT core. Enterprise: RBAC, HA, SOC 2. The HashiCorp model. | Planned |
| Multi-Framework SDK | Python + Node + Go. LangChain, CrewAI, AutoGen integrations. | Planned |
| Scope Playground / Simulator | Web UI: test scope configs without running the full system. | Planned |
| Webhooks on Ceiling Change | Slack/Teams notifications. CI/CD triggers. | Planned |
| Anomaly Detection | Behavioral baselines. Auto-revocation on suspicious patterns. | Planned |

**Progress: 0/6 complete**

---

## Progress Summary

| Metric | Value |
|--------|-------|
| Total Features | 51 |
| Completed | 28 (55%) |
| Blocked | 1 |
| Planned | 22 |
| Phases Completed | 4 of 7 |
| Total Lines of Code | 55,550 |
| Go Dependencies | 0 (zero supply chain risk) |
| Go Test Coverage | 147% (9,607 test / 6,531 source) |
| Cost to Recreate | $493,000 |

---

*Branch: coWork/demo-realism-fixes · Updated: February 17, 2026*
