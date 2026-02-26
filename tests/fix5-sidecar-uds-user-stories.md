# User Stories — Fix 5: Sidecar UDS Listen Mode

**Fix branch:** `fix/sidecar-uds`
**Priority:** P1 (Operations / Security Hardening)
**Date:** 2026-02-24
**Related plan:** `plans/implementation-plan.md` — Fix 5

---

## Story 1 — Sidecar listens on a Unix socket instead of a TCP port

> As an **operator**, when I set `AA_SOCKET_PATH=/var/run/agentauth/myapp.sock`, I want the sidecar to listen on that Unix Domain Socket instead of a TCP port, so agent-to-sidecar traffic never touches the network and I can run many sidecars on one host without port conflicts or firewall rules.

**Acceptance criteria:**
- Start sidecar with `AA_SOCKET_PATH=/tmp/test-sidecar.sock`
- The socket file exists at that path
- `curl --unix-socket /tmp/test-sidecar.sock http://localhost/v1/health` returns 200
- No TCP port is opened by the sidecar

**Covered by:** `live_test.sh --fix5` → Story 1

---

## Story 2 — Developer requests tokens via Unix socket

> As a **3rd-party developer**, when the sidecar is configured with a Unix socket, I want to request tokens by connecting to that socket path, so my agent authenticates over a local-only channel with no network exposure.

**Acceptance criteria:**
- Sidecar is running on `AA_SOCKET_PATH=/tmp/test-sidecar.sock`
- `POST /v1/token` via the Unix socket returns a valid agent token
- `GET /v1/health` via the Unix socket returns sidecar health with `sidecar_id`

**Covered by:** `live_test.sh --fix5` → Story 2

---

## Story 3 — TCP mode requires explicit opt-in and logs a security warning

> As a **security engineer**, when `AA_SOCKET_PATH` is not set and the sidecar falls back to TCP, I want the sidecar to log a clear security warning that agent-to-sidecar traffic is exposed on the network, so operators are aware they are running in a less secure configuration and can remediate.

**Acceptance criteria:**
- Start sidecar with `AA_SOCKET_PATH` unset (TCP fallback on `AA_SIDECAR_PORT`)
- Sidecar starts and functions normally on TCP
- Broker logs include a `WARN`-level message: "sidecar listening on TCP — consider
  AA_SOCKET_PATH for production deployments"
- No silent fallback — the operator is always informed

**Covered by:** `live_test.sh --fix5` → Story 3

---

## Security Rationale

The sidecar's TCP listener is an attack surface: any process on the host can connect and
request tokens. UDS restricts access to processes with filesystem permissions on the
socket file. This aligns with Pattern v1.2's transport security requirements — Fix 1
secures broker-facing traffic with TLS/mTLS, and Fix 5 secures agent-facing traffic by
removing it from the network entirely. TCP fallback is preserved for backward
compatibility but should be treated as a development-only configuration.
