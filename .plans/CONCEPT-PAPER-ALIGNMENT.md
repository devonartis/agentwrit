# Concept Paper Alignment Checklist

Features and capabilities recommended in the NCCoE concept paper response. AgentAuth should implement or verify everything we recommend — if we recommend it, we should be doing it.

For the first release: ship what's already built. Quick enhancements get added before release. Larger gaps are tracked for future phases.

Status key: DONE = implemented, QUICK-ADD = can add before release, FUTURE = larger effort for later phases.

---

## Identification

| # | Feature | Status | Notes |
|---|---------|--------|-------|
| I-1 | SPIFFE-compatible agent identifiers (`spiffe://trust-domain/agent/{orch_id}/{task_id}/{instance_id}`) | DONE | SPIFFE ID format used in identity package |
| I-2 | Unique instance identifier per agent | DONE | Agent registration returns unique agent ID |
| I-3 | Task/orchestration context in identity | FUTURE | Launch tokens carry task context, agent ID doesn't embed orch/task yet |
| I-4 | Original initiating principal (human or system) tracked | FUTURE | `original_principal` field — needs delegation chain support |
| I-5 | Delegation chain embedded in identity | FUTURE | No delegation chain support yet |
| I-6 | Agent framework/version metadata | FUTURE | Not captured |
| I-7 | Behavioral baseline information (expected tool call patterns) | FUTURE | Not captured |
| I-8 | Ephemeral identity — unique per task, retired on completion | DONE | Ed25519 challenge-response, short-lived JWTs |

## Authentication

| # | Feature | Status | Notes |
|---|---------|--------|-------|
| A-1 | mTLS at transport layer | FUTURE | Not implemented |
| A-2 | Task-scoped JWT tokens with Ed25519 signatures | DONE | `internal/token/` |
| A-3 | Token validation on every request | DONE | `internal/authz/` ValMw |
| A-4 | Bidirectional credential validation for multi-agent | FUTURE | Not implemented |
| A-5 | Token lifetime matches task duration (5 min default, 1-60 min range) | DONE | Default 300s TTL, configurable |

## Authorization

| # | Feature | Status | Notes |
|---|---------|--------|-------|
| Z-1 | Scope taxonomy (`action:resource:identifier` format) | DONE | `read:customers:*`, `write:orders:*` |
| Z-2 | Scope attenuation at each delegation hop | FUTURE | Needs delegation chain |
| Z-3 | App scope ceiling enforcement | DONE | Phase 1B — handler enforces ceiling |
| Z-4 | Dynamic scope request within task-type boundaries | DONE | Apps request scopes at auth, ceiling enforced |
| Z-5 | NGAC for aggregate access pattern evaluation | FUTURE | Not implemented |
| Z-6 | Data aggregation risk detection | FUTURE | Not implemented |

## Key Management

| # | Feature | Status | Notes |
|---|---------|--------|-------|
| K-1 | Runtime credential issuance via attestation | DONE | Launch token → agent registration flow |
| K-2 | Unique credential per task (not refreshed, re-issued) | DONE | New JWT per auth, no refresh tokens |
| K-3 | Token-level revocation | DONE | `internal/revoke/` RevSvc |
| K-4 | Agent-level revocation | DONE | Revoke by agent sub claim |
| K-5 | Task-level revocation | FUTURE | No task-scoped revocation |
| K-6 | Delegation-chain-level revocation (cascade downstream) | FUTURE | Needs delegation chain |
| K-7 | Push-based revocation (<30s propagation) | FUTURE | Currently pull-based (token expiry) |
| K-8 | Revocation tested under partial service degradation | FUTURE | Not tested |

## Delegation

| # | Feature | Status | Notes |
|---|---------|--------|-------|
| D-1 | Delegation token includes delegator identity | FUTURE | No delegation tokens yet |
| D-2 | Delegation token includes original initiating principal | FUTURE | |
| D-3 | Complete delegation chain with crypto signatures at each hop | FUTURE | |
| D-4 | Scope attenuation enforced at each hop | FUTURE | |
| D-5 | Resource servers verify complete chain | FUTURE | |
| D-6 | Maximum delegation depth enforcement (5 hops) | FUTURE | |
| D-7 | OAuth 2.0 Token Exchange (RFC 8693) compatibility | FUTURE | |
| D-8 | Transaction Tokens draft compatibility | FUTURE | |

## Audit & Logging

| # | Feature | Status | Notes |
|---|---------|--------|-------|
| L-1 | Timestamp | DONE | Every audit event |
| L-2 | Agent identity (full SPIFFE ID) | QUICK-ADD | Agent ID captured, could add full SPIFFE ID |
| L-3 | Task and orchestration context | QUICK-ADD | Launch token has task context, could propagate to audit |
| L-4 | Action taken | DONE | Event type field |
| L-5 | Resource accessed | QUICK-ADD | Endpoint in some events, could add consistently |
| L-6 | Authorization decision and scope used | DONE | scope_violation events log required vs actual |
| L-7 | Outcome (success/failure) | DONE | Outcome field on every event |
| L-8 | Delegation depth | FUTURE | Needs delegation chain |
| L-9 | Delegation chain hash | FUTURE | Not implemented |
| L-10 | `original_principal` field | FUTURE | Needs delegation chain |
| L-11 | Append-only storage | DONE | SQLite with tamper-evident hash chain |
| L-12 | Tamper-proof (entries cannot be modified) | DONE | `computeHash` covers all fields |
| L-13 | Full multi-agent workflow reconstruction from audit logs | FUTURE | Needs delegation chain support first |

## Verification Tests (Demonstration)

| # | Test | Status | Notes |
|---|------|--------|-------|
| V-1 | Single-agent workflow fully reconstructable from audit trail | QUICK-ADD | Should work today with L-2, L-3, L-5 additions |
| V-2 | Multi-agent delegation chain reconstructable | FUTURE | Needs D-1 through D-6 |
| V-3 | Revocation under partial service degradation | FUTURE | Needs K-7, K-8 |
| V-4 | Scope attenuation verified across delegation hops | FUTURE | Needs D-4 |
| V-5 | Human-in-the-loop authorization in delegation chain | FUTURE | Needs D-2 |

---

## First Release Plan

**Ship as-is (DONE):** I-1, I-2, I-8, A-2, A-3, A-5, Z-1, Z-3, Z-4, K-1, K-2, K-3, K-4, L-1, L-4, L-6, L-7, L-11, L-12

**Quick adds before release (QUICK-ADD):**
- L-2: Full SPIFFE ID in audit events
- L-3: Task/orchestration context in audit events
- L-5: Resource accessed consistently in all audit events
- V-1: Single-agent workflow reconstruction test

**Future phases (FUTURE):** Everything in the Delegation section (D-1 through D-8), delegation-dependent audit fields (L-8 through L-10, L-13), mTLS (A-1), push-based revocation (K-7), NGAC (Z-5).
