# P0-K3 — Configurable Key Path via AA_SIGNING_KEY_PATH [PRECONDITION]

Who: The operator.

What: The operator wants to control where the signing key is stored. By
default it goes to /data/signing.key in Docker. The operator can override
this with the AA_SIGNING_KEY_PATH environment variable. This test verifies
that docker-compose.yml passes the variable through to the container.

Why: If the operator can't control the key path, they can't integrate the
broker with secret management systems (Vault, KMS, mounted secrets) or
follow their organization's file layout conventions.

How to run: Check docker-compose.yml for the env var. Exec into the
container and confirm the key is at the expected path.

Expected: docker compose config shows AA_SIGNING_KEY_PATH, and the key
file exists at /data/signing.key inside the container.

## Test Output

docker compose config | grep SIGNING_KEY_PATH:
      AA_SIGNING_KEY_PATH: /data/signing.key

Key file inside container:
-rw-------    1 root     root           119 Mar 29 13:35 /data/signing.key


## Verdict

PASS — docker-compose.yml passes AA_SIGNING_KEY_PATH=/data/signing.key to the container. Key file exists at that path with correct permissions (0600).
