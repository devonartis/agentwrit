# P0-K1 — Key File Created with Secure Permissions [PRECONDITION]

Who: The security reviewer.

What: When the broker starts for the first time, it creates a signing key
file on disk. The file must have 0600 permissions so that only the broker
process owner can read the private key material. If the file were
world-readable, any process on the same host could steal the signing key
and forge tokens.

Why: If the key file permissions are wrong, any process on the host can
read the private key and forge valid tokens — a complete auth bypass.

How to run: Exec into the broker container. Check that /data/signing.key
exists, has 0600 permissions, and contains a PEM private key header.

Expected: File exists at /data/signing.key, permissions are -rw------- (0600),
file starts with -----BEGIN PRIVATE KEY-----.

## Test Output

-rw-------    1 root     root           119 Mar 29 13:35 /data/signing.key
---
-----BEGIN PRIVATE KEY-----


## Verdict

PASS — File exists at /data/signing.key with 0600 permissions (-rw-------) and starts with -----BEGIN PRIVATE KEY-----. Only the broker process owner can read the key.
