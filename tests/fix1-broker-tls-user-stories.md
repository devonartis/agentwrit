# User Stories — Fix 1: Broker TLS/mTLS

**Fix branch:** `fix/broker-tls`
**Date:** 2026-02-24
**Related plan:** `plans/implementation-plan.md` — Fix 1

---

## Story 1 — Default mode (no regression)

> As an operator, when I start the broker without any TLS env vars, it should start on plain HTTP exactly as before, so existing deployments are not broken.

**Acceptance criteria:**
- Broker starts with no `AA_TLS_MODE` set (defaults to `none`)
- `GET /v1/health` responds over `http://`
- All existing smoketest steps pass unchanged

**Covered by:** `live_test.sh --self-host`

---

## Story 2 — TLS mode

> As an operator, when I set `AA_TLS_MODE=tls` with a valid cert and key, the broker should serve HTTPS and reject plain HTTP connections, so traffic is encrypted in transit.

**Acceptance criteria:**
- Broker starts with `AA_TLS_MODE=tls`, `AA_TLS_CERT`, `AA_TLS_KEY` set
- `GET /v1/health` responds over `https://` with valid TLS handshake
- Response contains `"status"` field confirming broker is healthy

**Covered by:** `live_test.sh --tls`

---

## Story 3 — mTLS mode

> As an operator, when I set `AA_TLS_MODE=mtls` with a server cert, key, and client CA, the broker should require a valid client certificate on every connection, so only authorized clients can connect.

**Acceptance criteria:**
- Broker starts with `AA_TLS_MODE=mtls`, `AA_TLS_CERT`, `AA_TLS_KEY`, `AA_TLS_CLIENT_CA` set
- Client WITH a cert signed by the configured CA → connection succeeds, health returns `"status"`
- Client WITHOUT a cert → TLS handshake fails, connection rejected (not HTTP 200)

**Covered by:** `live_test.sh --mtls`

---

## Story 4 — Misconfiguration fails at startup

> As an operator, if I set `AA_TLS_MODE=tls` but provide a path to a missing cert file, the broker should fail to start with a clear error, so misconfiguration is caught immediately and not silently ignored.

**Acceptance criteria:**
- Broker started with `AA_TLS_CERT=/nonexistent/cert.pem` exits with non-zero code
- Process does not stay running

**Covered by:** `live_test.sh --tls` (Story 4 section)

---

## Test commands

```bash
# Story 1 — regression (no TLS)
./scripts/live_test.sh --self-host

# Story 2 — TLS mode
./scripts/live_test.sh --tls

# Story 3 + Story 4 — mTLS mode + bad cert
./scripts/live_test.sh --mtls

# Docker (run separately)
./scripts/live_test.sh --docker
```
