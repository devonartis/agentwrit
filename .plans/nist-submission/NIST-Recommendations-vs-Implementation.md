# NIST Recommendations vs AgentAuth Implementation

Analysis of every recommendation made in the NIST NCCoE Public Comment submission, compared against what AgentAuth actually implements.

**Source:** `.plans/nist-submission/NIST-NCCoE-Public-Comment-AgentAuth.md`
**Date:** 2026-03-05
**Pattern:** [Ephemeral Agent Credentialing v1.2](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.2.md)

---

## Status Key

- **YES** — code does what we recommended
- **PARTIAL** — partially implemented, gap identified
- **NO** — not implemented

## Placement Key

- **Phase 1C** — fits current implementing spec (already opening those files)
- **Tech Debt** — must fix, needs its own spec or gets added to an existing phase
- **Future Spec** — new capability area, needs its own spec

---

## Identity

| # | Recommendation | Status | Gap | Placement |
|---|---------------|--------|-----|-----------|
| 1 | SPIFFE identifiers `spiffe://domain/agent/{orch}/{task}/{instance}` | YES | — | — |
| 2 | Ephemeral identity — unique per task, retired on completion | PARTIAL | Agent records persist forever, never retired | Phase 1C |
| 3 | Cryptographic binding to runtime environment | NO | No platform attestation (TPM, K8s SA, container hash) | Future Spec |
| 4 | CIMD for cross-org identity | NO | No MCP ecosystem integration | Future Spec |
| 5 | Metadata: instance ID, task context, human principal, scope, delegation chain | PARTIAL | `original_principal` not a first-class JWT claim | Phase 1C |
| 6 | Optional metadata: agent framework, model, behavioral baseline | NO | Agent inventory/behavioral monitoring not built | Future Spec |
| 7 | Persistent code/model version separate from runtime identity | NO | No agent registry concept | Future Spec |

## Authentication

| # | Recommendation | Status | Gap | Placement |
|---|---------------|--------|-----|-----------|
| 8 | mTLS at transport layer | PARTIAL | Supported but optional, not default | Future Spec |
| 9 | Ed25519 JWT signatures (ECDSA P-256 for NIST-aligned) | PARTIAL | Ed25519 only, no ECDSA P-256 option | Future Spec |
| 10 | Token validation on every request | YES | — | — |
| 11 | Bidirectional credential validation (multi-agent) | PARTIAL | `internal/mutauth/` exists as Go API, not HTTP-routed | Phase 2 |
| 12 | Token lifetime 5 min default, 1-60 min configurable | PARTIAL | 5 min default yes, per-app configurable no | TD-006 (fix before Phase 1C) |
| 13 | Credentials issued via attestation validation | PARTIAL | Launch token works but is shared secret, not attestation | Future Spec |
| 14 | New credentials rather than refresh | YES | — | — |
| 15 | Push-based revocation (<30s propagation) | NO | No webhook/OCSP infrastructure | Future Spec |
| 16 | Multi-level revocation: token, agent, task, chain | YES | — | — |
| 17 | Chain-level revocation cascades downstream | YES | — | — |

## Authorization

| # | Recommendation | Status | Gap | Placement |
|---|---------------|--------|-----|-----------|
| 18 | Zero-trust per-request auth+authz | YES | — | — |
| 19 | Scope taxonomy `action:resource:identifier` | YES | — | — |
| 20 | Task-type scope boundaries with runtime requests | YES | App scope ceiling + agent requests within it | — |
| 21 | Policy engine as constraint, not agent judgment | YES | — | — |
| 22 | Purpose metadata in credential (`task_id`, `orchestration_id`) | PARTIAL | In SPIFFE ID path but not standalone JWT claims | Phase 1C |
| 23 | Intent classification (declared purpose vs observed actions) | NO | Behavioral analysis not built | Future Spec |
| 24 | Human authorization as cryptographic step in delegation chain | NO | Human-in-the-loop approval flow doesn't exist | Future Spec |
| 25 | Data aggregation risk detection | NO | No sensitivity classification | Future Spec |
| 26 | NGAC for aggregate access patterns | NO | No policy engine integration | Future Spec |

## Delegation

| # | Recommendation | Status | Gap | Placement |
|---|---------------|--------|-----|-----------|
| 27 | Delegation token: delegator + original principal + chain + attenuated scope | PARTIAL | `original_principal` not explicit field | Phase 1C |
| 28 | Scope attenuation (narrow only) | YES | — | — |
| 29 | Max delegation depth (5) | YES | — | — |
| 30 | Resource servers verify complete chain | YES | — | — |
| 31 | `original_principal` traces to authorizing human | NO | Inferable from `DelegChain[0].Agent` but not explicit | Phase 1C |
| 32 | Context grounding (downstream validated against original intent) | NO | No intent tracking infrastructure | Future Spec |
| 33 | OAuth 2.0 Token Exchange (RFC 8693) compatibility | NO | Standards compatibility layer | Future Spec |
| 34 | Transaction Tokens draft compatibility | NO | Standards compatibility layer | Future Spec |

