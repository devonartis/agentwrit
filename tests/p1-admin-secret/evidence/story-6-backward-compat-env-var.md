# P1-S6 — Backward Compatibility — Env Var Still Works [ACCEPTANCE]

Who: The developer.

What: The developer uses the pre-P1 workflow: AA_ADMIN_SECRET env var,
no config file, no aactl init. Everything must work exactly as before.

Why: If backward compat breaks, existing users can't upgrade without
changing their deployment.

How to run: Start broker with AA_ADMIN_SECRET only (no config file).
Auth with that secret, verify wrong secret fails.

Expected: Health 200, correct secret → 200, wrong → 401.

## Test Output — VPS Mode

Health:
"ok"
Auth with correct secret:
HTTP 200Auth with wrong secret:
HTTP 401
## Test Output — Container Mode

Health:
"ok"
Auth with correct secret:
HTTP 200Auth with wrong secret:
HTTP 401
## Verdict

PASS — Both VPS and Container modes: AA_ADMIN_SECRET env var (no config file) works. Health 200, correct secret → 200, wrong → 401. Full backward compatibility with pre-P1 workflow.
