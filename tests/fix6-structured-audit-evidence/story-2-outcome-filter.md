# Story 2: Admin Can Filter Audit Events by Outcome

## What Was Tested

After generating a mix of successful and denied operations, we queried the audit API with `?outcome=denied` and verified that:
1. Only denied events come back (no success events leak through)
2. The total count matches the actual number of denied operations

This is the core filtering feature — an admin investigating a security incident can pull up just the denied operations without wading through hundreds of success events.

## How to Reproduce

```bash
# 1. Start the Docker stack
./scripts/stack_up.sh

# 2. Run the smoketest (generates both success and denied events)
go run ./cmd/smoketest http://127.0.0.1:8080 change-me-in-production

# 3. Query only denied events
export AACTL_BROKER_URL=http://127.0.0.1:8080
export AACTL_ADMIN_SECRET=change-me-in-production
go run ./cmd/aactl/ audit events --outcome denied --json

# Or via CLI table format:
go run ./cmd/aactl/ audit events --outcome denied
```

## What to Look For

- The response should contain ONLY events where `"outcome": "denied"`
- No event with `"outcome": "success"` should appear
- The `total` count should match the number of events in the array

## Raw Evidence

Command: `aactl audit events --outcome denied --json`

```json
{
  "events": [
    {
      "id": "evt-000010",
      "timestamp": "2026-02-27T02:53:05.831468679Z",
      "event_type": "sidecar_activation_failed",
      "detail": "activation token replay detected",
      "outcome": "denied",
      "hash": "78e56ac8d3395bd74af77f74bc3156ba1446c54874a5be6850600e4df31ccbf4",
      "prev_hash": "aeea704dd5b84fdeca789a80b6d72a5a08839a8922406fa7d3baf341abd1b5c5"
    },
    {
      "id": "evt-000012",
      "timestamp": "2026-02-27T02:53:05.836159137Z",
      "event_type": "sidecar_exchange_denied",
      "agent_id": "sidecar:2fad33cee405144f3071750684acc34b",
      "detail": "scope escalation denied: requested=[write:data:*] ceiling=[read:data:*]",
      "outcome": "denied",
      "hash": "0ea2e8fef99f1761a3b52f30de4c9a287f85ed24627689283081e9ca34f56bbe",
      "prev_hash": "468dc233b16d99f361ec4e0be739272e6a4dbb563d7de8e0f79df279d29e9d7c"
    }
  ],
  "total": 2,
  "offset": 0,
  "limit": 100
}
```

## Verification

- Total events returned: 2
- Events with `outcome=denied`: 2 (100%)
- Events with `outcome=success`: 0 (none leaked through)
- The two denied events are:
  - **sidecar_activation_failed** — a replayed activation token was correctly rejected
  - **sidecar_exchange_denied** — a sidecar tried to request `write:data:*` when its ceiling only allowed `read:data:*`
- Total events in unfiltered query: 13 (so the filter correctly excluded 11 success events)

**RESULT: PASS**
