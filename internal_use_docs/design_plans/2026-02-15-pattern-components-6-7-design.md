# Pattern Components 6+7: Mutual Auth & Delegation Chain — Design

## Goal

Make all 7 core components of the Ephemeral Agent Credentialing pattern
(Security Pattern v1.2) callable over HTTP through both broker and sidecar.
Components 1-5 are already exercisable. This work wires Components 6
(Agent-to-Agent Mutual Authentication) and 7 (Delegation Chain Verification)
end-to-end.

## Guiding Principle

The Security Pattern (`plans/archive/Security-Pattern-That-Is-Why-We-Built-AgentAuth.md`)
is the source of truth. Every design decision maps to a specific component or
threat in the pattern. If it's not in the pattern, it's not in scope.

## Pattern Mapping

| # | Pattern Component | Status Before | Status After |
|---|-------------------|---------------|--------------|
| 1 | Ephemeral Identity Issuance | Done | Done |
| 2 | Short-Lived Task-Scoped Tokens | Done | Done |
| 3 | Zero-Trust Enforcement | Done (ValMw) | Done |
| 4 | Automatic Expiration & Revocation | Done (4-level) | Done |
| 5 | Immutable Audit Logging | Done (hash-chain) | Done |
| 6 | Agent-to-Agent Mutual Auth | Code exists, no HTTP routes | Broker endpoints + sidecar proxy |
| 7 | Delegation Chain Verification | Broker wired, no sidecar proxy | Sidecar proxy endpoint |

## Not In Scope

- Discovery registry (operational concern, not a pattern component)
- Heartbeat manager (operational concern, not a pattern component)
- Additional metrics (build after the app reveals gaps)
- Demo application (build after all 7 components are proven)

---

## Design

### Service Layer Refactor

`MutAuthHdl.RespondToHandshake` currently takes `ed25519.PrivateKey` and signs
the nonce internally. Private keys must never cross the wire. Refactor to accept
`signedNonce []byte` instead. The agent signs locally; the broker validates the
signature against the agent's registered public key (defense in depth).

**Before:**
```go
func (h *MutAuthHdl) RespondToHandshake(req *HandshakeReq, responderToken string, responderKey ed25519.PrivateKey) (*HandshakeResp, error)
```

**After:**
```go
func (h *MutAuthHdl) RespondToHandshake(req *HandshakeReq, responderToken string, signedNonce []byte) (*HandshakeResp, error)
```

Add signature verification in step 2 against the registered public key. Don't
wait for step 3. Update existing 19 unit tests to sign before calling.

### Broker HTTP Endpoints (3 new)

All behind `ValMw` (Bearer auth). All logged to audit trail.

#### POST /v1/handshake/initiate

Pattern: Steps 1-2 (Agent A requests, gets challenge nonce)

```
Auth:     Bearer <initiator token>
Body:     { "target_agent_id": "spiffe://..." }
Response: { "nonce": "hex...", "initiator_id": "spiffe://...",
            "target_agent_id": "spiffe://..." }
```

#### POST /v1/handshake/respond

Pattern: Steps 3-5 (Target signs nonce, presents credentials)

```
Auth:     Bearer <responder token>
Body:     { "initiator_token": "...", "initiator_id": "spiffe://...",
            "target_agent_id": "spiffe://...", "nonce": "hex...",
            "signed_nonce": "base64..." }
Response: { "responder_id": "spiffe://...", "responder_token": "...",
            "signed_nonce": "base64...", "counter_nonce": "hex..." }
```

#### POST /v1/handshake/complete

Pattern: Steps 6-7 (Agent A validates Agent B, exchange proceeds)

```
Auth:     Bearer <initiator token>
Body:     { "responder_token": "...", "responder_id": "spiffe://...",
            "signed_nonce": "base64...", "counter_nonce": "hex...",
            "original_nonce": "hex..." }
Response: { "verified": true }
```

### Sidecar Proxy Endpoints (4 new)

