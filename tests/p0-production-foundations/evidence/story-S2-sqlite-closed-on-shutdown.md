# P0-S2 — SQLite Closed on Shutdown [ACCEPTANCE]

Who: The operator.

What: The operator wants to be sure that when the broker shuts down, the
SQLite database is properly closed. An improperly closed database can
leave WAL journals behind or corrupt data.

Why: If SQLite isn't properly closed, WAL journals accumulate, data may
be corrupted, and the next startup may need recovery — adding latency
and risk.

How to run: Generate some DB activity (admin auth writes audit events),
stop broker, check logs for database close confirmation.

Expected: Logs contain "database closed" and no SQLite error messages.

## Test Output

Step 1: Generate DB activity (admin auth)
"Bearer"

Step 2: Stop broker
 Container agentauth-core-broker-1 Stopping 
 Container agentauth-core-broker-1 Stopped 

Step 3: Check logs for database close
broker-1  | [AA:BROKER:OK] 2026-03-29T13:41:07Z | shutdown | signal received | signal=terminated
broker-1  | [AA:store:OK] 2026-03-29T13:41:07Z | sqlite | database closed
broker-1  | [AA:BROKER:OK] 2026-03-29T13:41:07Z | shutdown | database closed
broker-1  | [AA:BROKER:OK] 2026-03-29T13:41:07Z | shutdown | clean exit


## Verdict

PASS — After DB activity (admin auth wrote an audit event), broker shutdown logs show: "sqlite | database closed" and "shutdown | database closed" followed by "clean exit". No SQLite error messages. Database properly closed on shutdown.
