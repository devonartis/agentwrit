# Changelog

Release history for AgentAuth.

---

## Version 2.0.0 (Current)

**MVP Prototype — Ephemeral Agent Credentialing**

### Core Features
- **Broker:** Central credential service with Ed25519 token signing
- **Sidecar:** Lightweight proxy with automatic agent registration
- **aactl CLI:** Operator command-line tool for broker management
- **SPIFFE IDs:** Standard agent identity format
- **EdDSA JWT tokens:** Short-lived, cryptographically signed credentials
- **Challenge-response registration:** Ed25519 signature-based identity verification
- **Scope attenuation:** `action:resource:identifier` format with wildcard support
- **4-level revocation:** Token, Agent, Task, and Chain-level revocation
- **Hash-chained audit trail:** Tamper-evident logging with SHA-256 chaining
- **Delegation chains:** Up to 5 hops with scope narrowing enforcement

### Phase 1B: App-Scoped Launch Tokens
- Single-use launch tokens for secure agent bootstrap
- Scope ceiling enforcement on launch tokens
- Configurable TTL (default 30 seconds)

### Security
- 6 compliance fixes (P0-P1 severity)
- RFC 7807 error responses
- Rate limiting on admin endpoints (5 req/s per IP)
- Constant-time credential comparison

### Deployment
- Docker Compose support (default, TLS, mTLS, UDS overlays)
- systemd service files for bare-metal deployment
- Multi-stage Dockerfile (broker and sidecar targets)

### Sidecar Features
- Auto-activation with exponential backoff
- Circuit breaker for broker connectivity
- Token renewal loop
- Scope ceiling enforcement
- UDS and TCP listener modes

### Documentation
- Comprehensive documentation suite (12+ guides)
- OpenAPI 3.0.3 specification
- Getting started guides for Users, Developers, and Operators

### Known Issues
See [[Known Issues]] for tracked limitations.

---

## Version 1.x (End of Life)

No longer supported. Upgrade to 2.0.x.

---

## Next Steps

- [[Home]] — Wiki home
- [[Security]] — Security policy
- [[Known Issues]] — Current limitations