The sidecar's value-add: managed agents never touch keys. The sidecar holds
Ed25519 private keys in its agent registry and auto-signs for step 2.

#### POST /v1/handshake/initiate

```
Body:     { "agent_name": "my-agent", "target_agent_id": "spiffe://..." }
Sidecar:  Resolves agent from registry, retrieves token, proxies to broker.
```

#### POST /v1/handshake/respond

```
Body:     { "agent_name": "my-agent",
            "handshake_req": { nonce, initiator_id, target_agent_id, initiator_token },
            "signed_nonce": "base64..." }   <-- optional, BYOK only
Managed:  Sidecar looks up private key from registry, signs nonce, proxies.
BYOK:     Developer provides signed_nonce, sidecar passes through.
```

Detection: if `signed_nonce` is present in body -> BYOK pass-through.
If absent -> auto-sign with registry key. Same pattern as existing
BYOK registration flow.

#### POST /v1/handshake/complete

```
Body:     { "agent_name": "my-agent",
            "handshake_resp": { responder_token, responder_id, signed_nonce, counter_nonce },
            "original_nonce": "hex..." }
Sidecar:  Proxies to broker with agent's token.
```

#### POST /v1/delegate

```
Body:     { "agent_name": "my-agent", "delegate_to": "spiffe://...",
            "scope": ["read:resource:id"], "ttl": 300 }
Sidecar:  Proxies to broker's POST /v1/delegate with agent's token.
```

### Live Tests (All 7 Components)

Extend Docker E2E to prove all 7 pattern components in one flow:

1. **Ephemeral Identity** - Register Agent A and Agent B
2. **Task-Scoped Tokens** - Exchange tokens with specific scopes
3. **Zero-Trust** - Every endpoint validates Bearer (already enforced)
4. **Expiration & Revocation** - Revoke Agent B, verify failure
5. **Audit Logging** - Query trail, verify all actions recorded
6. **Mutual Auth** - 3-step handshake A <-> B via sidecar
7. **Delegation** - A delegates narrowed scope to B

---

## Files Changed

| Layer | File | Change |
|-------|------|--------|
| Service | `internal/mutauth/mut_auth_hdl.go` | Refactor `RespondToHandshake` signature |
| Tests | `internal/mutauth/mut_auth_hdl_test.go` | Update 19 tests (sign before calling) |
| Broker handlers | `internal/handler/handshake_hdl.go` (new) | 3 HTTP handlers |
| Broker wiring | `cmd/broker/main.go` | Wire 3 routes behind ValMw |
| Sidecar client | `cmd/sidecar/broker_client.go` | 4 new methods |
| Sidecar handlers | `cmd/sidecar/handshake_handler.go` (new) | 3 handshake proxies |
| Sidecar delegation | `cmd/sidecar/delegate_handler.go` (new) | 1 delegation proxy |
| Sidecar wiring | `cmd/sidecar/main.go` | Wire 4 routes |
| Live tests | `scripts/live_test_docker.sh` | Extended for 7 components |
| Live tests | `scripts/live_test_sidecar.sh` | Extended for 7 components |
| Docs | `docs/API_REFERENCE.md` | 3 new broker + 4 new sidecar endpoints |
| Docs | `docs/DEVELOPER_GUIDE.md` | Handshake + delegation architecture |
| Docs | `CHANGELOG.md` | New entries |

## Decision Record

**ADR: Discovery and heartbeats excluded.** These are operational concerns not
listed in the Security Pattern's 7 core components. They will be evaluated when
the demo application reveals actual operational gaps. The pattern mentions
heartbeats zero times and discovery zero times.

**ADR: Service refactor over HTTP key transmission.** `RespondToHandshake`
refactored to accept pre-signed nonces instead of private keys. Agents sign
locally; the broker validates. Aligns with the pattern: agents prove identity,
the broker verifies.

**ADR: BYOK detection by field presence.** If `signed_nonce` is in the sidecar
request body, the agent manages its own keys. If absent, the sidecar auto-signs.
Consistent with existing BYOK registration pattern.
