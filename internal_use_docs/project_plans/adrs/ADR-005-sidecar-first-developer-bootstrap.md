# ADR-005: Sidecar-First Developer Bootstrap and Runtime Observability

## Status
Proposed

## Context

AgentAuth MVP currently works for core broker flows, but developer onboarding and runtime diagnostics still have friction:

1. App developers can end up coupled to admin/bootstrap details that should be platform-owned.
2. HTTP-level observability is inconsistent at runtime (developers need clearer request-level logs).
3. Error responses are standards-compliant (RFC 7807), but developer troubleshooting still depends too heavily on reading docs.
4. The team wants a path where Python/TypeScript developers can securely integrate quickly without handling privileged broker controls directly.

Constraints:

1. Keep MVP momentum; avoid over-engineering before production hardening.
2. Preserve auditable identity and authorization boundaries.
3. Minimize secrets in app code.

## Decision

Adopt a **sidecar-first developer access model** for local and early integration environments.

Decision details:

1. Developer apps do not use admin-secret workflows directly.
2. A local sidecar handles broker bootstrap/exchange and serves short-lived bearer tokens to the app.
3. CLI bootstrap remains a fallback for early adoption and troubleshooting.
4. Broker will add request-level structured logging contract (method, path, status, latency, request_id, caller identity when available).
5. Error contract remains RFC 7807 and is extended with stable machine-readable error code and concise hint.

## Options considered

### Option A: Local Sidecar (selected)

Pros:

1. Removes privileged bootstrap logic from app code.
2. Works across languages (Python, TypeScript, others).
3. Natural home for token refresh/retry behavior and local diagnostics.
4. Supports consistent integration UX for new developers.

Cons:

1. Adds another process/component to run.
2. Requires sidecar lifecycle tooling and documentation discipline.

### Option B: Central Token Proxy Service

Pros:

1. Centralized policy and token exchange control.
2. No local sidecar process management for developers.

Cons:

1. New shared service introduces availability and blast-radius concerns.
2. Additional operational complexity earlier than needed for MVP iteration.

### Option C: CLI-only Token Bootstrap

Pros:

1. Fastest implementation.
2. Minimal architecture change.

Cons:

1. More manual and error-prone for long-running flows.
2. Token lifecycle handling can drift into ad hoc scripts.
3. Weaker default developer ergonomics than sidecar model.

## Consequences

Positive:

1. Clear separation of platform credential handling from developer app logic.
2. Better developer onboarding path for secure integration.
3. Better runtime operability through standardized request logs and improved error diagnostics.
4. Easier path to consistent SDK helpers across Python/TypeScript.

Negative / Tradeoffs:

1. Sidecar component must be designed, versioned, tested, and documented.
2. Additional local runtime moving part may complicate initial troubleshooting.
3. Team must maintain compatibility between sidecar contract and broker API.

Follow-up requirements:

1. Define token exchange contract between sidecar and broker.
2. Define sidecar-to-app token interface (local HTTP or socket API).
3. Add request logging middleware in broker and request-id propagation.
4. Extend error schema (`error_code`, `hint`) and publish error catalog.
5. Add E2E tests for “new developer secure start” flow without admin-secret in app code.
6. ADR acceptance is gated by container deployment compliance in `/Users/divineartis/proj/agentAuth/plans/AgentAuth-MVP-Container-Deployment-Spec-v1.0.md`.

## Implementation notes (non-binding in this ADR)

1. Start with sidecar support for Python and TypeScript sample apps.
2. Keep CLI fallback while sidecar matures.
3. Keep observability endpoint exposure policy explicit per environment (local vs shared vs production).

## Supersession

This ADR may be superseded by a production IAM/attestation ADR once SSO or platform attestation becomes the default bootstrap model.
