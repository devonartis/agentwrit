# BIG BAD GAP — App Registration Workflow is Broken

**Date discovered:** 2026-02-28 (Session 18)
**Severity:** BLOCKER — must fix before demo and before develop → main merge
**Related:** KI-001 (admin secret blast radius), but this goes deeper

---

## How We Found It

While trying to run `agentauth-app` against the current broker, traced the sidecar bootstrap flow:

1. Wait for broker health
2. Admin auth with `cfg.AdminSecret`
3. Create its own activation token
4. Activate with that token

The sidecar always uses the admin secret to self-provision. There's no code path to start a sidecar with a pre-created activation token. Every sidecar instance holds the admin secret and creates its own activation.

---

## What's Wrong

### Problem 1: Apps don't exist in the broker

There is no:
- `aactl app register --name my-app --scopes read:data:*`
- `POST /v1/admin/apps` endpoint
- App identity in the broker
- Way to onboard an app without touching deployment config and sharing the admin secret

Apps are invisible to AgentAuth. Only agents (via Ed25519) and sidecars (via activation) have identity. The app itself has no representation.

### Problem 2: Sidecar self-provisions with the admin secret

Every sidecar holds `AA_ADMIN_SECRET` — the same master key. A compromised sidecar has full admin scope. The `create_sidecar_activation` endpoint exists but nothing external uses it — the sidecar calls it internally with the admin secret.

### Problem 3: Adding an app = editing infrastructure config

Adding a new app means: edit `docker-compose.yml`, add a sidecar service block, paste the admin secret, set a scope ceiling, `docker compose up`. That's infrastructure config masquerading as app registration.

**User's key insight:** "Docker is infrastructure. This should be able to run on someone's virtual server. If it can't, it's wrong."

### Problem 4: No app-to-sidecar binding

The sidecar isn't tied to an app — it's tied to a scope ceiling. There's no concept of "this sidecar belongs to this app." Anything that can reach the sidecar's port or socket can request tokens. The isolation is only network access (TCP/UDS) + scope ceiling — not enforced identity.

### Problem 5: Sidecars boot whether or not apps exist

If no app ever calls `/v1/token`, the sidecar just sits there with an active registration and the admin secret. Five apps = five sidecars all holding the same master key, all self-activating regardless of whether their apps are ready.

---

## What It Should Be

The correct operator workflow:

1. **Operator registers an app** via `aactl app register --name my-app --scopes read:data:*` → gets a one-time activation token with a specific scope ceiling
2. **Operator gives the activation token** to the app deployment (env var, secret manager, etc.) — NOT the admin secret
3. **Sidecar boots with just the activation token** — no admin secret needed
4. **Sidecar activates** using that token, gets its scoped credentials, never has admin access
5. **App connects** to the sidecar and starts requesting tokens

This means:
- Apps are first-class entities in the broker
- The admin secret stays with the operator, never leaves the admin environment
- Each sidecar has only the minimum credentials it needs
- Onboarding is an API call, not an infrastructure change

### Problem 6: A sidecar with no app serves no purpose

The sidecar exists because we decided NOT to have apps connect directly to the broker with bearer tokens (ADR-002, Session 15). The sidecar is a proxy that enforces scope ceilings on behalf of an app. A sidecar with no app is a proxy with nothing to proxy for — it's meaningless.

But we made them independent. The sidecar boots, self-activates, and sits there whether an app exists or not. They should be coupled:

- **Register an app → that creates (or assigns) its sidecar**
- **No app = no sidecar**
- **Remove an app → sidecar credentials are revoked**

The sidecar is an implementation detail of app onboarding, not a separate entity operators manage independently. Operators should think in terms of apps and scopes — the sidecar is the mechanism, not the concept.

---

## What Already Exists (Half the Plumbing)

- `POST /v1/admin/sidecar-activations` — creates a one-time activation token with scoped permissions
- `POST /v1/sidecar/activate` — exchanges an activation token for sidecar credentials
- `aactl` CLI framework — can be extended with `app register` command
- Scope ceiling enforcement at sidecar and broker — already works

The activation endpoint is supposed to be the registration flow. The sidecar just doesn't use it the right way — it bypasses it with the admin secret.

---

## What Needs to Change

### Sidecar bootstrap flow
- New env var: `AA_ACTIVATION_TOKEN` (alternative to `AA_ADMIN_SECRET`)
- If activation token is set, sidecar uses it to activate directly — no admin auth, no self-provisioning
- If neither is set, sidecar refuses to start
- Deprecate (or remove) the admin-secret self-provisioning path

### Broker: App registration
- New entity: App (name, scopes, associated sidecar)
- `POST /v1/admin/apps` — register an app, returns activation token
- `GET /v1/admin/apps` — list registered apps
- `DELETE /v1/admin/apps/{id}` — deregister an app

### aactl: App commands
- `aactl app register --name <name> --scopes <scopes>` — register app, print activation token
- `aactl app list` — list registered apps
- `aactl app update --name <name> --scopes <scopes>` — change ceiling and add/remove scopes by app name
- `aactl app remove --name <name>` — deregister

### App-level scope management (not just sidecar-level)

The ceiling management endpoints already exist (`PUT /v1/admin/sidecars/{id}/ceiling`, `aactl sidecars ceiling set`) — but they operate on sidecar IDs, not app names. Operators shouldn't have to know or track sidecar IDs. The correct UX is managing scopes by app name:

