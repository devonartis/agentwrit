# AgentAuth Agent Integration Guide

This guide is for a new developer integrating a Python or TypeScript agent with the AgentAuth broker.

Goal: get a working agent identity flow with no guesswork.

If you follow this guide end-to-end, you will:
1. authenticate as admin
2. create a launch token for an agent
3. perform challenge-response registration with Ed25519
4. receive agent identity + short-lived token
5. validate and renew tokens
6. know common failure modes and exact fixes

Runtime policy for this guide:
1. always run broker + sidecar using Docker Compose (`./scripts/stack_up.sh`)
2. do not run the broker directly with `go run ./cmd/broker` for integration work

## 1. Pattern Questions This Guide Answers

This section explicitly answers the key security-pattern questions so a new developer can verify the implementation is aligned.

1. **How do we securely authenticate ephemeral agents?**
   - Agent proves key ownership by signing broker nonce (`GET /v1/challenge` -> `POST /v1/register`).
   - Broker verifies Ed25519 signature and issues unique SPIFFE-style `agent_id`.

2. **How do we authorize without over-privilege?**
   - Agent requests scope at registration.
   - Broker enforces `requested_scope` subset of launch-token `allowed_scope`.
   - Over-scope requests are rejected (`403`).

3. **How do we avoid long credential exposure windows?**
   - Tokens are short-lived.
   - Launch tokens are single-use and short TTL.
   - Renewal is explicit (`POST /v1/token/renew`) and can be denied after revocation.

4. **How do we solve bootstrap/secret-zero safely?**
   - Only orchestrator/admin handles `AA_ADMIN_SECRET`.
   - Orchestrator creates per-agent launch tokens; agent never gets admin secret.

5. **How do we contain compromise of one agent?**
   - Credentials are task-scoped and short-lived.
   - Revocation API supports token/agent/task/chain levels.
   - Agent code must fail closed on `401`/`403`.

6. **How do we maintain accountability?**
   - Broker records audit events for auth, registration, token lifecycle, revocation, and denials.
   - Developers can query evidence via `GET /v1/audit/events`.

7. **How do we support multi-agent workflows safely?**
   - Delegation endpoint (`POST /v1/delegate`) attenuates scope and preserves chain metadata.
   - Delegation overreach/depth violations are denied.

8. **What does this MVP intentionally not solve yet?**
   - Full production transport/deployment hardening.
   - LLM-behavior controls (prompt injection/model-level defenses).
   - Those are production hardening tracks, not agent-integration steps.

## 2. Pattern Component Coverage (Developer View)

| Pattern component | Broker capability | What Python/TS developer must do |
|---|---|---|
| Ephemeral identity issuance | Challenge + registration + SPIFFE-style agent ID | Generate Ed25519 keypair, sign nonce bytes correctly, register once per agent instance |
| Short-lived task-scoped tokens | Issue/validate/renew token lifecycle | Use least-privilege `requested_scope`, renew before expiry, handle token rotation |
| Zero-trust enforcement | Bearer validation and scope checks on protected endpoints | Always send Bearer token, treat `401/403` as hard authz failures |
| Expiration and revocation | TTL enforcement + revoke endpoints | Stop privileged actions on revoke/expiry; re-bootstrap through orchestrator |
| Immutable audit logging | Audit events + query API | Add run/test checks that verify expected audit events appear |
| Agent-to-agent mutual auth | MutAuth package is available for integration path | Use when direct peer trust is required; otherwise keep broker-mediated flow |
| Delegation chain verification | Delegation endpoint + chain constraints | Delegate only narrowed scopes; reject app paths that attempt escalation |

## 3. Prerequisites

1. Broker + sidecar running with Docker Compose:
```bash
export AA_ADMIN_SECRET="dev-admin-secret-change-me"
./scripts/stack_up.sh
```

2. Broker health check:
```bash
curl -s http://127.0.0.1:8080/v1/health
```

3. Tools:
```bash
# macOS example
brew install jq
```

4. Optional cleanup after you finish this guide:
```bash
./scripts/stack_down.sh
```

## 4. Contract You Must Follow

Critical encoding contract for `POST /v1/register`:
1. `public_key`: base64 of raw Ed25519 public key bytes (32 bytes)
2. `signature`: base64 of raw Ed25519 signature bytes (64 bytes)
3. signed message: nonce bytes (nonce is hex from `/v1/challenge`, decode hex before signing)

