# Security

AgentAuth's security model, threat analysis, and vulnerability reporting.

---

## Security Design Principles

| Principle | How AgentAuth Implements It |
|-----------|---------------------------|
| **Short-lived tokens** | Default 5-minute TTL. Max 15 minutes. |
| **Ed25519 signatures** | All tokens are EdDSA-signed. Compact, fast, secure. |
| **Scope attenuation** | Permissions only narrow, never expand. |
| **Zero-trust validation** | Every request is validated independently. No trusted sessions. |
| **Immutable audit** | Hash-chained audit trail. Tamper-evident. |
| **Minimal state** | Ephemeral signing keys. Broker restarts clean. |
| **Defense in depth** | Multiple layers: crypto, scopes, revocation, audit, TLS. |

---

## Threat Model

### What AgentAuth Defends Against

| Threat | Mitigation |
|--------|-----------|
| **Credential theft** | Tokens expire in minutes. Stolen tokens are useless quickly. |
| **Compromised agents** | Task-scoped credentials limit blast radius. Unique identity per instance. |
| **Lateral movement** | Scope boundaries prevent cross-resource access. |
| **Agent impersonation** | Mutual authentication with Ed25519 signatures. Anti-spoofing checks. |
| **Delegation exploitation** | One-way scope attenuation. Signed chains. Depth limit (5). |
| **Malicious insiders** | Immutable audit trail. Scope enforcement. |
| **Rogue agents** | Credentials expire regardless of agent behavior. |

### What AgentAuth Does NOT Defend Against

| Threat | Why It's Out of Scope |
|--------|----------------------|
| **Broker compromise** | Broker is the root of trust. Protect it as critical infrastructure. |
| **Prompt injection** | LLM-level attacks are outside credential management. AgentAuth limits blast radius. |
| **Data poisoning** | Affects agent decisions below the credential layer. |
| **Physical access** | Attackers with physical access can extract keys. |
| **Cryptographic breaks** | Assumes Ed25519 and SHA-256 remain secure. |
| **Denial of service** | Requires infrastructure-level mitigations. |

### Trust Boundaries

```
Root of Trust:
  └── Broker (must be protected as critical infrastructure)
       ├── Issues credentials to Orchestrators (trusted within policy)
       ├── Issues credentials to Agents (not trusted beyond credentials)
       ├── Publishes signing key to Resource Servers (independent validation)
       └── All actions recorded in Audit Trail (append-only)
```

---

## Known Limitations

| ID | Issue | Severity | Mitigation |
|----|-------|----------|-----------|
| KI-001 | Admin Secret Blast Radius | High | Single `AA_ADMIN_SECRET` grants full control. Rotate regularly. Use dedicated secrets management. |
| KI-002 | TCP Default Listener | Medium | Sidecar listens on TCP by default. Use `AA_SOCKET_PATH` (UDS) in production. |
| KI-003 | Sidecars Indistinguishable in Audit | Medium | Multiple sidecars share the same admin credential. Track sidecar ID in logs. |
| KI-004 | Ephemeral Agent Registry | Low | Agent registry is in-memory. Lost on broker restart. By design — agents re-register. |

---

## How AgentAuth Compares

| Capability | Shared API Keys | OAuth 2.0 | Cloud IAM | AgentAuth |
|------------|----------------|-----------|-----------|-----------|
| Unique agent identity | No | Possible | Possible | Yes (SPIFFE) |
| Task-scoped credentials | No | Rarely | Possible | Yes |
| Credential lifetime | Months | 15-60 min | 1-12 hours | 1-15 min |
| Immediate revocation | Rotate all | Sometimes | Propagation delay | 4-level |
| Per-agent audit trail | No | Partial | Yes | Yes (hash-chained) |
| Delegation verification | No | No | No | Yes (signed chain) |
| Mutual agent auth | No | No | No | Yes (3-step) |
| Built for AI agents | No | No | No | Yes |

---

## Vulnerability Reporting

**Email:** security@agentauth.dev

**Supported Versions:**

| Version | Supported |
|---------|-----------|
| 2.0.x | Yes (current) |
| 1.x | End of life |

**What to Include:**
- Description of the vulnerability
- Steps to reproduce
- Impact assessment
- Suggested fix (if any)

**Response Time:** 48 hours for initial acknowledgment.

---

## Production Security Checklist

- [ ] `AA_ADMIN_SECRET` is a strong random value (`openssl rand -hex 32`)
- [ ] TLS or mTLS enabled for broker-sidecar communication
- [ ] UDS mode for sidecar-application communication
- [ ] Sidecar scope ceilings are as narrow as possible
- [ ] Audit trail is monitored for `denied` outcomes
- [ ] Revocation procedures are documented and tested
- [ ] Broker runs as a non-root user with minimal filesystem access
- [ ] `AA_ADMIN_SECRET` is not committed to version control
- [ ] Token TTLs are appropriate (not longer than needed)
- [ ] Network access to broker is restricted (firewall rules)

---

## Case Study: CVE-2025-68664 (LangGrinch)

In December 2025, a critical vulnerability (CVSS 9.3) in langchain-core allowed attackers to execute code via serialization injection. Exfiltrated credentials gave attackers access to everything in the agent's environment.

**With AgentAuth:**
- No credentials stored in agent environment (obtained at runtime)
- Task-scoped: only current task's resources at risk
- Expires in minutes: exfiltrated tokens become useless
- Full audit trail detects anomalous access

**Lesson:** Agents should never hold long-lived secrets in their environment.

---

## Next Steps

- [[Architecture]] — System design details
- [[Known Issues]] — Current limitations
- [[Operator Guide]] — Production deployment
