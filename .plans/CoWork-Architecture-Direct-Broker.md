# Architecture Design: Direct-to-Broker App Registration

**Date:** 2026-03-01
**Question:** Do we need the Token Proxy (sidecar)? What if apps register directly with the broker?
**Pattern compliance check:** Ephemeral Agent Credentialing Pattern v1.2

---

## The Question Divine Asked

> "Could we not still have a register app and it give the user an app_id with client_id or secret id or all three that just says you can use the broker... all the other things for agents security is still there but separate. The reason we initially had the sidecar was so the 3rd party developer doesn't have to talk to the broker but why shouldn't it talk to the broker?"

This is the right question. Let's think it through completely.

---

## Pattern Compliance First

Before redesigning anything, we checked whether removing the mandatory sidecar violates the Ephemeral Agent Credentialing Pattern v1.2. It does not.

**What the pattern REQUIRES:**
- Ephemeral identity per agent instance (Ed25519 keypair + SPIFFE ID)
- Cryptographic signatures on all tokens (Ed25519 or RSA)
- JWT with required claims: sub, aud, exp, jti, scope
- Immutable audit logging (append-only, tamper-proof)
- Scope validation (task-scoped authorization)
- mTLS between agent and server

**What we use instead of SPIRE (and why it's pattern-compliant):**

The pattern references SPIRE as one example of an identity provider, but explicitly allows **"or equivalent identity provider."** AgentAuth opted out of SPIRE specifically because of the heavy infrastructure it requires — SPIRE needs a dedicated server, sidecar agents on every node, a certificate authority, and a persistence layer just to issue identities. That's exactly the kind of infrastructure overhead we're trying to avoid.

Instead, AgentAuth implements a **self-contained Ed25519 challenge-response identity system** built directly into the broker:

1. Agent generates an Ed25519 keypair in memory (ephemeral, per-instance)
2. Broker issues a cryptographic nonce (64-char hex, 30-second TTL, single-use)
3. Agent signs the nonce with its Ed25519 private key
4. Broker verifies the signature against the public key
5. Broker generates a SPIFFE-format ID: `spiffe://trust-domain/agent/{orchID}/{taskID}/{instanceID}`
6. Broker issues a scoped, short-lived JWT bound to that identity

This satisfies the pattern because:
- **Ephemeral identity** — each agent instance gets a unique cryptographically-bound identity (same as SPIRE)
- **Cryptographic attestation** — Ed25519 signature verification proves the agent holds the private key (same as SPIRE's X.509 SVIDs, just simpler)
- **SPIFFE-format IDs** — agent identities follow the standard SPIFFE path format (interoperable)
- **No external infrastructure** — no SPIRE server, no workload attestation agents, no certificate authority, no PKI chain

The same logic now applies to the sidecar question. The pattern cares about WHAT happens (attestation, credential issuance, scope enforcement), not HOW the communication is structured. A sidecar is one implementation. An SDK that talks directly to the broker is another. Both satisfy the pattern — just like the broker satisfies the identity provider role without needing SPIRE.

**What the pattern says about app registration:**
The pattern describes registration with `client_id`, `client_name`, `client_uri`, and `token_endpoint_auth_method`. This implies apps should be registered entities — exactly what's missing today.

**Verdict: Removing the mandatory sidecar is fully pattern-compliant.** The security primitives (Ed25519 challenge-response, scope attenuation, short-lived JWTs, delegation chains, revocation, audit) all live in the broker. They don't depend on the sidecar.

---

## What the Sidecar Actually Does (and What We Keep)

The Token Proxy currently serves five functions. Let's evaluate each:

| Function | What It Does | Stays in Broker? | Needs SDK? |
|----------|-------------|-----------------|------------|
| **API simplifier** | Turns 5 broker calls into 1 `/v1/token` call | N/A — broker already has the endpoints | Yes — SDK wraps the multi-call flow |
| **Ed25519 key manager** | Generates keypairs for agents, stores them locally | N/A — this is an app-side concern | Yes — SDK handles key generation |
| **Scope enforcer** | Checks scope ceiling locally before calling broker | Stays — broker enforces scope at registration | SDK validates locally too |
| **Circuit breaker** | Caches tokens, serves stale when broker is down | Stays as optional middleware | SDK can cache too |
| **Master key holder** | Authenticates as admin for every new agent | **REMOVED** — this is the problem we're solving | App uses scoped credentials instead |

**The sidecar's value is convenience, not security.** Every security function (identity verification, scope enforcement, revocation, audit) happens in the broker. The sidecar is a DX layer that got promoted to a mandatory dependency.

---

## The Proposed Architecture: Apps as First-Class Entities

### The Core Idea

Think of it like this analogy: today, the only way to get into the building (broker) is through a specific lobby desk (sidecar) that happens to have a copy of the master key. What we're proposing is: give each company (app) their own badge (credentials) that opens the front door directly. The lobby desk can still exist for convenience, but it's not the only entrance.

### How App Registration Works

**Step 1: Operator registers the app**
```
aactl app register --name "weather-bot" --scopes "read:weather:*,write:logs:*"
```

The broker creates:
- An **App Record** (app_id, name, scope ceiling, status, created_at)
- A **Client ID** (unique identifier, like `app-weather-bot-a1b2c3`)
- A **Client Secret** (random 64-char hex, stored hashed in broker)
- An **Activation Token** (single-use JWT for initial bootstrap, optional)

The operator receives:
```json
{
  "app_id": "app-weather-bot-a1b2c3",
  "client_id": "wb-a1b2c3d4e5f6",
  "client_secret": "sk_live_...",
  "activation_token": "eyJ...",
  "scopes": ["read:weather:*", "write:logs:*"],
  "broker_url": "https://broker.company.com"
}
```

**Step 2: Developer configures the app**

Option A — Using the Python SDK (recommended):
```python
from agentauth import AgentAuthClient

client = AgentAuthClient(
    broker_url="https://broker.company.com",
    client_id="wb-a1b2c3d4e5f6",
    client_secret="sk_live_..."
)

# Get a token for an agent
token = client.get_token(
    agent_name="forecast-agent",
    scope=["read:weather:current"]
)
```

Option B — Using the Token Proxy (optional, for resilience):
```bash
# Operator deploys proxy with activation token — NOT the master key
AA_ACTIVATION_TOKEN=eyJ... AA_BROKER_URL=https://broker.company.com ./token-proxy
```

Option C — Direct HTTP (no SDK, no proxy):
```bash
# Authenticate the app
curl -X POST https://broker.company.com/v1/app/auth \
  -d '{"client_id": "wb-a1b2c3d4e5f6", "client_secret": "sk_live_..."}'

# Register an agent (challenge-response)
curl -X POST https://broker.company.com/v1/challenge \
  -H "Authorization: Bearer $APP_TOKEN" \
  -d '{"agent_name": "forecast-agent"}'

# ... complete challenge-response, get agent token
```

### What Stays the Same

Everything about agent security stays identical:

- **Ed25519 challenge-response:** Agents still generate keypairs, sign nonces, prove identity. The pattern's core security mechanism is untouched.
- **SPIFFE IDs:** Agents still get `spiffe://trust-domain/agent/orch/task/instance` identities.
- **Scope attenuation:** Scopes still narrow one-way. An app with `read:weather:*` cannot grant `write:weather:*`.
- **Short-lived JWTs:** Tokens still expire in minutes, not hours.
- **Delegation chains:** Multi-agent delegation still works with cryptographic chain verification.
- **4-level revocation:** Token, agent, task, and chain revocation all still work — and now you can also revoke by app.
- **Hash-chained audit trail:** Still tamper-evident, still persistent — and now you can query by app name.

### What Changes

| Concern | Today | Proposed |
|---------|-------|----------|
| **App identity** | Does not exist | App entity in broker with name, scopes, credentials |
| **How apps authenticate** | Token Proxy uses master key | Apps use client_id + client_secret (scoped) |
| **Master key location** | In every proxy config | Only on the broker and in the operator's vault |
| **Adding an app** | Edit infrastructure config | API call: `POST /v1/admin/apps` |
| **Removing an app** | Remove infrastructure | API call: `DELETE /v1/admin/apps/{id}` |
| **Listing apps** | Impossible | `GET /v1/admin/apps` or `aactl app list` |
| **Audit by app** | Impossible (all "sidecar") | Query by app_id: "what did weather-bot do?" |
| **Token Proxy** | Mandatory | Optional — use for DX/resilience, not required |
| **Developer experience** | Hand-code HTTP to proxy | SDK: `client.get_token(scope=[...])` |
| **Security review** | Fails (master key everywhere) | Passes (scoped credentials, no master key spread) |

---

## How This Solves Every Problem We Found

### Problem 1: Apps don't exist → SOLVED
Apps are now first-class entities. Register, list, update, revoke, audit — all by app name.

### Problem 2: Token Proxy is mandatory → SOLVED
Three paths: SDK (direct), Token Proxy (optional), raw HTTP (possible). The proxy adds value but isn't required.

### Problem 3: Master key everywhere → SOLVED
Apps authenticate with their own client_id/client_secret. The master key stays with the operator. Period. If an app's credentials are compromised, you revoke that ONE app — not rebuild the entire system.

### Problem 4: Broker restart destroys everything → PARTIALLY SOLVED
This is a separate problem from app registration. The architecture change doesn't fix ephemeral signing keys directly. BUT: with an SDK handling renewal and with app-level credentials (not master key), recovery after restart is:
1. SDK detects token failure
2. SDK re-authenticates with client_id/client_secret (still valid — stored hashed in broker DB)
3. SDK re-registers agents
4. No master key involved

The signing key persistence problem (Section 2.5 of the gap analysis) still needs its own fix, but recovery is much cleaner.

### Problem 5: Audit trail useless → SOLVED
Every token now carries `app_id` in its claims. Every audit event records which app caused it. "What did weather-bot do last Tuesday?" becomes a simple query.

### Problem 6: Key rotation is a coordinated outage → MOSTLY SOLVED
Since apps have their own credentials (not the master key), rotating the ADMIN master key only affects the operator's access — not every app in the system. App credentials can be rotated independently per-app.

### Problem 7: Token validation is a single point of failure → NOT SOLVED BY THIS
This still needs a JWKS endpoint (separate enhancement). But it's independent of the sidecar question.

---

## What Needs to Be Built

### Phase 1: App Registration (P0 — unblocks everything)

**Broker changes:**

1. **New data model: AppRecord**
   - Fields: app_id, name, client_id, client_secret_hash, scope_ceiling, status, associated_sidecar_id (optional), created_at, updated_at, created_by
   - Stored in SQLite (persistent, survives restarts)

2. **New service: AppSvc**
   - `RegisterApp(name, scopes, createdBy)` → creates AppRecord, generates client_id + client_secret, returns credentials
   - `AuthenticateApp(clientID, clientSecret)` → validates credentials, returns scoped JWT
   - `ListApps()` → returns all registered apps
   - `UpdateApp(appID, newScopes)` → updates scope ceiling
   - `DeregisterApp(appID)` → revokes all tokens, marks inactive
   - `RevokeApp(appID)` → revokes all tokens for this app immediately

3. **New handler endpoints:**
   - `POST /v1/admin/apps` — register a new app
   - `GET /v1/admin/apps` — list all apps
   - `GET /v1/admin/apps/{id}` — get app details
   - `PUT /v1/admin/apps/{id}` — update app scopes
   - `DELETE /v1/admin/apps/{id}` — deregister app
   - `POST /v1/app/auth` — app authenticates with client_id + client_secret (NOT admin key)

4. **New CLI commands:**
   - `aactl app register --name NAME --scopes SCOPES`
   - `aactl app list`
   - `aactl app update --id ID --scopes SCOPES`
   - `aactl app remove --id ID`
   - `aactl app revoke --id ID`

5. **Token claims extension:**
   - Add `app_id` and `app_name` to TknClaims
   - Backward-compatible (empty for legacy tokens)

6. **Audit trail extension:**
   - All app-level operations recorded
   - Agent tokens carry app_id → audit events attributable to apps

### Phase 2: Activation Token Bootstrap (P0 — stops master key spread)

1. **Token Proxy supports `AA_ACTIVATION_TOKEN`**
   - If set, skip admin auth entirely
   - Call `POST /v1/sidecar/activate` directly
   - Sidecar gets scoped credentials, tied to an app

2. **App registration returns activation token**
   - Operator creates app → gets activation token for proxy deployment
   - Token is single-use, scoped to the app's ceiling
   - Master key never leaves the operator's environment

3. **Deprecate `AA_ADMIN_SECRET` in proxy config**
   - Still works (backward compatible) but logs a warning
   - Documentation updated to recommend activation token path

### Phase 3: Python SDK (P1 — developer experience)

1. **`agentauth` Python package**
   - `AgentAuthClient(broker_url, client_id, client_secret)`
   - `client.get_token(agent_name, scope)` — handles full flow (challenge-response, Ed25519, token exchange)
   - `client.renew_token(token)` — automatic renewal
   - `client.validate_token(token)` — local validation (when JWKS available) or online
   - Built-in retry, backoff, token caching

2. **Handles Ed25519 internally**
   - SDK generates keypair per agent
   - SDK signs challenges
   - Developer never sees Ed25519 — just calls `get_token()`

3. **Makes the Token Proxy truly optional**
   - SDK provides the same DX (one call to get a token) without requiring infrastructure deployment
   - Token Proxy remains available for environments that want circuit breaker + caching at the infrastructure level

### Phase 4: JWKS Endpoint (P1 — removes validation SPOF)

1. **`GET /.well-known/jwks.json`** on broker
   - Exposes current Ed25519 public key
   - Resource servers cache and validate locally
   - Online validation (`POST /v1/token/validate`) remains for revocation checks

2. **Requires key persistence (Phase 5)**
   - JWKS only works if the public key doesn't change every restart

### Phase 5: Key Persistence (P1 — broker restart resilience)

1. **Persist Ed25519 signing key to encrypted file or DB**
   - Broker loads existing key on restart
   - Existing tokens remain valid
   - OR: Dual-key rotation (new key signs, old key verifies for grace period)

---

## Architecture Comparison Diagram

### Today: Sidecar-Mandatory Architecture

```
┌─────────────┐
│  OPERATOR    │
│              │
│  Edits YAML  │──── deploys ────┐
│  Pastes key  │                 │
└─────────────┘                 ▼
                         ┌──────────────┐
                         │ TOKEN PROXY  │
                         │              │
                         │ Holds master │
                         │ key (!)      │
┌─────────────┐          │              │         ┌──────────────┐
│  DEVELOPER  │──────────│ /v1/token    │─────────│    BROKER    │
│  (app code) │  only    │              │  admin  │              │
│             │  path    │ Self-         │  auth   │ Ed25519 keys │
└─────────────┘          │ provisions   │         │ Tokens, auth │
                         └──────────────┘         │ Audit trail  │
                                                  └──────────────┘
```

**Problems:** Master key in every proxy. No app entity. Infrastructure-as-registration. Audit blind.

### Proposed: Direct Registration + Optional Proxy

```
┌─────────────┐
│  OPERATOR    │
│              │
│  aactl app   │──── API call ────┐
│  register    │                  │
└─────────────┘                  ▼
                          ┌──────────────┐
                          │    BROKER    │
                          │              │
                          │ App registry │
                          │ Ed25519 keys │
                          │ Tokens, auth │
                          │ Audit trail  │
                          └──────┬───────┘
                                 │
                    ┌────────────┼────────────┐
                    │            │            │
                    ▼            ▼            ▼
             ┌──────────┐ ┌──────────┐ ┌──────────┐
             │  PATH A  │ │  PATH B  │ │  PATH C  │
             │          │ │          │ │          │
             │  SDK     │ │  Proxy   │ │  HTTP    │
             │  direct  │ │  (opt.)  │ │  direct  │
             │          │ │          │ │          │
             │ No proxy │ │ Scoped   │ │ No SDK   │
             │ needed   │ │ token    │ │ no proxy │
             │          │ │ only (!) │ │          │
             └──────────┘ └──────────┘ └──────────┘
                    │            │            │
                    ▼            ▼            ▼
             ┌─────────────────────────────────────┐
             │           DEVELOPER (app)           │
             │                                      │
             │  Same agent security regardless of   │
             │  which path: Ed25519, SPIFFE, scopes │
             └─────────────────────────────────────┘
```

**What changed:** Apps register via API. Three paths to the broker (SDK, proxy, HTTP). Master key stays with operator. Proxy gets scoped token, not master key.

---

## Answering the Original Questions

**"Do we need the sidecar?"**
No, not as a mandatory component. The sidecar provides genuine operational value (simplified API, circuit breaker, cached tokens), but the security model does not depend on it. Every security function lives in the broker. The sidecar should be an optional deployment choice, not a requirement.

**"Is that the problem?"**
It's the ROOT of the problem. Making the sidecar mandatory caused a cascade:
1. No app registration (because registration = deploy a sidecar)
2. Master key everywhere (because sidecars self-provision)
3. Audit blindness (because all sidecars look the same)
4. Infrastructure-as-process (because adding an app = deploying infrastructure)

Remove the mandatory sidecar, and every one of these problems has a clean solution.

**"Could we just register an app and give it credentials to talk to the broker?"**
Yes. App registers → gets client_id + client_secret → authenticates with broker → broker issues scoped tokens for the app's agents. All the agent security (Ed25519 challenge-response, SPIFFE IDs, scope attenuation, delegation chains, revocation, audit) stays exactly the same. The app just talks to the broker instead of talking to a proxy that talks to the broker.

**"Why shouldn't the developer talk to the broker?"**
There's no reason they shouldn't. The original idea was "protect the developer from complexity," but the complexity was created by the sidecar itself. With an SDK that wraps the broker API, the developer experience is just as simple: `client.get_token(scope=[...])`. The SDK handles Ed25519, challenge-response, and renewal internally — exactly what the proxy does, but without requiring an infrastructure deployment.

**"Is this compliant with the pattern?"**
Yes. We already proved this once — we opted out of SPIRE and built a self-contained Ed25519 challenge-response identity system instead. That's fully pattern-compliant because the pattern requires "ephemeral identity via cryptographic attestation," not specifically SPIRE's certificate-based workload identity. The same reasoning applies to the sidecar: the pattern requires the security primitives (ephemeral identity, cryptographic signatures, scope enforcement, audit), not a specific network topology or middleware layer. The broker + SDK satisfies the pattern just like the broker + Ed25519 satisfies the identity requirement without SPIRE.

---

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Client secret compromise | Medium | Per-app secrets (not master key). Rotate one app without affecting others. Revoke instantly via `aactl app revoke`. |
| SDK bugs in Ed25519 handling | Medium | Reference implementation + thorough testing. Crypto code is well-understood (same as current proxy). |
| Losing circuit breaker resilience | Low | SDK can implement token caching. Proxy still available for environments that want infrastructure-level resilience. |
| Backward compatibility | Low | Phase in gradually. Master key path still works (deprecated). Existing proxies keep running. |
| More broker load (no proxy caching) | Low | SDK caches tokens locally. JWKS endpoint enables local validation. Proxy available for heavy users. |

---

## Implementation Priority

| Phase | What | Effort | Unblocks |
|-------|------|--------|----------|
| **1** | App registration API + CLI | 2-3 days | Everything — this is the blocker |
| **2** | Activation token bootstrap | 1-2 days | Master key removal from proxies |
| **3** | Python SDK | 3-5 days | Developer experience, proxy-optional |
| **4** | JWKS endpoint | 1 day | Local token validation |
| **5** | Key persistence | 1-2 days | Broker restart resilience |

**Total estimated effort: 8-13 days to transform AgentAuth from a security library into an operationalizable product.**

---

## Peer Review: Implementation Gaps Identified

This architecture was reviewed by a 3rd-party developer. Their verdict: **"The design works. The gaps are implementation details, not architectural flaws."** Here are the 7 gaps they identified and our decisions on each:

### Gap 1: Agent Registration Flow — How do apps create agents?

**The problem:** Today's flow requires a launch token in the `POST /v1/register` request body. The doc's direct-HTTP example shows app authentication → challenge → register, but doesn't explain how the app's JWT replaces the launch token.

**Decision: Option A — Apps create their own launch tokens (Phase 1b).**

The app authenticates → gets a JWT with `app:launch-tokens:*` scope → calls `POST /v1/admin/launch-tokens` (scoped to its own ceiling) → proceeds with the existing challenge-response flow. This reuses existing broker code, requires the smallest change, and the SDK hides the extra call from the developer.

Option B (new direct registration path where the app JWT replaces the launch token) is cleaner long-term but requires changes to the register handler's auth logic. That's a Phase 3+ optimization.

### Gap 2: App JWT Scopes — What scopes does the app JWT carry?

**Decision: New `app:` scope family.**

```
app:launch-tokens:*    → create launch tokens within app ceiling
app:agents:*           → register agents, manage agent tokens
app:audit:read         → query audit events for own app only
```

This matters because the broker's `ValMw.RequireScope()` middleware makes authorization decisions based on these scope strings. These scopes must be defined and enforced in Phase 1a.

### Gap 3: Drop `associated_sidecar_id` from AppRecord

**Decision: Agreed — drop it.**

The whole point is decoupling apps from sidecars. If an operator deploys a sidecar for an app, the sidecar gets the app's activation token — that's the only link needed. No field in the data model.

### Gap 4: Client Secret Rotation Mechanism

**Decision: Add `POST /v1/admin/apps/{id}/rotate-secret` (Phase 1c).**

Returns new secret. Old secret remains valid for a configurable grace period (default 24h). Without this, "rotating credentials" means downtime. Dual-secret support (primary + secondary) is a Phase 3+ enhancement.

### Gap 5: Per-App Rate Limiting on `POST /v1/app/auth`

**Decision: Per-client_id rate limiting in Phase 1a.**

A compromised client_id shouldn't be able to DoS the auth endpoint for other apps. The current `POST /v1/admin/auth` rate limiter is per-IP only. The app auth endpoint needs per-client_id limiting (e.g., 10 auth attempts per minute per client_id).

### Gap 6: Legacy Agents Have No `app_id` in Audit Trail

**Decision: Explicitly document that legacy sidecar agents have `app_id: null` in audit events.**

This is expected behavior during the transition period. Two classes of agents in the audit trail (with and without app_id) is fine for backward compatibility. Operators can identify legacy agents by the null app_id and migrate them over time.

### Gap 7: App-Level Revocation = 5th Revocation Level

**Decision: Add `"level": "app"` to `POST /v1/revoke` (Phase 1c).**

```json
{"level": "app", "target": "app-weather-bot-a1b2c3"}
```

This cascades — revoking an app revokes all its agents' tokens. The `RevSvc` needs to index tokens by `app_id` to make this performant. This extends the existing 4-level revocation pyramid (token → agent → task → chain) to 5 levels (token → agent → task → chain → app).

---

## Revised Implementation Phases (Post-Review)

The reviewer correctly identified that Phase 1 as originally written is too big. Splitting it:

| Phase | What | Effort | Unblocks |
|-------|------|--------|----------|
| **1a** | AppRecord data model + `POST /v1/admin/apps` + `POST /v1/app/auth` + `aactl app register/list` + app JWT scopes + per-app rate limiting | 1-2 days | Core concept validation |
| **1b** | App-scoped launch tokens (apps can create agents within their ceiling) | 1 day | Agent registration via app credentials |
| **1c** | App-level revocation + `app_id` in token claims + audit attribution + secret rotation endpoint | 1 day | Production operations |
| **2** | Activation token bootstrap for sidecar (no more master key in proxy config) | 1 day | Master key removal from proxies |
| **3** | Python SDK with direct-to-broker flow | 3-5 days | Developer experience, proxy-optional |
| **4** | JWKS endpoint | 1 day | Local token validation |
| **5** | Key persistence | 1-2 days | Broker restart resilience |

**Revised total: 9-14 days.** Phase 1a validates the core concept before building the full chain.

---

## The Showcase: Agent Registration Still Works

The 3rd-party developer's feedback confirms: **agent registration is the core of the showcase.** Ed25519 challenge-response → scoped token → scope enforcement → revocation → audit trail — that's what proves AgentAuth works. Everything else (app registration, SDK, audit improvements) supports it, but the security story is the registration flow.

The key change: instead of the sidecar creating launch tokens with the master key, the app creates launch tokens with its own scoped credentials. The agent registration flow itself (challenge → sign nonce → verify → issue JWT) stays identical.

---

## Full Pattern Flow: How Every Capability Works End-to-End

The architecture change (apps as first-class entities, optional sidecar) only touches HOW apps authenticate to the broker. Everything downstream — the entire Ephemeral Agent Credentialing Pattern — stays identical. This section proves it by walking through every capability the system implements and showing what's preserved vs. what changes.

### The Complete Agent Lifecycle

```
App Registration → App Auth → Launch Token → Agent Identity → Token Issuance
     (NEW)          (NEW)     (preserved)    (preserved)      (preserved)
                                    ↓
Scope Enforcement ← Delegation ← Token Use → Revocation → Audit Trail
   (preserved)      (preserved)  (preserved)  (enhanced)   (enhanced)
```

### Stage 1: App Registration (NEW)

**Today:** Apps don't exist as entities. An operator deploys a sidecar with the admin master key baked into its config. "Registration" means infrastructure deployment.

**Proposed:** Operator runs `aactl app register --name "weather-bot" --scopes "read:weather:*"`. The broker creates an AppRecord with client_id, client_secret (hashed), and a scope ceiling. The operator gives the developer these credentials. No infrastructure deployed. No master key shared.

**Pattern requirement met:** The pattern describes registration with `client_id`, `client_name`, `client_uri`, and `token_endpoint_auth_method`. This is exactly what we're implementing.

### Stage 2: App Authentication (NEW)

**Today:** The sidecar calls `POST /v1/admin/auth` with the master key → gets an admin JWT with full privileges.

**Proposed:** The app calls `POST /v1/app/auth` with client_id + client_secret → gets a scoped JWT limited to the app's ceiling. The app JWT carries `app:launch-tokens:*`, `app:agents:*`, `app:audit:read` scopes.

**What's different:** The credential is scoped (not admin), per-app (not shared), and revocable independently.

### Stage 3: Launch Token Creation (PRESERVED)

**Today:** The sidecar (authenticated as admin) calls `POST /v1/admin/launch-tokens` to create a single-use launch token for agent registration. The launch token has a scope ceiling and max TTL.

**Proposed:** The app (authenticated with its own JWT) calls the same endpoint. The broker enforces that the launch token's scope cannot exceed the app's ceiling. Same endpoint, same logic, same validation — different credential providing authorization.

**Code path:** `handler.AdminHandler.HandleCreateLaunchToken()` → `store.LaunchStore.CreateLaunchToken()`. No changes needed to this code. The only change is the auth middleware accepting app JWTs in addition to admin JWTs.

### Stage 4: Agent Identity — Ed25519 Challenge-Response (PRESERVED — Zero Changes)

This is the core of the pattern. Every step stays identical:

1. **Agent generates Ed25519 keypair** in memory (ephemeral, per-instance)
   - Code: `ed25519.GenerateKey(rand.Reader)` in the agent/SDK
   - The keypair exists only in the agent's process memory

2. **Agent requests a challenge nonce**
   - `POST /v1/challenge` with the launch token + public key
   - Code: `handler.RegHandler.HandleChallenge()` → `store.NonceStore.CreateNonce()`
   - Nonce: 64-char hex, 30-second TTL, single-use, stored in `nonce_store` table

3. **Agent signs the nonce**
   - Agent signs the nonce bytes with its Ed25519 private key
   - Code: `ed25519.Sign(privateKey, nonceBytes)`

4. **Agent registers with signed nonce**
   - `POST /v1/register` with public key + signed nonce + orchestrator/task/instance IDs
   - Code: `handler.RegHandler.HandleRegister()`
   - Broker verifies: `ed25519.Verify(publicKey, nonce, signature)`
   - Broker generates SPIFFE ID: `spiffe://{trustDomain}/agent/{orchID}/{taskID}/{instanceID}`
   - Agent record created in `agents` table

**Nothing changes here.** The challenge-response flow doesn't know or care whether the launch token came from an admin-authenticated sidecar or an app-authenticated SDK. The launch token is the launch token.

### Stage 5: Token Issuance (PRESERVED — Enhanced with app_id)

**After successful registration, the broker issues a JWT:**

Token claims (existing):
- `sub` — SPIFFE ID of the agent
- `aud` — intended audience
- `exp` — expiration (short-lived, minutes not hours)
- `jti` — unique token ID (UUID, for revocation tracking)
- `scope` — array of `action:resource:identifier` strings
- `iss` — broker identity
- `delegation_chain` — chain hash if delegated

Token claims (new, added):
- `app_id` — which app this agent belongs to (null for legacy agents)
- `app_name` — human-readable app name

**Code path:** `token.TknSvc.Issue()` → signs with Ed25519 → returns JWT string. The signing logic is unchanged; we're adding two claims to the payload.

**Token lifecycle operations (all preserved):**
- **Verify:** `POST /v1/token/validate` — checks signature + expiration + revocation status + audience
- **Renew:** `POST /v1/token/renew` — issues fresh JWT (new JTI, new timestamps) if current token is valid
- **Release:** `POST /v1/token/release` — agent self-revokes (adds JTI to revocation list)

### Stage 6: Scope Enforcement (PRESERVED — Zero Changes)

The scope system is entirely broker-internal and doesn't touch app authentication at all.

**Scope format:** `action:resource:identifier` (e.g., `read:weather:current`, `write:logs:*`)

**Scope attenuation:** One-way narrowing only. A token with `read:weather:*` can delegate `read:weather:current` but NEVER `write:weather:current`. This is enforced by `authz.AuthzSvc.Attenuate()` which walks the scope tree and rejects any scope not covered by the parent.

**Enforcement points:**
- `ValMw.RequireScope()` — middleware on every protected endpoint
- `ValMw.RequireAnyScope()` — at least one scope must match
- Bearer token extraction → signature verification → revocation check → audience validation → scope check

**Wildcard handling:** `*` in the identifier position matches any specific identifier. `read:weather:*` covers `read:weather:current` and `read:weather:forecast`.

### Stage 7: Delegation Chains (PRESERVED — Zero Changes)

Multi-agent delegation is a broker-level feature with no dependency on app authentication.

**How delegation works:**
1. Agent A (the delegator) calls `POST /v1/delegate` with its token + target agent + narrowed scopes
2. Broker verifies: delegated scopes ⊆ Agent A's scopes (attenuation)
3. Broker creates a new JWT for Agent B with:
   - Narrowed scopes
   - `delegation_chain` claim containing a SHA-256 hash linking back to Agent A
   - Chain depth incremented (max 5)
4. Agent B's token is cryptographically bound to the delegation chain

**Chain integrity:** SHA-256 hash chain where each link includes the previous hash + delegator SPIFFE ID + delegatee SPIFFE ID + scopes. Tampering with any link breaks the chain verification.

**Chain depth cap:** Maximum 5 levels (A→B→C→D→E→F). Prevents unbounded delegation chains. Enforced in `authz.AuthzSvc.ValidateDelegation()`.

### Stage 8: Revocation (PRESERVED — Enhanced with App Level)

The revocation system gains a 5th level but all existing levels stay identical.

**Current 4-level revocation pyramid:**

| Level | Target | What's Revoked | Cascade |
|-------|--------|---------------|---------|
| Token | JTI (UUID) | Single token | None |
| Agent | SPIFFE ID | All tokens for that agent | All tokens |
| Task | Task ID | All agents in a task | All agents → all tokens |
| Chain | Chain hash | Entire delegation chain | All delegated tokens |

**New 5th level (enhanced):**

| Level | Target | What's Revoked | Cascade |
|-------|--------|---------------|---------|
| App | app_id | All agents belonging to app | All agents → all tokens |

**Code path:** `handler.RevHandler.HandleRevoke()` → `store.RevStore.Revoke()`. The existing code handles levels via a switch statement. Adding `"app"` is one new case that queries tokens by `app_id` and revokes them all.

**Revocation check on every request:** `ValMw` checks every incoming token's JTI against the revocation store before allowing the request. This is unchanged.

### Stage 9: Hash-Chained Audit Trail (PRESERVED — Enhanced with App Attribution)

The audit trail is append-only, tamper-evident, and persists to SQLite. Every operation in the system generates an audit event.

**Existing audit events (30+ types):**
- Agent registration, token issuance, token renewal, token release
- Scope attenuation, delegation, chain creation
- Revocation (all levels), admin authentication
- Launch token creation, nonce generation
- Health checks, validation failures

**Each event contains:**
- Timestamp, event type, actor (SPIFFE ID or admin)
- Target resource, action taken, result (success/failure)
- Previous event hash (SHA-256 chain link)
- Sanitized payload (PII stripped)

**What's enhanced:**
- Events now include `app_id` and `app_name` when the action was triggered by an app-authenticated entity
- New event types: `app_registered`, `app_authenticated`, `app_revoked`, `app_secret_rotated`
- Query by app: "Show me everything weather-bot did" → `GET /v1/admin/audit?app_id=...`
- Legacy events (pre-app-registration) have `app_id: null` — this is documented and expected

**Hash chain integrity:** Each new audit event includes `SHA256(previous_event_hash + current_event_data)`. This means any modification to historical events breaks the chain verification. This mechanism is completely independent of how apps authenticate.

### Capability Preservation Summary

| # | Capability | Status | Details |
|---|-----------|--------|---------|
| 1 | App Registration | **NEW** | Apps become first-class entities with client_id/client_secret |
| 2 | App Authentication | **NEW** | Scoped app JWT replaces admin master key |
| 3 | Launch Token Creation | **PRESERVED** | Same endpoint, same logic, different auth credential |
| 4 | Ed25519 Challenge-Response | **PRESERVED** | Zero changes — core identity mechanism untouched |
| 5 | SPIFFE ID Generation | **PRESERVED** | Zero changes — same format, same validation |
| 6 | JWT Issuance | **ENHANCED** | Same signing, adds app_id + app_name claims |
| 7 | Token Verify/Renew/Release | **PRESERVED** | Zero changes to lifecycle operations |
| 8 | Scope Format & Attenuation | **PRESERVED** | Zero changes — action:resource:identifier unchanged |
| 9 | Scope Enforcement Middleware | **PRESERVED** | Zero changes — ValMw checks every request identically |
| 10 | Delegation Chains | **PRESERVED** | Zero changes — SHA-256 chain hash, depth cap 5 |
| 11 | 4-Level Revocation | **PRESERVED** | Token, agent, task, chain — all unchanged |
| 12 | App-Level Revocation | **NEW** | 5th level — revoke all tokens by app_id |
| 13 | Hash-Chained Audit Trail | **ENHANCED** | Adds app_id attribution, new event types |
| 14 | SQLite Persistence | **ENHANCED** | New `apps` table, app_id column on tokens |
| 15 | Admin Operations | **PRESERVED** | All existing admin endpoints unchanged |
| 16 | aactl CLI | **ENHANCED** | New `app` subcommand, existing commands unchanged |
| 17 | Health & Metrics | **PRESERVED** | Zero changes |

**Count: 8 preserved, 5 enhanced (backward-compatible), 4 new. Zero capabilities removed or broken.**

### The 10 Security Invariants — All Maintained

These are the non-negotiable security properties enforced by AgentAuth. Every single one is maintained in the new architecture:

1. **Every agent has a unique, cryptographically-bound identity** — Ed25519 keypair + SPIFFE ID. Unchanged.
2. **Every token is signed with Ed25519** — broker's signing key. Unchanged.
3. **Every token has a JTI for individual revocation** — UUID per token. Unchanged.
4. **Scopes can only narrow, never widen** — attenuation enforced at issuance and delegation. Unchanged.
5. **Delegation chains have bounded depth** — max 5 levels. Unchanged.
6. **Delegation chains are tamper-evident** — SHA-256 hash linking. Unchanged.
7. **Every revocation is immediate and persistent** — written to SQLite, checked on every request. Unchanged.
8. **Every operation generates an audit event** — append-only, hash-chained. Enhanced (app attribution added).
9. **Nonces are single-use and time-limited** — 30s TTL, consumed on use. Unchanged.
10. **Launch tokens are single-use with scope ceilings** — consumed on registration. Unchanged.

---

## The Bottom Line

The Token Proxy was built to answer: "How do we simplify the broker API for developers?" That's a valid question. But the answer should have been an SDK, not a mandatory infrastructure component. The proxy turned a developer experience question into an infrastructure deployment, and that single decision cascaded into every problem we found: no app entity, master key everywhere, audit blindness, infrastructure-as-registration.

The fix is straightforward: make apps real, give them their own credentials, let them talk to the broker directly (via SDK), and keep the proxy as an optional deployment for teams that want infrastructure-level resilience. The security model doesn't change at all — it gets stronger, because the master key stops spreading to every app deployment.
