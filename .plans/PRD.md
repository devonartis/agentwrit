# PRD: Apps as First-Class Entities

**Initiative:** Transform AgentAuth from sidecar-dependent to app-centric architecture
**Owner:** Divine
**Date:** 2026-03-02
**Status:** Spec complete, ready for implementation
**Pattern:** [Ephemeral Agent Credentialing v1.2](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.2.md)
**Architecture design:** [CoWork-Architecture-Direct-Broker.md](./CoWork-Architecture-Direct-Broker.md)

---

## What Is AgentAuth?

AgentAuth is a Go broker that issues short-lived, scoped JWTs to AI agents using Ed25519 challenge-response. It implements the Ephemeral Agent Credentialing Pattern v1.2 — a security pattern for giving AI agents cryptographically-bound identities with enforced scope boundaries, delegation chains, revocation, and tamper-evident audit trails.

Think of AgentAuth as the identity and access management layer for AI agents. An orchestration platform registers an agent, the agent proves its identity via cryptographic challenge-response, and the broker issues a scoped, time-limited JWT that the agent uses to access resources. Everything is audited, everything is revocable, and permissions can only narrow (never widen).

---

## The Problem

AgentAuth has a fundamental architectural flaw: **applications don't exist as entities.** The only way to connect an application to the broker is by deploying a Token Proxy (sidecar) that holds the admin master key. This single design decision cascaded into five production-blocking problems:

### 1. Master Key Everywhere
Every sidecar deployment has a copy of the admin master key — the single most powerful credential in the system. A compromise of ANY sidecar compromises the ENTIRE system. There is no way to scope, limit, or independently revoke a sidecar's access because they all share the same credential.

### 2. No App Identity
The broker has no concept of "which app is this?" All sidecars look identical. You can't list apps, you can't query what an app did, you can't revoke one app without affecting others. The audit trail shows "sidecar" as the actor — but which one?

### 3. Infrastructure-as-Registration
"Adding a new app" means deploying a sidecar instance with the master key baked into its config. This is an infrastructure operation requiring DevOps involvement, not a simple API call. Scaling from 3 apps to 30 means deploying 30 sidecar instances, each holding the master key.

### 4. Audit Blindness
The audit trail can't answer "what did weather-bot do last Tuesday?" because it doesn't know what weather-bot is. All agent activity is attributed to anonymous sidecars. Incident investigation, compliance reporting, and access reviews are all blind at the app level.

### 5. No Graceful Credential Management
There's no way to rotate the master key without a coordinated outage across all sidecars. There's no per-app revocation. There's no way to decommission one app's access without redeploying infrastructure.

### Impact

These problems block AgentAuth from being production-ready. No security team would approve a system where the master key is distributed to every deployment. No operations team can manage a fleet of apps they can't list, audit, or independently revoke. AgentAuth's core security primitives (Ed25519 identity, scope attenuation, delegation chains, revocation, hash-chained audit) are solid — but the app connectivity layer makes the system undeployable in any real environment.

---

## The Solution: Apps as First-Class Entities

Give each application its own identity and scoped credentials. Apps register with the broker via API, receive a `client_id` + `client_secret`, and authenticate directly. The master key stays with the operator. The sidecar becomes optional (a convenience, not a requirement).

### What Changes

- **App registration:** Operator creates an app via `aactl app register` → broker returns `client_id` + `client_secret`
- **App authentication:** App calls `POST /v1/app/auth` with its own credentials → gets a scoped JWT
- **App-scoped agents:** App creates launch tokens within its ceiling → agents register and inherit the app's identity
- **App-level operations:** List, audit, revoke, rotate credentials — all per-app
- **Three paths to the broker:** SDK (direct), Token Proxy (optional), raw HTTP — developer chooses
- **Master key isolation:** Master key exists only in the broker config and the operator's vault

### What Stays the Same

The entire agent security layer is untouched:

- Ed25519 challenge-response identity (zero changes)
- SPIFFE ID generation (zero changes)
- Scope attenuation — one-way narrowing only (zero changes)
- Delegation chains with SHA-256 linking and depth cap of 5 (zero changes)
- 4-level revocation: token → agent → task → chain (preserved, extended with app level)
- Hash-chained audit trail — tamper-evident, append-only (preserved, enhanced with app attribution)
- Short-lived JWTs with JTI for individual revocation (zero changes)
- Single-use nonces with 30-second TTL (zero changes)
- Single-use launch tokens with scope ceilings (zero changes)

**Count: 8 capabilities preserved, 5 enhanced (backward-compatible), 4 new. Zero capabilities removed or broken.**

---

## Success Metrics

### Must-Hit

