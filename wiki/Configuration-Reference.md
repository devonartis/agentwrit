# Configuration Reference

Every environment variable for AgentAuth broker, sidecar, and aactl in one place.

---

## Broker Configuration

### Required

| Variable | Description |
|----------|-------------|
| `AA_ADMIN_SECRET` | Admin secret for authentication. **Must be set.** Generate with `openssl rand -hex 32`. |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_PORT` | `8080` | HTTP listen port |
| `AA_DB_PATH` | _(in-memory)_ | SQLite database path. Set for persistence. |
| `AA_LOG_LEVEL` | `standard` | `quiet`, `standard`, `verbose`, `trace` |

### Identity

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_TRUST_DOMAIN` | `agentauth.local` | SPIFFE trust domain |

### Tokens

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_DEFAULT_TTL` | `300` | Default token TTL (seconds) |
| `AA_MAX_TTL` | `900` | Maximum allowed TTL (seconds) |
| `AA_SEED_TOKENS` | _(empty)_ | Pre-provisioned launch tokens (dev only) |

### TLS

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_TLS_MODE` | `none` | `none`, `tls`, or `mtls` |
| `AA_TLS_CERT` | _(empty)_ | Server certificate PEM path |
| `AA_TLS_KEY` | _(empty)_ | Server private key PEM path |
| `AA_TLS_CA_CERT` | _(empty)_ | CA certificate for mTLS client verification |

---

## Sidecar Configuration

### Required

| Variable | Description |
|----------|-------------|
| `AA_ADMIN_SECRET` | Must match the broker's value |
| `AA_SIDECAR_SCOPE_CEILING` | Comma-separated max scopes (e.g., `read:data:*,write:data:*`) |

### Network

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_BROKER_URL` | `http://localhost:8080` | Broker URL |
| `AA_SIDECAR_PORT` | `8081` | TCP port (ignored when `AA_SOCKET_PATH` set) |
| `AA_SOCKET_PATH` | _(empty)_ | Unix domain socket path |

### TLS Client

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_SIDECAR_CA_CERT` | _(empty)_ | CA cert for broker TLS verification |
| `AA_SIDECAR_TLS_CERT` | _(empty)_ | Client cert for mTLS |
| `AA_SIDECAR_TLS_KEY` | _(empty)_ | Client key for mTLS |

### Circuit Breaker

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_SIDECAR_CB_WINDOW` | `30` | Sliding window (seconds) |
| `AA_SIDECAR_CB_THRESHOLD` | `0.5` | Failure rate to trip (0.0–1.0) |
| `AA_SIDECAR_CB_PROBE_INTERVAL` | `5` | Probe interval when open (seconds) |
| `AA_SIDECAR_CB_MIN_REQUESTS` | `5` | Min requests before tripping |

### Other

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_SIDECAR_LOG_LEVEL` | `standard` | `quiet`, `standard`, `verbose`, `trace` |
| `AA_SIDECAR_RENEWAL_BUFFER` | `0.8` | Self-renewal fraction (0.5–0.95) |

---

## aactl Configuration

| Variable | Description |
|----------|-------------|
| `AACTL_BROKER_URL` | Broker URL (e.g., `http://localhost:8080`) |
| `AACTL_ADMIN_SECRET` | Admin secret for authentication |

---

## Quick Setup Templates

### Development (.env)

```bash
AA_ADMIN_SECRET=dev-secret-change-me-in-production
AA_DB_PATH=./agentauth.db
AA_LOG_LEVEL=verbose
AA_TRUST_DOMAIN=agentauth.local
AA_DEFAULT_TTL=300
```

### Production (.env)

```bash
AA_ADMIN_SECRET=<64-char-hex-from-openssl-rand>
AA_DB_PATH=/var/lib/agentauth/agentauth.db
AA_LOG_LEVEL=standard
AA_TRUST_DOMAIN=your-company.agentauth
AA_DEFAULT_TTL=300
AA_MAX_TTL=900
AA_TLS_MODE=mtls
AA_TLS_CERT=/etc/agentauth/certs/broker.pem
AA_TLS_KEY=/etc/agentauth/certs/broker-key.pem
AA_TLS_CA_CERT=/etc/agentauth/certs/ca.pem
```

---

## Next Steps

- [[Operator Guide]] — Deployment guide
- [[Sidecar Deployment]] — Sidecar patterns
- [[Troubleshooting]] — Fix config errors
