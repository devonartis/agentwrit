# Operator Guide

Deploy, configure, and manage AgentAuth in production. This guide covers the broker, sidecars, TLS, monitoring, and the aactl CLI.

> **Audience:** Platform operators, DevOps engineers, SREs
>
> **Prerequisites:** Docker or Go 1.24+, basic command-line experience

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Broker Configuration](#broker-configuration)
3. [Sidecar Configuration](#sidecar-configuration)
4. [TLS and mTLS](#tls-and-mtls)
5. [The aactl CLI](#the-aactl-cli)
6. [Monitoring](#monitoring)
7. [Backup and Recovery](#backup-and-recovery)

---

## Quick Start

### Docker Compose (Recommended)

```bash
# Clone the repo
git clone https://github.com/devonartis/agentauth-core.git
cd agentauth-core

# Generate a strong admin secret
export AA_ADMIN_SECRET="$(openssl rand -hex 32)"

# Start broker + sidecar
docker compose up -d

# Verify
curl http://localhost:8080/v1/health   # Broker
curl http://localhost:8081/v1/health   # Sidecar
```

### Local Go Build

```bash
# Build broker and sidecar
go build -o broker ./cmd/broker
go build -o sidecar ./cmd/sidecar
go build -o aactl ./cmd/aactl

# Run broker
export AA_ADMIN_SECRET="$(openssl rand -hex 32)"
./broker &

# Run sidecar
export AA_BROKER_URL="http://localhost:8080"
export AA_SIDECAR_SCOPE_CEILING="read:data:*,write:data:*"
./sidecar &
```

---

## Broker Configuration

All configuration uses environment variables (prefix: `AA_`).

### Required

| Variable | Description |
|----------|-------------|
| `AA_ADMIN_SECRET` | Admin authentication secret. **Must be set.** Use `openssl rand -hex 32` to generate. |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_PORT` | `8080` | HTTP listen port |
| `AA_DB_PATH` | _(empty, in-memory)_ | SQLite database file path. Set for persistent storage. |
| `AA_TRUST_DOMAIN` | `agentauth.local` | SPIFFE trust domain for agent IDs |
| `AA_DEFAULT_TTL` | `300` | Default token TTL in seconds |
| `AA_MAX_TTL` | `900` | Maximum allowed token TTL |
| `AA_LOG_LEVEL` | `standard` | Log verbosity: `quiet`, `standard`, `verbose`, `trace` |
| `AA_TLS_MODE` | `none` | TLS mode: `none`, `tls`, `mtls` |
| `AA_TLS_CERT` | _(empty)_ | TLS certificate PEM path |
| `AA_TLS_KEY` | _(empty)_ | TLS private key PEM path |
| `AA_TLS_CA_CERT` | _(empty)_ | CA certificate for mTLS client verification |
| `AA_SEED_TOKENS` | _(empty)_ | Comma-separated pre-provisioned launch tokens |

### Security Notes

- **`AA_ADMIN_SECRET`** is the single most sensitive value. It grants full broker control. Treat it like a root password.
- **Ephemeral signing keys:** The broker generates a new Ed25519 signing key pair on every startup. Tokens issued by a previous broker instance cannot be validated by a new one.
- **`AA_SEED_TOKENS`** are convenience tokens for development. In production, create launch tokens on demand via the API.

---

## Sidecar Configuration

### Required

| Variable | Description |
|----------|-------------|
| `AA_ADMIN_SECRET` | Must match the broker's value |
| `AA_SIDECAR_SCOPE_CEILING` | Comma-separated maximum scopes (e.g., `read:data:*,write:data:*`) |

### Network

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_BROKER_URL` | `http://localhost:8080` | Broker URL |
| `AA_SIDECAR_PORT` | `8081` | TCP listen port |
| `AA_SOCKET_PATH` | _(empty)_ | Unix domain socket path (recommended for production) |

### Circuit Breaker

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_SIDECAR_CB_WINDOW` | `30` | Window duration (seconds) |
| `AA_SIDECAR_CB_THRESHOLD` | `0.5` | Failure rate to trip (0.0–1.0) |
| `AA_SIDECAR_CB_PROBE_INTERVAL` | `5` | Seconds between health probes when open |
| `AA_SIDECAR_CB_MIN_REQUESTS` | `5` | Min requests before circuit can trip |

### Other

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_SIDECAR_LOG_LEVEL` | `standard` | Verbosity |
| `AA_SIDECAR_RENEWAL_BUFFER` | `0.8` | Fraction of TTL at which to self-renew |

---

## TLS and mTLS

### TLS (Encrypted Communication)

```bash
# Generate test certificates
./scripts/gen_test_certs.sh

# Start with TLS
docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d
```

### mTLS (Mutual Authentication)

```bash
docker compose -f docker-compose.yml -f docker-compose.mtls.yml up -d
```

### Manual TLS Configuration

```bash
# Broker
export AA_TLS_MODE=tls
export AA_TLS_CERT=/path/to/cert.pem
export AA_TLS_KEY=/path/to/key.pem

# For mTLS, also set:
export AA_TLS_MODE=mtls
export AA_TLS_CA_CERT=/path/to/ca.pem  # Verify client certs

# Sidecar
export AA_BROKER_URL=https://broker:8080
export AA_SIDECAR_CA_CERT=/path/to/ca.pem
# For mTLS:
export AA_SIDECAR_TLS_CERT=/path/to/sidecar-cert.pem
export AA_SIDECAR_TLS_KEY=/path/to/sidecar-key.pem
```

---

## The aactl CLI

### Installation

```bash
go build -o aactl ./cmd/aactl
```

### Setup

```bash
export AACTL_BROKER_URL="http://localhost:8080"
export AACTL_ADMIN_SECRET="your-admin-secret"
```

### Common Commands

```bash
# List sidecars
aactl sidecars list

# Get sidecar scope ceiling
aactl sidecars ceiling get <sidecar-id>

# Update scope ceiling (may revoke excess tokens)
aactl sidecars ceiling set <sidecar-id> --scopes read:data:*

# Revoke tokens
aactl revoke --level agent --target "spiffe://..."
aactl revoke --level task --target "task-123"

# Query audit trail
aactl audit events
aactl audit events --event-type token_revoked --since 2026-02-27T10:00:00Z
aactl audit events --outcome denied

# JSON output for scripts
aactl --json sidecars list
aactl audit events --json | jq '.events | length'
```

See [[aactl CLI Reference]] for the complete reference.

---

## Monitoring

### Health Checks

```bash
# Broker health
curl http://localhost:8080/v1/health

# Sidecar health (includes scope ceiling)
curl http://localhost:8081/v1/health
```

### Metrics

```bash
# Prometheus-format metrics from sidecar
curl http://localhost:8081/v1/metrics
```

### Key Things to Monitor

| Metric / Signal | What It Means |
|-----------------|---------------|
| Health endpoint returns non-200 | Service is down |
| Increasing `denied` audit events | Possible abuse or misconfiguration |
| Circuit breaker opening | Broker may be overloaded or unreachable |
| Token renewal failures | Agents may lose access unexpectedly |

---

## Backup and Recovery

### Database

If using persistent storage (`AA_DB_PATH`), back up the SQLite file:

```bash
# Stop broker, copy DB, restart
docker compose stop broker
cp /data/agentauth.db /backup/agentauth-$(date +%Y%m%d).db
docker compose start broker
```

### Signing Keys

Signing keys are **ephemeral by design** — generated fresh on each broker startup. This means:
- No key backup needed
- Tokens from a previous broker instance are invalid after restart
- Agents must re-register after a broker restart

This is intentional: it limits the blast radius of a key compromise.

---

## Next Steps

- [[Sidecar Deployment]] — Detailed sidecar deployment patterns
- [[aactl CLI Reference]] — Complete CLI reference
- [[Configuration Reference]] — All environment variables in one place
- [[Troubleshooting]] — Fix common operator errors
