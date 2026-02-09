# Identity Module (M01)

## Purpose

The identity module performs ephemeral agent registration with proof-of-possession:

1. Broker issues nonce via `GET /v1/challenge`.
2. Agent signs nonce with its Ed25519 private key.
3. Broker validates launch token, nonce freshness, and signature in `POST /v1/register`.
4. Broker returns a SPIFFE ID and initial access token.

## Design decisions

**Challenge-response over mutual TLS**: Agents prove identity by signing a broker-issued nonce with their Ed25519 private key. This avoids the complexity of certificate infrastructure and works in ephemeral container environments where mTLS setup is impractical.

**Single-use launch tokens**: Launch tokens are consumed on first registration to prevent token reuse attacks. An attacker who intercepts a launch token cannot register a second agent with it.

**SPIFFE ID format**: AgentAuth uses SPIFFE-compatible identifiers to enable future federation with SPIFFE-aware service meshes. The hierarchical format (`/agent/{orch}/{task}/{instance}`) supports granular revocation at any level.

## SPIFFE ID format

AgentAuth uses:

`spiffe://{trustDomain}/agent/{orchId}/{taskId}/{instanceId}`

Example:

`spiffe://agentauth.local/agent/orch-456/task-789/cc07a7f4194cbe4bfecacb1d94a6bd2c`

## Launch token lifecycle

Launch token path:
- create (`CreateLaunchToken`)
- persist to store with TTL
- consume once at registration
- reject on reuse or expiration

Security property:
- launch token is single-use and short-lived by design.

## Nonce challenge lifecycle

Nonce path:
- generate cryptographically random 32-byte nonce
- store with 60-second TTL
- consume once at registration

Security property:
- prevents replay of stale registration proofs.

## Ed25519 key handling

- Broker keypair generation: `GenerateSigningKeyPair`
- Agent JWK parsing: `ParseAgentPubKey` (`kty=OKP`, `crv=Ed25519`)
- Signature verification in registration flow with `ed25519.Verify`

## Running tests

Unit:

```bash
go test ./internal/identity ./internal/handler -v
```

Integration:

```bash
./scripts/integration_test.sh
```

Live:

```bash
./scripts/live_test.sh
```

## Relevant configuration

- `AA_TRUST_DOMAIN` (default: `agentauth.local`)
- `AA_DEFAULT_TTL` (default: `300`)
- `AA_LOG_LEVEL` (default: `verbose`)

