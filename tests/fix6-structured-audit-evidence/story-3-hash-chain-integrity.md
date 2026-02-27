# Story 3: Hash Chain Covers Structured Fields

## What Was Tested

The audit log uses a hash chain for tamper evidence — each event's `hash` is computed from its own data plus the previous event's hash (`prev_hash`). If anyone modifies an event after the fact (e.g., changing `outcome` from `"denied"` to `"success"` to cover tracks), the chain breaks and the tampering is detectable.

Fix 6 added structured fields (`outcome`, `resource`, `deleg_depth`, etc.) to the hash computation. This test verifies that:
1. Every event's `prev_hash` matches the previous event's `hash` (chain is unbroken)
2. All hashes are non-empty (no events skipped hashing)

## How to Reproduce

```bash
# 1. Start the Docker stack
./scripts/stack_up.sh

# 2. Run the smoketest to generate events
go run ./cmd/smoketest http://127.0.0.1:8080 change-me-in-production

# 3. Query all events and check the chain
export AACTL_BROKER_URL=http://127.0.0.1:8080
export AACTL_ADMIN_SECRET=change-me-in-production
go run ./cmd/aactl/ audit events --json
```

Then verify:
- Event 1's `prev_hash` is all zeros (genesis event)
- Event 2's `prev_hash` equals Event 1's `hash`
- Event 3's `prev_hash` equals Event 2's `hash`
- ...and so on for every event

## What to Look For

Look at the `hash` and `prev_hash` fields on consecutive events. For the chain to be intact, each event's `prev_hash` must exactly match the previous event's `hash`. If even one character is different, the chain is broken — meaning either the events were reordered, an event was inserted/deleted, or a field was modified after hashing.

## Raw Evidence — Chain Verification

Here is the chain link-by-link, extracted from the full event list:

```
Event evt-000001 (admin_auth, outcome=success)
  hash:      e91c347b1a07661672e81a3e42af31dc72f9ce034513f658a7823a12bfccd913
  prev_hash: 0000000000000000000000000000000000000000000000000000000000000000  ← genesis

Event evt-000002 (sidecar_activation_issued, outcome=success)
  hash:      fc11d8320e7f2d70d900d31b26d955216ca2b3d48a75775de9e21bf3bc4260a4
  prev_hash: e91c347b1a07661672e81a3e42af31dc72f9ce034513f658a7823a12bfccd913  ← matches evt-000001 hash ✓

Event evt-000003 (sidecar_activated, outcome=success)
  hash:      52cfe79ad6dd7724e55e1d16510acb7dd85c56c40c76fe2dd01a01d931b0b0af
  prev_hash: fc11d8320e7f2d70d900d31b26d955216ca2b3d48a75775de9e21bf3bc4260a4  ← matches evt-000002 hash ✓

Event evt-000004 (admin_auth, outcome=success)
  hash:      d232c51d481b7448a52e627ab7e37ac7784ae743157b2a9d375e4ce691b2ebff
  prev_hash: 52cfe79ad6dd7724e55e1d16510acb7dd85c56c40c76fe2dd01a01d931b0b0af  ← matches evt-000003 hash ✓

Event evt-000005 (launch_token_issued, outcome=success)
  hash:      bc812d4b1f7ae706ac3c1b31f8ddc3b10708e5fa45b27f3745a9aa88628b36fa
  prev_hash: d232c51d481b7448a52e627ab7e37ac7784ae743157b2a9d375e4ce691b2ebff  ← matches evt-000004 hash ✓

Event evt-000006 (agent_registered, outcome=success)
  hash:      2c223d6dddb095094a03e1a847449cd3478d3549ecf5ea9b0b8fdcac53ca2263
  prev_hash: bc812d4b1f7ae706ac3c1b31f8ddc3b10708e5fa45b27f3745a9aa88628b36fa  ← matches evt-000005 hash ✓

Event evt-000007 (token_issued, outcome=success)
  hash:      9cc30dfca13f65d87300b3538f904a2336184f9d0851a5b8250efb7ddd6a1b42
  prev_hash: 2c223d6dddb095094a03e1a847449cd3478d3549ecf5ea9b0b8fdcac53ca2263  ← matches evt-000006 hash ✓

Event evt-000008 (sidecar_activation_issued, outcome=success)
  hash:      91dc48231c5fc1bac51b5091fc20526b3318db645f130718022c0eb942ba8d8e
  prev_hash: 9cc30dfca13f65d87300b3538f904a2336184f9d0851a5b8250efb7ddd6a1b42  ← matches evt-000007 hash ✓

Event evt-000009 (sidecar_activated, outcome=success)
  hash:      aeea704dd5b84fdeca789a80b6d72a5a08839a8922406fa7d3baf341abd1b5c5
  prev_hash: 91dc48231c5fc1bac51b5091fc20526b3318db645f130718022c0eb942ba8d8e  ← matches evt-000008 hash ✓

Event evt-000010 (sidecar_activation_failed, outcome=denied)
  hash:      78e56ac8d3395bd74af77f74bc3156ba1446c54874a5be6850600e4df31ccbf4
  prev_hash: aeea704dd5b84fdeca789a80b6d72a5a08839a8922406fa7d3baf341abd1b5c5  ← matches evt-000009 hash ✓

Event evt-000011 (sidecar_exchange_success, outcome=success)
  hash:      468dc233b16d99f361ec4e0be739272e6a4dbb563d7de8e0f79df279d29e9d7c
  prev_hash: 78e56ac8d3395bd74af77f74bc3156ba1446c54874a5be6850600e4df31ccbf4  ← matches evt-000010 hash ✓

Event evt-000012 (sidecar_exchange_denied, outcome=denied)
  hash:      0ea2e8fef99f1761a3b52f30de4c9a287f85ed24627689283081e9ca34f56bbe
  prev_hash: 468dc233b16d99f361ec4e0be739272e6a4dbb563d7de8e0f79df279d29e9d7c  ← matches evt-000011 hash ✓

Event evt-000013 (admin_auth, outcome=success)
  hash:      15ad63ceeb3b6f1119a2a5698cb7a7cb75286e8af8e06953c2b303c76e77926f
  prev_hash: 0ea2e8fef99f1761a3b52f30de4c9a287f85ed24627689283081e9ca34f56bbe  ← matches evt-000012 hash ✓
```

## Verification

- Total events: 13
- Chain links verified: 12 (event 1 is genesis with all-zero prev_hash)
- Broken links: 0
- Empty hashes: 0

The hash input format includes all structured fields:
```
prev_hash|id|timestamp|event_type|agent_id|task_id|orch_id|detail|resource|outcome|deleg_depth|deleg_chain_hash|bytes_transferred
```

If someone changed `outcome` from `"denied"` to `"success"` on evt-000010, its hash would change, which would break the link from evt-000011 (whose `prev_hash` still points to the original hash).

**RESULT: PASS**
