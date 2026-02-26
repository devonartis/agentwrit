# Post-Merge Roadmap: fix/sidecar-uds

**Date:** 2026-02-25
**Context:** After merging Fix 5 (sidecar UDS) and the sidecar architecture decision (ADR-002) to develop.

---

## Priority 1: Documentation Deep Dive

The architecture decision identified documentation as the hard merge blocker. These docs need to be written or updated so operators and developers can actually use what's been built.

### TODO: Operator Guide Additions (`docs/getting-started-operator.md`)
- [ ] Step-by-step: deploying a sidecar for a new application
- [ ] Guidance: one-per-app vs one-per-team vs one-per-trust-boundary (from ADR-002 Q6)
- [ ] Scope ceiling configuration examples with real scope patterns
- [ ] UDS as production default — update quickstart to show UDS first, TCP as dev-only
- [ ] Docker Compose: add `AA_SOCKET_PATH` to sidecar service (commented, with production note)
- [ ] Document ephemeral agent registry restart behavior (KI-004)
- [ ] Reference KNOWN-ISSUES.md for admin secret blast radius (KI-001)

### TODO: Developer Guide (`docs/getting-started-developer.md`)
- [ ] How to connect an application to a sidecar (TCP and UDS)
- [ ] Example `POST /v1/token` calls with agent_name, scope, ttl
- [ ] What agent_name and task_id mean — naming conventions
- [ ] Error handling: what happens when scope exceeds ceiling
- [ ] What happens on sidecar restart (lazy re-registration latency)

### TODO: Architecture FAQ (`docs/architecture.md` or new `docs/architecture-faq.md`)
- [ ] Why sidecars exist — the 5 concrete benefits (from ADR-002 Q3)
- [ ] What sidecars provide vs direct broker access
- [ ] Scope ceiling mechanism explained in plain language
- [ ] Trust boundary model — when to share sidecars, when not to
- [ ] Known security properties: real vs theater (from ADR-002)

---

## Priority 2: Fix 6 — Structured Audit Fields

Last compliance fix remaining. User stories already at `tests/fix6-structured-audit-user-stories.md`.

---

## Priority 3: Admin Secret Narrowing (KI-001 Fix)

From ADR-002 Action Plan item 3:
- [ ] Design `POST /v1/sidecar/launch-tokens` endpoint
- [ ] Gate behind `sidecar:manage:*` scope (already in sidecar bearer)
- [ ] Sidecars use bearer for all ops; admin secret only at bootstrap
- [ ] Per-sidecar `client_id` values (fixes KI-003 audit trail)

---

## Priority 4: SDK Development

Build SDKs so operators and developers can use AgentAuth programmatically without raw HTTP.

### Python SDK
- [ ] Package: `agentauth` (PyPI)
- [ ] Client for sidecar API (`POST /v1/token`, token renewal)
- [ ] Admin client for broker API (launch tokens, revocation, audit, sidecar management)
- [ ] `aactl`-equivalent CLI commands as Python library functions
- [ ] Async support (httpx or aiohttp)
- [ ] Type hints throughout

### TypeScript SDK
- [ ] Package: `@agentauth/sdk` (npm)
- [ ] Client for sidecar API
- [ ] Admin client for broker API
- [ ] Full TypeScript types for all request/response shapes
- [ ] Works in Node.js and edge runtimes (Bun, Deno)

### SDK Design Principles
- SDKs wrap the HTTP API — they don't replace it
- Every `aactl` command should have an SDK equivalent
- SDKs handle token caching/renewal (same pattern as sidecar, but in-process)
- SDKs should work with both TCP and UDS sidecar endpoints

---

## Priority 5: Merge develop to main (Release)

After documentation + Fix 6 + admin secret fix are all on develop:
- [ ] Run full gate check (`gates.sh task`)
- [ ] Docker live test of entire system
- [ ] Merge develop to main
- [ ] Tag release
