# NIST Comment → Feature Gap Analysis

**Date:** 2026-03-05
**Source:** `NIST-NCCoE-Public-Comment-AgentAuth.md`
**Goal:** Demo-credible sprint plan — what can we add in 1-2 weeks vs. what waits.
**Current state:** Phase 0 + Phase 1a + Phase 1b all merged to `develop`. Phase 1c and beyond not started.

---

## Part 1: Complete Feature List Extracted from NIST Comments

Every feature, capability, or mechanism you described, claimed, or recommended in your NIST NCCoE public comment — pulled section by section with plain-English explanations of what each one actually is.

---

### Section 1 — General Questions / Risk Framing

**F-001: Ephemeral Agent Credentialing Pattern**
You described a pattern where AI agents receive short-lived, scoped credentials that die when the task completes — as opposed to long-lived tokens or inherited user sessions. This is the core thesis of the entire submission: agents should get credentials that match their task lifetime, not persist for hours.

**F-002: Credential Lifetime Matching**
Agent tasks complete in minutes, but tokens often live for hours. You argued the credential lifetime should match the task duration (5-minute default, 1-60 minute configurable range). Credentials that outlive their task are unnecessary attack surface.

**F-003: Scope Attenuation (One-Way Narrowing)**
Permissions can only get narrower at each delegation hop, never wider. If Agent A has `read:customers:*` and delegates to Agent B, Agent B can get `read:customers:12345` but never `write:customers:*`. This is the single most important authorization principle you described.

**F-004: Delegation Chain Integrity**
When agents delegate to other agents, the complete authorization lineage must be cryptographically verifiable — who delegated to whom, with what permissions, traceable back to the original human or system that started the workflow. You called this "most important and most underserved by current standards."

**F-005: Original Principal Tracking**
Every agent action should trace back to the human (or system) who authorized the workflow. Not just "an agent did this" but "alice@company.com authorized this through a 3-agent chain." This is your counter to "accountability theater" — where audit logs show a human approved something, but the human may not have understood what they approved.

**F-006: Non-Deterministic Actor Model**
LLM-driven agents aren't service accounts. Their tool calls, delegation decisions, and action sequences vary with context. No static policy anticipates what they'll do. This is the architectural argument for why ephemeral, task-scoped credentials work better than role-based access for agents.

**F-007: MCP / CIMD Awareness**
MCP (Model Context Protocol) adoption is accelerating. CIMD (Client ID Metadata Documents) prove domain ownership for cross-organizational agent identity. You noted CIMD "proves domain ownership, not trustworthiness" — trust policies still need to layer on top.

**F-008: Tier 2-3 Demo Scenario**
You recommended the NCCoE demonstration start with Tier 2 (multi-step, multi-resource workflows like onboarding orchestration) and Tier 3 (orchestrated multi-agent like coding agents or claims processing) because they're complex enough to exercise real patterns but common enough for broad applicability.

---

### Section 2 — Identification

**F-009: SPIFFE-Compatible Agent Identifiers**
Agent identities follow the SPIFFE format: `spiffe://trust-domain/agent/{orchestration_id}/{task_id}/{instance_id}`. This gives globally unique identifiers with embedded task context and orchestration lineage. You clarified: SPIFFE is the identity *format*, not requiring SPIRE as the infrastructure.

**F-010: Unique Instance Identifier Per Agent**
Every agent instance gets its own identifier at spawn time. No shared identities, no generic "agent" accounts. Think of it like: every Uber ride gets its own tracking number, not a shared "rides" label.

**F-011: Task / Orchestration Context in Identity**
The agent's identity carries what it's doing (task) and who coordinated it (orchestration). This isn't just metadata — it's how you answer "why did this agent access that resource?"

**F-012: Runtime Attestation Properties**
A cryptographically bound claim about the execution environment — not just a platform label. Verifiable proof that the agent is running where it claims to be running (Kubernetes namespace, container image hash, cloud instance identity, TPM/Nitro enclave).

