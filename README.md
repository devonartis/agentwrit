# AgentWrit

[![CI](https://github.com/devonartis/agentwrit/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/devonartis/agentwrit/actions/workflows/ci.yml)
[![CodeQL](https://github.com/devonartis/agentwrit/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/devonartis/agentwrit/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/devonartis/agentwrit/badge)](https://securityscorecards.dev/viewer/?uri=github.com/devonartis/agentwrit)
[![Go Reference](https://pkg.go.dev/badge/github.com/devonartis/agentwrit.svg)](https://pkg.go.dev/github.com/devonartis/agentwrit)
[![Go Report Card](https://goreportcard.com/badge/github.com/devonartis/agentwrit)](https://goreportcard.com/report/github.com/devonartis/agentwrit)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/compose/)
[![Security Policy](https://img.shields.io/badge/Security-Policy-green?logo=shield)](SECURITY.md)
[![EdDSA](https://img.shields.io/badge/Signing-Ed25519%20EdDSA-8B5CF6)](https://ed25519.cr.yp.to/)
[![SPIFFE](https://img.shields.io/badge/Identity-SPIFFE-0F9D58)](https://spiffe.io/)

> [!IMPORTANT]
> **Building in public — pre-1.0.** The broker core is stable and we use it daily, but the Python SDK and demo app are still landing. Feel free to try it as we build in the open. For anything non-lab, pin to a versioned tag like `v2.0.0` or a commit-pinned digest like `main-899e4ca3` — `:latest` moves with every `main` commit and will change without notice. Issues are welcome; external PRs are paused until the contribution workflow is ready. See [CHANGELOG.md](CHANGELOG.md) for what shipped recently.

---

## What is AgentWrit?

**AgentWrit gives AI agents temporary, task-scoped credentials instead of long-lived API keys.**

When an AI agent needs to do something — read a customer record, call a vendor API, run a query — it asks the AgentWrit broker for a token. The token works for that specific task, expires in minutes, and can be yanked at four different levels (token, agent, task, or entire delegation chain) the moment anything feels wrong. The agent never touches your long-lived credentials at all.

Think of it as an issuer of legal **writs** for software: narrow authority, time-limited, revocable at the source, with a tamper-evident audit trail of every credential event.

### The problem AgentWrit solves

When you give an AI agent a long-lived API key, the agent can do anything the key allows, for as long as the key exists. If the agent misbehaves, leaks the key, hits a prompt-injection, or is repurposed for a task you didn't sanction — the blast radius is everything that key can touch. Rotation is a manual, dangerous operation. Revocation is instant to decide and slow to implement.

AgentWrit inverts that model:

- **Tokens are requested per task**, not issued in advance
- **Scopes are narrow** — one task, one resource, one agent
- **Lifetimes are short** — minutes, not years — with renewal via a fresh broker call
- **Revocation is instant** at token, agent, task, or delegation-chain granularity
- **Every issue / renew / revoke / delegate / release event is audited** in a tamper-evident hash chain

The broker is the only component that holds your long-lived keys. Agents only ever see short-lived JWTs.

AgentWrit implements the [Ephemeral Agent Credentialing v1.3](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.3.md) security pattern — an 8-component architecture built specifically for autonomous agents. Tokens are signed with Ed25519 (EdDSA), agents have [SPIFFE](https://spiffe.io/)-format identities, and the broker talks plain HTTP so you can put it behind whatever ingress you already run.

More depth: [docs/concepts.md](docs/concepts.md).

---

## Release status

**Current:** v2.0.0 — pattern validation release. The broker is stable, the full token lifecycle works, but the surrounding ecosystem (Python SDK, demo, TypeScript SDK) is still being built in public. See the banner at the top and [CHANGELOG.md](CHANGELOG.md) for current state.

---

## Quick Start — five minutes from zero to first agent token

You're going to (1) start the broker, (2) authenticate as admin, (3) create a **launch token** (a one-shot registration credential), and (4) use that launch token to issue a scoped agent token. At the end you'll have a JWT your AI agent can hand to a resource server to prove it has permission for exactly one task.

Prerequisites: [Docker](https://docs.docker.com/get-docker/). That's it for Option A. (Option B needs Docker Compose and a clone, Option C needs Go 1.24+.)

### Option A: Pre-built image from Docker Hub (fastest)

The signed, multi-arch (`linux/amd64` + `linux/arm64`) broker image is published to Docker Hub on every push to `main` and on release tags. Pull and run directly — no git clone, no Go toolchain.

#### Step 1 — Set a strong admin secret and start the broker

```bash
# The broker exits on startup if AA_ADMIN_SECRET is unset. Use a real random value.
export AA_ADMIN_SECRET="$(openssl rand -base64 32)"

docker run -d --name agentwrit \
  -p 8080:8080 \
  -e AA_ADMIN_SECRET \
  -e AA_BIND_ADDRESS=0.0.0.0 \
  -e AA_DB_PATH=/data/data.db \
  -e AA_SIGNING_KEY_PATH=/data/signing.key \
  -v agentwrit-data:/data \
  devonartis/agentwrit:latest

# Confirm it's up
curl -s http://localhost:8080/v1/health | jq .
# {"status":"ok","version":"2.0.0","uptime":3,"db_connected":true,"audit_events_count":0}
```

**What the env vars do:**
- `AA_ADMIN_SECRET` — the root credential. Anyone who knows this can create launch tokens and revoke credentials. Treat like a root password.
- `AA_BIND_ADDRESS=0.0.0.0` — inside the container, bind to all interfaces so Docker's port forwarding can reach it. (The default `127.0.0.1` is for VPS mode.)
- `AA_DB_PATH` / `AA_SIGNING_KEY_PATH` — persist the SQLite audit log and the Ed25519 signing key to the named volume so they survive container restarts.

#### Step 2 — Authenticate as admin

The admin secret isn't a bearer token — you exchange it for a short-lived admin JWT:

```bash
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AA_ADMIN_SECRET\"}" | jq -r '.access_token')

echo "${ADMIN_TOKEN:0:20}..."  # should print "eyJhbGciOiJFZERTQSIs..."
```

#### Step 3 — Create a launch token

A launch token is a one-time registration credential. The agent presents it once (along with a signed challenge nonce) to get its real task-scoped JWT. You, the operator, decide what scopes the resulting agent is allowed to request:

```bash
LAUNCH_TOKEN=$(curl -s -X POST http://localhost:8080/v1/admin/launch-tokens \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "demo-agent",
    "orch_id": "quickstart",
    "allowed_scope": ["read:data:*"],
    "ttl_seconds": 300
  }' | jq -r '.launch_token')

echo "Launch token: ${LAUNCH_TOKEN:0:20}..."
```

#### Step 4 — Register the agent and get a scoped JWT

The agent generates an Ed25519 key pair, gets a nonce from the broker, signs it, and registers with the launch token plus the signed nonce. The broker verifies the signature, issues a SPIFFE identity, and returns a short-lived JWT scoped to `read:data:*`.

The crypto here is enough code that the easy path is the Python SDK:

```python
from agentauth import AgentAuthApp

# The SDK hides the challenge-response signing flow
agent = AgentAuthApp(broker_url="http://localhost:8080").register(
    launch_token=LAUNCH_TOKEN,
    task_id="read-customer-42",
    requested_scope=["read:data:customers:42"],  # must be subset of allowed_scope
)

# Now make a request to your resource server
import httpx
resp = httpx.get("https://your-api.example.com/customers/42",
                 headers=agent.bearer_header)

# When the task is done
agent.release()
```

**Want to see the raw HTTP registration flow** (challenge + signed nonce + register) without the SDK? See [docs/getting-started-user.md](docs/getting-started-user.md) for the manual 6-step walkthrough with curl + openssl. Good for understanding what the SDK is doing under the hood.

### Image tags and verification

Available Docker Hub tags:

| Tag format | Example | Moves | Use for |
|---|---|---|---|
| `latest` | `latest` | Every `main` push | Lab and evaluation |
| `main-<commit-sha>` | `main-899e4ca3…` | Every `main` push | Reproducible deployments — pins to a specific commit |
| `v<major>.<minor>.<patch>` | `v2.0.0` | Release tags | **Production** — pins to an exact semver release |
| `v<major>.<minor>` | `v2.0` | Release tags | Tracks patch updates within a minor version |
| `v<major>` | `v2` | Release tags | Tracks minor and patch updates within a major version |

Never pin production to `:latest`. Pin to a versioned tag (for example `v2.0.0`) or a commit-pinned digest (for example `main-899e4ca3`) and upgrade on a schedule you control.

**Verify the image signature** (optional, recommended for production):

```bash
cosign verify devonartis/agentwrit:latest \
  --certificate-identity-regexp='^https://github.com/devonartis/agentwrit/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

Signed keyless with Sigstore via GitHub Actions OIDC — no long-lived signing key to rotate. If verification fails, **do not run the image** and report the discrepancy.

### Option B: Docker Compose (clone + build locally)

```bash
# 1. Clone and enter the repo
git clone https://github.com/devonartis/agentwrit.git
cd agentwrit

# 2. Set the admin secret (required — broker exits without it)
export AA_ADMIN_SECRET="$(openssl rand -base64 32)"

# 3. Start the broker
./scripts/stack_up.sh

# 4. Verify the broker is healthy
curl -s http://localhost:8080/v1/health | jq .
# {"status":"ok","version":"2.0.0","uptime":5,"db_connected":true,"audit_events_count":0}

# 5. Authenticate as admin and get a token
curl -s -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AA_ADMIN_SECRET\"}" | jq .
# {"access_token":"eyJ...","expires_in":300,"token_type":"Bearer"}
```

To use the pre-built image with Compose instead of a local build, edit `docker-compose.yml` and replace the `build: .` block with `image: devonartis/agentwrit:latest`.

### Option C: Build from source

```bash
# 1. Build the broker and operator CLI
go build -o bin/broker ./cmd/broker/
go build -o bin/awrit  ./cmd/awrit/

# 2. Generate a config file with a secure admin secret
./bin/awrit init --config-path /tmp/agentwrit/config

# 3. Start the broker
AA_CONFIG_PATH=/tmp/agentwrit/config \
AA_DB_PATH=/tmp/agentwrit/data.db \
AA_SIGNING_KEY_PATH=/tmp/agentwrit/signing.key \
  ./bin/broker

# 4. In a new terminal, verify health
curl -s http://127.0.0.1:8080/v1/health | jq .
```

### What just happened?

The broker started an HTTP server on port 8080 with an Ed25519 signing key, a SQLite database for audit events and revocations, and a bcrypt-hashed admin secret. You authenticated with the admin secret and received a short-lived JWT. That token can now be used to register apps, create launch tokens, query the audit trail, or revoke credentials.

### Next steps

The typical workflow after setup is: create an app, issue a launch token for that app, then have your agent use the launch token to register and get its own scoped credential. See [Getting Started](docs/getting-started-user.md) for the full walkthrough.

---

## SDKs

| Language | Repo | Install | Status |
|----------|------|---------|--------|
| **Python** | [agentwrit-python](https://github.com/devonartis/agentwrit-python) | `pip install agentauth` *(PyPI rename pending)* | v0.3.0 — 15 acceptance tests passing |
| **TypeScript** | Coming soon | — | Planned |

The Python SDK wraps the broker's Ed25519 challenge-response flow into simple function calls:

```python
from agentauth import AgentAuthApp, validate

app = AgentAuthApp(
    broker_url="http://localhost:8080",
    client_id=os.environ["AGENTAUTH_CLIENT_ID"],
    client_secret=os.environ["AGENTAUTH_CLIENT_SECRET"],
)

# Create an agent with task-scoped credentials
agent = app.create_agent(
    orch_id="my-service",
    task_id="read-customer-data",
    requested_scope=["read:data:customers"],
)

# Use the token, then release when done
response = httpx.get(url, headers=agent.bearer_header)
agent.release()
```

See the [Python SDK documentation](https://github.com/devonartis/agentwrit-python) for the full API reference, delegation patterns, and error handling.

---

## See It In Action — MedAssist AI Demo

The Python SDK includes **MedAssist AI**, an interactive healthcare demo that showcases every AgentWrit capability against a live broker.

A FastAPI web app where you enter a patient ID and a plain-language request. A local LLM chooses which tools to call. The app dynamically creates broker agents with only the scopes those tools need, for that specific patient. You see scope enforcement, cross-patient denial, delegation, token renewal, and release — all in a real-time execution trace.

| Capability | How the demo shows it |
|------------|----------------------|
| **Dynamic agent creation** | Agents spawn on demand as the LLM selects tools — clinical, billing, prescription |
| **Per-patient scope isolation** | Each agent's scopes are parameterized to one patient ID |
| **Cross-patient denial** | LLM asks for another patient's records → `scope_denied` in the trace |
| **Delegation** | Clinical agent delegates `write:prescriptions:{patient}` to the prescription agent |
| **Token lifecycle** | Renewal and release shown at end of each encounter |
| **Audit trail** | Dedicated audit tab showing hash-chained broker events |

**Run it:** See the [MedAssist AI demo](https://github.com/devonartis/agentwrit-python/tree/main/demo) in the Python SDK repo, including a [beginner's guide](https://github.com/devonartis/agentwrit-python/blob/main/demo/BEGINNERS_GUIDE.md) with architecture diagrams and a [presenter's guide](https://github.com/devonartis/agentwrit-python/blob/main/demo/PRESENTERS_GUIDE.md) for live demos.

---

## Architecture

AgentWrit is a single broker binary. Operators manage it with the `awrit` CLI. Developers and agents interact with it over HTTP.

```mermaid
flowchart TB
    subgraph External["External Actors"]
        AGENT["AI Agent"]
        DEV["Developer App"]
        ADMIN["Operator"]
        RS["Resource Servers"]
    end

    subgraph AA["AgentWrit Broker :8080"]
        IDENTITY["Identity Service\nChallenge-response registration"]
        TOKEN["Token Service\nEdDSA JWT issue / verify / renew"]
        AUTHZ["Authz Middleware\nScope enforcement"]
        REVOKE["Revocation Service\n4-level revoke"]
        AUDIT["Audit Log\nHash-chain tamper-evident"]
        DELEG["Delegation Service\nScope attenuation"]
        APPSVC["App Service\nApp credential lifecycle"]
        ADMINSVC["Admin Service\nAdmin auth + launch tokens"]
        STORE["Store\nSQLite persistence"]
    end

    AACTL["awrit\nOperator CLI"]

    AGENT -- "POST /v1/register\n(launch token + signed nonce)" --> IDENTITY
    AGENT -- "Bearer token" --> RS
    DEV -- "POST /v1/app/auth\n(client credentials)" --> APPSVC
    DEV -- "POST /v1/delegate\n(scope-attenuated)" --> DELEG
    ADMIN --> AACTL
    AACTL -- "admin API calls" --> ADMINSVC

    IDENTITY --> TOKEN
    TOKEN --> STORE
    REVOKE --> STORE
    AUDIT --> STORE
```

### Components

| Component | Package | What it does |
|-----------|---------|-------------|
| **Identity Service** | `internal/identity` | Challenge-response agent registration. Issues SPIFFE IDs, verifies Ed25519 signatures against nonces. |
| **Token Service** | `internal/token` | Issues, verifies, and renews EdDSA JWTs. Enforces algorithm (EdDSA only), key ID matching, MaxTTL ceiling, and revocation checks. |
| **Authz Middleware** | `internal/authz` | Validates Bearer tokens and enforces scope requirements on every protected route. |
| **Revocation Service** | `internal/revoke` | Revokes credentials at four levels: single token, all tokens for an agent, all tokens for a task, or an entire delegation chain. |
| **Audit Log** | `internal/audit` | Records every credential event in a hash-chain (each entry includes a SHA-256 hash of the previous entry). Tamper-evident by design. |
| **Delegation Service** | `internal/deleg` | Creates scope-attenuated delegation tokens. A delegated token can never have more permissions than its parent. |
| **App Service** | `internal/app` | Manages application registrations (client_id/client_secret) and app-level authentication. |
| **Admin Service** | `internal/admin` | Admin secret authentication (bcrypt), launch token creation. |
| **Observability** | `internal/obs` | Structured logging with configurable verbosity, Prometheus metrics endpoint. |
| **Store** | `internal/store` | SQLite persistence for agents, apps, audit events, revocations, and nonces. |

See [Architecture](docs/architecture.md) for component diagrams, data flow diagrams, and design decisions.

---

## API Endpoints

### Public (no authentication required)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/health` | Health check — returns status, version, uptime, DB state |
| `GET` | `/v1/metrics` | Prometheus metrics |
| `GET` | `/v1/challenge` | Obtain a cryptographic nonce for agent registration (30s TTL) |
| `POST` | `/v1/token/validate` | Verify a token and return decoded claims |

### Agent and app authentication

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/v1/admin/auth` | None (rate-limited: 5/s) | Authenticate operator with admin secret. Returns `access_token`. |
| `POST` | `/v1/app/auth` | None (rate-limited: 10/min per client_id) | App credential exchange. Returns `access_token` with app scopes. |
| `POST` | `/v1/register` | Launch token | Register agent with signed nonce + public key. Returns `agent_id` and `access_token`. |

### Token operations (require Bearer token)

| Method | Path | Scope required | Description |
|--------|------|----------------|-------------|
| `POST` | `/v1/token/renew` | Any valid token | Renew token — old token is revoked, new token issued |
| `POST` | `/v1/token/release` | Any valid token | Self-revocation — agent signals task completion (returns 204) |
| `POST` | `/v1/delegate` | Any scope | Create scope-attenuated delegation token |

### Admin operations (require Bearer + admin scope)

| Method | Path | Scope required | Description |
|--------|------|----------------|-------------|
| `POST` | `/v1/admin/launch-tokens` | `admin:launch-tokens:*` | Create a launch token for agent registration |
| `POST` | `/v1/admin/apps` | `admin:launch-tokens:*` | Register a new app |
| `GET` | `/v1/admin/apps` | `admin:launch-tokens:*` | List all registered apps |
| `GET` | `/v1/admin/apps/{id}` | `admin:launch-tokens:*` | Get app details |
| `PUT` | `/v1/admin/apps/{id}` | `admin:launch-tokens:*` | Update app scopes or TTL |
| `DELETE` | `/v1/admin/apps/{id}` | `admin:launch-tokens:*` | Deregister an app |
| `POST` | `/v1/revoke` | `admin:revoke:*` | Revoke tokens (4 levels: token, agent, task, chain) |
| `GET` | `/v1/audit/events` | `admin:audit:*` | Query the audit trail |

All error responses use [RFC 7807](https://tools.ietf.org/html/rfc7807) `application/problem+json`. See the [API Reference](docs/api.md) for complete request/response schemas and examples.

---

## Configuration

All broker environment variables use the `AA_` prefix. The broker also reads config files generated by `awrit init` (see [Getting Started: Operator](docs/getting-started-operator.md)).

### Required

| Variable | Description |
|----------|-------------|
| `AA_ADMIN_SECRET` | Shared secret for admin authentication. Broker exits if unset. Use `awrit init` to generate one securely. |

### Broker settings

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_PORT` | `8080` | HTTP listen port |
| `AA_BIND_ADDRESS` | `127.0.0.1` | Bind address. Set to `0.0.0.0` for Docker containers. |
| `AA_LOG_LEVEL` | `verbose` | Logging verbosity: `quiet`, `standard`, `verbose`, `trace` |
| `AA_TRUST_DOMAIN` | `agentwrit.local` | SPIFFE trust domain for agent identity URIs |
| `AA_AUDIENCE` | *(empty)* | Expected JWT audience claim. Unset or empty skips audience validation. |
| `AA_SEED_TOKENS` | `false` | Print seed tokens on startup (dev/demo only) |

### Token lifetimes

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_DEFAULT_TTL` | `300` | Default token lifetime in seconds (5 minutes) |
| `AA_APP_TOKEN_TTL` | `1800` | App JWT lifetime in seconds (30 minutes) |
| `AA_MAX_TTL` | `86400` | Maximum token lifetime ceiling (24 hours). Set to `0` to disable. |

If `AA_DEFAULT_TTL` exceeds `AA_MAX_TTL`, the broker logs a warning at startup and clamps all issued tokens to the MaxTTL ceiling.

### Persistence

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_DB_PATH` | `./data.db` | SQLite database path (audit events, revocations, agents, apps) |
| `AA_SIGNING_KEY_PATH` | `./signing.key` | Ed25519 signing key path. Auto-generated on first startup. |
| `AA_CONFIG_PATH` | *(none)* | Path to config file from `awrit init`. Optional — env vars override config file values. |

### TLS / mTLS

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_TLS_MODE` | `none` | Transport security: `none`, `tls`, or `mtls` |
| `AA_TLS_CERT` | *(none)* | TLS certificate PEM path (required for `tls` and `mtls`) |
| `AA_TLS_KEY` | *(none)* | TLS private key PEM path (required for `tls` and `mtls`) |
| `AA_TLS_CLIENT_CA` | *(none)* | Client CA certificate PEM path (required for `mtls` only) |

### Operator CLI environment

| Variable | Description |
|----------|-------------|
| `AACTL_BROKER_URL` | Broker base URL (e.g., `http://localhost:8080`) |
| `AACTL_ADMIN_SECRET` | Admin secret for awrit authentication |

---

## Running Tests

```bash
# All tests
go test ./...

# Unit tests only (skip integration)
go test ./... -short

# Single package
go test ./internal/token/... -v

# Quality gates (build + lint + unit tests + security scan)
./scripts/gates.sh task

# Full gates (+ integration + Docker E2E)
./scripts/gates.sh module
```

---

## Docker Deployment

```bash
# Start the broker
export AA_ADMIN_SECRET="$(openssl rand -base64 32)"
./scripts/stack_up.sh

# Verify it's running
curl -s http://localhost:8080/v1/health | jq .

# Tear down (removes volumes)
./scripts/stack_down.sh
```

The Docker Compose stack runs the broker on port 8080 (override with `AA_HOST_PORT`), persists data to a named volume (`broker-data`), and binds to `0.0.0.0` inside the container for port forwarding.

---

## Operator CLI (awrit)

`awrit` is the operator's command-line tool for managing the AgentWrit broker. It auto-authenticates with the broker using `AACTL_BROKER_URL` and `AACTL_ADMIN_SECRET`.

```bash
# Build
go build -o bin/awrit ./cmd/awrit/

# Configure
export AACTL_BROKER_URL=http://localhost:8080
export AACTL_ADMIN_SECRET="your-admin-secret-here"
```

### Config generation

```bash
awrit init                                    # Dev mode (plaintext secret in config)
awrit init --mode=prod                        # Prod mode (bcrypt hash in config, plaintext shown once)
awrit init --force --config-path /etc/aa/cfg  # Force overwrite at custom path
```

### App management

```bash
awrit app register --name my-pipeline --scopes "read:data:*,write:logs:*"
awrit app list
awrit app get <app-id>
awrit app update --id <app-id> --scopes "read:data:*"
awrit app remove --id <app-id>
```

### Revocation and audit

```bash
awrit revoke --level token --target <jti>       # Revoke a single token
awrit revoke --level agent --target <agent-id>  # Revoke all tokens for an agent
awrit audit events                              # Full audit trail
awrit audit events --outcome denied --limit 20  # Filter for denied events
```

### Token operations

```bash
awrit token release --token <jwt>               # Self-revoke a token
```

All commands support `--json` for machine-readable output. See [awrit CLI Reference](docs/awrit-reference.md) for the complete command reference.

---

## Production Deployment

### TLS

```bash
export AA_TLS_MODE=tls
export AA_TLS_CERT=/path/to/cert.pem
export AA_TLS_KEY=/path/to/key.pem
```

### Mutual TLS (recommended for production)

```bash
export AA_TLS_MODE=mtls
export AA_TLS_CERT=/path/to/cert.pem
export AA_TLS_KEY=/path/to/key.pem
export AA_TLS_CLIENT_CA=/path/to/ca.pem
```

### Reverse proxy (TLS termination handled externally)

If your infrastructure handles TLS at the load balancer or reverse proxy, run the broker in `AA_TLS_MODE=none` behind nginx, Envoy, or Caddy:

```nginx
server {
    listen 443 ssl;
    server_name agentwrit.example.com;
    ssl_certificate     /etc/ssl/certs/agentwrit.pem;
    ssl_certificate_key /etc/ssl/private/agentwrit-key.pem;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

---

## Documentation

### Getting Started

| Guide | Audience | Description |
|-------|----------|-------------|
| [Getting Started](docs/getting-started-user.md) | Everyone | Installation, your first token, end-to-end walkthrough |
| [Getting Started: Developer](docs/getting-started-developer.md) | Developers | Python/TypeScript agent integration, token renewal patterns |
| [Getting Started: Operator](docs/getting-started-operator.md) | Operators | Broker deployment, config management, monitoring |

### Reference

| Document | Description |
|----------|-------------|
| [API Reference](docs/api.md) | Complete endpoint docs with request/response schemas and examples |
| [OpenAPI Spec](docs/api/openapi.yaml) | Machine-readable API contract (OpenAPI 3.0.3) |
| [Architecture](docs/architecture.md) | Component diagrams, data flows, middleware stack, design decisions |
| [Concepts](docs/concepts.md) | Security pattern, threat model, 8-component breakdown |
| [Common Tasks](docs/common-tasks.md) | Step-by-step workflows for developers and operators |
| [Troubleshooting](docs/troubleshooting.md) | Error messages, diagnostic flowchart, fixes by role |

### SDKs

| SDK | Description |
|-----|-------------|
| [Python SDK](https://github.com/devonartis/agentwrit-python) | Full SDK with agent lifecycle, delegation, scope checking, and the MedAssist AI demo |

### Guides

| Document | Description |
|----------|-------------|
| [Integration Patterns](docs/integration-patterns.md) | Real-world patterns with Python examples: multi-agent pipelines, delegation chains, token release, emergency revocation |
| [awrit CLI Reference](docs/awrit-reference.md) | Complete operator CLI reference: all commands, flags, examples |

### Project

| Document | Description |
|----------|-------------|
| [Code of Conduct](CODE_OF_CONDUCT.md) | Community standards |
| [Security Policy](SECURITY.md) | Vulnerability reporting, security design principles |
| [Changelog](CHANGELOG.md) | Release history |

---

## License

AgentWrit is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**. See [LICENSE](LICENSE) for the full text.

AGPL-3.0 Section 13 ("Remote Network Interaction") requires anyone offering modified AgentWrit as a network service to make the source code available. Self-hosting, local modification, and internal use are unrestricted. See [LICENSE](LICENSE) for the exact terms.