If you sign UTF-8 nonce text instead of decoded nonce bytes, registration will fail with signature errors.

## 5. End-to-End Flow (curl + scripts)

Set environment:
```bash
export BROKER="http://127.0.0.1:8080"
export ADMIN_SECRET="dev-admin-secret-change-me"
```

### Step 1: Admin auth
```bash
ADMIN_RESP=$(curl -sS -X POST "$BROKER/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"client_id\":\"admin\",\"client_secret\":\"$ADMIN_SECRET\"}")

echo "$ADMIN_RESP" | jq .
export ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jq -r '.access_token')
```

Expected: HTTP `200`, JSON includes `access_token`.

### Step 2: Create launch token
```bash
LT_RESP=$(curl -sS -X POST "$BROKER/v1/admin/launch-tokens" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "agent_name":"python-demo-agent",
    "allowed_scope":["read:Customers:12345"],
    "max_ttl":300,
    "single_use":true,
    "ttl":60
  }')

echo "$LT_RESP" | jq .
export LAUNCH_TOKEN=$(echo "$LT_RESP" | jq -r '.launch_token')
```

Expected: HTTP `201`, JSON includes `launch_token`.

### Step 3: Get challenge nonce
```bash
NONCE_RESP=$(curl -sS "$BROKER/v1/challenge")
echo "$NONCE_RESP" | jq .
export NONCE=$(echo "$NONCE_RESP" | jq -r '.nonce')
```

Expected: HTTP `200`, nonce hex string.

## 6. Python Integration (runnable)

Install:
```bash
python3 -m venv .venv
source .venv/bin/activate
pip install httpx cryptography
```

Create `python_agent_register.py`:
```python
import os
import json
import base64
import binascii
import httpx
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

BROKER = os.getenv("BROKER", "http://127.0.0.1:8080")
LAUNCH_TOKEN = os.environ["LAUNCH_TOKEN"]

# 1) Generate Ed25519 keypair (raw public key bytes)
priv = Ed25519PrivateKey.generate()
pub_raw = priv.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
pub_b64 = base64.b64encode(pub_raw).decode("ascii")

# 2) Challenge
challenge = httpx.get(f"{BROKER}/v1/challenge", timeout=5)
challenge.raise_for_status()
nonce_hex = challenge.json()["nonce"]

# 3) Sign nonce BYTES (hex-decoded)
nonce_bytes = binascii.unhexlify(nonce_hex)
sig = priv.sign(nonce_bytes)
sig_b64 = base64.b64encode(sig).decode("ascii")

# 4) Register
payload = {
    "launch_token": LAUNCH_TOKEN,
    "nonce": nonce_hex,
    "public_key": pub_b64,
    "signature": sig_b64,
    "orch_id": "orch-demo-001",
    "task_id": "task-demo-001",
    "requested_scope": ["read:Customers:12345"],
}
reg = httpx.post(f"{BROKER}/v1/register", json=payload, timeout=10)
print("register_status:", reg.status_code)
print(json.dumps(reg.json(), indent=2))
reg.raise_for_status()

access_token = reg.json()["access_token"]

# 5) Validate token
val = httpx.post(
    f"{BROKER}/v1/token/validate",
    json={"token": access_token},
    timeout=5,
)
print("validate_status:", val.status_code)
print(json.dumps(val.json(), indent=2))

# 6) Renew token
renew = httpx.post(
    f"{BROKER}/v1/token/renew",
    headers={"Authorization": f"Bearer {access_token}"},
    timeout=5,
)
print("renew_status:", renew.status_code)
print(json.dumps(renew.json(), indent=2))
```

Run:
```bash
export BROKER="http://127.0.0.1:8080"
export LAUNCH_TOKEN="$LAUNCH_TOKEN"
python3 python_agent_register.py
```

Expected:
1. `register_status: 200`
2. `validate_status: 200` and `"valid": true`
3. `renew_status: 200`

## 7. TypeScript Integration (runnable)

This example uses `tweetnacl` because it gives raw Ed25519 key bytes directly.

Install:
```bash
npm init -y
npm install tweetnacl
```

