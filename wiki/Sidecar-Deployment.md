# Sidecar Deployment

How to deploy, configure, and scale AgentAuth sidecars across different environments.

> **Audience:** Platform operators and DevOps engineers
>
> **Prerequisites:** [[Operator Guide]] for basic broker setup, [[Key Concepts Explained]] for why sidecars exist

---

## What Is a Sidecar?

The sidecar is a **lightweight proxy** between your applications and the broker. It handles all the cryptographic complexity so developers can get tokens with a single HTTP call.

Think of it like an AWS IAM instance profile: developers never see root credentials. The sidecar holds `AA_ADMIN_SECRET` and a scope ceiling; developers call `POST /v1/token` and get a scoped, short-lived JWT.

```
┌─────────────────────────────────────┐
│  Trust Boundary                     │
│                                     │
│  App 1 ──▶ Sidecar ──▶ Broker      │
│  App 2 ──▶   ↑                     │
│              (handles crypto,       │
│               key gen, registration)│
└─────────────────────────────────────┘
```

---

## Trust Boundaries: The Scaling Unit

**One sidecar per trust boundary. Not one per application. Not one per container.**

A trust boundary is a group of applications that share the same maximum permission set. Ask:

1. **Same maximum permissions?** If App A needs `read:data:*` and App B needs `write:billing:*`, they need separate sidecars.
2. **Compromise isolation?** If App A is compromised, could the attacker get tokens that affect App B? If that's unacceptable, split them.
3. **Same host/pod?** Apps sharing a filesystem are already in the same trust boundary.

### Examples

**Single-team services (one sidecar):**
```
Trust Boundary: data-team
├── ingestion-svc     → requests read:data:*, write:data:*
├── transform-svc     → requests read:data:*, write:data:*
├── reporting-svc     → requests read:data:*
└── Sidecar (ceiling: read:data:*, write:data:*)
```

**Multi-team platform (two sidecars):**
```
Trust Boundary: support-team
├── support-bot       → requests read:tickets:*, write:tickets:*
└── Sidecar A (ceiling: read:tickets:*, write:tickets:*)

Trust Boundary: billing-team
├── invoice-agent     → requests read:billing:*, write:billing:*
└── Sidecar B (ceiling: read:billing:*, write:billing:*)
```

---

## Deployment: Docker Compose

### Minimal Setup

```yaml
services:
  broker:
    build: { context: ., target: broker }
    ports: ["8080:8080"]
    environment:
      - AA_ADMIN_SECRET=${AA_ADMIN_SECRET}
      - AA_DB_PATH=/data/agentauth.db
    volumes: [broker-data:/data]
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/v1/health"]
      interval: 2s
      timeout: 3s
      retries: 10

  sidecar:
    build: { context: ., target: sidecar }
    ports: ["8081:8081"]
    environment:
      - AA_BROKER_URL=http://broker:8080
      - AA_ADMIN_SECRET=${AA_ADMIN_SECRET}
      - AA_SIDECAR_SCOPE_CEILING=read:data:*,write:data:*
      - AA_SIDECAR_PORT=8081
    depends_on:
      broker: { condition: service_healthy }

volumes:
  broker-data:
```

### Multiple Sidecars

```yaml
  sidecar-billing:
    build: { context: ., target: sidecar }
    ports: ["8082:8082"]
    environment:
      - AA_BROKER_URL=http://broker:8080
      - AA_ADMIN_SECRET=${AA_ADMIN_SECRET}
      - AA_SIDECAR_SCOPE_CEILING=read:billing:*,write:billing:*
      - AA_SIDECAR_PORT=8082
    depends_on:
      broker: { condition: service_healthy }
```

### UDS Mode (Production)

Unix domain sockets restrict access to processes sharing the socket file — no network exposure.

```bash
docker compose -f docker-compose.yml -f docker-compose.uds.yml up -d
```

Applications access the sidecar via the shared socket:
```bash
curl --unix-socket /var/run/agentauth/app1.sock \
  -X POST http://localhost/v1/token \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"my-agent","scope":["read:data:*"]}'
```