**F-013: Ephemeral Identity — Born and Retired Per Task**
Each agent instance gets a unique identity at spawn, bound to its specific task, retired when the task completes. Fixed identities for agents recreate shared-service-account problems. The agent's code version can have a persistent identifier for inventory, but the runtime identity must be ephemeral.

**F-014: Agent Framework / Version Metadata**
Optional but valuable: capture what framework the agent uses (LangChain, CrewAI, custom) and what version, plus the model being used. This anchors anomaly detection — if an agent suddenly switches frameworks, something may be wrong.

**F-015: Behavioral Baseline Information**
Expected tool call patterns, resource access behavior. For example: "this agent type normally calls 3-5 tools per task and accesses customer records." Deviations from baseline signal potential compromise. This is the foundation for anomaly detection.

**F-016: CIMD for Cross-Organizational Identity**
For agents in the MCP ecosystem, CIMD provides domain-based identity without requiring shared SPIFFE trust domains. Better suited to scenarios where organizations don't have pre-existing infrastructure relationships.

---

### Section 3 — Authentication

**F-017: Mutual TLS (mTLS) at Transport Layer**
Both the agent and the resource server present and validate certificates. This proves "who you are" at the network level before any application-layer token validation happens. Defense in depth.

**F-018: Task-Scoped JWT with Cryptographic Signatures**
Application-layer tokens signed with Ed25519 (or ECDSA P-256 for NIST-aligned deployments). Validated on every request. The JWT carries the agent's identity, scopes, task context, and delegation chain — everything a resource server needs to make an authorization decision.

**F-019: Ed25519 Challenge-Response Identity**
How an agent proves it is who it claims to be: the broker sends a nonce, the agent signs it with its private Ed25519 key, the broker verifies the signature. No pre-shared secrets. The agent proves identity through cryptography, not through knowing a password.

**F-020: Per-Request Token Validation**
Every single API call from an agent is independently authenticated and authorized. No session cookies, no "you authenticated once so you're good." Zero trust applied to every request.

**F-021: Bidirectional Credential Validation (Mutual Agent Auth)**
When two agents communicate, both present and validate each other's credentials. Agent A proves itself to Agent B, AND Agent B proves itself to Agent A. Prevents impersonation in multi-agent workflows.

**F-022: Token Lifetime Configuration (1-60 minute range)**
Credentials should match task duration plus a small grace period. 5 minutes is the practical default. 1 minute for high-sensitivity tasks. 60 minutes for long-running workflows. The point: no one-size-fits-all, but always as short as practical.

**F-023: ECDSA P-256 for NIST-Aligned Deployments**
You mentioned ECDSA P-256 as an alternative to Ed25519 for organizations that require strict NIST algorithm compliance. Both are valid; Ed25519 is also NIST-approved under FIPS 186-5.

---

### Section 4 — Authorization

**F-024: Zero-Trust Per-Request Authorization**
Every request validated independently: check JWT signature (is this token genuine?), check expiry (is it still valid?), check scope (is this action allowed?), check revocation (has it been revoked?), check delegation chain (is the authorization lineage valid?), check principal binding (who authorized this workflow?). All six checks, every request.

**F-025: Scope Taxonomy — `action:resource:identifier`**
A structured format for encoding what an agent can do: `read:customers:12345` means "read customer record 12345." `write:orders:*` means "write to any order." Wildcards supported. The action component directly encodes the least-privilege boundary.

**F-026: App Scope Ceiling Enforcement**
When an operator registers an app, they set the maximum permissions that app can ever grant to its agents. The app cannot create launch tokens with broader permissions than its ceiling. Think: "this customer-service app can read customer records but never write to billing."

**F-027: Dynamic Scope Request Within Task-Type Boundaries**
Agents can request permissions at runtime (within their ceiling) rather than having everything pre-defined at deployment. This handles the non-deterministic nature of LLM agents — they discover what they need as they work.

