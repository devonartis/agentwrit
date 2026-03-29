# P1-S4 — Operator Force-Overwrites Config [ACCEPTANCE]

Who: The operator.

What: The operator deliberately resets the secret with --force. The old
secret is gone forever. This is the recovery/rotation path.

Why: Without a force option, there's no way to rotate a compromised secret.

How to run: Note old secret, run aactl init --force, verify new secret differs.

Expected: Exit 0, new secret different from old, config file updated.

## Test Output

Old secret: Y3l3WPHow8RDOnLQzuR1wwht3ine9EiR2PYpeeGGon8
WARNING: Overwriting existing config at /tmp/aa-test-p1-dev/config
Config written to: /tmp/aa-test-p1-dev/config

Admin secret: -sM6f7juylkBrVQwwi-CMfUmIgTDaXA_7CzE_ybf05U

Dev mode: secret is also stored in the config file.
---
New secret: -sM6f7juylkBrVQwwi-CMfUmIgTDaXA_7CzE_ybf05U
Secrets are DIFFERENT (good)

## Verdict

PASS — Force overwrite succeeded (exit 0), printed new secret, old and new are different. Config file updated with new value.
