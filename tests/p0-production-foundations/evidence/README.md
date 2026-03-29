# P0: Production Foundations — Evidence

| Story | Name | Verdict | File |
|-------|------|---------|------|
| P0-K1 | Key File Created with Secure Permissions | **PASS** | story-K1-key-file-permissions.md |
| P0-K2 | Token Survives Broker Restart | **PASS** | story-K2-token-survives-restart.md |
| P0-K3 | Configurable Key Path | **PASS** | story-K3-configurable-key-path.md |
| P0-K4 | Token Renewal After Restart | **PASS** | story-K4-token-renewal-after-restart.md |
| P0-K5 | Corrupt Key File Fails Fast | **PASS** | story-K5-corrupt-key-fails-fast.md |
| P0-S1 | Graceful Shutdown on SIGTERM | **PASS** | story-S1-graceful-shutdown.md |
| P0-S2 | SQLite Closed on Shutdown | **PASS** | story-S2-sqlite-closed-on-shutdown.md |

## Open Issues

None.

## Test Environment

- **Stack:** `./scripts/stack_up.sh` (docker-compose, broker image built from `fix/p0-persistent-key`)
- **Broker version:** 2.0.0
- **Date:** 2026-03-29 (re-run on agentauth-core after B1 cherry-pick)
- **Admin secret:** `live-test-secret-32bytes-long-ok`
