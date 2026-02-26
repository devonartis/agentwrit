# User Stories — Fix 3: Audience Validation

**Fix branch:** `fix/audience-validation`
**Priority:** P1 (Compliance)
**Date:** 2026-02-24
**Related plan:** `plans/implementation-plan.md` — Fix 3

---

## Story 1 — Tokens for the wrong audience are rejected

> As a **security engineer**, when `AA_AUDIENCE` is configured on the broker, I expect tokens issued for a different audience to be rejected, so that a token stolen from one environment cannot be replayed against another broker.

**Acceptance criteria:**
- Start broker with `AA_AUDIENCE=broker-production`
- Obtain a token that carries `aud: ["broker-staging"]` (or no `aud` claim at all)
- `POST /v1/token/validate` rejects the token with an audience mismatch error
- `POST /v1/token/renew` (Bearer auth) also rejects with audience mismatch

**Covered by:** `live_test.sh --fix3` → Story 1

---

## Story 2 — Backward compatible when AA_AUDIENCE is unset

> As an **operator**, when I have not configured `AA_AUDIENCE`, I expect the broker to accept all tokens regardless of their `aud` claim (or lack thereof), so existing deployments are not broken by this change.

**Acceptance criteria:**
- Start broker with `AA_AUDIENCE` unset (empty string)
- Register an agent and obtain a token (no `aud` claim present)
- `POST /v1/token/validate` accepts the token — audience check is skipped entirely
- Broker logs do NOT show audience validation errors

**Covered by:** `live_test.sh --fix3` → Story 2

---

## Story 3 — Correct audience tokens pass validation

> As a **3rd-party developer**, when my agent's token carries the correct audience claim matching the broker's `AA_AUDIENCE`, I expect all authenticated endpoints to work normally, so legitimate agents in correctly-configured environments have zero friction.

**Acceptance criteria:**
- Start broker with `AA_AUDIENCE=broker-production`
- Register an agent — the broker issues a token with `aud: ["broker-production"]`
- `POST /v1/token/validate` accepts the token
- `POST /v1/token/renew` succeeds and returns a renewed token with the same audience

**Covered by:** `live_test.sh --fix3` → Story 3

---

## Implementation Notes

Injection point is `ValMw.Wrap()` in `internal/authz/val_mw.go`. The configured audience
from `cfg.Audience` is passed into the middleware at construction time. When non-empty,
every token's `Aud` slice is checked for a matching string. When empty, the check is
skipped — not fail-closed. New env var `AA_AUDIENCE` added to `cfg.go` and
`docker-compose.yml`.
