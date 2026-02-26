# User Stories — Fix 2: Revocation Persistence

**Fix branch:** `fix/revocation-persistence`
**Priority:** P0 (Security)
**Date:** 2026-02-24
**Related plan:** `plans/implementation-plan.md` — Fix 2

---

## Story 1 — Revoked tokens stay revoked after broker restart

> As an **operator**, when I restart the broker after a token has been revoked, I expect the revocation to survive the restart so that a compromised agent cannot regain access simply because the broker was cycled.

**Acceptance criteria:**
- Register an agent and obtain a valid token
- Revoke the token via `POST /v1/revoke` (admin scope)
- `POST /v1/token/validate` returns rejection for the revoked token
- Restart the broker container (`docker compose restart broker`)
- After restart, `POST /v1/token/validate` still rejects the revoked token

**Covered by:** `live_test.sh --fix2` → Story 1

---

## Story 2 — Valid tokens still work after restart (no false positives)

> As a **3rd-party developer**, when the broker restarts, my agent's non-revoked tokens should continue to work normally, so that legitimate agents are not disrupted by routine maintenance.

**Acceptance criteria:**
- Register an agent and obtain a valid token (do NOT revoke it)
- Restart the broker container
- `POST /v1/token/validate` still accepts the non-revoked token
- Note: tokens fail signature verification after restart because signing keys are
  ephemeral — this story validates revocation-layer behavior specifically. The validate
  endpoint returns a clear "signature" error, not a "revoked" error.

**Covered by:** `live_test.sh --fix2` → Story 2

---

## Story 3 — Revocation entries persist in SQLite

> As an **admin**, when I query the database after revoking a token, I expect the revocation record to exist in SQLite, so I can audit which tokens were revoked and confirm persistence is working.

**Acceptance criteria:**
- Revoke a token via `POST /v1/revoke`
- Inspect the SQLite database (`agentauth.db`) — a `revocations` table exists
  with the revoked token's identifier
- Restart the broker — `LoadAllRevocations()` populates the in-memory revocation
  set from SQLite on startup (visible in broker startup logs)

**Covered by:** `live_test.sh --fix2` → Story 3

---

## Implementation Notes

Per the design document: signing keys are ephemeral (regenerated on each startup). After
a restart, all pre-restart tokens fail signature verification before the revocation check
runs. Pre-restart revocation entries in SQLite are therefore dead weight for signature-
verified tokens — but they remain correct for the revocation layer itself. The
`revocations` table has no `expires_at` column by design. Safe cleanup is deferred to a
future PR; the operator runbook should note that the table grows indefinitely until then.
