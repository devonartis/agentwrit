# Demo2 Minimum Viable Demo Checklist (M14-M15)

Version: 1.1  
Date: 2026-02-09  
Status: Active

## Goal

Deliver a compelling "Gap vs Fix" demo quickly.

This checklist intentionally optimizes for demo speed and proof value over architecture polish.

## Rigor split policy (mandatory)

1. Go broker rigor (`M00-M10`) does NOT apply to Demo2 modules.
2. Demo2 modules are `M11-M15` and must run in Demo Velocity Mode.
3. For Demo2, block only on:
- `P0` security claim failure
- `P1` demo-flow breakage
4. Do NOT block Demo2 on `P2` polish/style/refactor unless operator runability is harmed.
5. Keep implementation explicit and minimal; avoid architecture churn.

## Demo velocity guardrails

1. Block only on `P0` or `P1` issues.
2. Treat `P2` issues as backlog unless they break the demo narrative.
3. Prefer simple, explicit code over abstractions.
4. Ship vertical slices end-to-end before refactors.
5. No new framework decisions unless required to unblock demo flow.

## Module M14 (Dashboard) - minimum deliverable

## Must-have outputs

1. A single dashboard page that shows:
- current mode (`insecure` or `secure`)
- workflow status (`idle`, `running`, `done`, `failed`)
- event stream/timeline
- attack summary table (`attack`, `result`, `reason`)
2. Controls:
- `Run Insecure Demo`
- `Run Secure Demo`
- `Reset`
3. Live updates via SSE (or equivalent push/poll fallback if SSE is unstable).
4. A clear visual comparison: insecure attacks succeed, secure attacks are blocked.

## Functional acceptance for M14

1. Clicking `Run Insecure Demo` produces a full run and event stream with at least one successful attack.
2. Clicking `Run Secure Demo` produces a full run and shows attack blocking outcomes.
3. `Reset` clears UI state and allows immediate rerun.
4. UI does not expose secrets or raw tokens.

## Non-goals for M14

1. No pixel-perfect design pass.
2. No deep component framework re-architecture.
3. No production-level dashboard RBAC.

## Module M15 (Final Assembly) - minimum deliverable

## Must-have outputs

1. One-command startup for full demo stack.
2. One command/script to run insecure flow and secure flow back-to-back.
3. A simple final E2E script that asserts:
- insecure run shows attack success
- secure run shows attack blocks
- dashboard receives and displays events
4. A concise operator runbook with exact commands and expected outputs.

## Functional acceptance for M15

1. `docker compose up` (or equivalent) brings up required services for demo.
2. Demo operator can run both modes without manual code edits.
3. Final script exits non-zero when expected mode outcomes are violated.
4. Demo can be run start-to-finish in under 5 minutes.

## Non-goals for M15

1. Production-grade resilience hardening.
2. Full post-MVP optimization and refactors.
3. Any enhancement that delays the core demo narrative.

## Required evidence per task

1. Task ID and scope.
2. Commit hash and changed files.
3. Brief note on insecure vs secure impact.
4. Short run output snippet proving behavior.
5. Updated docs/changelog pointer.

## Demo2 definition of done

1. Insecure mode demonstrates the security gap visibly.
2. Secure mode demonstrates the fix visibly.
3. Same workflow, same UI, different security outcome.
4. Operator can run the demo quickly and repeatably.
