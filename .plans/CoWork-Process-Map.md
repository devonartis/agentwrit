# CoWork Process Map — AgentAuth End-to-End

**Date:** 2026-03-01
**Evaluation lens:** Can this be operationalized? Would a real company deploy this?
**Not evaluated:** Whether code compiles or tests pass. A process that "works mechanically" but no operator would actually follow is marked BROKEN.
**Production test:** Could a company with 3–10 agentic apps use this process on a virtual server (no Docker requirement)?

---

## Process 1: Operator Sets Up AgentAuth

| Step | Action | Verdict | Why |
|------|--------|---------|-----|
| 1.1 | Generate an admin master key | OK | Standard secret generation |
| 1.2 | Configure the broker via environment variables | OK | Standard practice |
| 1.3 | Start the broker binary | OK | Works on any server |
| 1.4 | Broker validates master key exists | OK | Fail-fast on missing config |
| 1.5 | Broker generates fresh Ed25519 signing keys | **BAD DESIGN** | Keys are ephemeral — every restart invalidates ALL credentials system-wide. No persistence, no rotation. This means you cannot restart the broker for maintenance without disrupting every app. |
| 1.6 | Broker initializes SQLite persistence | OK for demo | SQLite is fine for small deployments but has no path to distributed storage. Blocks HA. |
| 1.7 | Broker loads audit trail + revocations from DB | OK | Data survives restarts |
| 1.8 | Broker starts listening | OK | |

**Process 1 verdict: PARTIALLY BROKEN.** The broker starts and serves requests, but the ephemeral key design means any maintenance = total outage. No sane operator would accept this for a production system. The broker itself is a single point of failure with no HA path.

---

## Process 2: Operator Onboards a New App

| Step | Action | Verdict | Why |
|------|--------|---------|-----|
| 2.1 | Operator decides what app needs credentials | OK | Business decision |
| 2.2 | Operator determines the scope ceiling | OK | Policy decision |
| 2.3 | Operator registers the app via CLI or API | **DOES NOT EXIST** | There is no `register-app` command. There is no `/v1/admin/apps` endpoint. The concept of "app" does not exist in the data model. |
| 2.4 | Broker creates app entity + returns credentials | **DOES NOT EXIST** | No app entity. No activation token returned. |
| 2.5 | Operator hands credentials to the app deployment | **DOES NOT EXIST** | No credentials to hand over. |

**What the operator actually has to do instead:**

| Step | What Actually Happens | Verdict | Why |
|------|----------------------|---------|-----|
| 2.3a | Edit docker-compose.yml or systemd config to add a Token Proxy service block | **BROKEN** | Registering an app should be an API call, not an infrastructure change. A customer on a bare VM cannot do this without manually creating service configs. |
| 2.3b | Paste the Admin Master Key into the Token Proxy's config | **BROKEN** | The master key — which grants full admin access to the entire system — is now in the app's deployment config. This would fail any security review. |
| 2.3c | Set a scope ceiling in the config | OK mechanically | But it's in an infra config file, not managed through an API. |
| 2.3d | Deploy the Token Proxy instance | **BROKEN** | This is an infrastructure deployment to accomplish a business action. |
| 2.3e | Token Proxy self-activates using the master key | **BAD DESIGN** | The proxy creates its own activation token using admin credentials, then activates itself. The intended external activation flow (operator creates token, hands to proxy) is bypassed. |
| 2.3f | The "app" is now "onboarded" — but invisible to the system | **BROKEN** | No app entity was created. The broker knows a Token Proxy exists but doesn't know what app it serves. Cannot list, audit, revoke, or manage by app name. |

**Process 2 verdict: BLOCKER.** This entire process is fundamentally broken. Onboarding an app requires infrastructure changes, spreads the master key, and creates no trackable entity. For 10 apps, you need 10 infrastructure deployments, 10 copies of the master key, and zero visibility into what apps exist.

---

## Process 3: Token Proxy Bootstraps with the Broker

