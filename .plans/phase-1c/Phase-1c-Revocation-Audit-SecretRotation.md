# Phase 1c: App Revocation, Audit Attribution, Secret Rotation & NIST Alignment

**Status:** Spec
**Priority:** P0 — completes app lifecycle for production operations + closes NIST submission gaps
**Effort estimate:** 2 days
**Depends on:** Phase 1b (app-scoped launch tokens, app_id on agents), TD-006 (app JWT TTL)
**Architecture doc:** `../.plans/CoWork-Architecture-Direct-Broker.md`
**NIST analysis:** `../.plans/nist-submission/NIST-Recommendations-vs-Implementation.md`

---

## Overview: What We're Building and Why

AgentAuth is transforming from a sidecar-dependent system into an app-centric architecture. Phase 1a gave apps their own identity and credentials. Phase 1b let apps create agents within their scope ceiling and established the traceability chain (App → Launch Token → Agent). But apps aren't production-ready yet — there's no way to revoke an entire app's access in an emergency, agent JWTs don't carry app identity (so downstream services can't make app-aware decisions), and there's no credential rotation without downtime.

**Phase 1c completes the app lifecycle and closes gaps identified in the NIST NCCoE submission.** The NIST analysis (`.plans/nist-submission/NIST-Recommendations-vs-Implementation.md`) compared every recommendation we made in our public comment against what AgentAuth actually implements. Several gaps fall squarely in the code Phase 1c is already touching — JWT claims, revocation, and audit — so we fix them here rather than carrying tech debt.

Phase 1c delivers six capability areas:

1. **App-level revocation** — one command revokes everything belonging to an app, extending the revocation pyramid from 4 levels to 5
2. **`app_id` in JWT claims** — every agent token now carries its app's identity, making the audit trail and token validation app-aware
3. **Client secret rotation** with a grace period — credentials can be rotated without coordination or downtime
4. **NIST JWT claim alignment** — `original_principal`, `task_id`, and `orch_id` as first-class JWT claims (we recommended these in the NIST submission, our code should do them)
5. **Audit hardening** — `original_principal` and `intermediate_agents` in audit events, hash chain integrity verification endpoint, read-only audit access role
6. **Token hygiene** — predecessor invalidation on renewal, JTI blocklist pruning, agent record expiry

After this phase, the core app model is complete AND every recommendation we made about JWT claims, audit fields, and revocation in the NIST submission is backed by working code.

**What changes:** 5th revocation level (app), `app_id` + `app_name` + `original_principal` + `task_id` + `orch_id` added to agent JWT claims, secret rotation endpoint, audit query filtering by `app_id` and `original_principal`, `intermediate_agents` audit field, `GET /v1/audit/verify` endpoint, read-only `audit-reader` role, token predecessor revocation on renewal, JTI pruning, agent record expiry, four new `aactl` commands.

**What stays the same:** Existing 4-level revocation unchanged, JWT signing and verification unchanged, all existing endpoints and flows preserved.

---

## Problem Statement

After Phase 1b, apps can register and create agents — but there's no way to revoke an entire app's tokens at once, agent JWTs don't carry the `app_id` (so token-level queries can't identify which app a token belongs to), and there's no way to rotate an app's client secret without downtime.

Additionally, the NIST NCCoE submission analysis revealed that several recommendations we made — `original_principal` traceability, `task_id`/`orch_id` as standalone claims, audit integrity verification, and read-only audit access — are not implemented in the codebase. These gaps all live in the JWT claims and audit code that Phase 1c is already modifying.

Two token lifecycle issues also need to be resolved: renewed tokens don't invalidate their predecessors (creating a window where two valid tokens exist for the same agent), and consumed JTIs are never pruned from memory.

---

## Goals

1. Operators can revoke all tokens belonging to an app with a single command (app-level revocation)
2. Agent JWTs carry `app_id`, `app_name`, `original_principal`, `task_id`, and `orch_id` in their claims
3. Operators can rotate an app's client secret with a grace period so there's no authentication downtime
4. The audit trail supports queries by `app_id` and `original_principal`
5. Audit events for delegated actions carry `intermediate_agents` for workflow reconstruction
6. The audit hash chain can be verified on demand to detect tampering
7. Auditors can access audit data through a read-only role without write permissions
8. Token renewal invalidates the predecessor token
9. Agent identity records expire when their token TTL elapses

---

## Non-Goals

