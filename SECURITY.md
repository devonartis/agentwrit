# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in AgentAuth, please report it responsibly to **security@agentauth.dev** rather than using public issue trackers.

**We ask that you:**
- Provide detailed information about the vulnerability
- Include steps to reproduce (if applicable)
- Allow us 90 days to develop and release a fix
- Refrain from public disclosure during this period
- Contact us directly if you don't receive a response within 7 days

All vulnerability reports are taken seriously and will be investigated promptly.

## Supported Versions

| Version | Status | Security Updates |
|---------|--------|------------------|
| 2.0.x   | Current | Yes |
| 1.x     | EOL | No |
| < 1.0   | Unsupported | No |

Users should upgrade to 2.0.x to receive security patches. Version 1.x is no longer supported and will not receive updates.

## What Constitutes a Security Vulnerability

We consider the following as security vulnerabilities:

- **Authentication Bypass**: Mechanisms allowing unauthenticated access to protected operations
- **Token Forgery**: Ability to create valid tokens without proper authorization
- **Scope Escalation**: Obtaining permissions beyond those granted by a token
- **Audit Trail Tampering**: Modification or deletion of audit logs
- **Signature Verification Bypass**: Circumventing Ed25519 signature validation
- **State Injection**: Injecting or manipulating broker state through API calls
- **Cryptographic Weaknesses**: Flaws in token generation, signing, or validation
- **Access Control Flaws**: Improper validation of token claims or scopes

## What Is NOT a Security Vulnerability

The following items are **not** considered security vulnerabilities:

- **Denial of Service against single-instance brokers**: The broker is designed to run as a single instance in trusted environments
- **Issues requiring physical access**: Attacking the host system or underlying infrastructure
- **Social engineering**: Techniques that manipulate users outside the system
- **Client-side vulnerabilities**: Issues in applications using AgentAuth that don't stem from AgentAuth itself
- **Configuration errors**: Improper deployment or configuration by operators
- **Third-party dependency vulnerabilities**: These should be reported to the maintainers of those dependencies

## Security Design Principles

AgentAuth is built on these core security principles:

1. **Short-Lived Tokens**: Authorization tokens have bounded lifetimes to limit exposure
2. **Ed25519 Signatures**: All tokens are cryptographically signed using Ed25519
3. **Scope Attenuation**: Token scopes are validated against requested operations
4. **Zero-Trust Validation**: Every request is validated independently; no trust is cached
5. **Immutable Audit Trail**: All authorization decisions are logged and cannot be modified
6. **Minimal State**: The broker maintains only essential state to reduce attack surface
7. **Defense in Depth**: Multiple validation layers protect against bypass attempts

## Known Limitations

Users should be aware of these limitations when deploying AgentAuth:

- **Single Broker Instance**: The broker is designed as a single instance within a secure network. It does not support clustering or horizontal scaling. High availability requires load balancer failover.
- **Ephemeral Signing Keys**: The broker generates a fresh Ed25519 key pair on every startup. All previously issued tokens become invalid after a broker restart. Key persistence is not currently supported.
- **Hybrid Persistence**: Critical state (audit events, revocations) persists to SQLite via `AA_DB_PATH`. Transient state (nonces, agent records, launch tokens) lives in memory and is cleared on restart.
- **X-Forwarded-For Trust**: The broker trusts `X-Forwarded-For` headers for request attribution in audit logs. This requires proper reverse proxy configuration and should only be used in trusted networks.
- **No Built-In Rate Limiting**: Rate limiting must be implemented at the load balancer or reverse proxy layer.

See [KNOWN-ISSUES.md](KNOWN-ISSUES.md) for tracked security and operational issues with mitigations.

## Threat Model

For detailed information about AgentAuth's threat model, security assumptions, and architectural security considerations, see [docs/concepts.md](docs/concepts.md).

## Encrypted Communications

For sensitive security correspondence, you may use PGP encryption. The project's PGP key is available at:
```
KEY_ID: [TO_BE_POPULATED]
FINGERPRINT: [TO_BE_POPULATED]
```

Please request the current key from security@agentauth.dev if needed.

## Security Updates

Security patches will be released on an as-needed basis. Users should monitor the GitHub releases page and upgrade promptly when patches are available.
