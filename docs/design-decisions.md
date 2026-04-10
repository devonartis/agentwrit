# AgentWrit Design Decisions — Why We Built It This Way

> This document traces the reasoning behind every major design choice in AgentWrit. Not what the system does — why it does it that way. Each section starts with the problem we faced, the options we considered, and why we chose what we chose.

---

## Decision 1: Why Tokens at All?

### The Problem

AI agents need to access resources — databases, APIs, other services. They need to prove they're authorized. The simplest approach: give the agent a password or API key and let it send that on every request.

### Why That Doesn't Work for Us

Our agents are **ephemeral**. They spin up, do a 2-minute task, and die. But a password or API key doesn't die with them. It sits there, valid, long after the agent is gone. If it leaks — through logs, a compromised orchestrator, a debugging session — the attacker has a permanent credential.

We also run **many agents simultaneously**. If 50 agents share one API key, we can't tell which one did what. We can't revoke one without revoking all. We can't give one agent narrower permissions than another.

### What We Needed

1. **Short-lived credentials** — die automatically after the task is done
2. **Individually identifiable** — each agent has its own credential, its own identity
3. **Scoped** — each agent gets only the permissions its task requires
4. **Revocable** — if something goes wrong, kill it instantly without affecting other agents
5. **Auditable** — trace exactly which agent did what, under whose authority

### Why We Chose Tokens

A token is a credential the server issues after verifying your identity once. Instead of sending the secret on every request, you send the token. The secret is exposed once; the token is exposed repeatedly but it's short-lived, scoped, and revocable.

This is the fundamental trade we made: **exchange a long-lived powerful secret for a short-lived scoped token.** The secret proves who you are. The token proves what you're allowed to do right now.

---

## Decision 2: Why JWTs Specifically?

### The Options We Had

| Option | How it works | Why we didn't choose it |
|--------|-------------|------------------------|
| **Session tokens** | Random string → server looks up in database | Every request requires a database lookup. If the database is down, nothing works. Doesn't scale to external verifiers |
| **OAuth 2.0 tokens** | Industry standard, widely supported | OAuth is designed for user-delegated access ("let this app access my Google Drive"). Our problem is machine-to-machine with no user in the loop. OAuth's complexity (authorization servers, redirect flows, refresh tokens) is overhead we don't need |
| **API keys** | Random string → server looks up permissions | Long-lived, shared, no built-in expiry, no embedded identity. Everything we're trying to avoid |
| **JWTs (JSON Web Tokens)** | Self-contained signed tokens with embedded claims | Chosen — see below |

### Why JWT Won

**Self-contained verification.** A JWT carries its own data (who, what permissions, when it expires) and a cryptographic signature. The server verifies the signature and reads the claims. No database lookup needed. This means:

- External services can verify agent tokens without calling back to our broker
- The broker can verify tokens even if SQLite is temporarily unavailable
- Verification is fast — a signature check, not a database query

**Embedded scopes.** The permissions are inside the token. The server reads them directly from the claims. No separate permission lookup, no join table, no cache.

**Embedded identity.** The `sub` (subject) claim carries the agent's unique SPIFFE identity. Every token is traceable to a specific agent instance, task, and orchestrator.

**Standard format.** JWTs are a known standard (RFC 7519). Security reviewers know how to audit them. Libraries exist in every language. We're not inventing a proprietary format.

---

## Decision 3: Why Ed25519 (EdDSA)?

### The Options

| Algorithm | Key size | Signature size | Speed | Our concern |
|-----------|---------|----------------|-------|-------------|
| RSA-2048 | 256 bytes | 256 bytes | Slow to sign | Signing overhead on every token issuance. Key is large |
| RSA-4096 | 512 bytes | 512 bytes | Very slow to sign | Even more overhead |
| HMAC-SHA256 | Shared secret | 32 bytes | Very fast | **Both sides need the same key.** If an external service needs to verify tokens, it needs the signing key — which means it can also forge tokens |
| ECDSA P-256 | 32 bytes | 64 bytes | Fast | Good option. But two valid signature encodings for the same input (signature malleability) |
| **Ed25519 (EdDSA)** | 32 bytes | 64 bytes | Very fast | Chosen — see below |

### Why Ed25519

**Asymmetric.** The broker holds the private key (signs). Anyone with the public key can verify. External services, resource servers, federated brokers — they can all verify agent tokens without being able to forge them. HMAC can't do this.

**Go stdlib.** `crypto/ed25519` ships with Go. No third-party crypto library. This is a hard rule for us — every dependency is supply chain risk, and crypto dependencies are the highest risk. We have 5 total dependencies; none of them are crypto.

