# ADR-002: Keep Sidecar Model as Primary Architecture

**Date:** 2026-02-25
**Status:** Accepted
**Deciders:** 4-agent collaborative analysis (3 neutral analysts + devil's advocate with veto)
**Branch:** fix/sidecar-uds
**Supersedes:** None
**Related:** [KNOWN-ISSUES.md](../KNOWN-ISSUES.md) (admin secret blast radius, TCP default)

---

## Decision

**Keep sidecars as the primary and only current model. Document comprehensively before merging. Design the admin secret fix. Treat direct broker app access as future work blocked by broker code changes.**

The sidecar model provides real, code-verified security properties and developer experience value. It also has a genuine architectural weakness: every sidecar process holds the root admin secret, making the blast radius of a compromised sidecar wider than the scope ceiling would suggest.

Direct broker app access (`client_id`/`client_secret`) is architecturally feasible but is **future work, not a current option**. The broker's token exchange endpoint requires caller tokens carrying `sidecar:scope:X` claims (`internal/handler/token_exchange_hdl.go:131`). An application authenticating directly today receives admin-scoped tokens — not the sidecar-shaped credential the exchange endpoint requires. Building a direct-access path requires broker code changes (new endpoint, new AppRecord schema, exchange handler extension) that are not yet designed or built.

---

## The 6 Architecture Questions (MEMORY.md Session 14)

### Q1: How do operators create new sidecars?

Operators set three required environment variables and deploy the sidecar binary or container:

```
AA_ADMIN_SECRET      # must match broker — root of trust
AA_SIDECAR_SCOPE_CEILING  # comma-separated max scopes, e.g. read:data:*,write:data:*
AA_BROKER_URL        # e.g. http://broker:8080
```

The sidecar self-bootstraps on first start (`cmd/sidecar/main.go:95-120`):
1. Waits for broker health
2. Authenticates as admin with `AA_ADMIN_SECRET`
3. Creates a sidecar activation token with the configured scope ceiling
4. Exchanges it for a long-lived sidecar bearer JWT

There is no manual pre-creation step. The sidecar ID is derived from the activation JWT's JTI (`internal/admin/admin_svc.go:358`). There is currently no `aactl` command for pre-provisioning a sidecar ID before deployment — the ID is known only after the sidecar first bootstraps.

**Gap:** No aactl tooling for sidecar provisioning. Operators learn the sidecar ID from `aactl sidecars list` or `GET /v1/health` after startup.

### Q2: How do 3rd-party SDK consumers register apps to use sidecars?

SDK consumers do not register anything. They point their application at the sidecar URL and call:

```http
POST /v1/token
{"agent_name": "my-agent", "scope": ["read:data:*"], "ttl": 300}
```

The sidecar handles all broker complexity transparently via `lazyRegister()` (`cmd/sidecar/handler.go:157-229`): admin auth, launch token creation, Ed25519 key generation, challenge-response registration, and token exchange. The developer writes one HTTP call; the sidecar makes up to 10 broker calls under the hood.

**Prerequisite:** An operator must have already deployed a sidecar with a scope ceiling that covers what the SDK consumer needs. There is no self-service sidecar provisioning today.

**Gap:** No developer onboarding guide exists. The docs explain what the sidecar does but not how a developer connects their app to one.

### Q3: Why sidecars? Rationale vs. direct broker access

The sidecar provides five concrete benefits:

**1. Simplified developer API.**
One call (`POST /v1/token`) vs. a 10-step challenge-response flow (challenge, key generation, Ed25519 signing, registration, token exchange). The full broker protocol is correct to expose to sophisticated SDK consumers but is too complex as a default.

**2. Scope ceiling cryptographically baked into the sidecar credential.**
At activation, the broker issues a JWT with `sidecar:scope:read:data:*` entries embedded in the token (`internal/admin/admin_svc.go:359-363`). The broker extracts this ceiling at exchange time via `sidecarAllowedScopes()` (`internal/handler/token_exchange_hdl.go:131-152`). The ceiling is not a config lookup — it is cryptographically bound to the sidecar identity. A sidecar cannot request a wider ceiling at request time.

**3. Dual ceiling enforcement.**
The sidecar pre-checks scope locally (`cmd/sidecar/handler.go:78`) before calling the broker. The broker re-enforces independently (`token_exchange_hdl.go:142`). A bug in sidecar enforcement still hits the broker check. The broker check is authoritative and cannot be bypassed by a misconfigured or compromised sidecar — it reads the ceiling from a broker-signed JWT.

**4. Resilience features.**
Circuit breaker with sliding-window failure rate (`cmd/sidecar/circuitbreaker.go`), cached token fallback when the broker is unreachable (`handler.go:109-125`), and background token renewal (`main.go:129`). These are not available if the app calls the broker directly.

**5. UDS access control (Fix 5).**
When `AA_SOCKET_PATH` is set, the sidecar listens on a Unix domain socket with `0660` permissions (`cmd/sidecar/listener.go:22-24`). Only processes sharing the socket's Unix group can request tokens. This is real OS-level access control that TCP cannot provide.

### Q4: Can we remove sidecars entirely?

Not with the current broker code. The token exchange endpoint requires caller tokens carrying `sidecar:manage:*` scope and `sidecar:scope:X` ceiling entries (`internal/handler/token_exchange_hdl.go:131`). These are sidecar-shaped credentials. An application authenticating directly with `client_id`/`client_secret` receives admin-scoped tokens — a fundamentally different shape that the exchange endpoint does not accept. Building direct app access requires broker code changes that do not yet exist. See: Direct Broker Access as Future Work below.

What must be built to replace sidecars:
- New `/v1/apps` endpoint: register an app with `client_id`, `client_secret`, and a scope ceiling
- `AppRecord` table in SQLite (analogous to `SidecarRecord` at `internal/store/sql_store.go:73-79`)
- App credential JWT carrying `app:scope:X` entries — the broker's exchange handler treats it identically to a sidecar token
- SDK or documentation for apps to perform challenge-response themselves (or a simplified broker endpoint)

Removing sidecars without this infrastructure in place would leave operators with no scope siloing mechanism. That is worse than the current state.

### Q5: How would we silo scopes without sidecars?

The broker's ceiling enforcement mechanism is already generic. It extracts entries with the `sidecar:scope:` prefix from the authenticated caller's token (`token_exchange_hdl.go:235-246`). Changing this prefix to `app:scope:` — or making the prefix configurable — is a small code change.

The full scope siloing design without sidecars:

1. **App registration:** `POST /v1/apps` with `client_id`, `client_secret`, `scope_ceiling`. Broker stores `AppRecord` in SQLite.
2. **App authentication:** `POST /v1/apps/auth` — returns a JWT with `app:manage:*` and `app:scope:<ceiling>` entries. Structurally identical to a sidecar bearer token.
3. **Token exchange:** `POST /v1/token/exchange` — the handler already enforces the ceiling from the caller token. No change needed if the app token carries the right scope entries.
4. **Ceiling management:** `PUT /v1/apps/{app_id}/ceiling` — same pattern as `PUT /v1/admin/sidecars/{id}/ceiling`.

The broker ceiling check at `token_exchange_hdl.go:131` is the one place that matters. Everything else is plumbing.

### Q6: How do operators configure sidecars for specific applications?

**One sidecar per trust boundary.** A trust boundary is a group of agents that all share the same maximum permission ceiling.

Concrete guidance:
- **One sidecar per application** — each app gets its own sidecar with the narrowest ceiling that covers its needs. Strongest isolation.
- **One sidecar per team** — if multiple apps built by the same team need the same ceiling, they can share one sidecar. Reduces operational overhead.
- **Never share sidecars across trust boundaries** — if Team A's ceiling is `read:data:*` and Team B's is `write:data:*`, they must have separate sidecars. Sharing would give both teams the union of both ceilings.

The scope ceiling (`AA_SIDECAR_SCOPE_CEILING`) is set at deployment time and can be updated at runtime without restarting the sidecar via `aactl sidecars ceiling set`. Changes propagate within one renewal cycle (4-12 minutes at the default 900s TTL / 80% renewal buffer). For immediate scope reduction, use `aactl revoke` to revoke active tokens; the ceiling propagation handles future requests.

---

## What Security Properties Are Real vs. Theater

### Real

**Dual ceiling enforcement is not theater.** The scope ceiling is enforced at two cryptographically independent points. The sidecar pre-check at `handler.go:78` uses the in-process `ceilingCache`. The broker check at `token_exchange_hdl.go:131` reads the ceiling from the sidecar's Ed25519-signed JWT, which was issued by the broker at activation time. A rogue sidecar cannot claim a wider ceiling — the broker ignores `sidecar_id` values supplied by the client (`token_exchange_hdl.go:41, 167-169`).

**Sidecar anti-spoof is real.** The `sid` field in issued agent tokens is derived from `claims.Sid` in the authenticated sidecar token, never from client-supplied fields (`token_exchange_hdl.go:167`). An agent cannot forge its sidecar association.

**UDS access control is real** when `AA_SOCKET_PATH` is set. `0660` socket permissions restrict access to owner + group at the OS level. This is a meaningful security boundary that TCP cannot replicate.

### Theater / Weak

**The admin secret blast radius is wider than advertised.** Every sidecar holds `AA_ADMIN_SECRET` (`config.go:58`). This secret grants full admin scope: `admin:launch-tokens:*`, `admin:revoke:*`, and `admin:audit:*` (`internal/admin/admin_svc.go:44-48`). A compromised sidecar environment can revoke any token in the system, read the full audit trail, create new sidecar activations with arbitrary ceilings, and create launch tokens for any agent. The scope ceiling limits what the sidecar issues through its own `/v1/token` endpoint. It does not bound what an attacker with the admin secret can do by calling the broker directly.

**The admin secret is used at runtime, not just bootstrap.** `lazyRegister()` at `handler.go:183` calls `adminAuth(h.adminSecret)` for every new agent registration. Because agents are keyed by `agent_name + ":" + task_id` (`handler.go:98-99`) and the registry is in-memory and ephemeral, any sidecar restart re-registers all agents, and any new task triggers a fresh admin auth. Admin secret usage grows with agent churn, not just sidecar count.

**TCP mode is the default and provides no meaningful sidecar-specific security boundary.** `config.go:88` defaults `SocketPath` to empty string. Without `AA_SOCKET_PATH`, the sidecar listens on TCP (`listener.go:39`) with a `WARN` log as the only signal. In TCP mode, any process on the host can call `POST /v1/token`. The sidecar's local scope ceiling check is a performance optimization (fast rejection before a broker round-trip), not an independent security boundary — the broker already enforces the ceiling. The default deployment mode therefore adds admin secret exposure with no compensating OS-level access control.

---

## Decision Rationale

### Keep sidecars

The sidecar provides three irreplaceable values in the current system:

1. **Developer experience** — `POST /v1/token` is dramatically simpler than implementing challenge-response in every application
2. **Broker-verified scope ceiling** — the ceiling is cryptographically bound at activation, not configurable at runtime
3. **Resilience** — circuit breaker, token caching, and background renewal are production-grade features that would need to be rebuilt in an SDK

Removing sidecars before a direct-access alternative is production-ready would leave operators with no scope siloing and no developer simplification. That is a worse state.

### Defer direct app access until a use case demands it

The user question from MEMORY.md Session 14 — "why can't we register an application without using sidecars?" — is answered by the code analysis: the broker's ceiling enforcement at `token_exchange_hdl.go:131` reads `sidecar:scope:X` entries from the caller's JWT. An app credential would carry the same entries via the same activation flow. "Direct access" is the sidecar model under a different name.

The broker's ceiling enforcement mechanism is already generic enough to support app tokens. Adding an app registration path (`POST /v1/apps`, `AppRecord` in SQLite, `app:scope:X` claims) is an incremental broker change when a concrete use case appears — for example, serverless functions that cannot maintain a persistent sidecar process. Until that use case is real, building it adds complexity with no operator benefit.

### Fix the admin secret distribution

The most urgent security improvement is not the sidecar model itself — it is the admin secret in every sidecar. The fix is a new limited-scope sidecar credential: instead of `AA_ADMIN_SECRET`, sidecars receive a pre-scoped credential with only the permissions they actually need (`sidecar:register-agents:*`). This eliminates the `admin:revoke:*` and `admin:audit:*` blast radius from all sidecar environments.

---

## Action Plan

### Pre-merge blockers (fix/sidecar-uds must not merge without these)

**1. Complete operator and developer documentation. HARD BLOCKER.**

The merge block established in MEMORY.md Session 14 is correct. The following must exist before merge:

- **Operator guide additions:** Step-by-step for deploying a sidecar for a new application, guidance on one-per-app vs. one-per-team vs. one-per-trust-boundary, ceiling configuration examples with real scope patterns
- **Developer guide:** How to connect an application to a sidecar, what the sidecar URL looks like (TCP vs. UDS), example `POST /v1/token` calls, what the agent_name and task_id fields mean
- **Architecture FAQ:** Why sidecars exist, what they provide vs. direct access, when to use each, the scope ceiling mechanism explained in plain language

**2. Document UDS as the production default; mark TCP as development-only.**

The quickstart at `getting-started-operator.md:15-33` demonstrates TCP throughout. The Docker Compose default (`docker-compose.yml`) uses TCP. Operators following the quickstart land in TCP mode without understanding the security implications.

Required changes:
- Quickstart must show UDS configuration as the primary path
- Docker Compose should include `AA_SOCKET_PATH` in the sidecar service, commented out with a note that it is the recommended production setting
- The TCP `WARN` log at `listener.go:44` should be supplemented with documentation that makes the security implication explicit: in TCP mode, any process on the host can request tokens

### Short-term (after fix/sidecar-uds merges, before Fix 6)

**3. Replace `AA_ADMIN_SECRET` on sidecars with a limited-scope sidecar credential. NOT YET IMPLEMENTED — known security gap.**

This fix requires a new broker endpoint. Here is why the change is not straightforward:

`POST /v1/admin/launch-tokens` currently requires `admin:launch-tokens:*` scope (`cmd/broker/main.go:181`). Sidecars call this endpoint during every new agent lazy-registration (`handler.go:183-189`). Therefore any sidecar credential that allows agent registration must carry `admin:launch-tokens:*` — which today means the full `AA_ADMIN_SECRET`. Simply issuing a "narrower" credential doesn't help unless the broker exposes a narrower endpoint.

Required broker change: add `POST /v1/sidecar/launch-tokens` — a new endpoint that creates a launch token scoped to the calling sidecar's ceiling only, gated behind `sidecar:manage:*` (already held by the sidecar bearer token), with no additional admin scope required. With this endpoint in place, sidecars use their existing sidecar bearer for all operations and `AA_ADMIN_SECRET` is needed only once at bootstrap and never stored afterward.

Until this is implemented: `AA_ADMIN_SECRET` remains in every sidecar process at runtime. A compromised sidecar process has full admin access. Operators must treat sidecar environments as having the same trust level as the broker itself.

**4. Document ephemeral agent registry restart behavior.**

The sidecar's agent registry is pure in-memory (`cmd/sidecar/registry.go`). A sidecar restart clears all registered agent entries. On the next `POST /v1/token` request per agent after restart, the sidecar runs the full 5-step lazy registration flow again before returning a token. This adds one-time latency per agent per sidecar restart.

This is intentional — the broker generates fresh Ed25519 signing keys on every startup, invalidating all pre-restart tokens regardless. Operator documentation must set this expectation explicitly: plan for registration latency spikes after sidecar restarts, especially for sidecars serving many agents.

### Preserve

- The sidecar binary and all current features (circuit breaker, cached token fallback, background renewal, UDS, mTLS client)
- The dual ceiling enforcement mechanism — both the sidecar pre-check and the broker authoritative check
- The sidecar as the recommended production path for complex deployments with strict isolation requirements

---

## Code References

| Claim | Evidence |
|-------|----------|
| Sidecar scope ceiling embedded in JWT at activation | `internal/admin/admin_svc.go:359-363` |
| Broker extracts ceiling from sidecar token (authoritative check) | `internal/handler/token_exchange_hdl.go:131-152` |
| Sidecar pre-checks scope locally (fast rejection) | `cmd/sidecar/handler.go:78` |
| Anti-spoof: sid derived from broker token, not client field | `internal/handler/token_exchange_hdl.go:167-169` |
| Admin secret loaded and held in every sidecar process | `cmd/sidecar/config.go:58` |
| Admin auth called on every new agent registration | `cmd/sidecar/handler.go:183` |
| Full admin scope granted to anyone with admin secret | `internal/admin/admin_svc.go:44-48` |
| TCP is the default; UDS requires explicit opt-in | `cmd/sidecar/config.go:88`, `cmd/sidecar/listener.go:39-44` |
| UDS socket permissions 0660 | `cmd/sidecar/listener.go:24` |
| SidecarRecord model (direct-access AppRecord would mirror this) | `internal/store/sql_store.go:73-79` |
| Circuit breaker (would be lost without sidecar) | `cmd/sidecar/circuitbreaker.go` |
| Cached token failsafe (would be lost without sidecar) | `cmd/sidecar/handler.go:109-125` |

---

## Summary

The sidecar model is architecturally sound but operationally incomplete. The implementation is correct. The admin secret exposure is a real and wider blast radius than the scope ceiling would suggest — a compromised sidecar is effectively a full admin compromise, not a bounded-scope compromise. The documentation gap is a hard merge blocker. Direct app access is feasible when a concrete use case appears; it is not needed today.

**Merge fix/sidecar-uds after documentation is complete. Not before.**

---

## Rejected Alternatives

### Direct broker access (client_id/client_secret)
- The broker's `token_exchange_hdl.go:131` explicitly requires `sidecar:scope:X` claims in the caller's JWT. App credentials don't have these — the broker rejects them with `ErrExchangeSidecarScopeMissing`
- Building it requires: new `/v1/apps` endpoint, new `AppRecord` SQLite table, modified exchange handler — significant broker changes
- No concrete use case exists that sidecars can't serve today
- "Direct access is the sidecar model under a different name" — an app credential would carry the same scope entries via the same activation flow

### Hybrid (both sidecar + direct access)
- Creates two authentication paths operators must choose between with no clear decision criterion
- Doubles maintenance burden — two code paths, two sets of docs, two audit event shapes
- The devil's advocate called this "the complexity of both models with the clean security guarantees of neither"

### Remove sidecars entirely
- Loses developer DX (one `POST /v1/token` call vs 10-step challenge-response)
- Loses resilience (circuit breaker, cached token fallback, background renewal)
- Loses UDS access control (`0660` permissions)
- No replacement scope siloing mechanism exists yet — would leave operators with nothing