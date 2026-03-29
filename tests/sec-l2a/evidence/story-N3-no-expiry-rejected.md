# L2a-N3 — Tokens Without Expiry Are Rejected

Who: The security reviewer.

What: The reviewer verifies that tokens with no expiry (exp=0 or missing) are
rejected. Before this hardening, such tokens were treated as "never expires"
— a permanent credential that can never be rotated out by waiting.

Why: Permanent tokens violate the principle of least privilege. If an attacker
obtains one, it never expires. Breach recovery becomes much harder because you
can't wait for credentials to rotate.

How to run: Run the unit test TestVerify_RejectsZeroExpiry.

Expected: Unit test passes. Tokens with exp <= 0 return ErrNoExpiry.

## Test Output

=== RUN   TestVerify_RejectsZeroExpiry
--- PASS: TestVerify_RejectsZeroExpiry (0.00s)
PASS
ok  	github.com/divineartis/agentauth/internal/token	0.288s


## Verdict

PASS — Unit test TestVerify_RejectsZeroExpiry passes. Tokens with exp=0 correctly return ErrNoExpiry.
