# AgentAuth Developer Access and Observability Plan v1.0

## Document metadata
- Version: v1.0
- Status: Draft for execution planning
- Date: 2026-02-09
- Owner: AgentAuth team

## Why this plan exists
The current MVP works, but developer onboarding and operational clarity are still too manual:
1. Developers should not need direct admin-secret handling.
2. Observability should be visible by default in runtime logs, not only in docs.
3. Error handling must be easy to understand at runtime (machine code + human guidance), not only by reading documentation.

This plan defines the next architecture and execution path to fix those gaps.

## Scope
In scope:
1. Remove direct developer dependency on admin-level credentials.
2. Add production-style HTTP observability signals at runtime.
3. Improve error response usability and diagnostics.
4. Define an implementation path that can be handed to builder/reviewer agents.

Out of scope:
1. Full production IAM replacement.
2. Full enterprise SIEM design.
3. Complete performance optimization program.

## Design goals
1. Developer can start an app quickly without knowing broker internals.
2. Access to broker capabilities is still identity-bound and auditable.
3. Every request has runtime visibility (method, path, status, latency, request ID).
4. Errors are actionable in real time.
5. Security posture is improved without blocking MVP usability.

## Problem statements to solve
1. Direct credential burden: developers currently need launch/admin setup details.
2. Observability gap: no unified HTTP request logging middleware at broker boundary.
3. Metrics/audit access model is not aligned with least-privilege developer onboarding.
4. Error model is valid RFC 7807 but not optimized for day-1 troubleshooting.

## Architecture options (research + decision)

### Option A: Local sidecar (recommended default)
Summary:
1. Developer runs a local sidecar process with the app.
2. Sidecar holds bootstrap credential flow and token exchange logic.
3. App receives short-lived bearer tokens from sidecar on demand.

Benefits:
1. Removes admin secret from application code.
2. Supports language-agnostic integration (Python/TypeScript/others).
3. Natural place for token refresh, retry policy, and local request tracing.

Risks:
1. Adds one runtime component per developer/app.
2. Requires clear local lifecycle management.

### Option B: Central token-proxy service
Summary:
1. A central trusted proxy exchanges developer identity for broker tokens.
2. App calls proxy to get short-lived bearer credentials.

Benefits:
1. No local sidecar management.
2. Centralized policy and auditing.

Risks:
1. New central dependency and operational bottleneck.
2. Higher blast radius if proxy is compromised.

### Option C: CLI-only bootstrap helper
Summary:
1. CLI obtains short-lived tokens and exports env vars for the app.

Benefits:
1. Fastest to ship.
2. Minimal architectural change.

Risks:
1. Weaker automation and rotation model than sidecar/proxy.
2. Can regress into manual token handling.

## Proposed target direction
Use Option A (sidecar) as primary implementation and keep Option C as fallback for early adoption.

Reason:
1. Best balance of developer UX, security, and implementation speed.
2. Keeps admin-level control out of app code.
3. Provides a direct place to enforce consistent logging, retries, and token lifecycle.

## Access model changes
1. Add a developer bootstrap credential path that is not admin-secret distribution.
2. Bootstrap credential can be:
   - short-lived developer access token issued by admin workflow, or
   - SSO-backed exchange in a later phase.
3. Sidecar exchanges bootstrap credential for scoped short-lived bearer tokens.
4. Broker logs token exchange events with actor identity and purpose.
5. Developer apps never receive admin-secret material.

## Observability requirements
1. Add HTTP middleware for request logs:
   - request_id
   - method
   - path
   - status
   - latency_ms
   - caller identity (if available)
2. Ensure logs are emitted at startup and per request in a stable structured format.
3. Keep `/v1/metrics` access controlled by deployment policy:
   - private network only, or
   - token-protected observability endpoint in hardening phase.
4. Ensure audit and request logs can be correlated by request ID.

## Error handling requirements
1. Keep RFC 7807 as base contract.
2. Add stable machine-readable error code field (for example: `error_code`).
3. Include concise remediation hint field (for example: `hint`) where safe.
4. Publish one consolidated error catalog for developers:
   - code
   - HTTP status
   - meaning
   - immediate action
5. Ensure SDK/sidecar surfaces these fields directly to app logs.

## Work breakdown

### Phase 1: Decision and contracts
1. Write ADR selecting sidecar-first model and fallback strategy.
2. Define bootstrap credential contract and token exchange API.
3. Define request logging schema and error-code schema.

### Phase 2: Broker + sidecar foundation
1. Implement token exchange endpoint(s) for sidecar flow.
2. Implement HTTP request logging middleware at broker boundary.
3. Add request ID propagation.
4. Add error_code/hint to problem responses.

### Phase 3: Developer integration surface
1. Provide sidecar quickstart for Python and TypeScript apps.
2. Provide minimal SDK wrappers/helpers for token retrieval and refresh.
3. Publish "5-minute secure start" user guide path.

### Phase 4: Validation and release quality
1. Unit tests for new middleware and token-exchange paths.
2. Integration tests for sidecar-to-broker flow.
3. Live E2E tests proving developer app can start without admin-secret handling.
4. Docs review ensuring runtime troubleshooting is possible without deep source reading.

## Acceptance criteria
1. A new developer can run a sample Python or TypeScript app with broker security in <=10 minutes.
2. No developer app requires direct `AA_ADMIN_SECRET`.
3. Every broker request produces structured request log entries.
4. Error responses include stable code + actionable hint.
5. E2E tests validate full startup, token exchange, protected call, revocation, and troubleshooting evidence.

## Deliverables
1. ADR for access architecture decision.
2. Sidecar/proxy token exchange contract doc.
3. Updated USER_GUIDE and DEVELOPER_GUIDE onboarding path.
4. Error catalog and troubleshooting matrix.
5. E2E test evidence report.

## Open decisions to resolve in research
1. Bootstrap source: pre-issued short-lived token vs SSO-backed flow.
2. Sidecar packaging: standalone binary vs embedded library runner.
3. Observability endpoint protection model for local dev vs shared environments.
4. How strict to be on log redaction defaults for developer-mode traces.