1. **Master key exposure: 0** — the master key appears in exactly 2 places: broker config and operator vault. No proxy, no app deployment, no developer config.
2. **App registration time: < 5 seconds** — `aactl app register` creates an app and returns credentials in one command.
3. **App audit coverage: 100%** — every agent token can be traced to the app that created it. "What did weather-bot do?" is a single query.
4. **Backward compatibility: 100%** — all existing admin, sidecar, and agent flows work without modification after the transformation.

### Stretch

5. **Token survival across restart** — tokens issued before a broker restart remain valid after restart (Phase 5).
6. **Local token validation** — resource servers can validate tokens without calling the broker (Phase 4).
7. **SDK adoption** — developers prefer `client.get_token()` over deploying a sidecar (Phase 3).

---

## Scope: 7 Phases

The transformation is broken into 7 phases across two priority tiers. Each phase has its own detailed spec.

### P0 — Core (must ship together for a usable system)

| Phase | What | Why | Effort | Spec |
|-------|------|-----|--------|------|
| **1a** | App registration + authentication | Apps become entities with credentials | 1-2 days | [Spec](./phase-1a/Phase-1a-App-Registration-Auth.md) |
| **1b** | App-scoped launch tokens | Apps can create agents within their ceiling | 1 day | [Spec](./phase-1b/Phase-1b-App-Scoped-Launch-Tokens.md) |
| **1c** | App revocation + audit attribution + secret rotation | Production lifecycle management | 1 day | [Spec](./phase-1c/Phase-1c-Revocation-Audit-SecretRotation.md) |
| **2** | Activation token bootstrap | Master key removed from proxy deployments | 1 day | [Spec](./phase-2/Phase-2-Activation-Token-Bootstrap.md) |

**P0 total: 4-5 days.** After these 4 phases, the system is production-ready: apps register, authenticate, create agents, get audited, can be revoked, and the master key is contained.

### P1 — Enhancements (ship independently, high value)

| Phase | What | Why | Effort | Spec |
|-------|------|-----|--------|------|
| **3** | Python SDK | `client.get_token()` — sidecar truly optional | 3-5 days | [Spec](./phase-3/Phase-3-Python-SDK.md) |
| **4** | JWKS endpoint | Local token validation, removes validation SPOF | 1 day | [Spec](./phase-4/Phase-4-JWKS-Endpoint.md) |
| **5** | Key persistence | Tokens survive broker restarts | 1-2 days | [Spec](./phase-5/Phase-5-Key-Persistence.md) |

**P1 total: 5-8 days.** These phases improve developer experience, resilience, and operational maturity.

**Notes on P1 priority decisions:**
- **Phase 3 (SDK) is P1, not P0:** Without the SDK, developers using the broker directly face 8 manual steps (app auth → launch token → Ed25519 keypair → challenge → sign → register → cache → renew). This is impractical for most. However, the Token Proxy (sidecar) already handles this complexity for sidecar-based deployments — and Phase 2 fixes the sidecar's security problem. P0 delivers a working, secure system; Phase 3 delivers a better developer experience. Teams that want to avoid the sidecar can treat Phase 3 as a personal P0. Reconsider if sidecar adoption is blocked by the org's infrastructure policies.
- **Phases 4 and 5 should ship together:** Phase 4 (JWKS) without Phase 5 (key persistence) is a production stability trap — every broker restart invalidates all cached JWKS keys. Build Phase 4 first for development/testing, but do not enable JWKS caching in production until Phase 5 ships.

**Full initiative: 9-14 days total.**

---

## When Does the Sidecar Become Optional?

The sidecar (Token Proxy) is NOT removed — it transitions from **mandatory** to **optional** across the phases. Here's exactly when and how:

| Milestone | Phase | What Happens to the Sidecar |
|-----------|-------|-----------------------------|
| **Sidecar is mandatory** | Today | Only path to the broker. Holds master key. Must deploy one per app. |
| **Alternative path exists** | After 1a+1b | Apps CAN talk directly to the broker via HTTP (authenticate → create launch tokens → register agents). But the developer experience is raw HTTP with Ed25519 crypto — not practical for most developers. Sidecar is still the easier path. |
| **Sidecar fixed** | After Phase 2 | Sidecar bootstraps with activation token instead of master key. Still works, no longer a security liability. A developer who WANTS a sidecar can use one safely. |
| **Sidecar truly optional** | After Phase 3 (SDK) | `client.get_token()` gives the same one-call DX that the sidecar provides, but as a library instead of infrastructure. Developers choose: SDK (no infra), sidecar (infra-level caching), or raw HTTP (full control). |

**The sidecar remains in the codebase** as an optional deployment for teams that want infrastructure-level token caching and circuit breaking. It is not deprecated, not removed, and not broken by any phase. After Phase 2, it's actually BETTER (scoped credentials instead of master key).

**What we ARE removing:** The mandatory dependency on the sidecar. After Phase 3, no developer is FORCED to deploy a sidecar to use AgentAuth. They have three paths and can choose the one that fits their environment.

