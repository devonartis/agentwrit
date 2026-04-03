# Known Issues

Tracked issues with current mitigations. These are known limitations, not bugs.

---

## KI-001: Admin Secret Blast Radius

**Severity:** High

**Issue:** A single `AA_ADMIN_SECRET` grants full control over the broker. Anyone with this secret can:
- Create launch tokens with any scope
- Revoke any token
- Query the full audit trail
- Manage all sidecars

**Current Mitigation:**
- Treat `AA_ADMIN_SECRET` like a root password
- Rotate regularly
- Store in a secrets manager (not in code or environment files)
- Restrict network access to the broker

**Future:** Role-based admin access is planned.

---

## KI-002: TCP Is the Default Listener Mode

**Severity:** Medium

**Issue:** The sidecar listens on TCP (port 8081) by default. Any process that can reach the sidecar's TCP port can request tokens.

**Current Mitigation:**
- In production, use UDS mode: `AA_SOCKET_PATH=/var/run/agentauth/sidecar.sock`
- UDS restricts access to processes that share the socket file
- Use network policies (firewall rules, Kubernetes NetworkPolicy) if TCP is required

---

## KI-003: Sidecars Indistinguishable in Audit Trail

**Severity:** Medium

**Issue:** Multiple sidecars authenticate with the same `AA_ADMIN_SECRET`. The audit trail records admin operations but doesn't differentiate which sidecar performed them.

**Current Mitigation:**
- Track sidecar IDs in deployment-level logs (Docker, Kubernetes)
- Correlate sidecar IDs with audit event timestamps
- Use separate admin secrets per environment (dev vs. prod)

**Future:** Per-sidecar credentials are planned.

---

## KI-004: Ephemeral Agent Registry

**Severity:** Low

**Issue:** The agent registry is in-memory by default. When the broker restarts, all registered agents are lost. Agents must re-register after a broker restart.

**Current Mitigation:**
- This is by design — ephemeral state limits the blast radius of compromise
- Sidecars handle re-registration automatically
- Set `AA_DB_PATH` for SQLite persistence if needed

---

## Reporting New Issues

File issues at: https://github.com/devonartis/agentauth-core/issues

For security vulnerabilities: security@agentauth.dev

---

## Next Steps

- [[Security]] — Full security model
- [[Troubleshooting]] — Fix errors
- [[Operator Guide]] — Deployment guide
