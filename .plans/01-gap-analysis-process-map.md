# AgentAuth Gap Analysis — Process Map

**Date:** 2026-02-28
**Branch:** develop (all 6 compliance fixes merged)
**Source:** Architecture deep-dive of all 97 Go source files + BIG_BAD_GAP.md findings

---

## How to Read This Document

Each section maps a **production process** (what should happen), compares it to the **current implementation** (what actually happens), and calls out **gaps** with severity ratings:

- **BLOCKER** — Cannot ship to production without fixing
- **MAJOR** — Production will work but with significant security or operational risk
- **MINOR** — Rough edges, not blocking but should be addressed

---

## 1. APP REGISTRATION (BLOCKER — Front and Center)

### What Production Needs

```
Operator                        Broker                         Sidecar
   |                              |                              |
   |-- aactl app register ------->|                              |
   |   (name, scopes)             |                              |
   |                              |-- create app record           |
   |                              |-- create activation token     |
   |<-- activation_token ---------|                              |
   |                              |                              |
   |-- give token to deployment ---------------------------------------->|
   |   (env var or secret mgr)    |                              |
   |                              |                              |
   |                              |<-- POST /v1/sidecar/activate-|
   |                              |    (activation_token)         |
   |                              |-- validate token              |
   |                              |-- bind sidecar to app         |
   |                              |-- issue scoped sidecar JWT    |
   |                              |-->  sidecar_token ----------->|
   |                              |                              |
   |                              |                     Sidecar ready.
   |                              |                     No admin secret.
   |                              |                     Scoped credentials only.
```

### What Actually Happens Today

```
Operator                        docker-compose.yml              Sidecar              Broker
   |                              |                              |                     |
   |-- edit YAML file ----------->|                              |                     |
   |   add sidecar service        |                              |                     |
   |   paste AA_ADMIN_SECRET      |                              |                     |
   |   set scope ceiling          |                              |                     |
   |                              |                              |                     |
   |-- docker compose up -------->|-- start sidecar ------------>|                     |
   |                              |                              |                     |
   |                              |                              |-- POST /admin/auth ->|
   |                              |                              |   (admin secret)     |
   |                              |                              |<-- admin JWT --------|
   |                              |                              |                     |
   |                              |                              |-- create activation->|
   |                              |                              |   (using admin JWT)  |
   |                              |                              |<-- activation token -|
   |                              |                              |                     |
   |                              |                              |-- activate --------->|
   |                              |                              |<-- sidecar token ----|
   |                              |                              |                     |
   |                              |                     Sidecar ready.
   |                              |                     HOLDS ADMIN SECRET.
   |                              |                     Self-provisioned.
```

### Gaps

| # | Gap | Severity | Impact | Where to Fix |
|---|-----|----------|--------|--------------|
| G-1 | **No App entity in broker** — apps are invisible to the system | BLOCKER | Cannot track, manage, or audit apps. No app-level operations possible. | New: `internal/app/` package, `POST/GET/DELETE /v1/admin/apps` |
| G-2 | **No API-based app registration** — only docker-compose editing | BLOCKER | Onboarding requires infrastructure access, not an API call. Cannot integrate with CI/CD, self-service portals, or automation. | New: `aactl app register/list/remove/update` commands |
| G-3 | **Sidecar holds admin secret** — every sidecar is a full admin | BLOCKER | One compromised sidecar = full admin access (create tokens, revoke anything, read all audit). Blast radius is total. | Add `AA_ACTIVATION_TOKEN` env var path in `cmd/sidecar/bootstrap.go` |
| G-4 | **Sidecar self-provisions** — no external activation flow | BLOCKER | The activation endpoint exists (`POST /v1/sidecar/activate`) but nothing external ever calls it. The sidecar creates AND consumes its own activation token. | Modify bootstrap to use pre-created token when `AA_ACTIVATION_TOKEN` is set |
| G-5 | **No app-to-sidecar binding** — sidecars are orphaned entities | MAJOR | Cannot answer "which app does this sidecar serve?" Cannot revoke by app. Cannot audit by app. | App entity should reference its sidecar ID |
| G-6 | **Sidecar boots without an app** — proxy with nothing to proxy | MAJOR | Wastes resources, holds admin credentials unnecessarily, creates attack surface with no business value | App registration should gate sidecar creation |

