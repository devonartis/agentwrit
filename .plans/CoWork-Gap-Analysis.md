# CoWork Gap Analysis — AgentAuth

**Date:** 2026-03-01
**Perspective:** Can a real company operationalize this? Not "does the code run" but "would anyone actually deploy this in production?"
**Test applied to every finding:** Could a small company (3–10 agentic apps) or an enterprise actually use this? If not, why not?

---

## Executive Summary

AgentAuth has strong cryptographic foundations — Ed25519 challenge-response identity, scope-attenuated short-lived JWTs, a tamper-evident hash-chained audit trail, and a 4-level revocation system. The security pattern implementation is solid.

**The problem is not security engineering. The problem is that the application cannot be operationalized.**

The system has no concept of "app." You cannot register an app, list apps, audit by app, revoke by app, or manage apps through any API or CLI. The only way to add an app is to deploy infrastructure (a Token Proxy instance) by editing config files and pasting the admin master key. This is not a gap — this is a fundamental design flaw that makes the system unusable for any organization with more than one app.

---

## Section 1: Where the Application Fails

### 1.1 — There Is No Way to Register an Application

**What you'd expect:** An operator runs a command like `register-app --name my-app --scopes read:data` and the system creates the app, returns credentials, and tracks it from that point forward.

**What actually happens:** Nothing. The concept of "app" does not exist in the broker's data model. There is no apps table, no app entity, no app endpoint. The operator must deploy a Token Proxy by editing docker-compose.yml or a systemd config file, paste the admin master key into the proxy's environment, and restart the stack. The proxy self-activates using the master key and sits there whether or not any app ever connects to it.

**Why this is a failure, not a gap:** This is not a missing feature that would be nice to have. This is the fundamental action a customer would try to do on day one. Without app registration, AgentAuth cannot be demonstrated, sold, or used. Every downstream process (auditing, revocation, scope management, onboarding developers) depends on apps being identifiable entities.

**Impact on personas:**
- **Operator:** Cannot onboard an app without infrastructure changes. Cannot list what apps exist. Cannot remove an app through an API.
- **Developer:** Cannot self-service register. Must ask operator to deploy infrastructure.
- **Running app:** Has no identity. Invisible to the system it depends on.

### 1.2 — The Token Proxy Is Mandatory But Shouldn't Be

**What you'd expect:** Apps can connect to the credential broker directly (with an SDK), or optionally through a proxy for convenience.

**What actually happens:** The Token Proxy is the ONLY way for an app to get credentials. There is no direct-to-broker path for apps. The proxy handles Ed25519 key generation, challenge-response registration, and token exchange — all things an SDK could do. But there is no SDK, so the proxy is mandatory.

**Why this matters for operationalization:** Every app requires deploying a proxy instance. For a company with 10 apps, that's 10 proxy instances — each holding the admin master key, each requiring infrastructure configuration, each appearing as "proxy" in the audit trail. The proxy was designed as a convenience layer but became a mandatory dependency. It answers an infrastructure question ("how do we simplify the API?") instead of a product question ("how does a developer onboard?").

**The deeper question:** Do we even need the Token Proxy? The answer is: it should be optional. It provides real value (simplified API, circuit breaker, cached tokens), but apps should also be able to register directly via an SDK. Today that path does not exist.

### 1.3 — The Admin Master Key Is Everywhere

**What you'd expect:** The admin master key stays with the operator. Apps and proxies receive scoped, limited credentials.

**What actually happens:** Every Token Proxy holds the admin master key (`AA_ADMIN_SECRET`). The proxy uses it at bootstrap (to self-activate) AND at runtime (every time a new agent registers, the proxy authenticates as admin again). The master key grants full admin scope: create tokens, revoke any token, read the entire audit trail, create new proxy instances.

**Why this is a process failure:** A compromised proxy — any one of them — gives an attacker full admin access to the entire system. With 10 proxies, the attack surface is 10x. The admin master key was supposed to be a bootstrap mechanism. Instead, it became a runtime dependency baked into every proxy's environment.

**What "operationalize" means here:** No security team would approve deploying a system where the root credential is replicated to every service instance. This would fail any security review.

### 1.4 — Broker Restart Destroys Everything

**What you'd expect:** The broker can restart for maintenance or after a crash, and existing credentials continue working, or at minimum, recovery is graceful.

