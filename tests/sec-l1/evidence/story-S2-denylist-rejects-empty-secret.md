# S2 — Broker Rejects Empty Admin Secret [ACCEPTANCE]

Who: The operator.
What: Broker refuses to start with an empty admin secret.
Why: An empty secret means anyone can authenticate as admin.
Expected: Broker exits with error.

## Test Output

FATAL: admin secret is a known-weak default; run 'aactl init' to generate a secure config, or set a strong AA_ADMIN_SECRET
exit=1

## Verdict

PASS — Broker exited with code 1 and FATAL message. Empty secret rejected.
