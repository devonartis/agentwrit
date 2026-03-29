# P0-S1 — Graceful Shutdown on SIGTERM [ACCEPTANCE]

Who: The operator.

What: The operator stops the broker using docker compose stop, which sends
SIGTERM. The broker must log that it received the signal, finish work in
progress, close the database, and exit cleanly.

Why: Without graceful shutdown, in-flight requests get dropped and database
writes may be incomplete — risking data corruption.

How to run: Confirm broker is healthy, stop it, check logs for shutdown messages.

Expected: Logs contain shutdown/signal message and database close confirmation.

## Test Output

Step 1: Confirm healthy
{
  "status": "ok",
  "version": "2.0.0",
  "uptime": 10,
  "db_connected": true,
  "audit_events_count": 0
}

Step 2: Stop broker (sends SIGTERM)
 Container agentauth-core-broker-1 Stopping 
 Container agentauth-core-broker-1 Stopped 

Step 3: Check logs for shutdown messages
broker-1  | [AA:HTTP:OK] 2026-03-29T13:40:18Z | handler | request completed | method=GET, path=/v1/health, status=200, latency=34.375µs, request_id=8e010ed71b56d4fa
broker-1  | [AA:HTTP:OK] 2026-03-29T13:40:20Z | handler | request completed | method=GET, path=/v1/health, status=200, latency=25.167µs, request_id=141a27b988b0619f
broker-1  | [AA:HTTP:OK] 2026-03-29T13:40:22Z | handler | request completed | method=GET, path=/v1/health, status=200, latency=25.375µs, request_id=7c981486f07e083f
broker-1  | [AA:HTTP:OK] 2026-03-29T13:40:22Z | handler | request completed | method=GET, path=/v1/health, status=200, latency=44.292µs, request_id=719477d75ac465a6
broker-1  | [AA:BROKER:OK] 2026-03-29T13:40:22Z | shutdown | signal received | signal=terminated
broker-1  | 
broker-1  | Shutting down gracefully (signal: terminated)...
broker-1  | [AA:store:OK] 2026-03-29T13:40:22Z | sqlite | database closed
broker-1  | [AA:BROKER:OK] 2026-03-29T13:40:22Z | shutdown | database closed
broker-1  | [AA:BROKER:OK] 2026-03-29T13:40:22Z | shutdown | clean exit


## Verdict

PASS — Broker received SIGTERM ("signal received | signal=terminated"), logged "Shutting down gracefully", closed the database ("database closed"), and exited cleanly ("clean exit"). Full graceful shutdown sequence confirmed.
