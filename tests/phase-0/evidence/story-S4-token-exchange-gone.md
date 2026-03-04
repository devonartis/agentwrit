# P0-S4 — Token Exchange Endpoint Is Gone

Who: The security reviewer.

What: Before Phase 0, the broker had a route at POST /v1/token/exchange where a sidecar exchanged its bearer token for a short-lived agent token. This was the final step of the sidecar flow — after activating, the sidecar would call this to get scoped tokens for the agents it was proxying. We removed it because the sidecar is not in the stack.

Why: Without this endpoint, there is no way to exchange tokens through the old sidecar path. If it still responds, the old token flow is still alive.

How to run: Source the environment file. Then send a POST to the old token exchange URL on the broker.

Expected: HTTP 404 — the route no longer exists.

## Test Output

404 page not found

HTTP 404

## Verdict

PASS — The broker returned 404. The old token exchange route is fully removed.
