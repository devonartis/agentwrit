# Phase 1c: App Revocation, Audit Attribution & Secret Rotation

**Status:** Spec
**Priority:** P0 — completes app lifecycle for production operations
**Effort estimate:** 1 day
**Depends on:** Phase 1b (app-scoped launch tokens, app_id on agents)
**Architecture doc:** `../.plans/CoWork-Architecture-Direct-Broker.md`

---

## Overview: What We're Building and Why

AgentAuth is transforming from a sidecar-dependent system into an app-centric architecture. Phase 1a gave apps their own identity and credentials. Phase 1b let apps create agents within their scope ceiling and established the traceability chain (App → Launch Token → Agent). But apps aren't production-ready yet — there's no way to revoke an entire app's access in an emergency, agent JWTs don't carry app identity (so downstream services can't make app-aware decisions), and there's no credential rotation without downtime.

**Phase 1c completes the app lifecycle for production operations.** It adds three capabilities that make apps manageable at scale: (1) app-level revocation — one command revokes everything belonging to an app, extending the revocation pyramid from 4 levels to 5; (2) `app_id` in JWT claims — every agent token now carries its app's identity, making the audit trail and token validation app-aware; (3) client secret rotation with a grace period — credentials can be rotated without coordination or downtime.

After this phase, the core app model is complete: register, authenticate, create agents, audit, revoke, rotate credentials. The P0 milestone is met.

**What changes:** 5th revocation level (app), `app_id` + `app_name` added to agent JWT claims, secret rotation endpoint with configurable grace period, audit query filtering by `app_id`, two new `aactl` commands.

**What stays the same:** Existing 4-level revocation unchanged, JWT signing and verification unchanged, all existing endpoints and flows preserved.

---

## Problem Statement

After Phase 1b, apps can register and create agents — but there's no way to revoke an entire app's tokens at once, agent JWTs don't carry the `app_id` (so token-level queries can't identify which app a token belongs to), and there's no way to rotate an app's client secret without downtime. These are the three capabilities needed before apps are production-ready.

---

## Goals

1. Operators can revoke all tokens belonging to an app with a single command (app-level revocation)
2. Agent JWTs carry `app_id` and `app_name` in their claims so any token can be traced to its app
3. Operators can rotate an app's client secret with a grace period so there's no authentication downtime
4. The audit trail supports queries by `app_id` so operators can answer "what did this app do?"

---

## Non-Goals

1. **Permanent dual-active secrets** — Phase 1c supports one new + one old (grace period only). Always-two-active secrets is a future enhancement.
2. **Automatic secret rotation** — operator-triggered only, no scheduled rotation
3. **App-scoped audit endpoint** — apps querying their own audit events (future consideration)
4. **Agent-level management by apps** — apps revoking individual agents they own (future)

---

## User Stories

### Operator Stories

1. **As an operator**, I want to revoke all tokens belonging to a compromised app with a single command so that I can contain the blast radius immediately without hunting down individual agents.

2. **As an operator**, I want `aactl app revoke --id APP_ID` to cascade-revoke everything: the app's own JWT, all its agents' tokens, and all delegated tokens from those agents.

3. **As an operator**, I want to rotate an app's client secret so that I can comply with credential rotation policies without causing downtime for the app.

4. **As an operator**, I want the old secret to remain valid for a configurable grace period (default 24 hours) after rotation so that the developer has time to update their configuration.

5. **As an operator**, I want to query audit events by `app_id` so that I can answer "what did weather-bot do last Tuesday?" with a single query.

### Developer Stories

6. **As a developer**, I want my agent tokens to carry my app's identity so that when I inspect a token's claims, I can confirm which app it belongs to.

7. **As a developer**, I want a clear notification pattern when my client secret has been rotated so that I know to update my configuration before the grace period expires.

### Security Stories

8. **As a security reviewer**, I want app-level revocation to be a 5th level in the revocation pyramid (token → agent → task → chain → app) so that it follows the existing cascading revocation model.

9. **As a security reviewer**, I want every agent JWT to carry `app_id` in its claims so that token validation responses reveal which app the agent belongs to — enabling downstream services to make app-aware authorization decisions.

