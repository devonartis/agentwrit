# User Stories — Fix 6: Structured Audit Log Fields

**Fix branch:** `fix/structured-audit`
**Priority:** P2 (Compliance — Pattern v1.2 §5.2)
**Date:** 2026-02-24
**Related plan:** `plans/implementation-plan.md` — Fix 6

---

## Story 1 — Audit events include structured fields for compliance reporting

> As a **compliance auditor**, when I query the audit trail, I want each event to include structured fields (`resource`, `outcome`, `deleg_depth`, `bytes_transferred`) instead of a free-text `detail` string, so I can generate compliance reports without parsing unstructured text.

**Acceptance criteria:**
- Trigger a token exchange (or any audited operation)
- `GET /v1/audit/events` returns events with structured JSON fields:
  - `resource` — the resource being accessed
  - `outcome` — `"success"` or `"denied"`
  - `deleg_depth` — delegation chain depth (integer, 0 for direct tokens)
- Fields are present alongside the existing `detail` field (backward compatible)

**Covered by:** `live_test.sh --fix6` → Story 1

---

## Story 2 — Admin can filter audit events by outcome

> As an **admin**, when I need to investigate security incidents, I want to query `GET /v1/audit/events?outcome=denied` to see only denied operations, so I can quickly identify unauthorized access attempts without scrolling through thousands of success events.

**Acceptance criteria:**
- Perform one successful and one denied operation (e.g., valid token vs. revoked token)
- `GET /v1/audit/events?outcome=denied` returns only the denied event
- `GET /v1/audit/events?outcome=success` returns only the successful event
- Each event includes the structured `outcome` field

**Covered by:** `live_test.sh --fix6` → Story 2

---

## Story 3 — Structured fields are included in tamper-evident hash chain

> As a **security engineer**, when I verify the audit log integrity, I expect the new structured fields to be covered by the hash chain, so a tampered `outcome` or `resource` value breaks the chain and is detectable.

**Acceptance criteria:**
- Generate several audit events with the new structured fields
- `GET /v1/audit/events` returns events with valid `prev_hash` linking
- Manually verify that changing a structured field value (e.g., flipping `outcome`
  from `"denied"` to `"success"`) in the database would break the hash chain
- The hash input includes: `resource`, `outcome`, `deleg_depth`, `deleg_chain_hash`,
  `bytes_transferred` (in addition to existing fields)

**Covered by:** `live_test.sh --fix6` → Story 3