Create `ts_agent_register.mjs`:
```javascript
import nacl from "tweetnacl";

const BROKER = process.env.BROKER || "http://127.0.0.1:8080";
const LAUNCH_TOKEN = process.env.LAUNCH_TOKEN;
if (!LAUNCH_TOKEN) throw new Error("LAUNCH_TOKEN is required");

function b64(bytes) {
  return Buffer.from(bytes).toString("base64");
}

function hexToBytes(hex) {
  if (hex.length % 2 !== 0) throw new Error("invalid hex length");
  const out = new Uint8Array(hex.length / 2);
  for (let i = 0; i < hex.length; i += 2) {
    out[i / 2] = parseInt(hex.slice(i, i + 2), 16);
  }
  return out;
}

// 1) Ed25519 keypair (raw 32-byte public key)
const kp = nacl.sign.keyPair();
const pubB64 = b64(kp.publicKey);

// 2) Challenge
const challengeRes = await fetch(`${BROKER}/v1/challenge`);
if (!challengeRes.ok) throw new Error(`challenge failed: ${challengeRes.status}`);
const challenge = await challengeRes.json();
const nonceHex = challenge.nonce;

// 3) Sign nonce bytes (hex-decoded)
const nonceBytes = hexToBytes(nonceHex);
const sig = nacl.sign.detached(nonceBytes, kp.secretKey);
const sigB64 = b64(sig);

// 4) Register
const regRes = await fetch(`${BROKER}/v1/register`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    launch_token: LAUNCH_TOKEN,
    nonce: nonceHex,
    public_key: pubB64,
    signature: sigB64,
    orch_id: "orch-demo-001",
    task_id: "task-demo-001",
    requested_scope: ["read:Customers:12345"],
  }),
});
const regBody = await regRes.text();
console.log("register_status:", regRes.status);
console.log(regBody);
if (!regRes.ok) throw new Error("register failed");
const reg = JSON.parse(regBody);

// 5) Validate
const valRes = await fetch(`${BROKER}/v1/token/validate`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ token: reg.access_token }),
});
console.log("validate_status:", valRes.status);
console.log(await valRes.text());

// 6) Renew
const renewRes = await fetch(`${BROKER}/v1/token/renew`, {
  method: "POST",
  headers: { Authorization: `Bearer ${reg.access_token}` },
});
console.log("renew_status:", renewRes.status);
console.log(await renewRes.text());
```

Run:
```bash
export BROKER="http://127.0.0.1:8080"
export LAUNCH_TOKEN="$LAUNCH_TOKEN"
node ts_agent_register.mjs
```

## 8. How to Use Token in Your Agentic App

In your Python or TypeScript app:
1. bootstrap once (launch token -> register)
2. cache `access_token` in memory
3. attach `Authorization: Bearer <token>` on protected API requests
4. renew on timer (for example at 70-80% TTL)
5. on `401` or `403`, stop privileged actions and request re-bootstrap via orchestrator

Do not:
1. store admin secret in agent code
2. store launch token in persistent logs
3. keep reusing a single-use launch token

## 9. Troubleshooting (exact errors)

`401 invalid launch token` at `/v1/register`:
1. launch token expired
2. launch token already consumed
3. wrong token value passed

`401 nonce/signature failed`:
1. nonce signed as text instead of hex-decoded bytes
2. signature/public key not base64-encoded raw bytes
3. nonce reused after first attempt

`403 scope violation` at `/v1/register`:
1. `requested_scope` is broader than launch token `allowed_scope`

`429` at `/v1/admin/auth`:
1. rate limiting triggered
2. add exponential backoff with jitter

## 10. Minimum E2E Checklist for New Demo Apps

Before handing off a new Python/TS demo:
1. happy path works end-to-end (auth -> launch token -> challenge -> register -> validate -> renew)
2. launch token replay is denied
3. nonce replay is denied
4. over-scope registration is denied
5. revoked token is denied on subsequent protected call
6. audit query shows evidence for register/revoke/deny actions

## 11. Security Boundaries for MVP

MVP-valid:
1. local/dev/test environment
2. non-production data
3. broker-mediated identity and short-lived credentials

Not production-ready by default:
1. internet-facing deployment without hardened transport and infrastructure controls
2. unrestricted trust in deployment headers/proxy context
