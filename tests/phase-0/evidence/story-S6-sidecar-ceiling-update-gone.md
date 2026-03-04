# P0-S6 — Sidecar Ceiling Update Endpoint Is Gone

Who: The security reviewer.

What: Before Phase 0, the broker had a route at PUT /v1/admin/sidecars/{id}/ceiling where an admin changed what permissions a sidecar was allowed to delegate. We removed it because there are no sidecars to manage.

Why: Same as S5 — dead management routes shouldn't be exposed on the API.

How to run: Source the environment file. Then send a PUT to the old ceiling update URL using test-id as a placeholder sidecar ID.

Expected: HTTP 404 — the route no longer exists.

## Test Output

404 page not found

HTTP 404

## Verdict

PASS — The broker returned 404. The old sidecar ceiling update route is fully removed.