**Fast and small.** 32-byte keys, 64-byte signatures, and signing is fast enough that token issuance isn't a bottleneck even under load.

**No malleability.** Unlike ECDSA, Ed25519 has a single valid signature for each message+key pair. This prevents subtle attacks where an attacker creates a different-but-valid signature for the same token.

---

## Decision 4: Why Short-Lived with Renewal?

### The Trade-Off

Two ways to handle token lifetime:

**Option A: Long-lived tokens, revocation-only.** Issue a token that lasts hours or days. If something goes wrong, revoke it. Problem: if revocation fails or is delayed, the token is valid for a long time. The damage window is large.

**Option B: Short-lived tokens, renewal.** Issue a token that lasts minutes. If the task takes longer, the agent renews. Problem: renewal adds complexity. But the damage window is small — a leaked token is useless in minutes.

### What We Chose and Why

**Option B.** Our agents run tasks measured in minutes. A 5-minute token covers most tasks. If a task runs longer, the agent calls `POST /v1/token/renew` to get a fresh token.

The renewal has a critical property we enforce: **the old token is revoked BEFORE the new one is issued.** This prevents two valid tokens from existing simultaneously. And the renewed token carries the **original TTL**, not the default — so renewal can't escalate a 2-minute token into a 5-minute token. That would be a privilege escalation (we call this SEC-A1).

### Why Not Just Expire and Re-Register?

Re-registration requires a launch token. Launch tokens are single-use and short-lived (30 seconds). By the time the agent needs to renew, the launch token is long gone. Renewal lets the agent extend its session using the credential it already has, without going back to the app for a new launch token.

---

## Decision 5: Why Launch Tokens? (The Bootstrap Problem)

### The Problem

An agent needs a credential to prove who it is. But to get that credential, it needs to register with the broker. To register, it needs to prove it's authorized. But it has no credential yet.

This is the "secret zero" problem. Every identity system faces it.

### The Options

| Option | How it works | Why we didn't choose it |
|--------|-------------|------------------------|
| **Pre-provisioned API key** | Agent starts with a long-lived key baked into its config | Exactly what we're trying to eliminate. If the key leaks, anyone can register agents |
| **Mutual TLS certificate** | Agent proves identity via client cert | Requires certificate infrastructure (CA, distribution, rotation). Heavy for ephemeral agents |
| **OAuth client credentials** | Agent has a client_id/secret | Same problem as API keys — long-lived shared credentials |
| **Launch tokens** | Pre-authorized, single-use, short-lived opaque token | Chosen — see below |

### What We Chose and Why

A **launch token** is a one-time entry pass. It's:

- **Pre-authorized** — the app (or admin) creates it with a scope ceiling and max TTL. The agent can't exceed these limits.
- **Single-use** — consumed on first successful registration. Can't be replayed.
- **Short-lived** — 30 seconds by default. If the agent doesn't register in time, the token expires and a new one must be created.
- **Not a JWT** — it's an opaque random hex string. No claims, no signature. It's a lookup key that maps to a policy record in the broker. This is intentional: the launch token is consumed during registration and never used again, so it doesn't need to be self-verifiable.

The flow: app creates launch token → delivers to agent (env var, orchestrator) → agent fetches a challenge nonce → agent signs nonce with its Ed25519 key → agent registers with launch token + signed nonce → broker verifies everything, consumes the launch token, issues a JWT.

The challenge-response step (nonce signing) is critical — it cryptographically binds the agent to an Ed25519 key pair. The agent proves it holds a private key without transmitting it. This key binding is what makes mutual authentication possible later.

---

## Decision 6: Why `action:resource:identifier` Scopes?

### The Options

| Option | How it works | Why we didn't choose it |
|--------|-------------|------------------------|
| **Role-Based Access Control (RBAC)** | Assign roles like "reader", "writer", "admin" | Too coarse. An agent doing one specific task doesn't need a "reader" role that covers everything readable. And roles are hard to attenuate — how do you give a sub-agent a "narrower" role? |
| **Access Control Lists (ACLs)** | Explicit allow/deny per resource | Doesn't embed well in tokens. Requires server-side lookups. Hard to enforce attenuation |
| **OAuth scopes** | Strings like "read", "write", "admin" | Too coarse for task-level permissions. `read` means read everything. No resource or identifier granularity |
| **`action:resource:identifier`** | Three-part structured permission | Chosen — see below |

### Why Three Parts

We need to express: **what you can do**, **to what kind of thing**, **to which specific instance**.

