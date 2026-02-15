# Getting Started with AgentAuth

This guide walks you through installing AgentAuth and obtaining your first agent token.

## Prerequisites

- **Docker and Docker Compose** (recommended) or **Go 1.24+** for local builds
- **curl** or any HTTP client
- **openssl** (for Ed25519 key generation) or **Python 3** with `cryptography`

## Installation

### Option A: Docker Compose (recommended)

Clone the repository and start the stack:

```bash
git clone https://github.com/divineartis/agentauth.git
cd agentauth

export AA_ADMIN_SECRET="my-secret-key-change-in-production"
./scripts/stack_up.sh
```

This starts two containers:
- **Broker** on port 8080
- **Sidecar** on port 8081

Verify both are running:

```bash
curl http://localhost:8080/v1/health
```

Expected response:

```json
{"status": "ok", "version": "2.0.0", "uptime": 5}
```

### Option B: Local Go Build

```bash
git clone https://github.com/divineartis/agentauth.git
cd agentauth
go build ./...
```

Start the broker:

```bash
export AA_ADMIN_SECRET="my-secret-key-change-in-production"
go run ./cmd/broker
```

To also run the sidecar (in a second terminal):

```bash
export AA_ADMIN_SECRET="my-secret-key-change-in-production"
export AA_SIDECAR_SCOPE_CEILING="read:data:*,write:data:*"
go run ./cmd/sidecar
```

## Quick Start: Get a Token in 5 Steps

This section shows two paths: the **direct broker** flow (full challenge-response) and the **sidecar** flow (simplified).

### Path 1: Direct Broker (Challenge-Response)

#### Step 1: Authenticate as Admin

```bash
curl -s -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "admin",
    "client_secret": "my-secret-key-change-in-production"
  }'
```

Response:

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_in": 300,
  "token_type": "Bearer"
}
```

Save the admin token:

```bash
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id": "admin", "client_secret": "my-secret-key-change-in-production"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
```

#### Step 2: Create a Launch Token

```bash
curl -s -X POST http://localhost:8080/v1/admin/launch-tokens \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "agent_name": "my-agent",
    "allowed_scope": ["read:data:*"],
    "max_ttl": 300,
    "single_use": true,
    "ttl": 30
  }'
```

Response:

```json
{
  "launch_token": "a1b2c3d4e5f6...64-hex-chars",
  "expires_at": "2026-02-15T12:00:30Z",
  "policy": {
    "allowed_scope": ["read:data:*"],
    "max_ttl": 300
  }
}
```

Save the launch token:

```bash
LAUNCH_TOKEN="a1b2c3d4e5f6...paste-your-value-here"
```

#### Step 3: Get a Challenge Nonce

```bash
curl -s http://localhost:8080/v1/challenge
```

Response:

```json
{
  "nonce": "7f3a9c1b4d2e8f0a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8",
  "expires_in": 30
}
```

The nonce expires in 30 seconds. Complete step 4 before it expires.

#### Step 4: Generate Keys and Sign the Nonce

Using Python:

```bash
python3 -c "
import base64, json
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat, NoEncryption, PrivateFormat

# Generate Ed25519 key pair
private_key = Ed25519PrivateKey.generate()
public_key = private_key.public_key()

# Export keys
pub_bytes = public_key.public_bytes(Encoding.Raw, PublicFormat.Raw)
pub_b64 = base64.b64encode(pub_bytes).decode()

# Sign the nonce (hex-decoded)
nonce_hex = 'PASTE_YOUR_NONCE_HERE'
nonce_bytes = bytes.fromhex(nonce_hex)
sig = private_key.sign(nonce_bytes)
sig_b64 = base64.b64encode(sig).decode()

print(json.dumps({
    'public_key': pub_b64,
    'signature': sig_b64,
    'nonce': nonce_hex
}, indent=2))
"
```

#### Step 5: Register the Agent

```bash
curl -s -X POST http://localhost:8080/v1/register \
  -H "Content-Type: application/json" \
  -d '{
    "launch_token": "'"$LAUNCH_TOKEN"'",
    "nonce": "7f3a9c1b...paste-nonce-here",
    "public_key": "BASE64_PUBLIC_KEY",
    "signature": "BASE64_SIGNATURE",
    "orch_id": "my-orchestrator",
    "task_id": "task-001",
    "requested_scope": ["read:data:*"]
  }'
```

Response:

```json
{
  "agent_id": "spiffe://agentauth.local/agent/my-orchestrator/task-001/a1b2c3d4e5f6a7b8",
  "access_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_in": 300
}
```

The `access_token` is your agent's Bearer token. Use it in the `Authorization` header for authenticated endpoints.

### Path 2: Sidecar (Simplified)

The sidecar handles all the cryptography and registration automatically. One POST is all you need.

```bash
curl -s -X POST http://localhost:8081/v1/token \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "my-agent",
    "task_id": "task-001",
    "scope": ["read:data:*"],
    "ttl": 300
  }'
```

Response:

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_in": 300,
  "scope": ["read:data:*"],
  "agent_id": "spiffe://agentauth.local/agent/my-agent/task-001/c9d0e1f2a3b4c5d6"
}
```

The sidecar generates Ed25519 keys, completes the challenge-response flow with the broker, and returns the token. No manual key management needed.

## What You Just Did

AgentAuth uses a **challenge-response identity flow** to prove agent identity:

1. The broker issued a **nonce** (a random challenge).
2. Your agent signed the nonce with an **Ed25519 private key**, proving it holds the key.
3. A **launch token** authorized the agent to register with specific scopes.
4. The broker verified everything and issued a short-lived **JWT token** (default 5 minutes).
5. The agent received a **SPIFFE ID** -- a standard identity format (`spiffe://domain/agent/orch/task/instance`).

The token is short-lived by design. Agents must renew tokens before they expire or re-register.

## Next Steps

- [Common Tasks](common-tasks.md) -- renew tokens, delegate scopes, revoke access, query audits
- [Troubleshooting](troubleshooting.md) -- fix common errors
- [API Reference](api.md) -- full endpoint documentation
