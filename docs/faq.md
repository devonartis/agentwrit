# Frequently Asked Questions

Questions from real practitioners evaluating AgentWrit for their environments. If you have a question not covered here, [open an issue](https://github.com/devonartis/agentwrit/issues).

---

## Identity & Credentials

### AgentWrit proves who the agent is, but how does the agent get the actual API key or OAuth token for the service it's calling?

AgentWrit handles the **identity plane** — proving who the agent is, what scope it has, and how long it can operate. It does not handle the **data plane** — injecting the actual secret (OAuth refresh token, database password, API key) for the downstream service.

This is by design. The credential exchange bridge — where an AgentWrit identity token gets swapped for a real service credential via Vault, AWS Secrets Manager, cloud federation, or OIDC token exchange — is an enterprise module that plugs into the core via interfaces. Keeping secret injection out of the open core means the core has zero access to your actual secrets.

**Who needs the bridge:** Anyone running agents against real APIs (Google Calendar, AWS services, databases) in production.

**What you can do today:** The source code is available under the PolyForm Internal Use license. You're free to build your own bridge for internal use — wire up Vault, Secrets Manager, or any credential store. The [integration patterns doc](integration-patterns.md) covers the design.

**Related docs:**
- [Concepts — the 8 components](concepts.md) — where the identity/data plane boundary is drawn
- [Architecture](architecture.md) — how the broker is structured, what it owns, what it doesn't
- [Integration Patterns](integration-patterns.md) — credential exchange, resource server validation
- [Design Decisions](design-decisions.md) — why the boundary exists

---

### Will AgentWrit support OIDC token exchange?

OIDC provider and token exchange functionality is planned as an enterprise module. It would allow an AgentWrit identity token to be exchanged for an OIDC-compliant token that downstream services already trust — bridging AgentWrit's agent identity into existing OAuth2/OIDC infrastructure without those services needing to know about AgentWrit directly.

The core broker exposes interfaces for this. The enterprise module plugs in without modifying core code. If you need this today, the source is available — you can build an OIDC bridge for your own internal use under the license.

---

## Scope & Access Control

### How do you decide the "right amount" of access without making the system too complex?

You don't decide at the agent level — that's where it would get complex. AgentWrit splits the decision across three layers:

1. **Operator** sets a scope ceiling when registering an app — "this app can do X, Y, Z." One-time setup.
2. **App** creates launch tokens per task, scoped to just what the task needs. The app already knows what the agent is about to do — it's the one dispatching it.
3. **Agent** registers and gets a token bounded by both. It can ask for less, never more.

Scopes only go in one direction: down. Every boundary narrows. No one in the chain can escalate.

**What about TTL?** AgentWrit ships with a default of 5 minutes. Each app can have its own TTL override. If a task runs longer, the agent can renew — the old token is immediately revoked, a new one issued. You don't have to get it perfect on day one.

**Related docs:**
- [Scope Model](scope-model.md) — how scope ceilings and attenuation work
- [Roles](roles.md) — who decides what (operator → app → agent)
- [Credential Model](credential-model.md) — what a token actually contains

---

### Is there scope drift detection — granted vs actually-used?

Not yet. AgentWrit audits everything it *issues* — 24 event types in a tamper-evident hash chain. But it can't see what the agent *does* with the token at the resource server. A scope-usage audit that compares granted vs actually-used scopes would require resource servers to report back to the broker.

This is a real gap and a great feature idea. The audit infrastructure is already in place — the question is whether to build it into core or as an enterprise integration that includes resource server reporting. Either way, the "silent 403 for days because the agent had readonly when it needed write" scenario is exactly what it would catch.

**Related docs:**
- [Architecture — audit trail](architecture.md#key-design-decisions) — how the hash-chain audit works today
- [Token Lifecycle](token-lifecycle.md) — what events are captured

---

## SDKs & Integration

### Where are the SDKs? The broker works but custom HTTP is an adoption barrier.

The Python SDK is cleaned, tested, and publishing soon. TypeScript SDK is next. Until then, every integration is custom HTTP against the [API reference](api.md).

The API is clean — 19 endpoints, JSON in/out, RFC 7807 errors — but we know "a day to integrate" is a real barrier. SDKs are the next priority.

**Related docs:**
- [API Reference](api.md) — every endpoint, request/response shapes, error codes, rate limits
- [Getting Started: Developer](getting-started-developer.md) — building an agent that authenticates with AgentWrit

---

## Reliability & Operations

### SQLite on a single node — what's the HA plan? What happens during a broker restart?

The restart story is better than it might appear:

- **Signing key is persistent.** The Ed25519 key is stored on disk (`AA_SIGNING_KEY_PATH`). Tokens issued before a restart **remain valid** — they're self-contained JWTs verified against the persistent key.
- **Audit trail and revocation lists are persisted** to SQLite and reloaded on startup. No data loss.
- **What's lost on restart:** challenge nonces (30-second TTL — ephemeral by design), in-memory agent registration records, and unconsumed launch tokens.

So agents with valid tokens keep working during a broker restart. They just can't register *new* agents until the broker comes back. HA / clustering is future work.

**Related docs:**
- [Architecture — Key Design Decisions](architecture.md#key-design-decisions) — persistent signing key, hybrid persistence
- [Architecture — Security Assumptions](architecture.md#security-assumptions) — explicit limitations including single-instance design

---

## Demo & Examples

### Where's the end-to-end example? Show an agent getting a credential, calling a real API, credential expiring.

Coming. MedAssist is referenced in the docs but not shipped yet. An end-to-end demo — agent authenticates, gets scoped credential, calls a real API, credential expires — is on the roadmap. This is the thing that makes it real for people evaluating it.

In the meantime, the scenarios doc walks through the full flow with code:
- [Real-World Scenarios](scenarios.md) — the 8 components in production contexts

---

## Licensing & Usage

### Can I use AgentWrit in my company?

Yes. The PolyForm Internal Use License 1.0.0 means:

- **Free for any company to use internally** — startups, mid-market, enterprise. Use it, modify it, run it in production. No cost, no sales call, no license key.
- **Free for nonprofits, education, and research** — email `licensing@agentwrit.com` and it's done.
- **Not free to resell or host as a SaaS** — building a managed service on top of AgentWrit requires a commercial license.

### Can I build the enterprise features myself?

Yes — for your own internal use. The source code is public. You can build your own Vault bridge, OIDC exchange, scope telemetry, or anything else. Modify the code, extend it, deploy it internally. The only restriction is you can't resell it or offer it as a hosted service to others.

The [EAC v1.3 pattern](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.3.md) is fully public. The code is public. The ideas are free. Build what you need.

---

## The Big Picture

### "The identity layer is done. The last mile is not."

Fair summary. The identity plane is solid — challenge-response registration, SPIFFE identities, scope attenuation, 4-level revocation, tamper-evident audit, delegation chains. The bridge to real service credentials is the enterprise module. The SDKs and demo are in progress.

AgentWrit was purpose-built for agents — not retrofitted from human IAM. Traditional IAM was designed for humans and long-running services. Agents are ephemeral, task-scoped, and delegate to each other. That's a different lifecycle, and it needs a different credential system.

The big NHI vendors (Okta, CyberArk, Astrix) are working on this same problem at enterprise scale and enterprise prices. AgentWrit is working on it at the scale where most teams actually live.

---

*Back to [README](../README.md) · [Architecture](architecture.md) · [Concepts](concepts.md)*