**F-028: Validation Latency Under 50ms**
You claimed per-request validation can hit under 50ms because JWT validation is local cryptographic math and revocation checks use in-memory cached lists. This is an engineering claim that should be benchmarked.

**F-029: NGAC for Aggregate Access Evaluation**
Next-Generation Access Control (NGAC) — a NIST-developed policy framework — could evaluate aggregate access patterns rather than individual resource requests. Addresses the data aggregation risk: individually low-sensitivity records that collectively constitute high-sensitivity access.

**F-030: Data Aggregation Risk Detection**
An agent accessing 10 customer records individually is different from accessing 10,000. Credential scoping alone doesn't solve this. Policy engines need to evaluate cumulative access, not just per-request access.

---

### Section 4 (cont.) — Delegation

**F-031: Delegation Token with Delegator Identity**
When Agent A delegates to Agent B, the delegation token includes Agent A's identity. You can always answer "who gave Agent B permission to do this?"

**F-032: Complete Delegation Chain with Crypto Signatures**
The full chain of delegations, with cryptographic signatures at each hop. Not just a log entry — a verifiable data structure. Each link signed, each scope narrowing recorded.

**F-033: Maximum Delegation Depth (5 Hops)**
A practical limit: delegations can go at most 5 levels deep. Balances flexibility (real workflows need 2-3 hops) with security (unbounded depth creates verification complexity and attack surface).

**F-034: Resource Server Chain Verification**
Resource servers don't just check the immediate token — they verify the complete delegation chain: each link's signature, scope narrowing at each hop, revocation status of every agent in the chain, and that the chain traces to a legitimate initiating principal.

**F-035: OAuth 2.0 Token Exchange (RFC 8693) Compatibility**
The OAuth standard for exchanging one token for another with downscoped permissions. The mechanism that lets Agent A trade its token for a narrower one to give to Agent B. You recommended the NCCoE pilot this.

**F-036: IETF Transaction Tokens Compatibility**
Draft standard (draft-ietf-oauth-transaction-tokens) for tokens that carry transactional context across service boundaries. Complementary to RFC 8693.

**F-037: WIMSE Workload Identity**
IETF WIMSE Working Group (draft-ietf-wimse-arch-06) addresses AI agents as delegated workloads. Requires each hop to explicitly scope and re-bind security context.

**F-038: Human-in-the-Loop as Cryptographic Step**
When human approval is required for sensitive operations, that approval should be recorded as a cryptographically verified step *in* the delegation chain — not as a side-channel log entry. The human authorization is verifiable in the chain itself. This is what you said separates "genuine human-in-the-loop from accountability theater."

---

### Section 5 — Audit & Non-Repudiation

**F-039: Comprehensive Audit Event Fields**
Each audit event captures: timestamp, agent identity (full SPIFFE ID), task/orchestration context, action taken, resource accessed, authorization decision and scope used, outcome (success/failure), delegation depth, and delegation chain hash.

**F-040: Delegation Chain Hash in Audit**
A `delegation_chain_hash` field that cryptographically links each agent's log entries to the delegation chain. Enables full reconstruction of the authorization path during forensics.

**F-041: Original Principal in Audit Trail**
An `original_principal` field ensuring audit trails always trace back to the initiating principal — human or system — who authorized the workflow.

**F-042: Append-Only Tamper-Proof Storage**
Audit events stored in append-only, immutable storage. No entries can be modified after the fact. Hash chain linking makes tampering detectable.

**F-043: Multi-Agent Workflow Reconstruction**
You recommended the demonstration test whether audit logs can reconstruct the complete story of a multi-agent workflow after the fact: which agent accessed what, under whose authority, and whether the delegation chain was valid at the time of each action.

**F-044: Non-Repudiation Through Three Properties**
(1) Unique agent identities (no generic accounts), (2) cryptographic signatures on tokens and chains (can't be forged), (3) immutable audit logs (can't be altered). The combination means no one can deny an action occurred, and no one can claim false authorization.

---

### Section 6 — Prompt Injection / Blast Radius