| Step | Action | Verdict | Why |
|------|--------|---------|-----|
| 3.1 | Token Proxy starts HTTP server (health endpoint only) | OK | |
| 3.2 | Token Proxy waits for broker health with exponential backoff | OK | Good resilience pattern |
| 3.3 | Token Proxy authenticates as admin using the master key | **BAD DESIGN** | Every proxy holds the master key. This key grants full admin scope: create tokens, revoke any token, read all audit, create new proxies. A compromised proxy = total system compromise. |
| 3.4 | Token Proxy creates its own activation token | **BAD DESIGN** | The proxy self-provisions. The activation endpoint (`/v1/admin/sidecar-activations`) was built for external use, but the proxy bypasses it by calling it itself. |
| 3.5 | Token Proxy activates itself | OK mechanically | |
| 3.6 | Token Proxy registers app-facing routes | OK | |
| 3.7 | Token Proxy starts background renewal loop | OK | Good pattern |
| 3.8 | Token Proxy starts circuit breaker health probe | OK | Good pattern |

**Process 3 verdict: BAD DESIGN.** The bootstrap works, but the security model is wrong. The master key should not be in the proxy. The proxy should boot with a pre-created, scoped activation token — not the root credential. The plumbing to do this correctly (the activation endpoint) already exists but is not used correctly.

---

## Process 4: Developer Gets a Token

| Step | Action | Verdict | Why |
|------|--------|---------|-----|
| 4.1 | Developer calls `POST /v1/token` on the Token Proxy | OK | Simple API |
| 4.2 | Token Proxy checks scope against ceiling | OK | Good enforcement |
| 4.3 | If first-time agent: Token Proxy runs lazy registration (5 broker calls) | **BAD DESIGN** | First token request takes 5 round-trips (~100ms+). And the proxy authenticates as admin AGAIN for every new agent, meaning the master key is used at runtime — not just bootstrap. |
| 4.4 | Token Proxy requests scoped token from broker | OK | |
| 4.5 | Token Proxy caches token for circuit breaker fallback | OK | Good pattern |
| 4.6 | Developer receives short-lived JWT | OK | |

**Process 4 verdict: WORKS but with design flaws.** The developer-facing API is clean (single call), but behind the scenes, the master key is used at runtime and the 5-call lazy registration is expensive. Without an SDK, developers must hand-code this HTTP call and manage renewal themselves.

---

## Process 5: Running App Validates Tokens at Resource Server

| Step | Action | Verdict | Why |
|------|--------|---------|-----|
| 5.1 | App presents Bearer token to resource server | OK | Standard JWT pattern |
| 5.2 | Resource server calls `POST /v1/token/validate` on broker | **BAD DESIGN** | Every validation is a network call to a single broker instance. There is no JWKS endpoint for local verification. This makes the broker a single point of failure for ALL token validation across ALL apps. |
| 5.3 | Broker verifies signature + checks revocation | OK | |
| 5.4 | Resource server manually checks scope | **BAD DESIGN** | No SDK or middleware exists. Every resource server team re-implements scope checking from scratch. |
| 5.5 | Resource server allows or denies access | OK | |

**Process 5 verdict: BAD DESIGN.** Online-only validation with no local fallback means the broker must be available for any app to use its tokens. For 10 apps, all token validation flows through one process. If the broker is down, every resource server rejects every token.

---

## Process 6: Operator Investigates a Security Incident

| Step | Action | Verdict | Why |
|------|--------|---------|-----|
| 6.1 | Security team asks: "What did App C do last week?" | N/A | Business question |
| 6.2 | Operator queries audit trail: `aactl audit events` | OK mechanically | CLI works |
| 6.3 | Operator tries to filter by app name | **BROKEN** | There is no app name in any audit event. All Token Proxies authenticate as `client_id: "sidecar"`. |
| 6.4 | Operator tries to correlate proxy ID to app | **BROKEN** | Must manually track which proxy ID maps to which app. No tooling supports this. |
| 6.5 | Operator answers the security team's question | **CANNOT** | The information exists (agent IDs, scopes, timestamps) but cannot be attributed to apps because apps don't exist as entities. |

**Process 6 verdict: BROKEN.** The hash-chained audit trail is technically excellent (tamper-evident, queryable, persistent), but it's useless for the most basic operational question: "what did this app do?" This would fail a compliance audit.

---

## Process 7: Operator Rotates the Admin Master Key