10. **As a security reviewer**, I want secret rotation to invalidate the old secret after the grace period so that long-lived compromised secrets don't persist indefinitely.

---

## What Needs to Be Done

### 1. App-Level Revocation (5th Level)

Add `"app"` as a new revocation level alongside the existing token, agent, task, and chain levels. When an operator revokes an app:

- All agents belonging to that app are identified (via `app_id` on agent records)
- All tokens issued to those agents are revoked
- All delegation chains originating from those agents are revoked
- The app's own JWT (if active) is revoked
- A single `aactl app revoke --id APP_ID` command triggers all of this

This extends the existing revocation pyramid from 4 levels to 5:
```
Token → Agent → Task → Chain → App (NEW)
```

The revocation service needs to index tokens by `app_id` to make cascading revocation performant. Specifically, `RevSvc` must maintain an in-memory index:

```go
appAgents map[string][]string  // app_id → []agentID
```

This index is populated at startup (from the store) and updated on every agent registration and revocation. Without this index, revoking an app requires scanning all registered agents to find those belonging to the app — O(n) — which becomes a bottleneck at scale. With the index, app revocation is O(1) lookup + O(k) cascade where k = number of agents in that app.

### 2. `app_id` and `app_name` in Agent JWT Claims

When the broker issues a JWT for an agent that belongs to an app, the token claims should include:

- `app_id` — the app's unique identifier
- `app_name` — the app's human-readable name

This is backward compatible: tokens for legacy agents (no app) have these fields empty/absent. The token validation endpoint (`POST /v1/token/validate`) should return these claims in its response.

### 3. Client Secret Rotation

A new endpoint and CLI command for rotating an app's client secret. The grace period requires **dual-secret storage**: both old and new hashes must be active simultaneously during the grace period. This means the `apps` table needs two additional columns:

```sql
client_secret_hash_prev TEXT,          -- old hash (valid during grace period)
secret_rotated_at       TEXT,          -- when rotation was initiated
secret_grace_until      TEXT           -- when old secret expires (rotated_at + grace period)
```

**During the grace period:** `AuthenticateApp` checks the new secret first; if it fails, checks the old secret and accepts it if `NOW < grace_until`.
**After the grace period:** Only the new secret works. A cleanup job (or on-next-auth check) can null out `client_secret_hash_prev`.

Flow:
- Generates a new client secret
- Moves current hash to `client_secret_hash_prev`, sets `secret_rotated_at` and `secret_grace_until`
- Hashes and stores the new secret in `client_secret_hash`
- The old secret remains valid until `grace_until` (default 24 hours from rotation)
- After the grace period, only the new secret works
- The rotation event is recorded in the audit trail
- The new plaintext secret is returned exactly once (same as initial registration)

CLI: `aactl app rotate-secret --id APP_ID [--grace-period 24h]`

### 4. Audit Query by App ID

The existing audit query endpoint (`GET /v1/audit/events`) needs a new filter parameter: `app_id`. This enables queries like "show me all events for app-weather-bot-a1b2c3" — returning app auth events, launch token creations, agent registrations, token issuances, and revocations all tied to that app.

### 5. aactl Commands

- `aactl app revoke --id APP_ID` — triggers app-level cascade revocation
- `aactl app rotate-secret --id APP_ID` — rotates client secret, displays new secret with warning to save it

---

## Success Criteria

- `aactl app revoke` revokes all tokens for an app in one command
- Revoked app's agents can no longer use their tokens (immediate effect)
- Agent JWTs carry `app_id` and `app_name` in claims
- `POST /v1/token/validate` returns `app_id` and `app_name` for app-linked agents
- Secret rotation returns a new secret and the old one works during grace period
- Old secret stops working after grace period expires
- `GET /v1/audit/events?app_id=...` returns all events for that app
- Existing 4-level revocation (token, agent, task, chain) unchanged
- Legacy agents/tokens without `app_id` continue to work

---

## Testing Workflow

> **Before writing any test code**, extract the user stories from the `## User Stories` section above into a standalone file:
> `tests/phase-1c-user-stories.md`
>
> This is required by the project workflow (CLAUDE.md). The coding agent writes user stories first, saves them to `tests/`, then writes test code against them. Do not skip this step.
