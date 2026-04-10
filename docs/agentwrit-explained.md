# AgentWrit Explained

> For sales teams and developers new to the system. No prior knowledge of JWTs or cryptography required.

---

## The Problem

AI agents need credentials to do their work — read a database, call an API, write to a log. Most systems give those agents a long-lived API key that has broad access and never expires. When that key leaks — through a breach, an accident, a compromised vendor — the damage is wide and the fix is painful.

AgentWrit replaces long-lived keys with short-lived, task-scoped tokens that expire automatically, can be revoked instantly, and can only do what they were specifically created for.

---

## What a Token Is in This System

A token is a digitally signed string. When a caller makes a request to the broker, they put the token in the HTTP request header:

```
Authorization: Bearer <token>
```

The broker reads that header, verifies the token's signature, checks it hasn't been revoked, then checks whether it carries the scope required for that specific endpoint. If any check fails, the request is rejected with a 401 or 403. If all checks pass, the request proceeds.

This is how both the admin token and the app token work. Same mechanism. What separates them is what is embedded inside — specifically their **scopes**.

---

## The Admin Token — The Root of the System

The admin token is where everything starts. Nothing else in the system can exist without it.

**How it's obtained:**
```
POST /v1/admin/auth
{ "secret": "..." }
```

The operator sends the master admin secret. The broker verifies it against a bcrypt hash — the plaintext secret is never stored. If it matches, the broker issues a signed JWT that lasts **5 minutes**.

**What scopes it carries:**
```
admin:launch-tokens:*
admin:revoke:*
admin:audit:*
```

**What each scope does:**

`admin:launch-tokens:*` — this scope opens two things. First, it lets the admin register, update, and deregister applications. When an application is registered, the admin sets its **scope ceiling** — the maximum permissions any agent running under that app can ever receive. Second, it lets the admin create launch tokens directly, which is the bootstrap path before any apps exist.

`admin:revoke:*` — this scope is the kill switch. The admin can cancel any token, any agent, any task, or an entire delegation chain. No other token type can do this.

`admin:audit:*` — this scope gives the admin full visibility into the audit trail. Every credential issued, every failed attempt, every scope violation — all of it. No other token type can see the full trail.

**Why 5 minutes:** The admin token is the most powerful credential in the system. Keeping it short-lived limits the window of exposure if it's intercepted. The operator authenticates, does what they need to do, and the token expires.

---

## The App Token — Bounded Authority

Once the admin has registered an application and set its scope ceiling, the application authenticates with its own credentials to get its own token. This is the production path for creating agents.

**How it's obtained:**
```
POST /v1/app/auth
{ "client_id": "...", "client_secret": "..." }
```

The `client_id` and `client_secret` were generated when the admin registered the app. The secret is shown once at registration and never stored in plaintext — only a bcrypt hash is persisted.

**What scopes it carries:**
```
app:launch-tokens:*
app:agents:*
app:audit:read
```

**What each scope does:**

`app:launch-tokens:*` — lets the app create launch tokens for its agents. Every launch token created through this path is checked against the app's scope ceiling. If the requested scope exceeds the ceiling, the broker rejects it with a 403 and records the violation in the audit trail.

`app:agents:*` — reserved for agent management operations.

`app:audit:read` — reserved for the app reading its own audit events, scoped to its agents only.

**What it cannot do:** call any admin endpoint. The scope strings are different. An app token carrying `app:launch-tokens:*` cannot open a route that requires `admin:launch-tokens:*`. The broker enforces this on every request.

**Why the app token exists as a separate credential from the admin token:** The admin token is for a human or a deployment script. It's short-lived and powerful. The app token is for software running autonomously in production — an orchestrator, a pipeline, a backend service. It lives longer (30 minutes, configurable), authenticates differently (client_id/secret rather than a master secret), and is bounded by a ceiling the admin defined. If the app's credentials are compromised, the attacker can only create agents within that ceiling. They cannot revoke tokens, cannot read the full audit trail, cannot touch other apps.

---

## The Scope — What Actually Controls Access

The scope is a string in the format `action:resource:identifier`. Examples:

```
admin:revoke:*        — admin can revoke anything
app:launch-tokens:*   — app can create launch tokens
read:data:customers   — agent can read customer data
write:logs:run-42     — agent can write to a specific log
```

When a request arrives, the broker middleware calls `RequireScope` — it checks whether the token's scopes cover the required scope for that endpoint. If not, 403. The scope is not advisory. It is enforced in code on every single request.

The ceiling works the same way. When an app creates a launch token, the broker calls `ScopeIsSubset` — it checks whether every requested scope is covered by the app's ceiling. If the app tries to request a scope it doesn't hold, the request is denied before the launch token is created.

---

## The Launch Token — A Different Kind of Token

The launch token is not a JWT. It is a random 64-character hex string — 32 bytes of cryptographic randomness encoded as hex. It has no structure, no signature to verify, no claims to read. It is looked up from the database when presented.

```
a3f9c2d1e8b74f0a...  (64 hex characters)
```

It has one job: give a specific agent one opportunity to register. By default it expires in **30 seconds** and can only be used **once**. After it is consumed at registration, it is marked in the database and cannot be used again.

The launch token carries a policy — the scope the agent is allowed to request and the maximum TTL its credential can have. The agent cannot exceed either.

---

## The Agent Credential — Task-Scoped, Traceable

When the agent presents the launch token at `POST /v1/register`, it also proves it controls its own cryptographic key by signing a challenge the broker issued. If the launch token is valid and the signature checks out, the broker issues an Agent JWT.

The Agent JWT carries:
- The exact scopes the agent requested (within the launch token's policy)
- A SPIFFE-format identity that encodes the orchestrator, task, and instance — making every agent credential traceable to the exact task that created it
- A unique token ID so it can be cancelled individually

The agent uses this JWT as a Bearer token — the same `Authorization: Bearer <token>` header — when calling downstream services, which validate it at `POST /v1/token/validate`.

---

## How It All Connects

```
1. Admin authenticates        → gets Admin JWT (5 min, admin:* scopes)
2. Admin registers App        → sets scope ceiling, gets client_id + client_secret
3. App authenticates          → gets App JWT (30 min, app:* scopes)
4. App creates launch token   → ceiling enforced, 30s expiry, single-use
5. Launch token sent to agent → via environment variable or orchestrator config
6. Agent requests challenge   → signs it with its own key
7. Agent registers            → presents launch token + signed challenge → gets Agent JWT
8. Agent does its work        → uses Agent JWT as Bearer token
9. Admin can revoke anything  → at any level, any time, takes effect immediately
```

The admin token appears at steps 1 and 2 — then it expires and leaves the picture. From step 3 onward the app manages its own agents autonomously. The admin's decision at step 2 — the scope ceiling — controls what is possible from that point forward, enforced by the broker on every request.

---

## For Sales: The Three Questions You'll Get

**"How is this different from API keys?"**
API keys are permanent, usually over-permissioned, and require a full rotation when one leaks. AgentWrit credentials expire on their own, are scoped to a specific task, and can be cancelled in seconds without affecting anything else running at the same time.

**"What if an agent is compromised?"**
The admin revokes it. One call to `POST /v1/revoke` cancels the specific token, the specific agent, the specific task, or the entire delegation chain — whatever the situation requires. The revocation takes effect on the next request. No key rotation, no downtime for other agents.

**"Can a developer give an agent more access than they should?"**
No. The platform team sets the scope ceiling when registering the application. When the developer's code requests a launch token, the broker checks every requested scope against that ceiling. If any scope exceeds it, the request is rejected before the launch token is created.
