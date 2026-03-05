# TD006-S7 — TTL Changes Are Audited

Who: The security reviewer.

What: The security reviewer checks the audit trail to confirm that every
TTL change made during these tests was recorded. The reviewer is looking
for an app_updated event from S3 (where the operator changed ttl-custom-s2's
TTL from 3600 to 7200). The audit detail should include both the old and
new TTL values so that an investigator can reconstruct the change history.

Why: Audit is how organizations prove compliance and investigate incidents.
If TTL changes aren't audited, there's no way to know when a token lifetime
was extended — an attacker could quietly increase a TTL to keep their access
alive longer. The old and new values matter: knowing "TTL was changed" isn't
enough, you need to know "TTL was changed from 300 to 86400."

How to run: Source the environment file. Run aactl audit events filtered
to app_updated events. Check that the S3 TTL change appears with old and
new values.

Expected: At least one app_updated event showing the TTL change from 3600
to 7200 for app-ttl-custom-s2.

## Test Output

ID          TIMESTAMP                       EVENT TYPE   AGENT ID  OUTCOME  DETAIL
evt-000013  2026-03-05T17:37:50.058182335Z  app_updated            success  app_id=app-ttl-custom-s2-5a8397 token_ttl=3600->7200 upda...
Showing 1 of 1 events (offset=0, limit=100)

## Verdict

PASS — The audit trail contains one app_updated event (evt-000013) recording the TTL change from S3. The detail shows app_id=app-ttl-custom-s2-5a8397 with token_ttl=3600->7200 — both the old and new values are captured. An investigator can reconstruct when the TTL was changed and by how much.