## Audit

| # | Recommendation | Status | Gap | Placement |
|---|---------------|--------|-----|-----------|
| 35 | Append-only tamper-proof storage | YES | — | — |
| 36 | Required fields: timestamp, agent_id, task_id, orch_id, action, resource, outcome, deleg_depth, chain_hash, bytes_transferred | YES | — | — |
| 37 | `delegation_chain_hash` links audit entries to chain | YES | — | — |
| 38 | `original_principal` in audit events | NO | Not a field in audit events | Phase 1C |
| 39 | `intermediate_agents` in audit events | NO | Not captured, derivable from DelegChain | Phase 1C |
| 40 | Full multi-agent workflow reconstruction from logs | PARTIAL | Single-agent works, multi-agent untested | Phase 1C |
| 41 | Non-repudiation: unique IDs + crypto signatures + immutable logs | YES | — | — |
| 42 | Resilient logging (local queue + replay) | NO | Audit write is inline, no fallback | Tech Debt |
| 43 | Audit integrity verification on read (`VerifyChain()`) | NO | Hash computed on write, never verified on read | Phase 1C |
| 44 | Read-only audit access | NO | No read-only role/mode | Phase 1C |

## Token Lifecycle

| # | Recommendation | Status | Gap | Placement |
|---|---------------|--------|-----|-----------|
| 45 | Short-lived tokens die with task | YES | — | — |
| 46 | No static secrets in agent environment | PARTIAL | Admin secret is env var, launch token is shared secret | Tech Debt |
| 47 | Unique credential per task | YES | — | — |

## Prompt Injection Containment

| # | Recommendation | Status | Gap | Placement |
|---|---------------|--------|-----|-----------|
| 48 | Task-scoped credentials limit blast radius | YES | — | — |
| 49 | Short TTLs make exfiltrated creds useless | YES | — | — |
| 50 | Anomaly-based revocation for immediate termination | NO | Requires behavioral monitoring (item 23) | Future Spec |

---

## Summary

| Status | Count |
|--------|-------|
| YES | 24 |
| PARTIAL | 12 |
| NO | 14 |
| **Total** | **50** |

### Placement Breakdown

| Where | Items | Count |
|-------|-------|-------|
| Phase 1C | 2, 5, 22, 27, 31, 38, 39, 40, 43, 44 | 10 |
| TD-006 (before Phase 1C) | 12 | 1 |
| Tech Debt (needs spec) | 42, 46 | 2 |
| Phase 2 | 11 | 1 |
| Future Spec | 3, 4, 6, 7, 8, 9, 13, 15, 23, 24, 25, 26, 32, 33, 34, 50 | 16 |

---

## Phase 1C Additions — User Stories

These stories should be added to the Phase 1C spec. They fit because Phase 1C is already opening JWT claims, revocation, and audit code.

### Story 11: `original_principal` JWT Claim

**As a security reviewer**, I want every agent JWT to carry an `original_principal` claim so that any token can be traced back to the human or system that authorized the workflow, without walking the delegation chain manually.

**Acceptance criteria:**
- Agent JWTs issued via delegation include `original_principal` claim set to `DelegChain[0].Agent`
- Agent JWTs issued directly (no delegation) have `original_principal` set to the app identity or admin identity that created the launch token
- `POST /v1/token/validate` returns `original_principal` in response
- Existing tokens without the claim continue to validate (backward compatible)

**How to test:**
1. Register agent via app-scoped launch token, decode JWT, verify `original_principal` = app identity
2. Create 3-hop delegation chain, decode leaf JWT, verify `original_principal` = root agent
3. Validate token via API, verify `original_principal` in response
4. Validate a legacy token (no `original_principal`), verify no error

### Story 12: `task_id` and `orchestration_id` as Standalone JWT Claims

**As a developer**, I want `task_id` and `orchestration_id` as top-level JWT claims so that downstream services can make task-aware authorization decisions without parsing the SPIFFE ID.

**Acceptance criteria:**
- Agent JWTs include `task_id` and `orch_id` claims extracted from the SPIFFE ID at issuance
- `POST /v1/token/validate` returns these claims
- Backward compatible — tokens without these claims still validate

**How to test:**
1. Register agent with SPIFFE ID containing task/orch. Decode JWT. Verify claims present
2. Validate token via API. Verify `task_id` and `orch_id` in response
3. Validate legacy token. Verify no error

### Story 13: `original_principal` in Audit Events

**As an operator**, I want audit events to include `original_principal` so I can answer "what did Alice's agents do today?" with a single query.

**Acceptance criteria:**
- All audit events for agent actions include `original_principal` field
- `GET /v1/audit/events` supports `original_principal` as a query filter
- `aactl audit --original-principal alice@company.com` works

**How to test:**
1. Create agent via app-scoped flow. Perform actions. Query audit by `original_principal`. Verify events returned
2. Create delegation chain. Leaf agent performs action. Verify audit event carries root `original_principal`
3. Query with non-existent principal. Verify empty results

### Story 14: `intermediate_agents` in Audit Events

