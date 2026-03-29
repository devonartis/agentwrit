# S1 — Broker Rejects Known-Weak Admin Secret [ACCEPTANCE]

Who: The operator.
What: Broker refuses to start with "change-me-in-production" as the admin secret.
Why: This was the old docker-compose default. If it still works, every default deployment is compromised.
Expected: Broker exits with error containing "known-weak".

## Test Output

FATAL: admin secret is a known-weak default; run 'aactl init' to generate a secure config, or set a strong AA_ADMIN_SECRET
exit=1

## Verdict

PASS — Broker exited with code 1 and FATAL message about known-weak default. Rejects "change-me-in-production".