**What actually happens:** The broker generates a new Ed25519 signing key pair on every startup. This is by design — the pattern says tokens should be ephemeral. But the consequence is that every token ever issued becomes instantly unverifiable. Every proxy must re-bootstrap. Every agent must re-register (5 broker round-trips each). Resource servers reject all existing tokens because they were signed with the old key.

**Why this cannot be operationalized:** A maintenance window on the broker becomes a total outage for every app in the system. There is no rolling restart. There is no key rotation. There is no dual-key verification window. A company with 10 apps and 50 agents faces 250 broker round-trips just to recover. During that window, all apps lose credential access simultaneously.

**The circuit breaker doesn't help:** The Token Proxy has a circuit breaker that serves cached tokens when the broker is down. But those cached tokens were signed with the OLD key. Resource servers that validate against the broker will reject them after restart. The circuit breaker masks the problem at the proxy layer while making it worse at the resource server layer.

### 1.5 — Audit Trail Is Useless for App Attribution

**What you'd expect:** An operator can ask "what did App C do last Tuesday?" and get an answer.

**What actually happens:** All Token Proxies authenticate with the broker using `client_id: "sidecar"`. Every proxy looks identical in the audit trail. There is no app name, no app ID, no way to correlate a proxy to the app it serves. The audit trail records agent-level events (SPIFFE IDs, scopes, timestamps) which are useful for security forensics but completely useless for business questions like "which app is consuming the most tokens?" or "which app's agents are getting revoked?"

**Why this matters:** Audit is a compliance requirement. If a regulator or security team asks "show me all activity for App C," the answer today is "we can't." The data exists but it's not attributable to apps because apps don't exist as entities.

### 1.6 — No SDK Means No Developer Adoption

**What you'd expect:** A developer installs a package (`pip install agentauth`), configures a URL, and calls `client.get_token(scope=["read:data"])`.

**What actually happens:** Developers must hand-code HTTP calls to the Token Proxy. They must manage token renewal themselves (polling at 80% TTL). They must implement scope checking in their resource servers from scratch. There is no Python SDK, no TypeScript SDK, no middleware library, no reference implementation beyond code examples in the docs.

**Why this is a process failure, not just a missing feature:** Without an SDK, developer onboarding takes hours instead of minutes. Every developer re-invents the same renewal loop, the same error handling, the same scope checking logic. Bugs multiply. And critically — there is no SDK path for direct-to-broker access, which means the Token Proxy remains mandatory.

### 1.7 — Token Validation Is a Single Point of Failure

**What you'd expect:** Resource servers can validate tokens locally using a published public key (JWKS endpoint), with optional online validation for revocation checking.

**What actually happens:** Every token validation requires a network call to `POST /v1/token/validate` on the broker. There is no JWKS endpoint. There is no way to verify tokens locally. The broker is a single instance with no replication.

**Why this cannot be operationalized:** If the broker is down, no resource server can validate any token. A single broker serving 10 apps means one process handles all token issuance AND all token validation for the entire organization. This is a single point of failure at the most critical point in the system.

### 1.8 — Secret Rotation Is a Coordinated Outage

**What you'd expect:** The admin master key can be rotated without downtime. The broker accepts both old and new keys during a transition window.

**What actually happens:** Changing the admin master key requires: update the broker config, restart the broker (which destroys all tokens — see 1.4), update every proxy config (all 10), restart every proxy (which disrupts all apps). There is no graceful rotation, no dual-key window, no automated rotation mechanism.

**Why this fails operationalization:** Security policies typically require periodic key rotation. With AgentAuth, key rotation = coordinated total outage. Most operators would simply never rotate the key, which is the opposite of what a security system should encourage.

---

## Section 2: What Needs to Be Enhanced

### 2.1 — App Entity and Registration API

Build the concept of "app" into the broker. An app has a name, allowed scopes, an associated proxy (optional), and a lifecycle (registered → active → deregistered). Expose `POST /v1/admin/apps`, `GET /v1/admin/apps`, `DELETE /v1/admin/apps/{id}`. Add `aactl app register`, `aactl app list`, `aactl app remove`. This is the single most important enhancement — it unblocks everything else.

### 2.2 — Activation Token Bootstrap Path

Add `AA_ACTIVATION_TOKEN` to the Token Proxy config as an alternative to `AA_ADMIN_SECRET`. If the activation token is set, the proxy uses it to activate directly — no admin auth, no self-provisioning. The operator creates the activation token via the app registration API and passes it to the deployment. The admin master key never leaves the operator's environment.

### 2.3 — Direct-to-Broker SDK

