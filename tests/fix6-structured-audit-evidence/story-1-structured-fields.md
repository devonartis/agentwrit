# Story 1: Audit Events Include Structured Fields

## What Was Tested

After running the full sidecar lifecycle (admin auth, launch token, agent registration, sidecar activation, token exchange, scope escalation denial), we queried all audit events and checked that every single event has the `outcome` field populated with either `"success"` or `"denied"`.

This proves that the structured fields are being written to the database and returned in the API response — not just in memory.

## How to Reproduce

```bash
# 1. Start the Docker stack
./scripts/stack_up.sh

# 2. Run the smoketest to generate events
go run ./cmd/smoketest http://127.0.0.1:8080 change-me-in-production

# 3. Query all audit events
export AACTL_BROKER_URL=http://127.0.0.1:8080
export AACTL_ADMIN_SECRET=change-me-in-production
go run ./cmd/aactl/ audit events --json
```

## What to Look For

Every event in the response has:
- `"outcome": "success"` or `"outcome": "denied"` — never empty, never missing
- Events that failed (activation replay, scope escalation) have `"outcome": "denied"`
- Events that succeeded (admin auth, token issued, exchange) have `"outcome": "success"`

## Raw Evidence

Command: `aactl audit events --json`

```json
{
  "events": [
    {
      "id": "evt-000001",
      "timestamp": "2026-02-27T02:53:00.593580676Z",
      "event_type": "admin_auth",
      "detail": "admin authenticated as admin",
      "outcome": "success",
      "hash": "e91c347b1a07661672e81a3e42af31dc72f9ce034513f658a7823a12bfccd913",
      "prev_hash": "0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "id": "evt-000002",
      "timestamp": "2026-02-27T02:53:00.599001926Z",
      "event_type": "sidecar_activation_issued",
      "agent_id": "admin",
      "detail": "issued sidecar activation token scopes=[sidecar:activate:read:data:* sidecar:activate:write:data:*] exp=2026-02-27T03:03:00Z",
      "outcome": "success",
      "hash": "fc11d8320e7f2d70d900d31b26d955216ca2b3d48a75775de9e21bf3bc4260a4",
      "prev_hash": "e91c347b1a07661672e81a3e42af31dc72f9ce034513f658a7823a12bfccd913"
    },
    {
      "id": "evt-000003",
      "timestamp": "2026-02-27T02:53:00.60372876Z",
      "event_type": "sidecar_activated",
      "agent_id": "sidecar:7c1f8f37490882dd248c37f27b86950e",
      "detail": "sidecar activated with scope_ceiling=[sidecar:manage:* sidecar:scope:read:data:* sidecar:scope:write:data:*]",
      "outcome": "success",
      "hash": "52cfe79ad6dd7724e55e1d16510acb7dd85c56c40c76fe2dd01a01d931b0b0af",
      "prev_hash": "fc11d8320e7f2d70d900d31b26d955216ca2b3d48a75775de9e21bf3bc4260a4"
    },
    {
      "id": "evt-000004",
      "timestamp": "2026-02-27T02:53:05.811960762Z",
      "event_type": "admin_auth",
      "detail": "admin authenticated as admin",
      "outcome": "success",
      "hash": "d232c51d481b7448a52e627ab7e37ac7784ae743157b2a9d375e4ce691b2ebff",
      "prev_hash": "52cfe79ad6dd7724e55e1d16510acb7dd85c56c40c76fe2dd01a01d931b0b0af"
    },
    {
      "id": "evt-000005",
      "timestamp": "2026-02-27T02:53:05.815156345Z",
      "event_type": "launch_token_issued",
      "detail": "launch token issued for agent=smoke-agent scope=[read:data:*] max_ttl=600 created_by=admin",
      "outcome": "success",
      "hash": "bc812d4b1f7ae706ac3c1b31f8ddc3b10708e5fa45b27f3745a9aa88628b36fa",
      "prev_hash": "d232c51d481b7448a52e627ab7e37ac7784ae743157b2a9d375e4ce691b2ebff"
    },
    {
      "id": "evt-000006",
      "timestamp": "2026-02-27T02:53:05.819085179Z",
      "event_type": "agent_registered",
      "agent_id": "spiffe://agentauth.local/agent/smoke-orch/smoke-task/8c38c1c223347069",
      "task_id": "smoke-task",
      "orch_id": "smoke-orch",
      "detail": "Agent registered with scope [read:data:*]",
      "outcome": "success",
      "hash": "2c223d6dddb095094a03e1a847449cd3478d3549ecf5ea9b0b8fdcac53ca2263",
      "prev_hash": "bc812d4b1f7ae706ac3c1b31f8ddc3b10708e5fa45b27f3745a9aa88628b36fa"
    },
    {
      "id": "evt-000007",
      "timestamp": "2026-02-27T02:53:05.82062572Z",
      "event_type": "token_issued",
      "agent_id": "spiffe://agentauth.local/agent/smoke-orch/smoke-task/8c38c1c223347069",
      "task_id": "smoke-task",
      "orch_id": "smoke-orch",
      "detail": "Token issued, jti=0363645213ff97a8e12a95b6dbb0221f, ttl=600",
      "outcome": "success",
      "hash": "9cc30dfca13f65d87300b3538f904a2336184f9d0851a5b8250efb7ddd6a1b42",
      "prev_hash": "2c223d6dddb095094a03e1a847449cd3478d3549ecf5ea9b0b8fdcac53ca2263"
    },
    {
      "id": "evt-000008",
      "timestamp": "2026-02-27T02:53:05.82459872Z",
      "event_type": "sidecar_activation_issued",
      "agent_id": "admin",
      "detail": "issued sidecar activation token scopes=[sidecar:activate:read:data:*] exp=2026-02-27T02:55:05Z",
      "outcome": "success",
      "hash": "91dc48231c5fc1bac51b5091fc20526b3318db645f130718022c0eb942ba8d8e",
      "prev_hash": "9cc30dfca13f65d87300b3538f904a2336184f9d0851a5b8250efb7ddd6a1b42"
    },
    {
      "id": "evt-000009",
      "timestamp": "2026-02-27T02:53:05.829085887Z",
      "event_type": "sidecar_activated",
      "agent_id": "sidecar:2fad33cee405144f3071750684acc34b",
      "detail": "sidecar activated with scope_ceiling=[sidecar:manage:* sidecar:scope:read:data:*]",
      "outcome": "success",
      "hash": "aeea704dd5b84fdeca789a80b6d72a5a08839a8922406fa7d3baf341abd1b5c5",
      "prev_hash": "91dc48231c5fc1bac51b5091fc20526b3318db645f130718022c0eb942ba8d8e"
    },
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
      "id": "evt-000011",
      "timestamp": "2026-02-27T02:53:05.833656095Z",
      "event_type": "sidecar_exchange_success",
      "agent_id": "spiffe://agentauth.local/agent/smoke-orch/smoke-task/8c38c1c223347069",
      "task_id": "smoke-task",
      "orch_id": "smoke-orch",
      "detail": "sidecar_id=2fad33cee405144f3071750684acc34b scope=[read:data:*] ttl=300",
      "outcome": "success",
      "hash": "468dc233b16d99f361ec4e0be739272e6a4dbb563d7de8e0f79df279d29e9d7c",
      "prev_hash": "78e56ac8d3395bd74af77f74bc3156ba1446c54874a5be6850600e4df31ccbf4"
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
    },
    {
      "id": "evt-000013",
      "timestamp": "2026-02-27T02:53:11.082467042Z",
      "event_type": "admin_auth",
      "detail": "admin authenticated as admin",
      "outcome": "success",
      "hash": "15ad63ceeb3b6f1119a2a5698cb7a7cb75286e8af8e06953c2b303c76e77926f",
      "prev_hash": "0ea2e8fef99f1761a3b52f30de4c9a287f85ed24627689283081e9ca34f56bbe"
    }
  ],
  "total": 13,
  "offset": 0,
  "limit": 100
}
```

## Verification

- Total events: 13
- Events with `outcome` field: 13 (100%)
- Success events: 11 (admin_auth, sidecar_activation_issued, sidecar_activated, launch_token_issued, agent_registered, token_issued, sidecar_exchange_success)
- Denied events: 2 (sidecar_activation_failed, sidecar_exchange_denied)

**RESULT: PASS**