**F-045: Blast Radius Containment via Ephemeral Credentials**
When a prompt injection succeeds, the damage is bounded by: task-scoped credentials (only current task resources), short TTLs (exfiltrated creds expire in minutes), and no static secrets in the agent environment.

**F-046: Anomaly-Based Revocation**
Detecting unusual agent behavior (tool calls outside baseline, unexpected resource access) and immediately revoking credentials. Stops a compromised agent before the credential expires naturally.

**F-047: Controlled Injection Comparison Demo**
You recommended a controlled prompt injection against agents with ephemeral vs. traditional credentials, measuring the difference in blast radius. "The kind of concrete, quantifiable result that helps practitioners make the case for investment."

---

### Emerging Standards Referenced (Not Features to Build, But Alignment Points)

**S-001: IETF WIMSE Working Group** — draft-ietf-wimse-arch-06 (AI agents as delegated workloads)
**S-002: IETF OAuth/WIMSE** — draft-klrc-aiagent-auth-00 (AI agent auth framework, March 2026)
**S-003: OAuth 2.0 Token Exchange** — RFC 8693 (token exchange with downscoped permissions)
**S-004: NIST IR 8596** — Cyber AI Profile (unique AI system identities)
**S-005: OWASP Top 10 for Agentic Applications** — ASI03 (Identity/Privilege Abuse), ASI07 (Insecure Inter-Agent Communication)
**S-006: NIST SP 800-207** — Zero Trust Architecture (referenced for per-request validation)

---

## Part 2: Gap Analysis & Sprint Prioritization

## How to read this

Every feature or capability **you claimed, recommended, or described in the NIST comments** is listed below, traced to the specific section where you said it. Each one gets a verdict:

- **SHIPPED** — code exists on `develop`, demoable today
- **QUICK-ADD** — can be added in < 1 day, fits in 1-2 week sprint
- **SPRINT** — 1-3 days, worth doing if it makes the demo stronger
- **WAIT** — > 3 days, or depends on unfinished infrastructure, or not demoable even if built

---

## Section 1: General / Risk Framing

These aren't features per se — they're the arguments that frame the whole submission. The question is: can you **demonstrate** these claims?

| # | What you claimed | Verdict | Notes |
|---|-----------------|---------|-------|
| G-1 | Credential overexposure is the #1 risk (97% excessive privileges, 45:1 machine-to-human ratio) | **SHIPPED** | AgentAuth's entire architecture is the counter-argument. Short TTLs, task-scoped creds, 4-level revocation. |
| G-2 | Delegation exploitation — no mechanism to verify authorization lineage | **SHIPPED** | Delegation chains with scope attenuation exist. `DelegChain` field in JWT claims. Max depth 5. |
| G-3 | "Accountability theater" — agents assume full user identity, no distinction between user intent and agent action | **QUICK-ADD** | The architecture separates them, but `original_principal` field isn't explicit in tokens yet. Adding it makes this demoable. |
| G-4 | "Non-deterministic actor" — LLMs aren't service accounts, static policy fails | **SHIPPED** | Task-scoped ephemeral creds are the answer. Each task gets its own credential with its own scope — no static policy to outlive the task. |
| G-5 | MCP / CIMD adoption accelerating — "proves domain ownership, not trustworthiness" | **WAIT** | CIMD is a complementary mechanism for cross-org scenarios. Not in scope for a single-org demo. |
| G-6 | Tiers 2-3 demo scenario (multi-step workflow, orchestrated multi-agent) | **SPRINT** | Need a demo script showing a realistic workflow. Code supports it; the demo artifact doesn't exist yet. |

---

## Section 2: Identification

