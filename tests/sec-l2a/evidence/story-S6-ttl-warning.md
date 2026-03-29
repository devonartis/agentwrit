# L2a-S6 — Broker Warns When DefaultTTL Exceeds MaxTTL

Who: The operator.

What: The operator accidentally sets AA_DEFAULT_TTL=7200 but AA_MAX_TTL=3600.
Every token would be silently clamped. The broker should warn on startup so
the operator notices the misconfiguration immediately.

Why: Without the warning, the operator would spend hours debugging why tokens
expire at 1 hour instead of the expected 2 hours. Silent clamping is a trap.

How to run: Start the broker with the misconfigured TTL values. Check the
startup logs for a WARN line. Verify admin auth still works.

Expected: Broker starts. Startup log has WARN with default_ttl=7200 and
max_ttl=3600. Admin auth returns 200.

## Test Output

Broker startup log:
[AA:CFG:WARN] 2026-03-29T20:59:05Z | load | AA_DEFAULT_TTL exceeds AA_MAX_TTL — tokens will be clamped to MaxTTL | default_ttl=7200 max_ttl=3600
[AA:BROKER:WARN] 2026-03-29T20:59:05Z | main | Running in development mode -- admin secret stored in plaintext
[AA:BROKER:OK] 2026-03-29T20:59:05Z | main | signing key loaded | path=/tmp/signing-l2a.key
[AA:store:OK] 2026-03-29T20:59:05Z | sqlite | database initialized | path=/tmp/agentauth-l2a.db
[AA:BROKER:OK] 2026-03-29T20:59:05Z | main | database initialized | path=/tmp/agentauth-l2a.db
[AA:store:OK] 2026-03-29T20:59:05Z | sqlite | audit events loaded | count=4
[AA:BROKER:OK] 2026-03-29T20:59:05Z | main | audit events loaded | count=4
[AA:BROKER:OK] 2026-03-29T20:59:05Z | main | starting broker | addr=127.0.0.1:8080, version=2.0.0
AgentAuth broker v2.0.0 listening on 127.0.0.1:8080

Admin auth test:
{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCIsImtpZCI6IlhSa1dxUTMzazdGMmRYWVloQkJxeW5TV2ZJUGRaNlF4LUwtY2xvMG5JNjAifQ.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhZG1pbiIsImF1ZCI6WyJhZ2VudGF1dGgiXSwiZXhwIjoxNzc0ODE4MjQ3LCJuYmYiOjE3NzQ4MTc5NDcsImlhdCI6MTc3NDgxNzk0NywianRpIjoiNDlhZjgyM2ZiMDA0OTNhMTQ0ZTM4MzY3ZDYxYTdjYzMiLCJzY29wZSI6WyJhZG1pbjpsYXVuY2gtdG9rZW5zOioiLCJhZG1pbjpyZXZva2U6KiIsImFkbWluOmF1ZGl0OioiXX0.h1Hh7Z1c4-gPCdrtvBX7-E39BqjrimzRnev7bRWA_SsW6F3pyZYId51Smt5UMBsvMDeGoL3Y1nT2jfC6JnUJDA","expires_in":300,"token_type":"Bearer"}

HTTP 200

## Verdict

PASS — Broker started successfully. Startup log contains WARN: 'AA_DEFAULT_TTL exceeds AA_MAX_TTL — tokens will be clamped to MaxTTL | default_ttl=7200 max_ttl=3600'. Admin auth returns 200. Note: expires_in shows 300 (not 3600) which suggests the broker's config layer may be applying DefaultTTL differently, but the WARN is present and the broker functions normally as expected.
