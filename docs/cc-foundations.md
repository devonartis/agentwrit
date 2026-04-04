# AgentAuth Foundations — What Tokens Are and Why We Use Them

## What Is a Token?

A token is proof that someone already verified who you are. Instead of proving your identity on every request, you prove it once and get a token. Then you show the token instead.

Without tokens, the flow looks like this:

```
Request 1: "I'm the admin, here's my secret. Give me the app list."
Request 2: "I'm the admin, here's my secret. Create this launch token."
Request 3: "I'm the admin, here's my secret. Revoke this agent."
```

The secret is sent on every request. Every request is a chance for the secret to leak — in a log, in a network trace, in a crash dump. And every request requires the server to do the expensive verification (bcrypt comparison) again.

With tokens:

```
Request 1: "Here's my secret." → Server verifies → "Here's your token (valid 5 minutes)."
Request 2: "Here's my token. Give me the app list." → Server checks token → Done.
Request 3: "Here's my token. Create this launch token." → Server checks token → Done.
Request 4: "Here's my token. Revoke this agent." → Server checks token → Done.
```

The secret was sent once. The token is sent repeatedly, but:
- It expires — if it leaks, the damage window is 5 minutes, not forever
- It's cheap to verify — the server checks a cryptographic signature instead of running bcrypt
- It can carry permissions — the token says what the holder is allowed to do
- It can be revoked — the server can kill it without changing the secret

That's the fundamental trade: **exchange a long-lived, powerful secret for a short-lived, scoped, revocable token.**

---

## Why Not Just Use API Keys?

API keys are tokens — but they're bad tokens. Here's why.

An API key is a random string that the server looks up in a database. If the string matches, you're in. The problem:

| Property | API Key | AgentAuth Token (JWT) |
|----------|---------|-----------------------|
| **Lifetime** | Permanent until rotated | Minutes. Expires automatically |
| **Permissions** | Whatever the key is configured for — usually everything | Explicit scope list embedded in the token |
| **Verification** | Database lookup on every request | Cryptographic signature check — no database needed |
| **Revocation** | Delete from database | In-memory check, 4 granularity levels |
| **Identity** | "service-account-prod" — shared across all agents | Unique per agent instance — SPIFFE ID with task and orchestrator context |
| **Audit** | "service-account-prod did something" | "agent spiffe://domain/agent/orch-456/task-789/a1b2c3d4 did something" |

The critical failure: twenty agents sharing one API key means you can't tell which agent did what. You can't revoke one without revoking all. You can't scope one without scoping all. You can't even tell how many agents are using it.

---

## What Makes a Good Token?

A good token has five properties:

### 1. Self-contained

The token carries its own data — who the holder is, what they're allowed to do, when the token expires. The server doesn't need to look anything up to verify it. It checks the cryptographic signature, reads the embedded claims, and makes a decision.

In AgentAuth, this is a JWT (JSON Web Token) signed with Ed25519. The server holds the private key (signs tokens). Anyone with the public key can verify them — including external services that need to check agent credentials without calling back to the broker.

### 2. Short-lived

The token has a defined lifetime. When it expires, it's useless. This limits the damage window if the token leaks.

In AgentAuth:
- Admin JWT: 5 minutes
- App JWT: 30 minutes (configurable per app, 60s to 24h)
- Agent JWT: set by the launch token's `max_ttl`, clamped by the global `MaxTTL`
- Launch token: 30 seconds by default

Every token has an `exp` (expiration) claim. The server rejects any token where `now > exp`.

### 3. Scoped

The token says exactly what the holder is allowed to do. Not "everything the account can do" — just the specific permissions needed for this session or task.

In AgentAuth, scopes follow the format `action:resource:identifier`:

```
read:data:customers       — read customer data
write:logs:*              — write to any log
admin:revoke:*            — revoke any credential
```

The token's scope list is embedded in the JWT claims. Every protected endpoint checks that the token's scopes cover what the endpoint requires. If they don't: 403 Forbidden.

### 4. Individually identifiable

Every token has a unique ID (`jti` — JWT ID). Every agent has a unique identity (`sub` — subject, in SPIFFE format). This means:

- The audit trail shows exactly which token and which agent did what
- You can revoke one specific token without affecting any other
- You can trace the complete authorization chain: operator → app → launch token → agent → delegation

### 5. Revocable

If something goes wrong, you can kill the token immediately — before it expires. AgentAuth does this at four levels:

- **Token** — kill one specific credential by its JTI
- **Agent** — kill everything one agent holds, by its SPIFFE ID
- **Task** — kill everything associated with a task, across all agents
- **Chain** — kill everything in a delegation tree, from the root delegator down

Revocation is checked on every authenticated request. A revoked token gets rejected even if it hasn't expired yet.

---

## How Tokens Are Built (The JWT Structure)

AgentAuth tokens are JWTs — three base64-encoded parts separated by dots:

```
header.payload.signature
```

**Header** — algorithm and key ID:
```json
{
  "alg": "EdDSA",
  "typ": "JWT",
  "kid": "RFC-7638-thumbprint-of-public-key"
}
```

`EdDSA` means Ed25519 signatures — fast, small (64 bytes), and uses only Go standard library. No RSA, no HMAC, no third-party crypto. The `kid` (Key ID) lets external verifiers know which public key to use, which matters when keys are rotated.

