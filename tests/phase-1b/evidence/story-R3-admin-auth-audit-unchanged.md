# P1B-R3 — Admin Auth and Audit Flows Unchanged (Regression)

Who: The operator.

What: The operator confirms that admin authentication and audit retrieval still work
after Phase 1B changes. This is a regression test from Phase 1A — these are the most
basic operator functions. The operator logs in with the admin secret, then pulls the
audit trail. The operator also checks that the audit hash chain is intact — every
event has a hash that chains to the previous event's hash, forming a tamper-evident
log.

Why: If admin auth breaks, the operator is locked out of the system. If the audit
trail breaks, the operator loses visibility and compliance evidence. If the hash
chain breaks, the audit trail can't be trusted.

How to run: Source the environment file. Run aactl to authenticate as admin. Then
run aactl audit events to pull the audit trail. Check that events are present and
that the hash chain links correctly (each event's prev_hash matches the previous
event's hash).

Expected: Admin auth succeeds. Audit events are present. Hash chain is intact.

## Test Output — Step 1: Admin Auth

{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhZG1pbiIsImV4cCI6MTc3MjY3OTExMiwibmJmIjoxNzcyNjc4ODEyLCJpYXQiOjE3NzI2Nzg4MTIsImp0aSI6IjAxZmRhY2IxYjU0MzM4ODcyYmJiNTk0NmQyYmViMTgyIiwic2NvcGUiOlsiYWRtaW46bGF1bmNoLXRva2VuczoqIiwiYWRtaW46cmV2b2tlOioiLCJhZG1pbjphdWRpdDoqIl19.VNHmlqWnGBmbD_dhnEJMikxK537mdATf08PrDwzQbosyMw-CvCrj8AgobWY0uTQ3BZ_Uq_dTqk9ODAoiorzmCQ","expires_in":300,"token_type":"Bearer"}

HTTP 200

## Test Output — Step 2: Audit Events

ID          TIMESTAMP                       EVENT TYPE         AGENT ID                     OUTCOME  DETAIL
evt-000001  2026-03-04T14:34:11.469587841Z  admin_auth                                      success  admin authenticated as admin
evt-000002  2026-03-04T14:35:15.451494926Z  admin_auth                                      success  admin authenticated as admin
evt-000003  2026-03-04T14:35:15.721592801Z  app_registered                                  success  app=cleanup-test client_id=ct-09ccbf99777a scopes=[read:d...
evt-000004  2026-03-04T14:35:45.641544759Z  app_authenticated                               success  client_id=ct-09ccbf99777a app_id=app-cleanup-test-c0e7b8
evt-000005  2026-03-04T14:36:08.137592047Z  scope_violation    app:app-cleanup-test-c0e7b8  denied   scope_violation | required=admin:audit:* | actual=app:lau...
Showing 5 of 72 events (offset=0, limit=5)


## Test Output — Step 3: Hash Chain Integrity Check

Checking 10 events for hash chain integrity...
All 10 events chain correctly. Each prev_hash matches the previous event hash.


## Verdict

PASS — Admin auth returned HTTP 200 with a valid JWT. Audit trail returned 72 events across the session. Hash chain integrity check passed — all 10 sampled events chain correctly (each prev_hash matches the prior event's hash). No regression in admin auth or audit after Phase 1B.