Build a Python SDK (and later TypeScript) that lets apps register and get tokens directly from the broker without a Token Proxy. The SDK handles Ed25519 key generation, challenge-response, token renewal, and scope checking. This makes the Token Proxy optional — valuable for resilience and DX, but not required.

### 2.4 — JWKS Endpoint for Local Validation

Add `GET /.well-known/jwks.json` to the broker, exposing the current Ed25519 public key. Resource servers can validate token signatures locally without calling the broker. Online validation via `POST /v1/token/validate` remains available for revocation checking.

### 2.5 — Key Persistence or Rotation

Either persist the Ed25519 signing key across restarts (so existing tokens survive) or implement a dual-key rotation mechanism (new key signs, old key verifies for a grace period). Without this, broker restart = total disruption.

### 2.6 — Per-Proxy Credentials

Each Token Proxy should have unique credentials (not the admin master key). The broker should know which proxy is which. Audit events should show "proxy-for-app-C" not just "proxy." This requires the app registration API (2.1) and the activation token path (2.2).

### 2.7 — High Availability Path

Document (and eventually build) a path to multi-broker deployment. At minimum: shared SQLite via WAL mode + Litestream, or migration to PostgreSQL. A single broker for an enterprise is not acceptable.

---

## Section 3: What Needs to Be Added (Does Not Exist At All)

| # | Feature | Why It's Needed |
|---|---------|----------------|
| 3.1 | App entity in broker data model | Apps must be identifiable, trackable, revocable |
| 3.2 | App registration API + CLI | Onboarding must be an API call, not infra config |
| 3.3 | App-level revocation | "Revoke all tokens for App C" must be one command |
| 3.4 | App-level audit queries | "Show me what App C did" must be answerable |
| 3.5 | App-level scope management | "Change App C's scopes" by name, not sidecar ID |
| 3.6 | Python SDK | Developer onboarding cannot require hand-coded HTTP |
| 3.7 | JWKS endpoint | Token validation cannot depend on broker availability |
| 3.8 | Activation token boot path | Proxies must boot without the admin master key |
| 3.9 | Key persistence or rotation | Broker restart cannot destroy all credentials |
| 3.10 | Push revocation notifications | Revoked tokens must stop working immediately |
| 3.11 | Rate limiting on proxy | Rogue apps cannot flood token requests |
| 3.12 | App deregistration API | Removing an app must be an API call with cleanup |

---

## Section 4: Severity Priority Matrix

| Priority | Item | Rationale |
|----------|------|-----------|
| **P0 — Blocker** | App registration (3.1 + 3.2) | Cannot demo or use without this |
| **P0 — Blocker** | Activation token boot path (3.8) | Admin key must stop spreading |
| **P1 — Critical** | Python SDK (3.6) | Developers cannot onboard without it |
| **P1 — Critical** | Key persistence/rotation (3.9) | Broker restart = total outage |
| **P1 — Critical** | JWKS endpoint (3.7) | Single point of failure for validation |
| **P2 — High** | App-level revocation (3.3) | Operator cannot manage by app name |
| **P2 — High** | App-level audit (3.4) | Audit is useless for business questions |
| **P2 — High** | Per-proxy credentials (from 2.6) | Security review would fail |
| **P3 — Medium** | Push revocation (3.10) | Revocation has delay |
| **P3 — Medium** | HA path (from 2.7) | Single broker is a risk |
| **P3 — Medium** | Rate limiting (3.11) | Abuse protection |
| **P4 — Low** | TypeScript SDK | Second developer audience |
| **P4 — Low** | App deregistration (3.12) | Cleanup, not blocking |

---

## Section 5: The Bottom Line

AgentAuth has excellent security primitives. The Ed25519 challenge-response identity system, scope attenuation, delegation chains, and hash-chained audit trail are well-designed and well-implemented. The 6 compliance fixes from sessions 10–16 brought the codebase to high compliance with the Ephemeral Agent Credentialing Pattern v1.2.

But none of that matters if you can't register an app.

The system was built from the inside out — cryptography first, then protocols, then handlers, then infrastructure automation. What's missing is the outside-in perspective: what does an operator actually DO on day one? What does a developer actually DO to get started? The answer today is "edit YAML files and paste secrets" — and that is not a product.

The path forward is clear: build the app entity, build the activation token path, build the SDK, make the Token Proxy optional. These four changes turn AgentAuth from a well-engineered security library into a product that can be operationalized.
