# Story R3 — Admin auth and audit flows unchanged (Phase 1a regression)

## Purpose

Phase 1b changed the launch token endpoint to accept app JWTs alongside admin JWTs.
This story verifies that the core admin workflow — authenticating with the shared secret
and querying the audit trail — still works exactly as it did before Phase 1b.

If this breaks, operators lose visibility into the system.

## Preconditions

- Broker running in Docker (`./scripts/stack_up.sh`)
- `aactl` built to `./bin/aactl`
- Environment sourced: `source tests/phase-1b/env.sh`
  - `AACTL_BROKER_URL=http://127.0.0.1:8080`
  - `AACTL_ADMIN_SECRET=change-me-in-production`

## Steps

### Step 1: Authenticate as admin

**What we are testing:** The operator authenticates using the shared admin secret. This
is the same flow that existed before Phase 1b — we need to confirm it still returns a
valid admin JWT.

**Command:**
```bash
curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"secret": "change-me-in-production"}'
```

**What to look for:**
- HTTP 200 response
- Response contains `access_token`, `expires_in`, and `token_type: Bearer`
- The token is a valid JWT (three dot-separated base64 segments)

**Expected:** 200 with admin JWT. Same behavior as Phase 1a.

**Actual:**

> (filled after execution)

### Step 2: Query audit events via aactl

**What we are testing:** The operator uses `aactl audit events` to list all audit events.
This command authenticates as admin under the hood and calls `GET /v1/audit/events`.
We should see events from the app registration and authentication done in earlier stories.

**Command:**
```bash
aactl audit events --json
```

**What to look for:**
- Command succeeds (exit 0)
- Returns a JSON array of audit events
- Events include types like `app_registered`, `app_authenticated` from earlier test setup
- Each event has: `id`, `event_type`, `outcome`, `event_hash`, `timestamp`

**Expected:** Non-empty event list with sequential IDs. Events from earlier stories visible.

**Actual:**

> (filled after execution)

### Step 3: Verify audit hash chain integrity

**What we are testing:** The audit trail uses a hash chain — each event's hash is computed
from the previous event's hash plus the current event data. If the chain is intact,
the IDs are sequential and every event has a non-empty hash.

This is a tamper-evidence mechanism. If someone deletes or modifies an event, the chain
breaks. We need to confirm Phase 1b didn't break the chain.

**What to look for (from Step 2 output):**
- Event IDs are strictly sequential (1, 2, 3, ...)
- Every event has a non-empty `event_hash`
- No gaps in the ID sequence

**Expected:** All IDs sequential, all hashes present. Chain intact.

**Actual:**

> (filled after execution)

## Acceptance Criteria

| # | Criterion | Expected | Actual | Result |
|---|-----------|----------|--------|--------|
| 1 | Admin auth (`POST /v1/admin/auth`) returns 200 | 200 | | |
| 2 | `aactl audit events` returns events | Non-empty list | | |
| 3 | Audit hash chain is intact | Sequential IDs, all hashed | | |

## Verdict

> (filled after execution)
