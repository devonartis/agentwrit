# P0-S5 — Sidecar Ceiling Read Endpoint Is Gone

Who: The security reviewer.

What: Before Phase 0, the broker had a route at GET /v1/admin/sidecars/{id}/ceiling where an admin checked what permissions a sidecar was allowed to delegate to its agents. The ceiling was the maximum set of scopes. We removed it because there are no sidecars to manage.

Why: If this route still responds, the API is exposing sidecar management that doesn't exist anymore.

How to run: Source the environment file. Then call the old ceiling read URL using test-id as a placeholder sidecar ID. The actual ID doesn't matter — we just need to see if the route pattern still exists in the broker's router.

Expected: HTTP 404 — the route no longer exists.

## Test Output

404 page not found

HTTP 404

## Verdict

PASS — The broker returned 404. The old sidecar ceiling read route is fully removed.
