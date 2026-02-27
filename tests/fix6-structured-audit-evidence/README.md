# Fix 6: Structured Audit Log Fields — Test Evidence

**Branch:** `fix/structured-audit`
**Date:** 2026-02-27
**Docker stack:** `./scripts/stack_up.sh` (broker + sidecar containers)
**How events were generated:** `go run ./cmd/smoketest` runs the full sidecar lifecycle (admin auth, launch token, agent registration, sidecar activation, token exchange, scope escalation denial)

---

## What Fix 6 Does

Before Fix 6, audit events had a free-text `detail` field. If you wanted to know whether an operation succeeded or failed, you had to parse that text. That's fragile and useless for automated compliance reporting.

Fix 6 adds structured fields to every audit event:

| Field | Type | Purpose |
|-------|------|---------|
| `outcome` | `"success"` or `"denied"` | Did the operation succeed or fail? |
| `resource` | string | What resource was being accessed (e.g., `/v1/admin/launch-tokens`) |
| `deleg_depth` | integer | How deep in the delegation chain (0 = direct token) |
| `deleg_chain_hash` | string | Hash of the delegation chain for traceability |
| `bytes_transferred` | integer | Data volume for metering (future use) |

These fields are also included in the tamper-evident hash chain, so modifying them after the fact breaks the chain.

---

## How to Read the Evidence

Each story has its own evidence file with:
1. **What was tested** — plain English description
2. **The command that was run** — so you can reproduce it
3. **The raw output** — the actual JSON response from the running Docker stack
4. **What to look for** — specific things that prove the story passes

---

## Stories Tested

| Story | File | Result |
|-------|------|--------|
| 1. Structured fields present | [story-1-structured-fields.md](story-1-structured-fields.md) | PASS |
| 2. Outcome filter works | [story-2-outcome-filter.md](story-2-outcome-filter.md) | PASS |
| 3. Hash chain covers new fields | [story-3-hash-chain-integrity.md](story-3-hash-chain-integrity.md) | PASS |

## Smoketest Output

The smoketest (`go run ./cmd/smoketest`) runs 12 steps that exercise the full lifecycle. All 12 passed. See [smoketest-output.txt](smoketest-output.txt).