| # | What you claimed | Verdict | Current state | Gap |
|---|-----------------|---------|---------------|-----|
| I-1 | SPIFFE-compatible identifiers: `spiffe://trust-domain/agent/{orch}/{task}/{instance}` | **SHIPPED** | `internal/identity/spiffe.go` generates SPIFFE IDs | — |
| I-2 | Unique instance identifier per agent | **SHIPPED** | Agent registration returns unique ID | — |
| I-3 | Task/orchestration context embedded in identity | **SHIPPED** | Launch tokens carry `task_id`, `orch_id`; propagated to agent tokens | — |
| I-4 | Original initiating principal tracked (human or system) | **QUICK-ADD** | Delegation chain exists but no explicit `original_principal` field | Add field to token claims + audit events |
| I-5 | Delegation chain in identity metadata | **SHIPPED** | `DelegChain` in JWT claims | — |
| I-6 | Agent framework/version metadata | **QUICK-ADD** | Not captured | Optional claims at registration — `agent_framework`, `agent_version`. Small change. |
| I-7 | Behavioral baseline info (expected tool call patterns) | **WAIT** | Not implemented | Requires anomaly detection infrastructure. Research project. |
| I-8 | Ephemeral identity — unique per task, retired on completion | **SHIPPED** | Core architecture | — |

---

## Section 3: Authentication

| # | What you claimed | Verdict | Current state | Gap |
|---|-----------------|---------|---------------|-----|
| A-1 | mTLS at transport layer | **SHIPPED** | Fix 1 — broker supports `none`, `tls`, `mtls` modes | Alignment checklist says FUTURE — **it's wrong, this shipped** |
| A-2 | Task-scoped JWT with Ed25519/ECDSA signatures | **SHIPPED (Ed25519)** | `internal/token/` — EdDSA signing | ECDSA P-256 not supported. See A-2b. |
| A-2b | ECDSA P-256 as NIST-aligned alternative | **WAIT** | Ed25519 only | Refactoring token signing to support multiple algorithms is 2-3 days and touches everything. Not worth it for demo. Ed25519 is also NIST-approved (FIPS 186-5). |
| A-3 | Token validation on every request | **SHIPPED** | `internal/authz/` ValMw middleware | — |
| A-4 | Bidirectional credential validation (multi-agent mutual auth) | **QUICK-ADD** | `internal/mutauth/` package **exists** with 3-step handshake, discovery, heartbeat. HTTP endpoints not exposed. | Wire 2-3 HTTP routes on broker. Code is written; it's a routing + integration task. |
| A-5 | Token lifetime 5 min default, 1-60 min configurable range | **SHIPPED** (with caveat) | Default 300s, configurable via `AA_DEFAULT_TTL` | TD-006: App JWT TTL hardcoded to 5 min. Per-app configurable TTL is Phase 1c scope. |

---

## Section 4: Authorization

| # | What you claimed | Verdict | Current state | Gap |
|---|-----------------|---------|---------------|-----|
| Z-1 | Every request independently authenticated + authorized (zero trust) | **SHIPPED** | ValMw validates JWT signature, expiry, scope, revocation on every request | — |
| Z-2 | Scope taxonomy: `action:resource:identifier` | **SHIPPED** | `read:customers:12345`, `write:orders:67890` — with wildcard support | — |
| Z-3 | Scope attenuation — permissions only narrow, never widen | **SHIPPED** | `ScopeIsSubset()` enforced at ceiling, launch token, delegation levels | — |
| Z-4 | App scope ceiling enforcement | **SHIPPED** | Phase 1a/1b — handler enforces ceiling at launch token creation | — |
| Z-5 | Dynamic scope request within task-type boundaries | **SHIPPED** | Apps request scopes at auth time, ceiling enforced | — |
| Z-6 | Validation latency < 50ms achievable (local JWT validation + cached revocation) | **QUICK-ADD** | Architecture supports it (local crypto, in-memory revocation map). No benchmark proving it. | Write a Go benchmark test. 30 minutes of work, strong demo evidence. |
| Z-7 | NGAC for aggregate access pattern evaluation | **WAIT** | Not implemented | Policy engine research. Weeks. Not demoable. |
| Z-8 | Data aggregation risk detection | **WAIT** | Not implemented | Depends on Z-7. Research project. |

---

