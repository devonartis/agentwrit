# P1-S3 — Operator Cannot Accidentally Overwrite Config [ACCEPTANCE]

Who: The operator.

What: The operator accidentally runs aactl init again on an existing config.
The command must refuse to overwrite — doing so would generate a new secret
and lock the operator out of the broker.

Why: Silent overwrites destroy the existing secret with no recovery path.

How to run: Reuse S1 config, try aactl init again, verify refusal.

Expected: Non-zero exit, "already exists" error, original file untouched.

## Test Output

Attempt overwrite:
Error: config file already exists at /tmp/aa-test-p1-dev/config. Use --force to overwrite
Usage:
  aactl init [flags]

Flags:
      --config-path string   explicit config file path
      --force                overwrite existing config file
  -h, --help                 help for init
      --mode string          initialization mode: dev or prod (default "dev")

Global Flags:
      --json   output raw JSON

config file already exists at /tmp/aa-test-p1-dev/config. Use --force to overwrite
exit=1
---
Diff (should be empty = unchanged):
diff exit=0

## Verdict

PASS — Command refused with exit=1, error "config file already exists...Use --force to overwrite". Original file unchanged (diff exit=0).
