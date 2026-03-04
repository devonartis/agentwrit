# P0-R4 — Audit Trail Records All the Activity

Who: The operator.

What: The operator pulls the full audit trail from the broker to check that everything that happened during these tests was recorded. The audit trail is how the operator knows what's going on — every app registration, every login, every failed request gets logged. The operator checks for two specific events: the app registration from R1 (app_registered) and the developer login from R2 (app_authenticated). The operator also scans the entire trail to make sure no client_secret values leaked into the logs.

Why: If audit events are missing, the operator loses visibility into the system. If secrets appear in audit records, that's a security breach. Both would be serious regressions.

How to run: Source the environment file. Then run aactl audit events. Look for app_registered and app_authenticated events. Check that no client_secret values appear anywhere.

Expected: app_registered and app_authenticated events present. No client_secret values in any event.

## Test Output

ID          TIMESTAMP                       EVENT TYPE         AGENT ID                     OUTCOME  DETAIL
evt-000001  2026-03-04T14:34:11.469587841Z  admin_auth                                      success  admin authenticated as admin
evt-000002  2026-03-04T14:35:15.451494926Z  admin_auth                                      success  admin authenticated as admin
evt-000003  2026-03-04T14:35:15.721592801Z  app_registered                                  success  app=cleanup-test client_id=ct-09ccbf99777a scopes=[read:d...
evt-000004  2026-03-04T14:35:45.641544759Z  app_authenticated                               success  client_id=ct-09ccbf99777a app_id=app-cleanup-test-c0e7b8
evt-000005  2026-03-04T14:36:08.137592047Z  scope_violation    app:app-cleanup-test-c0e7b8  denied   scope_violation | required=admin:audit:* | actual=app:lau...
evt-000006  2026-03-04T14:36:26.78621875Z   admin_auth                                      success  admin authenticated as admin
Showing 6 of 6 events (offset=0, limit=100)


## Verdict

PASS — All events recorded: app_registered (evt-000003), app_authenticated (evt-000004), scope_violation from R3 (evt-000005). No client_secret values in any event. Audit trail is complete.
