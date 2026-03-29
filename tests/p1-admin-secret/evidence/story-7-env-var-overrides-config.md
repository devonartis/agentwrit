# P1-S7 — Env Var Overrides Config File [ACCEPTANCE]

Who: The developer.

What: Both config file and AA_ADMIN_SECRET env var are set to different
values. The env var must win — standard override pattern (env > file > defaults).

Why: If the config file wins, operators can't override secrets via environment
variables, which breaks Kubernetes Secrets, systemd overrides, etc.

How to run: Generate config, start broker with both config AND different env var.
Auth with env secret (should work), auth with config secret (should fail).

Expected: Env var secret → 200, config file secret → 401.

## Test Output — VPS Mode

Config written to: /tmp/aa-test-p1-s7-vps/config

Admin secret: KNQGxf1TgeQd_S-vHSDPS9wJ6rWkh9LQcqztcFgv_fc

Dev mode: secret is also stored in the config file.
Config secret: KNQGxf1Tge...
Env secret: env-override-secret-for-testing

Auth with ENV secret (should work):
HTTP 200Auth with CONFIG secret (should fail):
HTTP 401
## Verdict

PASS — Env var secret → HTTP 200, config file secret → HTTP 401. Environment variable correctly overrides config file value.
