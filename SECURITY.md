# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in AgentAuth, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

### How to Report

Email your findings to: **security@agentauth.dev** (or open a private security advisory on GitHub)

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### What to Expect

- **Acknowledgment** within 48 hours
- **Assessment** within 7 days
- **Fix timeline** communicated after assessment
- **Credit** in the security advisory (unless you prefer anonymity)

## Scope

The following are in scope for security reports:

- Authentication bypass (challenge-response, token validation)
- Authorization failures (scope enforcement, middleware bypass)
- Token forgery or replay attacks
- Delegation chain manipulation or privilege escalation
- Audit trail tampering
- Injection vulnerabilities (command, SQL, XSS)
- Sensitive data exposure (PII leakage, key material)

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.0.x   | Yes       |

## Security Design

AgentAuth implements the Ephemeral Agent Credentialing security pattern. Key security properties:

- Ed25519 signatures for identity proof and token signing
- Short-lived JWTs (configurable TTL, default 5 minutes)
- SPIFFE-format identities bound to agent instances
- 4-level token revocation (token, agent, task, delegation chain)
- SHA-256 hash-chain audit trail with PII sanitization
- Zero-trust middleware: every request is authenticated and authorized independently
