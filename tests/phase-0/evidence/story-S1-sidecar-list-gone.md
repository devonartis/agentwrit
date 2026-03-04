# P0-S1 — Sidecar List Endpoint Is Gone

Who: The security reviewer.

What: Before Phase 0, the broker had a route at GET /v1/admin/sidecars that listed all registered sidecars. We removed it because there are no sidecars running in the stack anymore.

Why: If this route still responds, the cleanup is incomplete and dead functionality is still exposed.

How to run: Make sure the Docker stack is running (./scripts/stack_up.sh). Source the environment file (source ./tests/phase-0/env.sh). Then call the old sidecar list URL on the broker at localhost:8080.

Expected: HTTP 404 — the route no longer exists.

## Test Output

404 page not found

HTTP 404

## Verdict

PASS — The broker returned 404. The old sidecar listing route is fully removed.
