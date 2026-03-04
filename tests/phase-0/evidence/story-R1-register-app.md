# P0-R1 — Operator Registers a New App

Who: The operator.

What: The operator registers a new app called cleanup-test on the broker using aactl. This is a regression test — app registration is the core Phase 1A feature. We need to confirm it still works after removing the sidecar routes and changing the admin login format in Phase 0. When the operator registers an app, the broker creates it and returns three credentials: app_id (internal identifier), client_id (what the developer uses to identify themselves), and client_secret (the developer's password). The operator hands the client_id and client_secret to the developer.

Why: If app registration broke during the Phase 0 cleanup, it means the cleanup damaged something it shouldn't have. This is the most important regression check.

How to run: Source the environment file. Then run aactl app register with the app name and scopes. Save the credentials — they're needed for R2, R3, and R4.

Expected: The broker creates the app and returns app_id, client_id, and client_secret. The CLI warns to save the secret.

## Test Output

FIELD          VALUE
APP_ID         app-cleanup-test-c0e7b8
CLIENT_ID      ct-09ccbf99777a
CLIENT_SECRET  c1878375389c36ec82b0746a6b13bcd61e5196d46f2bb030476a43b787eda145
SCOPES         read:data:*, write:logs:*

WARNING: Save the client_secret — it cannot be retrieved again.


## Verdict

PASS — App created successfully. app_id, client_id, client_secret all returned. CLI warned to save the secret.