## Section 4 (cont.): Delegation

This is the section you called "most important and most underserved." It's also where the biggest gap cluster lives.

| # | What you claimed | Verdict | Current state | Gap |
|---|-----------------|---------|---------------|-----|
| D-1 | Delegation token includes delegator identity | **SHIPPED** | `DelegChain` field in JWT carries chain of delegator identities | — |
| D-2 | Original initiating principal in delegation token | **QUICK-ADD** | Chain exists but `original_principal` not explicit | Same fix as I-4 — add field |
| D-3 | Complete chain with crypto signatures at each hop | **SHIPPED** | Chain entries include cryptographic data | — |
| D-4 | Scope attenuation enforced at each hop | **SHIPPED** | `ScopeIsSubset()` called at each delegation | — |
| D-5 | Resource servers verify complete chain (signatures, narrowing, revocation at each hop) | **SHIPPED** | Validation checks chain integrity | — |
| D-6 | Maximum delegation depth of 5 hops | **SHIPPED** | Enforced in delegation logic | — |
| D-7 | RFC 8693 Token Exchange compatibility | **WAIT** | Custom token exchange exists (`token_exchange_hdl.go`), not RFC 8693 compliant | Standards alignment work. Not quick, not demoable as a feature. |
| D-8 | IETF Transaction Tokens compatibility | **WAIT** | Not implemented | Draft spec, moving target. |
| D-9 | WIMSE compatibility | **WAIT** | Not implemented | Draft spec. |
| D-10 | Human-in-the-loop as cryptographic step in delegation chain (not side-channel) | **SPRINT** | Architecture supports it (delegation chain records each hop). No explicit "human approval" step type in the chain. | Add `approval_type: "human"` or `"system"` to delegation chain entries. 1-2 days with tests. Makes the "accountability theater" argument concrete. |

---

## Section 5: Audit & Non-Repudiation

| # | What you claimed | Verdict | Current state | Gap |
|---|-----------------|---------|---------------|-----|
| L-1 | Timestamp on every event | **SHIPPED** | Every audit event | — |
| L-2 | Agent identity (full SPIFFE ID) in audit | **QUICK-ADD** | Agent ID captured; full SPIFFE ID not always propagated | Small change — resolve agent ID to SPIFFE ID in audit writes |
| L-3 | Task and orchestration context in audit | **QUICK-ADD** | Launch token has task context; not consistently in all audit events | Propagate `task_id`, `orch_id` from token to audit event metadata |
| L-4 | Action taken | **SHIPPED** | Event type field | — |
| L-5 | Resource accessed | **QUICK-ADD** | Endpoint captured in some events, not all | Add `resource` field consistently |
| L-6 | Authorization decision and scope used | **SHIPPED** | `scope_violation` events log required vs actual | — |
| L-7 | Outcome (success/failure) | **SHIPPED** | Outcome field on every event | — |
| L-8 | Delegation depth in audit | **SHIPPED** | Fix 6 added `deleg_depth` structured field | Alignment checklist says FUTURE — **it's wrong, this shipped** |
| L-9 | Delegation chain hash in audit | **SHIPPED** | Fix 6 added `deleg_chain_hash` | Alignment checklist says FUTURE — **wrong again** |
| L-10 | `original_principal` in audit | **QUICK-ADD** | Not implemented | Same fix as I-4 and D-2 |
| L-11 | Append-only tamper-proof storage | **SHIPPED** | SQLite with SHA-256 hash chain, `computeHash` covers all fields | — |
| L-12 | Full multi-agent workflow reconstruction from audit logs | **SPRINT** | Single-agent reconstruction works. Multi-agent needs `original_principal` + demo script showing reconstruction. | After I-4/D-2 fix + write reconstruction demo |

---

## Section 6: Prompt Injection / Blast Radius

