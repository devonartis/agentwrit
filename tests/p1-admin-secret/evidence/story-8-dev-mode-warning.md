# P1-S8 — Dev Mode Startup Warning [ACCEPTANCE]

Who: The operator.

What: The broker starts in development mode. It must log a warning so the
operator knows the admin secret is stored as plaintext on disk.

Why: Without this warning, an operator might run a dev config in production
without realizing plaintext secrets are on disk.

How to run: Start broker with dev config, check stdout for warning.

Expected: Logs contain "development mode" warning about plaintext secrets.

## Test Output — VPS Mode

Broker log (grep for development/warning):
[AA:BROKER:WARN] 2026-03-29T15:41:02Z | main | Running in development mode -- admin secret stored in plaintext

## Verdict

PASS — Broker logs "[AA:BROKER:WARN] Running in development mode -- admin secret stored in plaintext". Warning is clear and visible in stdout for systemd/monitoring to capture.
