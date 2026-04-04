# Unlogged Branches — app-launch-tokens and docs-overhaul

**Date:** 2026-04-03

Two branches exist that are fully merged into `develop` but have no formal Action/Status entries in FLOW.md like B0–B6 do.

---

## `fix/app-launch-tokens-endpoint`

- **Origin:** Cherry-pick of `393d376` from `agentauth-internal`
- **When:** 2026-03-30
- **What:** Split the launch-token endpoint into separate admin and app routes (`/v1/admin/launch-tokens` and `/v1/app/launch-tokens`), 4/4 acceptance tests PASS
- **Why it exists:** During B6 acceptance testing, user caught the agent building tests against the admin flow instead of the app flow. That revealed only ONE launch-token endpoint was being used by both admin and apps. The fix split the route so each role uses its own path.
- **Decision documented in:** MEMORY.md (lesson #2 about role confusion) and the FLOW.md entry about code comments needing to explain roles. TD-013 tech debt item also relates — admin creating launch tokens with no scope ceiling.
- **NOT in FLOW.md as its own decision.** No explicit "Decision: create app-launch-tokens branch" entry. Came out of B6 acceptance test work but done as a separate branch.
- **Test evidence:** `tests/app-launch-tokens/` — 4 stories (ALT-S1 through ALT-S4), all PASS VPS mode.

## `fix/docs-overhaul`

- **When:** 2026-03-29 to 2026-03-30
- **What:** Major documentation rewrite — removed all sidecar references from public docs, added real-world scenarios (fintech, bank delegation), Go-native scenario, pattern-to-code mapping for all 8 v1.3 components, SVG diagrams
- **Why it exists:** Part of the "Next work sequence" decision from 2026-03-30 in FLOW.md — step 2 was "Public documentation update."
- **Decision documented in:** FLOW.md under "Decision: Next work sequence (2026-03-30)" — item #2.
- **Partial completion:** FLOW.md status at bottom says "Public documentation update (TD-012, TD-014) — still pending" — docs-overhaul was a partial pass, not the full TD-012/TD-014 completion.

---

## The Gap

Neither branch has its own "Action:" / "Status:" entry in FLOW.md the way B0–B6 do. They were done and merged but never formally logged as decisions/actions.

**Why:** These emerged organically from B6 work and the post-migration cleanup, rather than being planned as named batches.

**Action needed:** Add retroactive FLOW.md entries so the decision trail is complete.