1. **Permanent dual-active secrets** — Phase 1c supports one new + one old (grace period only). Always-two-active secrets is a future enhancement.
2. **Automatic secret rotation** — operator-triggered only, no scheduled rotation
3. **App-scoped audit endpoint** — apps querying their own audit events (future consideration)
4. **Agent-level management by apps** — apps revoking individual agents they own (future)
5. **Resilient logging** — event queue + replay on failure is a separate architectural concern (Phase 1d)
6. **Push-based revocation** — webhook/OCSP notification to external validators (future spec)
7. **Platform attestation** — TPM/K8s SA/container hash bootstrap (future spec)

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

### NIST Alignment Stories

11. **As a security reviewer**, I want every agent JWT to carry an `original_principal` claim so that any token can be traced back to the human or system that authorized the workflow without walking the delegation chain manually.

12. **As a developer**, I want `task_id` and `orchestration_id` as top-level JWT claims so that downstream services can make task-aware authorization decisions without parsing the SPIFFE ID.

13. **As an operator**, I want audit events to include `original_principal` so I can answer "what did Alice's agents do today?" with a single query.

14. **As a security reviewer**, I want audit events for delegated actions to include `intermediate_agents` — the full ordered list of agents in the delegation path — so I can reconstruct which agents were involved without re-walking the chain.

15. **As a security reviewer**, I want to verify the audit log hash chain integrity on demand so I can detect if any events were tampered with after the fact.

16. **As an auditor**, I want a read-only credential that can query audit events and verify chain integrity but cannot create tokens, register agents, or perform any write operations.

### Token Hygiene Stories

17. **As a security reviewer**, I want token renewal to revoke the predecessor token so that only one valid token exists per agent at any time — matching the unique-credential-per-task discipline we recommended in the NIST submission.

18. **As a security reviewer**, I want consumed JTIs to be pruned from memory after their token's TTL elapses so that the blocklist doesn't grow indefinitely.

19. **As a security reviewer**, I want agent identity records to expire when their token TTL elapses so that stale agent identities don't persist indefinitely.

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

### 6. NIST JWT Claim Alignment (`original_principal`, `task_id`, `orch_id`)

Three new claims added to agent JWTs, alongside `app_id` and `app_name` from item 2:

- `original_principal` — the human or system that authorized the workflow. For app-created agents, this is the app identity. For delegated tokens, this is `DelegChain[0].Agent` (the root delegator). For admin-created agents, this is the admin identity.
- `task_id` — extracted from the SPIFFE ID path at token issuance
- `orch_id` — extracted from the SPIFFE ID path at token issuance

All three are backward compatible: tokens without them continue to validate. `POST /v1/token/validate` returns all three in its response.

The `original_principal` is set once at the root of the chain and propagated through delegation — it never changes, even as delegation depth increases. This is the field that makes "trace every action back to the authorizing human" possible without chain-walking.

### 7. Audit Hardening (`original_principal`, `intermediate_agents`, `VerifyChain`, read-only access)

**7a. `original_principal` in audit events**

All audit events for agent actions include `original_principal`, matching the JWT claim. The audit query endpoint (`GET /v1/audit/events`) supports `original_principal` as a filter parameter. `aactl audit --original-principal alice@company.com` wraps this.

**7b. `intermediate_agents` in audit events**

Audit events from delegated tokens include `intermediate_agents` — an ordered JSON array of agent SPIFFE IDs from root to leaf. Non-delegated events have this as `null`. Derived from the existing `DelegChain` on the token claims at audit event creation time.

The `AuditEvent` struct needs two new fields:

```go
OriginalPrincipal  string   `json:"original_principal,omitempty"`
IntermediateAgents []string  `json:"intermediate_agents,omitempty"`
```

Both fields must be included in the `computeHash` function so the hash chain covers them.

**7c. `GET /v1/audit/verify` — Hash chain integrity verification**

New endpoint that walks the entire audit hash chain from genesis to the latest event and verifies each link. Returns:

```json
{
  "verified": true,
  "event_count": 1547,
  "first_event": "2026-03-01T00:00:00Z",
  "last_event": "2026-03-05T12:00:00Z"
}
```

On failure:

```json
{
  "verified": false,
  "event_count": 1547,
  "first_corrupted_event": {
    "id": "evt-123",
    "timestamp": "2026-03-04T09:15:22Z",
    "expected_hash": "abc...",
    "actual_hash": "def..."
  }
}
```