**As a security reviewer**, I want audit events for delegated actions to include the full list of intermediate agents so I can reconstruct which agents were involved without re-walking the delegation chain.

**Acceptance criteria:**
- Audit events from delegated tokens include `intermediate_agents` field (ordered list of agent SPIFFE IDs from root to leaf)
- Non-delegated events have `intermediate_agents` as empty/null
- Field included in audit query responses

**How to test:**
1. Create 3-hop delegation chain. Leaf agent performs action. Query audit. Verify `intermediate_agents` lists all 3 agents in order
2. Direct (non-delegated) agent action. Verify `intermediate_agents` is empty
3. Verify hash chain still valid after adding the new field

### Story 15: Audit Hash Chain Integrity Verification

**As a security reviewer**, I want to verify the audit log hash chain integrity on demand so I can detect if any events were tampered with after the fact.

**Acceptance criteria:**
- New endpoint `GET /v1/audit/verify` walks the entire hash chain and returns pass/fail
- On failure, response identifies the first corrupted event (ID and timestamp)
- `aactl audit verify` command wraps this endpoint
- Verification is read-only — does not modify any events

**How to test:**
1. Write 20 audit events. Call verify. Expect pass
2. Manually corrupt one SQLite row (change a detail field). Call verify. Expect failure identifying the corrupted event
3. Verify the endpoint is accessible with read-only credentials (once story 17 is implemented)

### Story 16: Agent Record Expiry

**As a security reviewer**, I want agent identity records to expire when their token TTL elapses so that stale agent identities don't persist indefinitely.

**Acceptance criteria:**
- Agent records have an `expires_at` field set to token expiry time at registration
- Expired agent records are marked `status=expired` (not deleted — audit trail needs them)
- Expired agents cannot renew, delegate, or perform authenticated actions
- `aactl` shows expired agents distinctly from active ones
- A background cleanup marks expired records periodically (configurable interval, default 60s)

**How to test:**
1. Register agent with 5-min TTL. Wait for expiry. Attempt renewal. Verify 401
2. Attempt delegation from expired agent. Verify rejection
3. Query agents via aactl. Verify expired agent shows with expired status
4. Verify audit trail still references the expired agent (records not deleted)

### Story 17: Read-Only Audit Access

**As an auditor**, I want a read-only API credential that can query audit events and verify chain integrity but cannot create tokens, register agents, or perform any write operations.

**Acceptance criteria:**
- New role/credential type: `audit-reader`
- `audit-reader` can: `GET /v1/audit/events`, `GET /v1/audit/verify`
- `audit-reader` cannot: any POST/PUT/DELETE endpoint, any token/agent/app management
- `aactl audit` commands work with audit-reader credentials
- Operator can create audit-reader credentials via `aactl audit create-reader`

**How to test:**
1. Create audit-reader credential. Query audit events. Verify success
2. Attempt to register agent with audit-reader credential. Verify 403
3. Attempt to revoke a token with audit-reader credential. Verify 403
4. Call `GET /v1/audit/verify` with audit-reader credential. Verify success

---

## Tech Debt Resolution — Where Each Item Lives

No tech debt is carried. Every item has a spec.

| ID | What | Spec | Why There |
|----|------|------|-----------|
| TD-007 | Resilient logging (queue + replay) | **Phase 1D** (`.plans/phase-1d/Phase-1d-Resilient-Audit-Pipeline.md`) | Changes HOW events are written — different concern from Phase 1C's WHAT gets written |
| TD-008 | Token predecessor invalidation on renewal | **Phase 1C** (stories 17) | Already in the token + revocation code |
| TD-009 | JTI blocklist pruning | **Phase 1C** (story 18) | Already in the revocation data structures |
| TD-010 | Static secrets in environment (`AA_ADMIN_SECRET`) | **Phase 2** (`.plans/phase-2/Phase-2-Activation-Token-Bootstrap.md`) | Phase 2's entire purpose is master key containment — this is the same problem |

---

## Future Specs — New Capability Areas

These are capabilities we recommended in the NIST submission that go beyond what current specs cover. Each needs its own spec when the time comes.

| Spec Name | Items | What It Covers |
|-----------|-------|---------------|
| Platform Attestation | 3, 13, 46 | TPM, K8s SA, container hash, cloud instance docs — real secret-zero solution |
| MCP/CIMD Integration | 4 | Client ID Metadata Documents for cross-org identity |
| Behavioral Monitoring | 6, 23, 50 | Intent classification, anomaly detection, behavioral baselines |
| NGAC Policy Engine | 25, 26 | Aggregate access patterns, sensitivity classification |
| Standards Compatibility | 33, 34 | OAuth Token Exchange (RFC 8693), Transaction Tokens |
| Algorithm Flexibility | 9 | ECDSA P-256 alongside Ed25519 |
| Human-in-the-Loop Chain | 24 | Cryptographic approval step in delegation chain |
| mTLS Default | 8 | mTLS as default transport, not optional |
| Agent Registry | 7 | Persistent code/model version tracking |
| Context Grounding | 32 | Downstream action validation against original intent |
