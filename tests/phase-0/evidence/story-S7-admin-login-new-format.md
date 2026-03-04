# P0-S7 — Operator Logs In With the New Admin Format

Who: The operator.

What: The operator uses aactl to list all registered apps on the broker. This tests the admin login flow end-to-end. When the operator runs any aactl command, the CLI first authenticates with the broker by sending the admin secret. In Phase 0, we changed the login format from the old shape (client_id + client_secret) to the new shape (just secret). If the app list comes back without errors, the operator's CLI is sending the new format and the broker is accepting it.

Why: If the new login format doesn't work, the operator is locked out of the system and can't manage anything.

How to run: Source the environment file (sets the broker URL and admin secret). Then run aactl app list. If the login works, the broker returns the list of apps — may be empty on a fresh stack, that's fine.

Expected: The app list comes back without authentication errors.

## Test Output

NAME  APP_ID  CLIENT_ID  STATUS  SCOPES  CREATED
Total: 0


## Verdict

PASS — The operator logged in successfully with the new admin format. App list returned (0 apps on fresh stack, no errors).