Implementation: `VerifyChain()` method on `AuditLog` that iterates all events in order, recomputes each hash from the previous hash + event fields, and compares. This is read-only — it never modifies events.

CLI: `aactl audit verify`

**7d. Read-only `audit-reader` role**

New credential type for auditors. Created by operators via `aactl audit create-reader`. The broker issues an `audit-reader` JWT with scope `audit:read:*`. The `ValMw` middleware already enforces scope — `audit:read:*` grants access to `GET /v1/audit/events` and `GET /v1/audit/verify` only. All write endpoints require different scopes and will return 403.

The `audit-reader` credential uses the same client_id/client_secret model as apps (reuse `AppSvc` infrastructure with a different role flag), or a simpler API key model — operator's choice at implementation time.

### 8. Token Predecessor Invalidation on Renewal

When `Renew()` issues a new JWT, it must revoke the old token's JTI via `RevSvc.Revoke("token", oldJTI)`. This ensures only one valid token exists per agent at any time.

The change is in `tkn_svc.go`'s `Renew` method — after successfully issuing the new token, call `revSvc.Revoke` on the old JTI. The audit event `token_renewed` should capture both old and new JTIs.

Edge case: if the revocation of the old token fails, the renewal should still succeed (the old token will expire naturally). Log a warning but don't fail the renewal.

### 9. JTI Blocklist Pruning

The `jtiConsumption` map in the identity service tracks consumed nonce JTIs to prevent replay. Currently this map grows forever. Add a background goroutine that runs every 60 seconds and removes entries older than the maximum possible token TTL (the nonce TTL of 30 seconds, so entries older than 30s are safe to prune).

Similarly, the revocation list should prune entries for tokens whose `exp` has passed — a revoked token that has already expired doesn't need to stay in the revocation check list.

### 10. Agent Record Expiry

Agent records need an `expires_at` field set to the token's expiry time at registration. A background goroutine (configurable interval, default 60s) marks expired agents as `status=expired`. Expired agents:

- Cannot renew tokens (401)
- Cannot delegate (401)
- Cannot perform any authenticated action
- Are NOT deleted — the audit trail needs the record for forensics
- Show as "expired" in `aactl` output

The `agents` table needs:

```sql
expires_at TEXT,    -- ISO 8601, set at registration to token exp
status     TEXT     -- 'active' or 'expired'
```

---

## Success Criteria

### App Lifecycle (Stories 1–10)
- `aactl app revoke` revokes all tokens for an app in one command
- Revoked app's agents can no longer use their tokens (immediate effect)
- Agent JWTs carry `app_id` and `app_name` in claims
- `POST /v1/token/validate` returns `app_id` and `app_name` for app-linked agents
- Secret rotation returns a new secret and the old one works during grace period
- Old secret stops working after grace period expires
- `GET /v1/audit/events?app_id=...` returns all events for that app
- Existing 4-level revocation (token, agent, task, chain) unchanged
- Legacy agents/tokens without `app_id` continue to work

### NIST Alignment (Stories 11–16)
- Agent JWTs carry `original_principal`, `task_id`, and `orch_id` claims
- `POST /v1/token/validate` returns all three new claims
- Delegated tokens propagate `original_principal` from root — never changes across hops
- `GET /v1/audit/events?original_principal=...` filters correctly
- Audit events from delegated actions include `intermediate_agents` ordered array
- `GET /v1/audit/verify` returns pass for an untampered chain
- `GET /v1/audit/verify` returns failure with corrupted event ID for a tampered chain
- `audit-reader` credential can query audit events and verify chain
- `audit-reader` credential gets 403 on all write endpoints
- Legacy tokens without new claims continue to validate

### Token Hygiene (Stories 17–19)
- After renewal, the old token's JTI is revoked (using old token returns 401)
- Only the new token is valid after renewal
- JTI blocklist entries are pruned after their TTL expires (memory doesn't grow indefinitely)
- Agent records have `expires_at` and transition to `status=expired` after TTL
- Expired agents cannot renew, delegate, or authenticate
- Expired agent records are NOT deleted (audit trail preserved)

---

## Testing Workflow

> **Before writing any test code**, extract the user stories from the `## User Stories` section above into a standalone file:
> `tests/phase-1c/user-stories.md`
>
> This is required by the project workflow (CLAUDE.md). The coding agent writes user stories first, saves them to `tests/`, then writes test code against them. Do not skip this step.