---

## 2. AGENT IDENTITY LIFECYCLE

### What Production Needs

```
App/Sidecar                     Broker
   |                              |
   |-- GET /v1/challenge -------->|
   |<-- nonce (30s TTL) ---------|
   |                              |
   |-- generate Ed25519 keypair   |
   |-- sign nonce with privkey    |
   |                              |
   |-- POST /v1/register -------->|
   |   (launch_token, nonce,      |
   |    pubkey, signature,        |
   |    orch_id, task_id, scope)  |
   |                              |-- validate nonce (exists, not expired, not consumed)
   |                              |-- validate launch token (exists, not expired, not consumed)
   |                              |-- check scope attenuation (requested <= allowed)
   |                              |-- verify Ed25519 signature
   |                              |-- consume nonce + launch token
   |                              |-- generate SPIFFE ID
   |                              |-- issue JWT
   |                              |-- persist agent record
   |                              |-- audit: agent_registered
   |<-- agent_id + JWT -----------|
```

### Current State: Mostly Works

The challenge-response registration flow is solid. Ed25519 crypto, nonce management, scope attenuation all implemented correctly.

### Gaps

| # | Gap | Severity | Impact | Where to Fix |
|---|-----|----------|--------|--------------|
| G-7 | **No agent deregistration API** — agents persist forever in store | MINOR | SQLite grows indefinitely. No way to clean up stale agents. No `aactl agent remove`. | New: `DELETE /v1/admin/agents/{id}`, `aactl agent remove` |
| G-8 | **No agent listing for operators** — no `aactl agent list` | MINOR | Operators cannot see which agents are registered, their scopes, or last activity. Have to query SQLite directly. | New: `GET /v1/admin/agents`, `aactl agent list` |
| G-9 | **Launch token creation requires admin JWT** — sidecar uses admin secret to create them | MAJOR | Ties back to G-3. In the sidecar lazy-registration flow, sidecar authenticates as admin to create launch tokens for each agent. Should use sidecar-scoped credentials instead. | `cmd/sidecar/handler.go` — agent resolution should use sidecar token, not admin auth |

---

## 3. TOKEN LIFECYCLE

### What Production Needs

```
Token Issuance:
  Agent registers → gets JWT (scope, TTL, SPIFFE ID)

Token Usage:
  Agent → Resource Server (Bearer token in header)
  Resource Server → POST /v1/token/validate (verify signature)

Token Renewal:
  Before expiry → POST /v1/token/renew → new JWT (same scope, new timestamps)
  Sidecar does this automatically in background goroutine

Token Revocation (4 levels):
  token  → revoke single JWT by JTI
  agent  → revoke all JWTs for a SPIFFE ID
  task   → revoke all JWTs for a task_id
  chain  → revoke all JWTs in a delegation chain

Token Release (self-revocation):
  Agent → POST /v1/token/release → 204 (token JTI added to revocation set)
```

### Current State: Solid

Token lifecycle is the strongest part of the implementation. Issuance, verification, renewal, revocation (4 levels), and release all work correctly with persistence to SQLite.

### Gaps

