# ADR — Phase 1A Tech Debt

**Date:** 2026-03-03
**Status:** Open
**Context:** Phase 1A live test (Session 23) identified issues that need resolution before or during Phase 1B.

---

## 1. `app_rate_limited` audit event not emitted

**Problem:** The rate limiter middleware returns 429 before the handler runs, so the audit trail never records rate-limit events. Story 9 requires `app_rate_limited` in the audit trail.

**Fix required:** Emit `app_rate_limited` audit event from the rate limiter middleware itself, or from a wrapper that intercepts the 429 response.

**Branch:** `fix/phase-1a-rate-limit-audit` (to be created)

**Priority:** P0 — blocks Phase 1A completion.

---

## 2. `sk_live_` prefix on client_secret — removed from criteria

**Decision:** The acceptance criteria was updated to "random 64-char hex string" without `sk_live_` prefix. The prefix adds identifiability (you can tell it's an AgentAuth secret at a glance) but also makes secrets easier to find in logs/code if leaked.

**Current state:** No prefix. If added later, it's a non-breaking change (just prepend to the generation function). This can be revisited when the SDK (Phase 3) defines the developer experience more concretely.

---

## 3. No operator onboarding flow

**Problem:** There is no `aactl init` or `aactl configure` command. The operator must know to:
1. Generate a secret (`openssl rand -hex 32`)
2. Set `AA_ADMIN_SECRET` on the broker
3. Set `AACTL_BROKER_URL` and `AACTL_ADMIN_SECRET` in their shell

This is undocumented. A first-time operator has no guidance.

**Future feature:** On first boot, the broker should generate the admin secret and display it. This was identified during Session 23 and deferred.

**Priority:** P2 — not blocking, but a real operator experience gap.

---

## 4. Sidecar removed — no defined use case

**Decision:** The sidecar was removed from `docker-compose.yml` and `stack_up.sh`. The PRD says it's "optional" but never defines optional for what. No Phase 1A story uses it. No future phase currently requires it.

**Action:** Sidecar can be re-added when a concrete use case is documented in the PRD. The code still exists in `cmd/sidecar/` — only the infrastructure was removed.

---

## 5. Docker compose must build fresh before live tests

**Problem:** During testing, stale images can mask bugs. The live test process must ensure `docker compose build --no-cache` runs before any test.

**Current state:** `stack_up.sh` already uses `--no-cache`. This ADR documents the requirement so future test scripts don't skip it.

---

## 6. Phase 1A regression stories for Phase 1B

**Carry forward:** The following Phase 1A stories should be re-run as regression tests during Phase 1B acceptance testing:
- Story 1 (register app) — verify registration still works after launch token changes
- Story 6 (developer auth) — verify app auth still issues correct JWT
- Story 5 (deregister) — verify deregistration still blocks auth
- Story 11 (regression) — always run
