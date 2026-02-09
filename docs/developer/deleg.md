# Delegation Chain Verification Module (M07)

## Purpose

The delegation module enables scope-attenuated token delegation between agents. An agent with broad scope (e.g., `read:Customers:*`) can delegate a narrower scope (e.g., `read:Customers:12345`) to another agent, creating a cryptographically verifiable chain of trust.

Components:
- **Attenuate**: Validates requested scope is a subset of parent scope
- **DelegSvc**: Creates delegation tokens with chain tracking, depth limits, and TTL enforcement
- **VerifyChain**: Validates delegation chains (Ed25519 signatures, scope narrowing, revocation)
- **DelegHdl**: HTTP handler for `POST /v1/delegate`

## Design decisions

### Scope attenuation as a security invariant

Delegation can never grant more than the delegator has. `Attenuate()` wraps `ScopeIsSubset` (from M02) but adds actionable error detail — when attenuation fails, the error names the specific scope that violated the constraint. This flows through to the RFC 7807 response body so agents can debug permission failures without reading source code.

### Ed25519 signatures per delegation record

Each `DelegRecord` is independently signed by the broker's Ed25519 key. This means chain verification doesn't require the full JWT — a verifier can check each hop's integrity in isolation. The signature covers the record *without* its `Signature` field (set to empty during signing), preventing circular dependencies.

### SHA-256 chain hash

The chain hash is a SHA-256 of the entire JSON-serialized delegation chain. This serves as a tamper-evident seal: if any record in the chain is modified, the hash changes. The hash is also used as a revocation target — revoking a chain hash invalidates all tokens derived from that delegation path.

### Max depth = 3 for MVP

The delegation depth limit of 3 balances flexibility (agents can sub-delegate twice) with security (limits blast radius of compromised chains). This is configurable via `NewDelegSvc(tknSvc, signingKey, maxDepth)`.

### TTL can only shrink

Delegated token TTL must be <= the delegator's remaining TTL. This prevents delegation from extending token lifetimes beyond the original grant.

## Package layout

```
internal/deleg/
  attenuate.go       — Attenuate(parent, requested) ([]string, error)
  attenuate_test.go  — 24 table-driven attenuation tests
  deleg_svc.go       — DelegSvc.Delegate(req) (*DelegResp, error)
  deleg_svc_test.go  — 9 DelegSvc tests
  chain.go           — VerifyChain, VerifyChainHash
  chain_test.go      — 12 chain verification tests
```

## API

### POST /v1/delegate

Request:
```json
{
  "delegator_token": "eyJ...",
  "target_agent_id": "spiffe://agentauth.local/agent/orch/task/agentB",
  "delegated_scope": ["read:Customers:12345"],
  "max_ttl": 60
}
```

Success (201):
```json
{
  "delegation_token": "eyJ...",
  "chain_hash": "a1b2c3...",
  "delegation_depth": 1
}
```

Error responses:
- **401** `urn:agentauth:error:invalid-token` — delegator token invalid
- **403** `urn:agentauth:error:scope-escalation` — requested scope broader than delegator's
- **403** `urn:agentauth:error:delegation-depth-exceeded` — chain exceeds max depth

## Dependencies

- `internal/token` — `TknSvc` (issue/verify), `ScopeIsSubset`, `DelegRecord`, `TknClaims`
- `internal/revoke` — `RevSvc` (chain verification checks agent revocation)
- `internal/obs` — structured logging