| # | Gap | Severity | Impact | Where to Fix |
|---|-----|----------|--------|--------------|
| G-10 | **Ephemeral signing key** — broker generates fresh Ed25519 keypair on every startup | MAJOR | ALL tokens from previous broker instance become invalid on restart. No key persistence, no key rotation strategy. In production, a broker restart invalidates every in-flight token across all apps. | `cmd/broker/main.go` — add option to load key from file/env, with rotation support |
| G-11 | **No token introspection endpoint** — can't check token status without presenting it | MINOR | Resource servers can validate signatures but can't check revocation status without calling the full validation middleware. No RFC 7662-style introspection. | New: `POST /v1/token/introspect` |
| G-12 | **Revocation is in-memory first, SQLite second** — revocation check is fast but not distributed | MAJOR | If running multiple broker instances (HA), revocations aren't shared. One broker revokes, another doesn't know. Single-broker deployments are fine. | Architectural decision needed for multi-broker scenarios |

---

## 4. SIDECAR PROXY OPERATION

### What Production Needs

```
App (localhost)                  Sidecar                         Broker
   |                              |                              |
   |-- POST /v1/token ---------->|                              |
   |   (agent_name, scope, ttl)  |                              |
   |                              |-- check scope <= ceiling      |
   |                              |-- resolve agent identity      |
   |                              |   (cache or lazy register)    |
   |                              |                              |
   |                              |-- POST /v1/token/exchange -->|
   |                              |   (agent_id, scope, ttl)     |
   |                              |                              |-- validate sidecar token
   |                              |                              |-- check scope <= ceiling (again)
   |                              |                              |-- issue agent JWT
   |                              |<-- agent JWT ----------------|
   |                              |                              |
   |<-- agent JWT + metadata ----|                              |
   |                              |                              |
   |-- use JWT with resource ---->  (Resource Server validates)  |
```

### Current State: Works but with G-3 dependency

The sidecar token exchange, scope ceiling enforcement (dual: sidecar + broker), circuit breaker, and renewal loop all work correctly. The problem is HOW the sidecar gets its initial credentials (see G-3, G-4).

### Gaps

| # | Gap | Severity | Impact | Where to Fix |
|---|-----|----------|--------|--------------|
| G-13 | **Sidecar lazy-registration uses admin auth** — creates launch tokens with admin JWT | MAJOR | Every agent registration through the sidecar involves an admin auth call. The sidecar should use its own scoped token to create launch tokens, or use a different mechanism entirely. | `cmd/sidecar/handler.go` `resolveAgent()` function |
| G-14 | **No sidecar health visible to apps** — app can't check if sidecar is healthy before requesting | MINOR | Sidecar has a `/v1/health` endpoint but doesn't expose circuit breaker state or broker connectivity status to the calling app. App gets 503 only after trying. | Add circuit breaker status to health response |
| G-15 | **Sidecar agent cache is ephemeral** — lost on restart, agents re-register | MINOR | Slight latency on sidecar restart. Not a real problem in practice since re-registration is fast, but means extra broker load on rolling deployments. | Could persist registry to local file, but may not be worth the complexity |

---

## 5. DELEGATION

### What Production Needs

```
Agent A (delegator)              Broker                        Agent B (delegate)
   |                              |                              |
   |-- POST /v1/delegate ------->|                              |
   |   (delegate_to: B's SPIFFE, |                              |
   |    scope: [subset],          |                              |
   |    ttl: 300)                 |                              |
   |                              |-- validate A's token          |
   |                              |-- check scope attenuation     |
   |                              |-- check delegation depth < 5  |
   |                              |-- verify B exists in store    |
   |                              |-- build delegation chain      |
   |                              |-- compute chain hash (SHA256) |
   |                              |-- issue JWT for B             |
   |                              |-- audit: delegation_created   |
   |<-- delegated JWT ------------|                              |
   |                              |                              |
   |-- pass JWT to B out-of-band -------------------------------->|
   |                              |                              |-- use JWT
```

### Current State: Works

Delegation with scope attenuation, depth limits (max 5), chain hashing, and chain-level revocation all implemented.

### Gaps