---

## Non-Goals (entire initiative)

1. **Removing the sidecar** — the sidecar becomes optional, not deleted. It still has value for infrastructure-level caching and circuit breaking.
2. **Multi-broker deployment** — distributed broker clusters with shared state. This is a separate initiative.
3. **OIDC/OAuth2 full compliance** — we implement the parts of the pattern we need (JWKS, client credentials), not a full OAuth2 authorization server.
4. **HSM/KMS integration** — hardware security modules for key storage. Enterprise feature for later.
5. **Web UI for app management** — CLI and API only. A dashboard is a separate product decision.
6. **Go SDK** — Go developers can use the HTTP API directly (same language as the broker).

---

## Dependency Chain

```
Phase 1a ──→ Phase 1b ──→ Phase 1c
    │
    └──→ Phase 2

Phase 1a + 1b ──→ Phase 3 (SDK)

Phase 4 (JWKS) ←──→ Phase 5 (Key Persistence)
   (independent, but Phase 5 makes Phase 4 production-grade)
```

Phases 1a → 1b → 1c are strictly sequential (each builds on the previous).
Phase 2 depends only on 1a (can be built in parallel with 1b/1c).
Phase 3 depends on 1a + 1b.
Phases 4 and 5 are independent of everything else (can be built any time).

---

## Risks

| Risk | Severity | Phase | Mitigation |
|------|----------|-------|------------|
| Client secret compromise | Medium | 1a | Per-app secrets (not master key). Rotate one app, revoke instantly. Blast radius limited to one app. |
| Scope ceiling bypass | High | 1b | `ScopeIsSubset()` enforced at launch token creation. Same attenuation logic used for delegation (battle-tested). |
| Legacy agent orphans | Low | 1b-1c | Agents without `app_id` continue to work. Operators migrate over time. Null `app_id` is documented and expected. |
| SQLite schema migration | Medium | 1b-1c | New `app_id` columns on existing tables must be `TEXT DEFAULT NULL` (SQLite `ALTER TABLE` restriction). Added via `InitDB()` with error-tolerant logic. Full spec in Phase 1a. |
| RevSvc O(n) at app revocation | Medium | 1c | `RevSvc` maintains an `appAgents map[string][]string` in-memory index for O(1) lookup. Spec'd in Phase 1c. |
| JWKS without key persistence | High | 4 | Do not enable JWKS caching in production without Phase 5. Broker restart changes the key, invalidating all cached JWKS. Ship Phase 4 and Phase 5 together in production. |
| SDK bugs in Ed25519 | Medium | 3 | Reference implementation using well-known Python crypto libraries. Same math as the existing Go sidecar. |
| Key file theft | High | 5 | File permissions (0600), optional encryption, audit events on key access. |
| Backward compatibility break | High | All | Every phase is additive. No existing endpoints, flows, or data models are modified — only extended. |

---

## Open Questions

1. **App credential format** — should `client_secret` use a prefix (`sk_live_...`) for easy identification in logs/config, or plain hex? Decision needed in Phase 1a.
2. **Grace period default for secret rotation** — 24 hours proposed. Is this too long or too short for the expected deployment patterns? Decision needed in Phase 1c.
3. **SDK language priority** — Python first is the plan. Is there demand for JavaScript/TypeScript as a Phase 3b? Decision can wait until Phase 3 is underway.
4. **JWKS cache TTL** — 1 hour proposed. Should this be configurable? Decision needed in Phase 4.

---

## How to Use This PRD

This document is the **single source of truth** for the initiative. Each phase has a detailed spec in its own folder:

```
.plans/
├── PRD.md                                              ← you are here
├── Codebase-Map.md                                     ← current architecture: structs, routes, services, patterns
├── phase-1a/Phase-1a-App-Registration-Auth.md          ← app registration & auth (detailed, with code paths)
├── phase-1b/Phase-1b-App-Scoped-Launch-Tokens.md       ← app-scoped launch tokens
├── phase-1c/Phase-1c-Revocation-Audit-SecretRotation.md ← revocation, audit, secret rotation
├── phase-2/Phase-2-Activation-Token-Bootstrap.md        ← activation token bootstrap
├── phase-3/Phase-3-Python-SDK.md                        ← Python SDK
├── phase-4/Phase-4-JWKS-Endpoint.md                     ← JWKS endpoint
├── phase-5/Phase-5-Key-Persistence.md                   ← key persistence
└── CoWork-Architecture-Direct-Broker.md                 ← full architecture design
```

**Workflow:**
1. Read this PRD for the "what" and "why"
2. Read `Codebase-Map.md` for the "where" — current structs, routes, services, patterns
3. Read the phase spec for the "how" and "how to verify"
4. Break the spec into testable user stories
5. Build, test, ship
