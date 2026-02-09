# Mutual Authentication Module (M06)

## Purpose

The mutual authentication module enables agent-to-agent identity verification. Where M01-M03 handle broker-to-agent authentication, M06 extends trust to lateral (peer) communication between agents in the same trust domain.

Components:
- **MutAuthHdl**: 3-step handshake protocol for cryptographic mutual authentication
- **DiscoveryRegistry**: Agent-to-endpoint binding with MITM-prevention verification
- **HeartbeatMgr**: Agent liveness tracking with optional automatic revocation

## Design decisions

### 3-step handshake over 2-step

**Context**: A 2-step handshake (challenge-response) proves identity in one direction. Agent-to-agent requires bidirectional proof.

**Decision**: Use a 3-step protocol where each agent proves identity through:
1. Valid broker-issued token (verified by `TknSvc.Verify`)
2. Registration in the agent store (verified by `SqlStore.GetAgent`)
3. Nonce signature with the agent's registered Ed25519 key

**Rationale**: This prevents token theft attacks — possessing a stolen token is insufficient without the corresponding private key. The nonce exchange also prevents replay attacks.

### Discovery binding for MITM prevention

**Context**: Agents resolving peer endpoints could be redirected to malicious services.

**Decision**: Bind agent SPIFFE IDs to specific endpoints in an in-memory registry. Verify presented identities match directory bindings before completing handshakes.

**Rationale**: Creates a trusted directory that must be consistent with actual agent identities. No external DNS or service mesh dependency.

### Optional auto-revocation in heartbeat

**Context**: The specification allows either "flag for investigation" or "auto-revoke" when agents miss heartbeats.

**Decision**: Make auto-revocation depend on whether `RevSvc` is wired into the `HeartbeatMgr`. When `revSvc` is nil, missed heartbeats produce `obs.Warn` logs only. When `revSvc` is non-nil, agents exceeding `maxMiss` (default 3) are automatically revoked.

**Rationale**: Avoids config flags or mode switches. The operator's wiring choice in `main.go` expresses the security posture directly in code.

## Architecture

### Package structure

```text
internal/mutauth/
  mut_auth_hdl.go       — 3-step handshake protocol
  mut_auth_hdl_test.go  — handshake unit tests
  discovery.go          — discovery binding registry
  discovery_test.go     — discovery unit tests
  heartbeat.go          — liveness monitoring
  heartbeat_test.go     — heartbeat unit tests
```

### Dependencies

| Component | Depends on | Purpose |
|-----------|-----------|---------|
| `MutAuthHdl` | `token.TknSvc` | Verify agent tokens |
| `MutAuthHdl` | `store.SqlStore` | Look up registered agents |
| `MutAuthHdl` | `*DiscoveryRegistry` (optional) | Verify discovery bindings during handshake |
| `HeartbeatMgr` | `revoke.RevSvc` (optional) | Auto-revoke unresponsive agents |
| `DiscoveryRegistry` | (standalone) | In-memory agent-endpoint map |

### Handshake protocol

```
Agent A                         Broker                          Agent B
   │                              │                                │
   │── InitiateHandshake ────────►│                                │
   │   (tokenA, targetB)          │                                │
   │◄── HandshakeReq ────────────│                                │
   │   (nonce, initiatorID)       │                                │
   │                              │                                │
   │── HandshakeReq ─────────────────────────────────────────────►│
   │                              │                                │
   │                              │◄── RespondToHandshake ────────│
   │                              │    (req, tokenB, privKeyB)     │
   │◄── HandshakeResp ──────────────────────────────────────────── │
   │   (signedNonce, counterNonce)│                                │
   │                              │                                │
   │── CompleteHandshake ────────►│                                │
   │   (resp, originalNonce)      │                                │
   │◄── (true, nil) ─────────────│                                │
```

### Security: Peer identity verification

`RespondToHandshake` enforces two-level peer binding to prevent peer substitution (MITM) attacks:

**Level 1a — Initiator identity check:** The initiator's token subject (`initClaims.Sub`) must match the declared `InitiatorID` in the `HandshakeReq`. This prevents an attacker from tamper-modifying the `InitiatorID` field to impersonate a different agent while carrying their own valid token. Mismatch produces `ErrInitiatorMismatch`.

**Level 1b — Responder identity check:** The responder's token subject (`respClaims.Sub`) must match the `TargetAgentID` set during initiation. If Agent A initiates a handshake targeting Agent B, only Agent B's token is accepted in the response step. Any other registered agent presenting a valid token is rejected with `ErrPeerMismatch`.

**Level 2 — Optional discovery binding:** When a `*DiscoveryRegistry` is wired into `MutAuthHdl` (non-nil), the handler also verifies that the target agent is bound in the discovery registry via `VerifyBinding`. This adds defense-in-depth by ensuring the agent's identity is consistent with the trusted directory. When `DiscoveryRegistry` is nil, this check is skipped (following the same nilable-optional pattern as `HeartbeatMgr` with `RevSvc`).

## Running tests

```bash
go test ./internal/mutauth/... -v                          # unit tests
go test ./tests/integration/... -tags=integration -v       # integration tests
```