| # | Gap | Severity | Impact | Where to Fix |
|---|-----|----------|--------|--------------|
| G-16 | **No delegation visibility for operators** — no way to see active delegation chains | MINOR | Operators can't see who delegated to whom. Must infer from audit trail. No `aactl delegation list`. | New: `GET /v1/admin/delegations`, `aactl delegation list` |
| G-17 | **Delegation requires knowing SPIFFE ID** — delegator must know delegate's exact ID | MINOR | In practice, agents don't know each other's SPIFFE IDs. Need out-of-band coordination. Could offer lookup by agent name or task_id. | Enhancement to `POST /v1/delegate` to accept agent_name lookup |

---

## 6. AUDIT TRAIL

### What Production Needs

```
Any authenticated operation       Broker                        SQLite
   |                              |                              |
   |-- (any API call) ----------->|                              |
   |                              |-- process request             |
   |                              |-- record audit event           |
   |                              |   (type, agent, task,          |
   |                              |    outcome, detail, resource)  |
   |                              |-- compute hash chain           |
   |                              |   SHA256(prevHash + eventData) |
   |                              |-- persist to SQLite ---------->|
   |                              |                              |

Operator queries:
   |-- aactl audit events ------->|                              |
   |   (--agent-id, --task-id,    |                              |
   |    --event-type, --outcome)  |                              |
   |                              |-- query with filters -------->|
   |                              |<-- matching events -----------|
   |<-- formatted table ----------|                              |
```

### Current State: Strong (Fix 6 completed)

30+ event types, hash chain integrity, SQLite persistence, outcome tracking, filtering by agent/task/event/outcome/time range. All working.

### Gaps

| # | Gap | Severity | Impact | Where to Fix |
|---|-----|----------|--------|--------------|
| G-18 | **No hash chain verification command** — can't prove chain integrity | MAJOR | The hash chain exists but there's no `aactl audit verify` to walk the chain and confirm no tampering. The security feature exists but can't be validated by operators. | New: `aactl audit verify` command |
| G-19 | **No audit export** — can't ship audit logs to external SIEM | MINOR | Audit stays in SQLite. No export to JSON, no webhook, no syslog forwarding. Production security teams need audit data in their SIEM. | New: `aactl audit export --format json` or webhook integration |
| G-20 | **No audit retention/rotation** — SQLite grows forever | MINOR | No max event count, no TTL on old events, no rotation. In long-running production, audit table grows unbounded. | Add `AA_AUDIT_RETENTION_DAYS` config |
| G-21 | **No app-level audit queries** — can't filter by app (because apps don't exist, see G-1) | MAJOR | Operators can filter by agent_id or task_id, but not by app. "Show me all audit events for my-app" is impossible. Depends on G-1 being fixed first. | After G-1: add app_id to audit events and filter support |

---

## 7. OPERATOR EXPERIENCE (aactl)

### What Production Needs

An operator should be able to manage the entire system through `aactl` without touching config files, databases, or raw HTTP calls.

### Current aactl Commands

| Command | Status |
|---------|--------|
| `aactl sidecars list` | Works |
| `aactl sidecars ceiling get <id>` | Works |
| `aactl sidecars ceiling set <id> <scopes>` | Works |
| `aactl token release --token <jwt>` | Works |
| `aactl revoke --level --target` | Works |
| `aactl audit events [filters]` | Works |

### Missing aactl Commands

| Command | Gap # | Severity |
|---------|-------|----------|
| `aactl app register --name --scopes` | G-1, G-2 | BLOCKER |
| `aactl app list` | G-1, G-2 | BLOCKER |
| `aactl app remove --name` | G-1, G-2 | BLOCKER |
| `aactl app update --name --scopes` | G-1, G-2 | BLOCKER |
| `aactl agent list` | G-8 | MINOR |
| `aactl agent remove --id` | G-7 | MINOR |
| `aactl audit verify` | G-18 | MAJOR |
| `aactl audit export` | G-19 | MINOR |
| `aactl delegation list` | G-16 | MINOR |

---

## 8. SECURITY POSTURE

