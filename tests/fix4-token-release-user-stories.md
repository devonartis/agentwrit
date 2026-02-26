# User Stories — Fix 4: Token Release (Task-Completion Signal)

**Fix branch:** `fix/token-release`
**Priority:** P1 (Compliance — Pattern v1.2 §4.4)
**Date:** 2026-02-24
**Related plan:** `plans/implementation-plan.md` — Fix 4

---

## Story 1 — Agent releases token after task completion

> As a **3rd-party developer**, when my agent finishes its task, I want to call a release endpoint to explicitly surrender the token, so the token cannot be misused after the task is done and I follow the principle of least privilege.

**Acceptance criteria:**
- Register an agent and obtain a valid token
- Call `POST /v1/token/release` with Bearer auth (the token being released)
- Response is `204 No Content` (or `200 OK` with confirmation body)
- Subsequent `POST /v1/token/validate` rejects the released token
- Subsequent `POST /v1/token/renew` rejects the released token

**Covered by:** `live_test.sh --fix4` → Story 1

---

## Story 2 — Token release appears in audit trail

> As an **admin/auditor**, when an agent releases a token, I expect a `token_released` event in the audit trail, so I can verify that agents are properly cleaning up after task completion and track the exact time of release.

**Acceptance criteria:**
- Agent registers and obtains a token
- Agent calls `POST /v1/token/release`
- `GET /v1/audit/events?event_type=token_released` returns an event with:
  - `agent_id` matching the releasing agent
  - `event_type` of `token_released`
  - timestamp of the release

**Covered by:** `live_test.sh --fix4` → Story 2

---

## Story 3 — Double-release is idempotent

> As an **operator**, when a client calls the release endpoint twice for the same token (network retry, bug in agent code), I expect the second call to succeed silently (idempotent) rather than returning an error, so that retry-safe clients don't need special error handling.

**Acceptance criteria:**
- Agent registers and obtains a token
- First `POST /v1/token/release` returns success
- Second `POST /v1/token/release` with the same token also returns success (not 4xx)
- Only one `token_released` audit event is recorded (or two are recorded but both are
  valid — implementation can choose, but the endpoint must not error)

**Covered by:** `live_test.sh --fix4` → Story 3

---

## Implementation Notes

Token release is effectively a self-revocation — the agent revokes its own token. This
differs from admin revocation (`POST /v1/revoke`) which requires `admin:revoke:*` scope.
The release endpoint requires only valid Bearer auth — the agent can only release its own
token. Under the hood, this adds the token's JTI to the revocation set (and, after Fix 2,
persists it to SQLite). A new audit event type `token_released` distinguishes voluntary
release from admin revocation.
