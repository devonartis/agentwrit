# P0-S3 — Sidecar Activate Endpoint Is Gone

Who: The security reviewer.

What: Before Phase 0, the broker had a route at POST /v1/sidecar/activate where a sidecar exchanged its one-time activation token for a bearer token. This was the most security-sensitive part of the sidecar flow — it's where tokens were issued. We removed it because there are no sidecars in the stack.

Why: If this route still responds, someone with a stolen activation token could potentially get a bearer token from the broker.

How to run: Source the environment file. Then send a POST to the old sidecar activation URL on the broker.

Expected: HTTP 404 — the route no longer exists.

## Test Output

404 page not found

HTTP 404
## Verdict

PASS — The broker returned 404. The old sidecar activate route is fully removed.