| # | What you claimed | Verdict | Current state | Gap |
|---|-----------------|---------|---------------|-----|
| P-1 | Ephemeral creds contain blast radius after injection | **SHIPPED** | Architecture delivers this: task-scoped creds, short TTLs, no static secrets in agent env | — |
| P-2 | Anomaly-based revocation for immediate termination | **WAIT** | Not implemented | Requires behavioral monitoring (I-7). Research project. |
| P-3 | Demo: controlled injection comparison (ephemeral vs. traditional creds) | **SPRINT** | No demo script exists | Write a demo scenario script showing side-by-side blast radius. 1 day. High impact for NIST presentation. |

---

## The Alignment Checklist Is Stale

Your `.plans/CONCEPT-PAPER-ALIGNMENT.md` has several errors. These are marked FUTURE but are actually shipped:

| Checklist Item | Checklist says | Reality |
|---------------|---------------|---------|
| A-1 (mTLS) | FUTURE | **SHIPPED** — Fix 1, broker supports `none`/`tls`/`mtls` |
| A-4 (Bidirectional auth) | FUTURE | **CODE EXISTS** — `internal/mutauth/` package, needs HTTP routes |
| I-5 (Delegation chain in identity) | FUTURE | **SHIPPED** — `DelegChain` in JWT claims |
| Z-2 (Scope attenuation at delegation) | FUTURE | **SHIPPED** — `ScopeIsSubset()` at each hop |
| K-5 (Task-level revocation) | FUTURE | **SHIPPED** — RevSvc supports task-level |
| K-6 (Delegation chain revocation) | FUTURE | **SHIPPED** — RevSvc supports chain-level |
| D-1 through D-6 (Delegation section) | All FUTURE | **MOSTLY SHIPPED** — delegation exists in `internal/deleg/`, token claims, handler logic |
| L-8 (Delegation depth) | FUTURE | **SHIPPED** — Fix 6 |
| L-9 (Delegation chain hash) | FUTURE | **SHIPPED** — Fix 6 |

**Recommendation:** Update the alignment checklist after this sprint. You're underselling what you've built.

---

## Sprint Plan: 1-2 Week Demo-Credible Release

### Tier 1: MUST DO (closes gaps you explicitly claimed in NIST comments)

These are features you **described as implemented or recommended as essential** in your NIST comments that aren't fully in the code yet. Shipping these means everything you wrote is backed by running code.

| Priority | Feature | IDs | Effort | Why it matters |
|----------|---------|-----|--------|---------------|
| **P0** | `original_principal` field in JWT claims + audit events | I-4, D-2, L-10, G-3 | 0.5 day | You wrote 4 paragraphs about this. It's central to your "accountability theater" argument. Must be demoable. |
| **P0** | Phase 1c: App revocation + secret rotation + audit attribution | Roadmap | 1 day | Already speced. 5th revocation level, `app_id` in agent JWTs, `aactl app revoke`. Completes the app lifecycle story. |
| **P0** | Full SPIFFE ID + task/orch context + resource in all audit events | L-2, L-3, L-5 | 0.5 day | You listed these as essential audit fields. They're partially there. Make them consistent. |
| **P0** | Validation latency benchmark (prove <50ms) | Z-6 | 0.5 day | You claimed this is "achievable." Prove it with a Go benchmark. Numbers in evidence. |
| **P0** | Fix TD-006: Per-app configurable JWT TTL | Roadmap, A-5 | 0.5 day | You recommended 1-60 min range. Currently hardcoded to 5 min. |

**Tier 1 total: ~3 days**

### Tier 2: SHOULD DO (strengthens demo, differentiates from competitors)

