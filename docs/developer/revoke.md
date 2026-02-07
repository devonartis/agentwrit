# Revocation Module (M04)

## Purpose

The revocation module provides immediate token invalidation across 4 levels, integrated into the authorization middleware for real-time enforcement.

## Revocation levels

| Level | Target | Effect |
|-------|--------|--------|
| `token` | Individual JTI | Revokes a single issued token |
| `agent` | Agent SPIFFE ID | Revokes all tokens for an agent instance |
| `task` | Task ID | Revokes all tokens associated with a task |
| `delegation_chain` | Chain hash (SHA-256) | Revokes all tokens sharing a delegation lineage |

Levels are checked in priority order: token > agent > task > delegation_chain. The first match short-circuits.

## Architecture

### RevChecker interface

```go
type RevChecker interface {
    IsTokenRevoked(jti string) bool
    IsAgentRevoked(agentID string) bool
    IsTaskRevoked(taskID string) bool
    IsChainRevoked(chainHash string) bool
    IsRevoked(jti, agentID, taskID, chainHash string) (bool, string)
}
```

`RevSvc` implements `RevChecker` with in-memory maps protected by `sync.RWMutex`. The interface exists to decouple the validation middleware from the storage backend.

### Integration with ValMw

The authorization middleware (`ValMw`) checks revocation after signature verification but before scope matching:

1. Extract bearer token
2. Verify signature and expiry (`TknSvc.Verify`)
3. **Check revocation** (`RevChecker.IsRevoked`)
4. Check required scope
5. Grant access

A revoked token returns 401 with `urn:agentauth:error:token-revoked`.

### Chain hash computation

The delegation chain hash is computed by JSON-serializing the `[]DelegRecord` array from claims and taking the SHA-256 hex digest. Empty chains produce an empty string, which skips the chain-level check.

## Decision record: In-memory over Redis

**Context**: The specification called for Redis pub/sub for revocation propagation across broker instances.

**Decision**: Use in-memory revocation sets, exposed via the `RevChecker` interface.

**Rationale**:
- The broker is currently single-process; multi-node propagation is not needed
- The zero-dependency constraint (`go.mod` has no `require` entries) would be violated by a Redis client
- The `RevChecker` interface provides a clean seam for future Redis pluggability
- In-memory maps with `RWMutex` provide the correct concurrency semantics for read-heavy workloads

**Consequences**:
- Revocations are lost on broker restart (acceptable for ephemeral tokens)
- Future multi-instance deployments will require implementing a Redis-backed `RevChecker`

## Redis migration path

1. Implement `RedisRevChecker` satisfying the `RevChecker` interface
2. Use Redis SET/GET for revocation storage with TTL matching token expiry
3. Use Redis pub/sub to propagate revocation events across instances
4. Swap the implementation in `cmd/broker/main.go` — no changes to `ValMw`

## API

### POST /v1/revoke

Request:
```json
{
  "level": "token",
  "target_id": "jti-value-here",
  "reason": "optional reason string"
}
```

Response (200):
```json
{
  "revoked": true,
  "level": "token",
  "target_id": "jti-value-here",
  "revoked_at": "2026-02-07T19:00:00Z"
}
```

Error (400): Invalid level or missing target_id, returned as RFC 7807 `application/problem+json`.

## Running revocation tests

```bash
go test ./internal/revoke/... -v
go test ./internal/handler/... -v -run Revoke
go test ./internal/authz/... -v -run Revoked
./scripts/integration_test.sh
./scripts/live_test.sh
```
