# Known Issues

Known security and operational issues with current mitigations and planned fixes.

---

## KI-001: Admin Secret Blast Radius (Security — High)

**Status:** Open — requires broker code change
**Found:** 2026-02-25 (ADR-002 sidecar architecture review)
**Affects:** All sidecar deployments

**Issue:** Every sidecar holds `AA_ADMIN_SECRET` at runtime (`cmd/sidecar/config.go:58`). This secret grants full admin scope: `admin:launch-tokens:*`, `admin:revoke:*`, `admin:audit:*` (`internal/admin/admin_svc.go:44-48`). The scope ceiling enforcement does NOT bound admin credentials. A compromised sidecar process has full admin access — it can revoke any token, read the full audit trail, create new sidecar activations with arbitrary ceilings, and create launch tokens for any agent.

The admin secret is used at runtime for every new agent registration (`handler.go:183`), not just bootstrap. Admin secret exposure grows with agent churn.

**Mitigation (current):** Operators must treat sidecar environments as having the same trust level as the broker itself. Use UDS mode (`AA_SOCKET_PATH`) to limit local access. Restrict container/host access to sidecar processes.

**Fix (planned):** New `POST /v1/sidecar/launch-tokens` broker endpoint gated behind `sidecar:manage:*` (already held by sidecar bearer token). Sidecars use their existing bearer for all operations; `AA_ADMIN_SECRET` needed only once at bootstrap and never stored afterward. See ADR-002 (archived — sidecar architecture decision) Action Plan item 3.

---

## KI-002: TCP Is the Default Listener Mode (Security — Medium)

**Status:** Open — documentation gap
**Found:** 2026-02-25 (ADR-002 sidecar architecture review)
**Affects:** All sidecar deployments that don't set `AA_SOCKET_PATH`

**Issue:** `cmd/sidecar/config.go:88` defaults `SocketPath` to empty string. Without `AA_SOCKET_PATH`, the sidecar listens on TCP (`listener.go:39`) with a `WARN` log as the only signal. In TCP mode, any process on the host can call `POST /v1/token` — no OS-level access control. The quickstart guide and Docker Compose default both demonstrate TCP.

**Mitigation (current):** Set `AA_SOCKET_PATH` in all production deployments. The sidecar logs a WARN when falling back to TCP.

**Fix (planned):** Update quickstart to show UDS as primary path. Add `AA_SOCKET_PATH` to Docker Compose (commented with production recommendation). Consider making UDS the default in a future release.

---

## KI-003: Sidecars Indistinguishable in Audit Trail (Operational — Medium)

**Status:** Open — requires per-sidecar credentials (blocked by KI-001 fix)
**Found:** 2026-02-25 (ADR-002 sidecar architecture review)
**Affects:** Multi-sidecar deployments

**Issue:** All sidecars use `client_id: "sidecar"` when communicating with the broker (`cmd/sidecar/broker_client.go:97-116`). The broker cannot distinguish which sidecar made which request. Combined with KI-001, a compromised sidecar leaves no distinguishable audit trail.

**Mitigation (current):** Correlate sidecar identity using the `sid` field in issued agent tokens (derived from broker-assigned sidecar JWT, not client-supplied).

**Fix (planned):** Per-sidecar credentials with unique `client_id` values, assigned at activation. Depends on KI-001 fix (narrow sidecar credential design).

---

## KI-004: Ephemeral Agent Registry (Operational — Low)

**Status:** By design — documentation needed
**Found:** 2026-02-25 (ADR-002 sidecar architecture review)
**Affects:** All sidecar deployments

**Issue:** The sidecar's agent registry is pure in-memory (`cmd/sidecar/registry.go`). A sidecar restart clears all registered agent entries. On the next `POST /v1/token` per agent after restart, the sidecar runs the full lazy registration flow (5 broker calls) before returning a token. This adds one-time latency per agent per sidecar restart.

**Mitigation:** This is intentional — the broker generates fresh Ed25519 signing keys on every startup, invalidating all pre-restart tokens regardless. Plan for registration latency spikes after sidecar restarts, especially for sidecars serving many agents.

**Fix:** None needed — document the behavior in operator guide.
