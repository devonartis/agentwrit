# AgentAuth Sidecar Phase 4-5 Execution Spec v1.0

## Document metadata
- Version: v1.0
- Status: Approved for execution
- Date: 2026-02-10
- Owner: AgentAuth team
- Applies to branch: `codex/sidecar-phase3-activation` (until merged)

## Purpose
Define exactly what must be completed for Sidecar bootstrap **Phase 4** and **Phase 5**, including code scope, docs scope, testing, gates, and merge-readiness criteria.

## Source-of-truth precedence
1. `/Users/divineartis/proj/agentAuth/plans/AgentAuth-Additional-Requirements-CodingAgent-v1.0.md`
2. `/Users/divineartis/proj/agentAuth/plans/AgentAuth-Technical-Spec-v2.0.md`
3. `/Users/divineartis/proj/agentAuth/plans/AgentAuth-MVP-Requirements-v1.0.md`
4. `/Users/divineartis/proj/agentAuth/plans/adrs/ADR-005-sidecar-first-developer-bootstrap.md`

## Scope
In scope:
1. Phase 4: Sidecar token exchange and scope attenuation completion.
2. Phase 5: Integration and observability completion for sidecar flows.
3. Documentation synchronization for operator, developer, and API usage.
4. Live E2E validation against a running backend (broker up before test).

Out of scope:
1. Performance optimization or load benchmarking.
2. Full production IAM/SSO replacement.
3. Multi-tenant persistent sidecar identity registry.

## Current baseline assumptions
1. Phase 1-3 sidecar work exists on current branch (activation + single-use replay protection + initial token exchange scaffolding).
2. Broker and sidecar containers can run via `docker compose`.
3. HTTP request logging middleware is present and emits request-completed logs when traffic exists.

## Phase 4 requirements (Token Exchange and Scoping)

### P4-R1: Sidecar token exchange contract
- Endpoint behavior for `POST /v1/token/exchange` is finalized for sidecar callers.
- Requires Bearer token with sidecar management scope.
- Returns short-lived token for agent identity with broker-issued claims.

### P4-R2: Scope attenuation enforcement
- Requested agent scopes must be subset of sidecar authority scope.
- On scope escalation attempt, return `403` with stable error code `scope_escalation_denied`.

### P4-R3: Sidecar lineage claim injection
- Broker sets lineage claim (`sid`/`sidecar_id` based on current claim model) on issued token.
- Client-supplied sidecar lineage fields are ignored/overwritten by broker.

### P4-R4: Revocation-aware behavior
- If sidecar authority token is revoked/invalid, exchange fails with appropriate auth error.

## Phase 5 requirements (Integration and Observability)

### P5-R1: Route and middleware integration
- Main route wiring reflects sidecar flow endpoints and required scope checks.
- Request ID propagation and request logging are active for sidecar-related endpoints.

### P5-R2: Audit coverage
- Sidecar flow events are present in audit trail with enough detail for reconstruction.
- Minimum event coverage:
  1. activation success/failure
  2. exchange success/failure
  3. security denial events (replay/escalation)

### P5-R3: Runtime observability evidence
- Demonstrate runtime HTTP log lines for sidecar flow requests.
- Demonstrate metrics endpoint availability during runtime.

### P5-R4: Docs and API sync
Required docs must reflect actual behavior:
1. `/Users/divineartis/proj/agentAuth/README.md`
2. `/Users/divineartis/proj/agentAuth/CHANGELOG.md`
3. `/Users/divineartis/proj/agentAuth/docs/USER_GUIDE.md`
4. `/Users/divineartis/proj/agentAuth/docs/DEVELOPER_GUIDE.md`
5. `/Users/divineartis/proj/agentAuth/docs/API_REFERENCE.md`
6. `/Users/divineartis/proj/agentAuth/docs/api/openapi.yaml`
7. Module-specific developer docs for sidecar flow.

## Task breakdown

### Phase 4 tasks
1. `P4-T01` Finalize token exchange request/response validation and error mapping.
2. `P4-T02` Enforce strict scope attenuation and escalation denial semantics.
3. `P4-T03` Finalize lineage claim injection rules and anti-spoof behavior.
4. `P4-T04` Add/complete unit + integration tests for P4-R1..R4.

### Phase 5 tasks
1. `P5-T01` Finalize route wiring + middleware ordering for sidecar endpoints.
2. `P5-T02` Finalize audit event recording and denial-path coverage.
3. `P5-T03` Finalize live-test strategy:
   - default external mode (backend already running)
   - optional docker mode for broker+sidecar stack
4. `P5-T04` Update docs and OpenAPI to exact runtime behavior.
5. `P5-T05` Produce final evidence package for review agents.

## Testing requirements

### Unit tests
1. Exchange success path.
2. Scope escalation denied path.
3. Missing/invalid sidecar token path.
4. Lineage claim injection and anti-spoof behavior.

### Integration tests
1. Activation -> exchange -> protected call chain.
2. Replay-denial and escalation-denial scenarios.
3. Revocation impact on sidecar authority.

### Live E2E tests
1. Broker is running **before** test starts.
2. Test hits health, metrics, challenge, and sidecar flow endpoints.
3. Captures evidence of:
   - request logs
   - expected denial behavior
   - final pass/fail summary

## Gate cadence (mandatory)
1. After each task: `./scripts/gates.sh task`
2. After Phase 4 complete: `./scripts/gates.sh module`
3. After Phase 5 complete: `./scripts/gates.sh module`
4. Failures block progression.

## Review protocol and merge criteria
1. Devil's advocate + spec + security + codex review all required.
2. Zero unresolved P0/P1 before merge.
3. Required review output:
   - executive risk summary
   - findings + fixes matrix
   - no-issue confirmations
   - final verdict: `ready | ready-with-edits | not-ready`

## Completion criteria
Phase 4/5 is complete only when all are true:
1. P4 and P5 requirements are implemented and tested.
2. Docs listed in P5-R4 are synchronized with runtime behavior.
3. Live E2E evidence is attached and reproducible.
4. Gate outputs are green and attached.
5. Review verdict is `ready` or `ready-with-edits` with no unresolved P0/P1.

## Handoff package format (required)
1. Task IDs completed.
2. Branch and commit hashes.
3. Diff summary by file.
4. Gate outputs.
5. Unit/integration/live outputs.
6. Doc updates made.
7. Deferred items with rationale.