`read:data:customers` says exactly: this agent can read the `data` resource, specifically the `customers` instance. Not all data — just customers.

The wildcard (`*`) in the identifier position lets you express broader permissions when needed: `read:data:*` means read any data resource. But the wildcard only works in the identifier — you can't wildcard the action or resource. This prevents accidentally granting `*:*:*` through composition.

### Why This Matters for Attenuation

The three-part format makes the subset check straightforward: scope B covers scope A if they have the same action, same resource, and B's identifier is `*` or matches A's identifier.

This means attenuation is a simple, deterministic check at every trust boundary:
- App creates launch token: launch token scope ⊆ app ceiling
- Agent registers: agent scope ⊆ launch token scope
- Agent delegates: delegate scope ⊆ delegator scope

One function (`authz.ScopeIsSubset`) enforces the invariant everywhere. No special cases, no role hierarchies, no inheritance chains.

### Why the Broker Doesn't Define Scopes

The broker doesn't maintain a registry of "valid" scopes. Any three-part string is accepted. `read:data:customers`, `execute:pipeline:deploy-prod`, `custom:anything:you-want` — all valid.

This is intentional. The broker doesn't know what `read:data:customers` means to your application. It only knows whether one scope covers another. Your application decides what permissions the scope actually grants.

This means AgentWrit works with any application, any resource model, any permission structure — as long as it can be expressed in `action:resource:identifier` format.

---

## Decision 7: Why Three Roles (Admin, App, Agent)?

### The Problem

We need at least two roles: someone who manages the system, and agents who do work. But there's a gap between them.

The operator (admin) is a human or automation that bootstraps and oversees the system. The agent is ephemeral software doing a specific task. In production, there are potentially hundreds of agents across many different applications. The operator can't manage each agent individually — they need to manage at the application level.

### The Roles

**Admin (Operator)** — Manages the system. Authenticates with the admin secret. Registers apps, sets scope ceilings, revokes credentials, queries audit trail. This is the human (or automation) that controls the deployment.

**App** — Software that manages its own agents. An orchestrator, a CI pipeline, a SaaS backend. Authenticates with its own client_id/secret (not the admin secret). Creates launch tokens for its agents, constrained by the scope ceiling the operator set. This is the middle layer that makes production manageable — the operator trusts the app up to a ceiling, and the app manages agents within that ceiling.

**Agent** — Does actual work. Registers with a launch token, gets a short-lived scoped JWT, does its task, releases the credential when done. The agent never touches the admin or app layers.

### Why We Need the App Layer

Without apps, the operator would need to create a launch token for every single agent. At scale (hundreds of agents across multiple applications), that's unmanageable.

With apps, the operator makes one decision per application: "this app can do X, Y, Z with a maximum TTL of N." Then the app handles agent management autonomously, within those constraints.

The scope ceiling is the key design element. It's a hard cap set by the operator that the app can never exceed. The app can create launch tokens with any scope within its ceiling, but not beyond it. This gives the app autonomy while keeping the operator in control of the security boundary.

---

## Decision 8: Why Did We Remove the Sidecar?

### What the Sidecar Was

Early versions of AgentWrit had a sidecar process that ran alongside each agent. The sidecar handled token management — the agent called the sidecar on localhost, and the sidecar called the broker. The idea was to keep agents simple: they don't need to know about tokens, just call the sidecar.

### Why We Removed It (Phase 0)

The sidecar was tied to Docker Compose as a mandatory dependency. You couldn't run AgentWrit without Docker orchestrating the sidecar alongside each agent container. That's a non-starter for:

- Bare-metal deployments
- Kubernetes (which has its own sidecar patterns)
- Serverless environments
- Any agent that isn't running in Docker

The architecture decision (March 2026): apps register directly with the broker. The sidecar becomes an optional deployment pattern, not a mandatory component. This is what B0 (Batch 0) of the migration removed — 2,220 lines of sidecar code, SidecarID from claims, sidecar Docker configs, sidecar metrics and audit events.

The direct-broker architecture is simpler, more portable, and doesn't force a deployment model on the user.

---

## Decision 9: Why Not OAuth?

This comes up often enough that it deserves its own section.

### What OAuth Is Designed For

OAuth 2.0 solves the delegated access problem: "Let this third-party app access my Google Drive on my behalf." There's a user in the loop who grants consent. The authorization server issues tokens that represent the user's delegated permission.

### Why It Doesn't Fit Our Problem

Our agents don't act on behalf of a user. They act on behalf of a task. There's no user to grant consent, no redirect flow, no consent screen. The authorization decision comes from the operator (who set the scope ceiling) and the app (who created the launch token within that ceiling).