| Step | Action | Verdict | Why |
|------|--------|---------|-----|
| 7.1 | Security policy requires periodic key rotation | N/A | Standard requirement |
| 7.2 | Update the master key on the broker | OK | Config change |
| 7.3 | Restart the broker | **BAD DESIGN** | Restarts generates new signing keys → ALL tokens invalid → total disruption (see Process 1, step 1.5) |
| 7.4 | Update the master key on ALL Token Proxies (10 of them) | **BROKEN** | Must edit 10 config files. No automation, no rolling update, no API. |
| 7.5 | Restart ALL Token Proxies | **BROKEN** | 10 proxies restart, all agents re-register, all apps disrupted. |
| 7.6 | All apps resume normal operation | Eventually | After re-bootstrapping (250 broker calls for 50 agents) |

**Process 7 verdict: BROKEN.** Key rotation — a basic security hygiene operation — causes a coordinated total outage. Most operators would simply never rotate the key, which defeats the purpose of having a security system.

---

## Process 8: App Survives a Broker Restart

| Step | Action | Verdict | Why |
|------|--------|---------|-----|
| 8.1 | Broker goes down (crash, maintenance, upgrade) | N/A | |
| 8.2 | Token Proxy circuit breaker opens | OK | Good pattern |
| 8.3 | Token Proxy serves cached tokens to apps | OK mechanically | But see next step |
| 8.4 | App presents cached token to resource server | **FAILS** | Resource server calls broker to validate → broker is down → validation fails → access denied. Even if broker comes back, it has NEW signing keys, so cached tokens fail signature verification. |
| 8.5 | Broker restarts with new signing keys | See Process 1 | |
| 8.6 | Token Proxies re-bootstrap with master key | OK mechanically | |
| 8.7 | ALL agents re-register (5 calls each) | **BAD DESIGN** | Recovery time scales linearly with agent count. 50 agents = 250 round-trips. |
| 8.8 | Apps eventually get new tokens | Eventually | Could be minutes |

**Process 8 verdict: BAD DESIGN.** The circuit breaker in the Token Proxy gives a false sense of resilience. It caches tokens that won't validate because the broker's signing keys changed. The net effect is that a broker restart = total credential disruption, and the circuit breaker just delays when apps discover this.

---

## Process 9: Do We Even Need the Token Proxy?

This is a design question, not a process step. The Token Proxy (what the code calls "sidecar") currently serves as:

1. **API simplifier:** Developers make 1 call instead of 5–10
2. **Key manager:** Generates and stores Ed25519 keys for agents
3. **Scope enforcer:** Checks scope ceiling locally before calling broker
4. **Resilience layer:** Circuit breaker + cached tokens

**But it also creates these problems:**

1. **Mandatory dependency:** Apps cannot talk to the broker directly
2. **Master key proliferation:** Every proxy holds the root credential
3. **Infrastructure overhead:** Adding an app = deploying a proxy
4. **Audit blindness:** All proxies are indistinguishable
5. **Design bypass:** The activation flow intended for external use is self-consumed

**Verdict: The Token Proxy should be OPTIONAL.** Apps should be able to register directly with the broker via an SDK. The proxy provides genuine operational value (simplified API, circuit breaker, scope caching), but making it mandatory creates the three biggest problems in the system: master key spread, infrastructure-as-registration, and audit blindness.

---

## Summary: Every Process Evaluated

| Process | Description | Verdict |
|---------|-------------|---------|
| 1 | Broker setup | **Partially broken** — ephemeral keys, single instance |
| 2 | App onboarding | **BLOCKER** — entire process missing |
| 3 | Token Proxy bootstrap | **Bad design** — master key in every proxy |
| 4 | Developer gets token | **Works** with design flaws |
| 5 | Token validation | **Bad design** — single point of failure |
| 6 | Audit investigation | **Broken** — no app attribution |
| 7 | Key rotation | **Broken** — causes total outage |
| 8 | Broker restart recovery | **Bad design** — false resilience, total disruption |
| 9 | Token Proxy necessity | **Design question** — should be optional, not mandatory |

**Of 9 processes evaluated: 0 are fully production-ready. 3 are outright broken. 4 have bad design that blocks operationalization. 2 work with significant caveats.**