**Payload** — the claims (who you are, what you can do, when it expires):
```json
{
  "iss": "agentauth",
  "sub": "spiffe://agentauth.local/agent/orch-456/task-789/a1b2c3d4",
  "aud": ["agentauth"],
  "exp": 1711810500,
  "nbf": 1711810200,
  "iat": 1711810200,
  "jti": "f47ac10b58cc4372",
  "scope": ["read:data:customers"],
  "task_id": "task-789",
  "orch_id": "orch-456",
  "sid": "",
  "delegation_chain": [],
  "chain_hash": ""
}
```

| Claim | What it carries |
|-------|----------------|
| `iss` | Issuer — always `"agentauth"`. Tokens claiming a different issuer are rejected |
| `sub` | Subject — who this token represents. `"admin"` for operators, `"app:{id}"` for apps, SPIFFE URI for agents |
| `aud` | Audience — who this token is intended for. Checked if configured |
| `exp` | Expiration — Unix timestamp. Token is dead after this |
| `nbf` | Not Before — Unix timestamp. Token isn't valid yet before this (prevents pre-dated tokens) |
| `iat` | Issued At — Unix timestamp. Used to derive TTL: `exp - iat` = token lifetime |
| `jti` | JWT ID — unique identifier. Used for revocation targeting and replay prevention |
| `scope` | Permission list — what this token holder is allowed to do |
| `task_id` | Which task this agent was created for (agents only) |
| `orch_id` | Which orchestrator launched this agent (agents only) |
| `sid` | Session ID (optional, carried through renewals) |
| `delegation_chain` | Who delegated to whom, with what scope, signed by the broker |
| `chain_hash` | SHA-256 of the delegation chain — tamper detection |

**Signature** — Ed25519 signature over `header.payload`:

The broker signs `base64(header) + "." + base64(payload)` with its Ed25519 private key. Anyone with the public key can verify the signature — if it's valid, the token hasn't been tampered with. If any claim was changed after signing, the signature check fails.

This is why the token is self-contained: the signature proves the claims are authentic without needing a database lookup.

---

## What Tokens CANNOT Do

Tokens are not magic. Understanding their limits is as important as understanding their properties.

**A token cannot be un-leaked.** If an agent's JWT appears in a log file, anyone who finds that log can use the token until it expires or is revoked. This is why tokens are short-lived — the damage window is bounded.

**A token cannot enforce what you do with the data.** A token with `read:data:customers` proves the holder is allowed to read customer data. It doesn't prevent the holder from leaking that data somewhere else. Token-based authorization is about access control, not data loss prevention.

**A token cannot prove the holder is the original recipient.** If Agent A's token is stolen and used by an attacker, the token itself doesn't know. This is why AgentAuth uses challenge-response registration (proving key possession) and why mutual authentication exists — to verify both ends of a communication.

**A token cannot revoke itself automatically.** When a task finishes, the token doesn't disappear. The agent should call `POST /v1/token/release` to self-revoke. If the agent crashes without releasing, the token remains valid until it expires. This is acceptable because the TTL is short (minutes, not hours).

---

## The Three Types of Token in AgentAuth

With the foundation of what tokens are and why they matter, here's what AgentAuth actually issues:

### JWT Tokens (Admin, App, Agent, Delegated)

EdDSA-signed, self-contained, carry scopes and identity. These are the "real" tokens — they're what gets presented in the `Authorization: Bearer {token}` header on every authenticated request.

Four variants exist, differing only in their `sub` (subject) and `scope` claims:

| Variant | Subject format | Scopes | Lifetime |
|---------|---------------|--------|----------|
| Admin JWT | `"admin"` | `admin:launch-tokens:*`, `admin:revoke:*`, `admin:audit:*` | 5 min |
| App JWT | `"app:{appID}"` | `app:launch-tokens:*`, `app:agents:*`, `app:audit:read` | 30 min (configurable) |
| Agent JWT | `"spiffe://{domain}/agent/{orch}/{task}/{instance}"` | Task-specific (e.g. `read:data:customers`) | Set by launch token |
| Delegated JWT | Same SPIFFE format (delegate's ID) | Narrower than delegator's | Set at delegation |

They're all signed the same way, verified the same way, and revoked the same way. The difference is what's inside.

### Launch Tokens (Opaque)

NOT a JWT. A 64-character random hex string that maps to a policy record stored in the broker. The token itself carries no data — it's a lookup key.

Launch tokens exist because agents have a chicken-and-egg problem: you need a credential to get a credential. The launch token is the bootstrap mechanism — a pre-authorized, single-use, short-lived entry pass.

The critical distinction: JWT tokens are **self-contained** (verifiable from the token alone). Launch tokens are **reference tokens** (the broker must look them up). This is by design — the launch token is consumed during registration and never used again, so there's no need for it to be self-verifiable.

### Nonces (Challenge-Response)

Also not a JWT. A 64-character random hex string used during agent registration. The agent signs the nonce with its Ed25519 private key to prove key possession. The nonce is consumed immediately and expires in 30 seconds.

Nonces aren't really "tokens" in the authorization sense — they're part of the identity verification protocol. But they're stored and managed by the same infrastructure, so they're worth mentioning.

---

## Now: What Are Scopes and How Do They Work?

With tokens defined, scopes are what give tokens their meaning. A token without scopes is a proof of identity — it says who you are but not what you're allowed to do.

See [Scope Model](cc-scope-model.md) for the complete scope deep dive: the format, the coverage rules, the four enforcement points, and how scopes flow through the attenuation chain.