- `aactl app update --name my-app --scopes read:data:*,write:data:*` (what it should be)
- `aactl sidecars ceiling set sc-abc123 --scopes read:data:*,write:data:*` (what we have today)

The app entity bridges this — it maps app names to their associated sidecars, so operators work with names they chose, not system-generated IDs. The existing sidecar ceiling API can stay as the low-level mechanism, with app commands as the operator-facing layer.

### Documentation
- Operator guide: "How to onboard a new app" using aactl, not docker-compose edits
- Developer guide: "How to connect your app" using the activation token

---

## Dialog That Led to This Finding

```
USER: did we fix the known issues because i am trying to run the app and it is not making sense

  1. Wait for broker health
  2. Admin auth with cfg.AdminSecret
  3. Create its own activation token
  4. Activate with that token

  The sidecar always uses the admin secret to self-provision. There's no code
  path to start a sidecar with a pre-created activation token. Every sidecar
  instance holds the admin secret and creates its own activation.

  This means the docs and code tell us:
  - Adding a new app = deploying a new sidecar instance with AA_ADMIN_SECRET
    and a different AA_SIDECAR_SCOPE_CEILING
  - The operator's role isn't creating activations manually — it's deciding the
    scope ceiling for each trust boundary and deploying sidecar instances
  - The sidecar handles its own activation internally

CLAUDE: No — KI-001 (admin secret blast radius) was explicitly not fixed. We
reviewed all 4 known issues in Session 16 and decided none blocked the demo.
Every sidecar holds AA_ADMIN_SECRET and self-provisions. The
create_sidecar_activation endpoint exists but nothing external uses it — the
sidecar calls it internally with the admin secret.

USER: so that does not make sense so what happens if we dont have any apps or
if we have five apps and if it boots together why if we dont have a app yet

CLAUDE: The sidecar boots, self-activates, and registers itself with the
broker — whether or not any app exists to use it. Five apps = five sidecars
all holding the same master key. The sidecar isn't tied to an app — it's tied
to a scope ceiling. There's no concept of "this sidecar belongs to this app."
This is deeper than KI-001. This is about the identity model — apps are
invisible to the broker.

USER: but we should not have to use a docker compose to register an app that
would never fly in the real world so something is terrible wrong

CLAUDE: In the real world, an operator would register an app through an API,
get back credentials, deploy with those credentials. What we have is: edit
YAML, paste admin secret, docker compose up. That's infrastructure config
masquerading as app registration. The activation endpoint is supposed to be
the registration flow. The sidecar just doesn't use it correctly.
```

---

## What the Pattern Actually Says

The Ephemeral Agent Credentialing Pattern v1.2 **does not mention sidecars anywhere.** The sidecar is our implementation choice, not a pattern requirement.

The pattern describes:
1. **Agents get unique identity** — SPIFFE format (`spiffe://domain/agent/...`), cryptographically bound
2. **Agents request tokens** — short-lived JWTs scoped to their task (1-15 min)
3. **A credential service** validates identity and issues tokens
4. **Every request is validated** independently (zero-trust)
5. **Revocation** at token, agent, task, or chain level
6. **Immutable audit** of everything

For bootstrap (the "secret zero" problem), the pattern lists options: platform attestation (SPIFFE/SPIRE), domain-based trust (CIMD), cloud IAM, or **"controlled initial bootstrapping where agents receive a one-time registration token that can only be used once."**

That last one — one-time registration token — is exactly what `POST /v1/admin/sidecar-activations` creates. The plumbing exists. We just made the sidecar bypass it by using the admin secret directly.

**AgentAuth uses the SPIFFE ID format but not SPIFFE/SPIRE infrastructure.** Bootstrap is Ed25519 challenge-response. No attestation platform needed.

The pattern describes a credential service that agents talk to. We built a mandatory proxy (sidecar) in front of it that the pattern never asked for. The sidecar adds DX value (1 call vs 10) and resilience (caching, circuit breaker), but it should be optional — not the only way in.

## Root Cause: We Answered a Product Question with Infrastructure

Divine asked a product question: "How does a developer onboard to AgentAuth? How do we register an app?" We answered with the sidecar — an infrastructure component that requires the admin secret, boots independently of any app, and can only be configured through deployment files.

The question was never "what proxy should sit between the app and the broker." The question was:
1. How does an operator register a new app?
2. How does a developer get credentials and start using AgentAuth?
3. How does the system know which app is which and what it's allowed to do?

Those are product features. We built a daemon instead.

Then in Session 14, Divine asked again:
- "why we cant register application without using sidecars"
- "why cant we remove the sidecars totally"
- "how would we silo scopes for apps if we dont use it"

The 4-agent team in Session 15 rejected direct broker access and said "keep sidecars as primary and only model" (ADR-002). But that decision was based on analyzing the current code, not whether the workflow makes sense for real users. The team defended the implementation instead of answering the original product question.

ADR-002 is invalidated. The sidecar can stay as an optional optimization (DX, caching, resilience) but it cannot be the answer to "how do I onboard an app."

## Priority

This is a **BLOCKER**. The demo cannot credibly show app onboarding if the only way to add an app is to edit docker-compose and share the admin secret. Fix this before demo work continues.
