# L2a-N5 — Renewal Fails When Predecessor Revocation Fails

Who: The security reviewer.

What: The reviewer verifies that renewal fails if the old token's revocation
fails. Before this hardening, revocation errors were silently ignored and
renewal always succeeded — leaving both old and new tokens valid.

Why: If revocation is silently skipped, an attacker who stole the old token
keeps a valid credential even after the legitimate user renewed. The blast
radius is unlimited.

How to run: Run the unit test TestRenew_RevokeFailureBlocksRenewal.

Expected: Unit test passes. Renewal fails when revocation fails. Error
contains "revoke predecessor".

## Test Output

=== RUN   TestRenew_RevokeFailureBlocksRenewal
--- PASS: TestRenew_RevokeFailureBlocksRenewal (0.00s)
PASS
ok  	github.com/divineartis/agentauth/internal/token	0.264s


## Verdict

PASS — Unit test TestRenew_RevokeFailureBlocksRenewal passes. Renewal correctly fails when predecessor revocation fails.