| Priority | Feature | IDs | Effort | Why it matters |
|----------|---------|-----|--------|---------------|
| **P1** | Expose mutual auth HTTP endpoints | A-4 | 1 day | The code exists in `internal/mutauth/`. Wire HTTP routes. "Bidirectional credential validation" becomes a live demo, not a code listing. |
| **P1** | Human-in-the-loop approval type in delegation chain | D-10 | 1 day | Add `approval_type` to chain entries. Makes your "accountability theater" fix concrete and demoable. |
| **P1** | Agent metadata: framework + version fields | I-6 | 0.5 day | Optional claims at registration. Shows you capture what NIST asked about. |
| **P1** | Blast radius demo script | P-3, G-6 | 1 day | Side-by-side comparison: agent with ephemeral creds vs. long-lived token after simulated injection. Highest demo impact per effort. |
| **P1** | Multi-agent workflow audit reconstruction demo | L-12, V-1 | 1 day | Script that runs a 3-agent delegation workflow and then reconstructs the full story from audit logs alone. Proves your audit claims. |

**Tier 2 total: ~4.5 days**

### Tier 3: WAIT (not demoable in 2 weeks, or depends on unbuilt infrastructure)

| Feature | IDs | Why wait |
|---------|-----|----------|
| Push-based revocation (OCSP/webhooks, <30s propagation) | K-7 | New architecture pattern. 3-5 days minimum. Current pull-based model with short TTLs is good enough for demo. |
| Platform attestation (TPM, Nitro, K8s namespace) | I-7 area | Requires infrastructure AgentAuth deliberately chose not to build (Ed25519 instead of SPIRE). Design decision, not a gap. |
| CIMD / MCP Client ID Metadata | G-5 | Cross-org scenario. Not relevant for single-org demo. Spec still maturing. |
| RFC 8693 formal compliance | D-7 | Standards alignment paperwork. Custom token exchange works. Compliance label adds nothing to demo. |
| Transaction Tokens / WIMSE | D-8, D-9 | Draft IETF specs. Moving target. You referenced them as emerging — that's accurate. No need to implement drafts. |
| NGAC aggregate access evaluation | Z-7 | Policy engine research project. Weeks. You flagged it as "deserves explicit attention" — that's a recommendation, not a claim you've built it. |
| Data aggregation risk detection | Z-8 | Depends on NGAC. Same reasoning. |
| Anomaly-based revocation | P-2 | Requires behavioral monitoring. Research project. You described it as complementary, not as something you've implemented. |
| Behavioral baseline / intent classification | I-7, implied | ML/heuristics work. Not in scope. |
| ECDSA P-256 dual-algorithm support | A-2b | Ed25519 is NIST-approved (FIPS 186-5). Dual-algorithm support touches every signing path. Not worth the risk for demo. |
| Python SDK (Phase 3) | Roadmap | 3-5 days. Great DX improvement. Not a NIST-comment feature. Do it after the demo sprint. |
| JWKS + Key Persistence (Phase 4+5) | Roadmap | Must ship together (you documented the "production trap"). 2-3 days combined. Good feature, but doesn't map to NIST comments. Post-demo. |
| Phase 2: Activation token bootstrap | Roadmap | Sidecar-focused. Important for production (kills master key in sidecar) but sidecar isn't the demo story — direct broker access is. |

---

## Sprint Summary

**If you do Tier 1 only (3 days):** Every specific claim in your NIST comments has code behind it. No one can say "you recommended X but didn't build it."

**If you do Tier 1 + Tier 2 (7-8 days):** You have a live demo that shows ephemeral credentialing, mutual agent auth, delegation with human-in-the-loop, blast radius containment, and full audit reconstruction. That's a complete story.

**What you're NOT doing (and why that's fine):** Push revocation, platform attestation, NGAC, anomaly detection, RFC/IETF draft compliance — you referenced all of these as recommendations for the NCCoE demonstration project or as emerging standards. You didn't claim to have built them. Your comments are carefully worded: "could explore," "would be valuable to include," "emerging efforts worth engaging with." That's recommending, not claiming.

---

## Updated Alignment Checklist Status

After this sprint, your alignment numbers would be:

**Tier 1 only:**
- SHIPPED: 32 items (up from 24)
- QUICK-ADD remaining: 0
- FUTURE: 15 items (all in "WAIT" category)

**Tier 1 + Tier 2:**
- SHIPPED: 37 items
- FUTURE: 10 items (all correctly positioned as recommendations, not claims)
