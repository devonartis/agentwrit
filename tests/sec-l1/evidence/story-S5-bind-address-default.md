# S5 — Broker Binds to 127.0.0.1 by Default [ACCEPTANCE]

Who: The operator.
What: Broker defaults to binding 127.0.0.1, not 0.0.0.0.
Why: Binding 0.0.0.0 exposes the broker to all network interfaces without TLS.
Expected: Startup log shows 127.0.0.1:8080.

## Test Output

Startup log (bind address):
[AA:BROKER:OK] 2026-03-29T19:05:31Z | main | starting broker | addr=127.0.0.1:8080, version=2.0.0
AgentAuth broker v2.0.0 listening on 127.0.0.1:8080

## Verdict

PASS — Broker bound to 127.0.0.1:8080 by default (not 0.0.0.0).
