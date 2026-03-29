# P1-S5a — Broker Starts with Config File — VPS Mode [ACCEPTANCE]

Who: The operator.

What: The operator deploys AgentAuth on a VPS. They ran aactl init --mode=prod
to generate a config file with a bcrypt hash. The broker binary reads
AA_CONFIG_PATH and loads the hash. No AA_ADMIN_SECRET env var is set.

Why: This is the standard production deployment path. If it doesn't work,
the entire P1 feature (config file + bcrypt) is broken.

How to run: Generate prod config, start broker binary, authenticate with
the secret, verify wrong secret fails.

Expected: Health 200, correct secret → 200, wrong secret → 401.

## Test Output

Config written to: /tmp/aa-test-p1-vps/config

Admin secret: R4DSwFHLA3mJ8k87c7eHiKHtHomLSmsw2FkcNNHfZOA

WARNING: Save this secret now. It will not be shown again.
Store it in your secrets manager (Vault, AWS Secrets Manager, etc.).
Captured secret: R4DSwFHLA3...

Starting broker (VPS mode)...
Health check:
{
  "status": "ok",
  "version": "2.0.0",
  "uptime": 1,
  "db_connected": true,
  "audit_events_count": 0
}

Auth with correct secret:
{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhZG1pbiIsImF1ZCI6WyJhZ2VudGF1dGgiXSwiZXhwIjoxNzc0Nzk5MDIzLCJuYmYiOjE3NzQ3OTg3MjMsImlhdCI6MTc3NDc5ODcyMywianRpIjoiY2QxYjNiOWE5OWI3MGU5ZjhlMTY1NGVkNGJjZTFjOWEiLCJzY29wZSI6WyJhZG1pbjpsYXVuY2gtdG9rZW5zOioiLCJhZG1pbjpyZXZva2U6KiIsImFkbWluOmF1ZGl0OioiXX0.XFPtnV9-9fyyfsD4EO9goGKiTFMY1a6M2hHGq8V0_avHPm8A5jnZVsVv_fPdbn8MdPy6lUNu5ReielZEgqPXDg","expires_in":300,"token_type":"Bearer"}

HTTP 200

Auth with wrong secret:
{"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"invalid credentials","instance":"/v1/admin/auth","error_code":"unauthorized","request_id":"b099048fb9b3c906"}

HTTP 401

## Verdict

PASS — VPS mode: broker started with config file (no AA_ADMIN_SECRET env var). Health 200. Correct prod secret → HTTP 200 with access_token. Wrong secret → HTTP 401. Bcrypt auth from config file works.