### What's Solid

- Ed25519 challenge-response (replay-proof)
- Scope attenuation at 3 enforcement points
- 4-level revocation with persistence
- Rate limiting on admin auth (5 req/s)
- Constant-time secret comparison
- Nonce TTL + single-use consumption
- Hash chain audit trail
- TLS/mTLS support

### Security Gaps

| # | Gap | Severity | Impact |
|---|-----|----------|--------|
| G-3 | Admin secret in every sidecar | BLOCKER | Full admin compromise from any sidecar |
| G-10 | Ephemeral signing key | MAJOR | Broker restart invalidates all tokens |
| G-22 | **No secret rotation** — changing AA_ADMIN_SECRET requires restarting everything | MAJOR | Can't rotate compromised secrets without full outage |
| G-23 | **Metrics endpoint is unauthenticated** — `GET /v1/metrics` is public | MINOR | Exposes token counts, sidecar counts, error rates to anyone who can reach the broker. Information leakage in production. |
| G-24 | **No request signing between sidecar and broker** — relies solely on bearer tokens | MINOR | If network is compromised, tokens can be replayed. mTLS mitigates this but isn't enforced by default. |

---

## GAP SUMMARY — SORTED BY PRIORITY

### BLOCKERS (Must fix before production / demo)

| # | Gap | Component |
|---|-----|-----------|
| G-1 | No App entity in broker | `internal/app/` (new) |
| G-2 | No API-based app registration | `internal/admin/`, `cmd/aactl/` |
| G-3 | Sidecar holds admin secret | `cmd/sidecar/bootstrap.go` |
| G-4 | Sidecar self-provisions (bypasses activation flow) | `cmd/sidecar/bootstrap.go` |
| G-5 | No app-to-sidecar binding | `internal/app/` (new) |
| G-6 | Sidecar boots without an app | `cmd/sidecar/main.go` |

### MAJOR (Should fix for production readiness)

| # | Gap | Component |
|---|-----|-----------|
| G-9 | Sidecar lazy-registration uses admin auth | `cmd/sidecar/handler.go` |
| G-10 | Ephemeral signing key (lost on restart) | `cmd/broker/main.go` |
| G-12 | Revocation not distributed (single-broker only) | `internal/revoke/` |
| G-13 | Sidecar agent registration uses admin JWT | `cmd/sidecar/handler.go` |
| G-18 | No hash chain verification command | `cmd/aactl/audit.go` |
| G-21 | No app-level audit queries | `internal/audit/`, `internal/handler/` |
| G-22 | No secret rotation mechanism | `internal/cfg/`, `internal/admin/` |

### MINOR (Polish for production)

| # | Gap | Component |
|---|-----|-----------|
| G-7 | No agent deregistration | `internal/admin/`, `cmd/aactl/` |
| G-8 | No agent listing for operators | `internal/admin/`, `cmd/aactl/` |
| G-11 | No token introspection endpoint | `internal/handler/` |
| G-14 | No sidecar health visibility for apps | `cmd/sidecar/` |
| G-15 | Sidecar agent cache is ephemeral | `cmd/sidecar/registry.go` |
| G-16 | No delegation visibility | `internal/admin/`, `cmd/aactl/` |
| G-17 | Delegation requires knowing SPIFFE ID | `internal/deleg/` |
| G-19 | No audit export | `cmd/aactl/audit.go` |
| G-20 | No audit retention/rotation | `internal/audit/` |
| G-23 | Metrics endpoint unauthenticated | `internal/handler/` |
| G-24 | No request signing sidecar↔broker | architectural |

---

## TOTAL: 24 Gaps

- **6 BLOCKERS** (all related to app registration)
- **7 MAJOR** (security + operational)
- **11 MINOR** (polish + completeness)

The blockers all trace to the same root cause: **apps don't exist as a concept in the broker.** Fixing G-1 (App entity) unlocks G-2 through G-6.