---

## Deployment: systemd (Bare Metal)

### Build Binaries

```bash
CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/agentauth-broker ./cmd/broker
CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/agentauth-sidecar ./cmd/sidecar
```

### Create System User

```bash
useradd --system --no-create-home --shell /usr/sbin/nologin agentauth
mkdir -p /var/lib/agentauth /var/run/agentauth /etc/agentauth
chown agentauth:agentauth /var/lib/agentauth /var/run/agentauth
```

### Broker Service

Create `/etc/systemd/system/agentauth-broker.service`:
```ini
[Unit]
Description=AgentAuth Broker
After=network.target

[Service]
Type=simple
User=agentauth
ExecStart=/usr/local/bin/agentauth-broker
EnvironmentFile=/etc/agentauth/broker.env
Environment=AA_DB_PATH=/var/lib/agentauth/agentauth.db
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=/var/lib/agentauth
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Sidecar Service

Create `/etc/systemd/system/agentauth-sidecar.service`:
```ini
[Unit]
Description=AgentAuth Sidecar
After=agentauth-broker.service
Requires=agentauth-broker.service

[Service]
Type=simple
User=agentauth
ExecStart=/usr/local/bin/agentauth-sidecar
EnvironmentFile=/etc/agentauth/sidecar.env
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=/var/run/agentauth
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Multiple Sidecars (systemd template)

Create `/etc/systemd/system/agentauth-sidecar@.service`:
```ini
[Unit]
Description=AgentAuth Sidecar (%i)
After=agentauth-broker.service

[Service]
Type=simple
User=agentauth
ExecStart=/usr/local/bin/agentauth-sidecar
EnvironmentFile=/etc/agentauth/sidecar-%i.env
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Then create per-boundary env files and enable:
```bash
sudo systemctl enable --now agentauth-sidecar@data
sudo systemctl enable --now agentauth-sidecar@billing
```

---

## Bootstrap Sequence

When a sidecar starts, it runs a 4-step auto-activation:

```
1. Start HTTP server (health + metrics only)
2. Wait for broker health → GET /v1/health
3. Authenticate → POST /v1/admin/auth
4. Activate → POST /v1/admin/sidecar-activations → POST /v1/sidecar/activate
5. Register token routes → READY
```

- Health endpoint available immediately (before bootstrap)
- Token endpoints available only after bootstrap succeeds
- Backoff: 1s, 2s, 4s, 8s, 16s, 32s, 60s (capped)
- Retries indefinitely — does not crash on bootstrap failure

---

## Circuit Breaker

The sidecar includes a circuit breaker to handle broker outages gracefully:

| State | Behavior |
|-------|----------|
| **Closed** | Normal operation — requests go to broker |
| **Open** | Broker is down — returns cached tokens or 503 |
| **Half-Open** | Probing — sends test requests to check broker recovery |

Configure with:
```bash
AA_SIDECAR_CB_WINDOW=30          # 30-second sliding window
AA_SIDECAR_CB_THRESHOLD=0.5      # Trip at 50% failure rate
AA_SIDECAR_CB_PROBE_INTERVAL=5   # Probe every 5 seconds when open
AA_SIDECAR_CB_MIN_REQUESTS=5     # Need 5 requests before tripping
```

---

## Runtime Scope Management

Operators can adjust sidecar scope ceilings at runtime using aactl:

```bash
# View current ceiling
aactl sidecars ceiling get <sidecar-id>

# Narrow the ceiling (revokes excess tokens automatically)
aactl sidecars ceiling set <sidecar-id> --scopes read:data:*

# Widen the ceiling
aactl sidecars ceiling set <sidecar-id> --scopes read:data:*,write:data:*
```

---

## Next Steps

- [[Operator Guide]] — Full operator guide
- [[aactl CLI Reference]] — Complete CLI reference
- [[Configuration Reference]] — All environment variables
- [[Troubleshooting]] — Fix deployment errors
