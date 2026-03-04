# P0-S2 — Sidecar Activation Creation Endpoint Is Gone

Who: The security reviewer.

What: Before Phase 0, the broker had a route at POST /v1/admin/sidecar-activations where an admin created one-time activation tokens for sidecars. A sidecar would use that token to register itself with the broker. We removed it because there are no sidecars to activate.

Why: If this route still responds, someone could try to create activation tokens for a component that no longer exists.

How to run: Source the environment file. Then send a POST to the old activation creation URL on the broker. No request body needed — we just want to see if the route exists.

Expected: HTTP 404 — the route no longer exists.

## Test Output

404 page not found

HTTP 404

## Verdict

PASS — The broker returned 404. The old sidecar activation creation route is fully removed.