Specific mismatches:

| OAuth Concept | Our Reality |
|---------------|------------|
| User grants consent | No user — the operator pre-authorizes via scope ceiling |
| Authorization server | We are the authorization server AND the token issuer AND the policy enforcer |
| Refresh tokens | We use renewal with TTL carry-forward, not refresh tokens (which would allow TTL escalation) |
| Client credentials grant | Close to what apps do, but we add scope ceiling enforcement that OAuth doesn't have |
| Token introspection | We have `POST /v1/token/validate`, but it's simpler — no active/inactive metadata, just valid/invalid + claims |

We're not anti-OAuth. Enterprise modules could bridge to OAuth/OIDC (that's what the planned OIDC provider add-on is for). But the core doesn't need OAuth's complexity.

---

## Decision 10: The Admin Launch Token Question (TD-013)

### The Current State

Admin can create launch tokens via `POST /v1/admin/launch-tokens`. When admin creates a launch token, there's no scope ceiling check — admin can put any scopes in the launch token. The agent registered with this token has no `app_id`, no app traceability, and no ceiling enforcement at the first enforcement point.

### Why This Exists

**Bootstrapping.** Before any apps are registered, the operator needs to test the system. The flow is: authenticate → create launch token → register test agent → verify the flow works. This is the `awrit init → awrit token create-launch → test` development workflow.

### Why It's a Problem in Production

In production, every agent should trace back to an app, every launch token should be ceiling-constrained, and the audit trail should show the full chain. The admin path breaks all three.

The question isn't "should admin be able to manage launch tokens" — yes, admin needs `admin:launch-tokens:*` for oversight (list, inspect, revoke). The question is: **should admin be able to CREATE launch tokens that bypass the scope ceiling?**

### Where We're Leaning

**Option B: Restrict to dev mode.** In development mode (`MODE=development`), admin can create launch tokens freely — that's the bootstrapping workflow. In production mode, admin can list, inspect, and revoke launch tokens, but creation goes through apps only. This gives a clean security model in production without breaking the development experience.

This is still an open decision. See TD-013 in `TECH-DEBT.md` for the full analysis.

---

## Decision 11: Why Four Revocation Levels?

### The Problem

When something goes wrong, you need to kill credentials. But "something going wrong" has different blast radii:

- One token was leaked → kill that token
- An agent is misbehaving → kill all its tokens
- A task has gone off the rails → kill all agents on that task
- A delegation chain is compromised → kill everything that traces back to the root

### Why Not Just Token-Level Revocation?

If you can only revoke individual tokens, and an agent has renewed 3 times (3 JTIs), you need to know and revoke all 3. If a task has 10 agents, you need to find and revoke each one. At scale, this is unmanageable.

### The Four Levels

| Level | Target | What it kills | When you use it |
|-------|--------|--------------|-----------------|
| `token` | JTI | One specific credential | A token leaked in a log. Kill it, everything else keeps running |
| `agent` | SPIFFE ID | Every credential this agent holds | The agent is compromised or behaving incorrectly |
| `task` | task_id | Every credential for this task, across all agents | The task itself is wrong — cancel everything working on it |
| `chain` | Root delegator agent ID | Every credential in a delegation tree | Agent A delegated to B, B delegated to C. A is compromised — kill A, B, and C |

Each level is checked on every authenticated request. The revocation check runs through all four maps in order: token → agent → task → chain. If any match, the request is rejected with 403.

---

## Summary: The Design Chain

Each decision builds on the previous:

1. **Agents need credentials** → use tokens (short-lived, scoped, revocable)
2. **Tokens need to be self-verifiable** → use JWTs (embedded claims + signature)
3. **JWTs need a signing algorithm** → use Ed25519 (asymmetric, stdlib, fast, no malleability)
4. **Agents outlive their token** → use renewal (with TTL carry-forward, old token revoked first)
5. **Agents need a first credential** → use launch tokens (single-use, short-lived, scope-constrained)
6. **Tokens need permissions** → use `action:resource:identifier` scopes (granular, deterministic attenuation)
7. **System needs management layers** → three roles: admin (operator), app (autonomous within ceiling), agent (does work)
8. **Deployment can't be coupled to Docker** → remove sidecar, direct broker architecture
9. **OAuth doesn't fit machine-to-machine ephemeral agents** → own token system, OIDC bridge as enterprise add-on
10. **Admin launch token creation bypasses ceiling** → restrict to dev mode (TD-013)
11. **Different failure scenarios need different blast radii** → four revocation levels
