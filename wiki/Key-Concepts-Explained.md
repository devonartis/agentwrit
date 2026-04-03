# Key Concepts Explained

Every concept in AgentAuth explained in plain English, with examples in both Python and TypeScript.

---

## Table of Contents

1. [Tokens](#tokens)
2. [Scopes](#scopes)
3. [The Broker](#the-broker)
4. [The Sidecar](#the-sidecar)
5. [SPIFFE IDs (Agent Identity)](#spiffe-ids-agent-identity)
6. [TTL (Time to Live)](#ttl-time-to-live)
7. [Revocation](#revocation)
8. [Delegation](#delegation)
9. [Launch Tokens (The Bootstrap Problem)](#launch-tokens-the-bootstrap-problem)
10. [Audit Trail](#audit-trail)
11. [Scope Attenuation](#scope-attenuation)
12. [Mutual Authentication](#mutual-authentication)

---

## Tokens

### What Is a Token?

A token is a **temporary pass** that proves who an agent is and what it's allowed to do. It's a [JWT (JSON Web Token)](https://jwt.io/) — a standard format used across the web.

### What's Inside a Token?

A token contains **claims** — facts about the agent:

```json
{
  "iss": "agentauth",
  "sub": "spiffe://agentauth.local/agent/orch-001/task-42/proc-abc",
  "exp": 1745405630,
  "iat": 1745405330,
  "jti": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
  "scope": ["read:data:customers"],
  "task_id": "task-42",
  "orch_id": "orch-001"
}
```

| Claim | Meaning | Example |
|-------|---------|---------|
| `iss` | Who issued it | `"agentauth"` (always) |
| `sub` | Who the token belongs to | The agent's SPIFFE ID |
| `exp` | When it expires | Unix timestamp |
| `iat` | When it was issued | Unix timestamp |
| `jti` | Unique token ID | 32-character hex string |
| `scope` | What it allows | `["read:data:customers"]` |
| `task_id` | What task it's for | `"task-42"` |
| `orch_id` | Which orchestrator launched it | `"orch-001"` |

### How to Use a Token

Include it in the `Authorization` header of your HTTP requests:

**Python:**
```python
headers = {"Authorization": f"Bearer {token}"}
response = requests.get("https://api.example.com/customers", headers=headers)
```

**TypeScript:**
```typescript
const response = await fetch("https://api.example.com/customers", {
  headers: { Authorization: `Bearer ${token}` }
});
```

---

## Scopes

### What Is a Scope?

A scope defines **what the token allows**. It follows the format:

```
action:resource:identifier
```

### Examples

| Scope | Action | Resource | Identifier | Meaning |
|-------|--------|----------|------------|---------|
| `read:data:customers` | read | data | customers | Read customer data |
| `write:data:orders` | write | data | orders | Write to orders |
| `read:data:*` | read | data | * (wildcard) | Read any data |
| `admin:system:*` | admin | system | * | Full admin access to system |

### Wildcards

The `*` wildcard means "anything":

- `read:data:*` → read any data resource
- `*:*:*` → everything (use only in development!)

### Multiple Scopes

A token can have multiple scopes:

```python
# Request multiple scopes
response = requests.post(f"{SIDECAR}/v1/token", json={
    "agent_name": "multi-scope-agent",
    "scope": ["read:data:customers", "write:data:orders"],
    "ttl": 300
})
```

```typescript
const response = await fetch(`${SIDECAR_URL}/v1/token`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    agent_name: "multi-scope-agent",
    scope: ["read:data:customers", "write:data:orders"],
    ttl: 300
  })
});
```

### Checking Scopes in Your Code

When a resource server receives a request with a token, it should check the scope:

**Python:**
```python
def check_scope(required_scope: str, token_scopes: list[str]) -> bool:
    """Check if the token has the required scope."""
    for scope in token_scopes:
        action, resource, identifier = scope.split(":")
        req_action, req_resource, req_identifier = required_scope.split(":")
        
        if (action == req_action or action == "*") and \
           (resource == req_resource or resource == "*") and \
           (identifier == req_identifier or identifier == "*"):
            return True
    return False

# Example
token_scopes = ["read:data:*"]
print(check_scope("read:data:customers", token_scopes))  # True
print(check_scope("write:data:customers", token_scopes)) # False
```

**TypeScript:**
```typescript
function checkScope(requiredScope: string, tokenScopes: string[]): boolean {
  const [reqAction, reqResource, reqId] = requiredScope.split(":");
  
  return tokenScopes.some(scope => {
    const [action, resource, identifier] = scope.split(":");
    return (action === reqAction || action === "*") &&
           (resource === reqResource || resource === "*") &&
           (identifier === reqId || identifier === "*");
  });
}

// Example
const scopes = ["read:data:*"];
console.log(checkScope("read:data:customers", scopes));  // true
console.log(checkScope("write:data:customers", scopes)); // false
```

---

## The Broker

### What Is the Broker?

The broker is the **central security service** — the "security desk" that:
- Issues tokens to agents
- Validates tokens for resource servers
- Manages revocation
- Keeps the audit trail

### Key Facts

| Property | Value |
|----------|-------|
| Default port | `8080` |
| Required config | `AA_ADMIN_SECRET` (must be set) |
| Token signing | Ed25519 (EdDSA) |
| Storage | SQLite (ephemeral by default) |

### Broker Endpoints (Most Common)

| Endpoint | Method | What It Does | Who Calls It |
|----------|--------|-------------|--------------|
| `/v1/health` | GET | Check if broker is running | Anyone |
| `/v1/token/validate` | POST | Check if a token is valid | Resource servers |
| `/v1/admin/auth` | POST | Authenticate as admin | Operators, sidecars |
| `/v1/register` | POST | Register a new agent | Sidecars |
| `/v1/revoke` | POST | Revoke tokens | Operators |

---

## The Sidecar

### What Is the Sidecar?

The sidecar is a **helper proxy** that sits between your application and the broker. It handles all the cryptographic complexity so your code doesn't have to.

**Without sidecar** (hard way — ~50 lines of crypto code):
```python
# You have to do all this yourself:
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
import base64, requests

# 1. Generate Ed25519 key pair
private_key = Ed25519PrivateKey.generate()
public_key = private_key.public_key()

# 2. Get challenge nonce
challenge = requests.get(f"{BROKER}/v1/challenge").json()
nonce = bytes.fromhex(challenge["nonce"])

# 3. Sign the nonce
signature = private_key.sign(nonce)

# 4. Base64 encode everything
pub_b64 = base64.b64encode(public_key.public_bytes(...)).decode()
sig_b64 = base64.b64encode(signature).decode()

# 5. Register with the broker (need launch token too!)
result = requests.post(f"{BROKER}/v1/register", json={...})
```

**With sidecar** (easy way — 1 HTTP call):
```python
# One call. That's it.
result = requests.post(f"{SIDECAR}/v1/token", json={
    "agent_name": "my-agent",
    "scope": ["read:data:*"]
}).json()
token = result["access_token"]
```

### When to Use the Sidecar

**Always use the sidecar** unless you have a specific reason not to. The only reason to skip the sidecar is if you need full control over key management (see the BYOK pattern in [[Integration Patterns]]).

---

## SPIFFE IDs (Agent Identity)

### What Is a SPIFFE ID?

Every agent gets a unique identity string called a [SPIFFE ID](https://spiffe.io/). It looks like a URL:

```
spiffe://agentauth.local/agent/orch-001/task-analyze-q4/proc-abc123
```

### Breaking It Down

```
spiffe://agentauth.local / agent / orch-001    / task-analyze-q4 / proc-abc123
│        │                  │       │              │                 │
│        trust domain       │       orchestrator   task ID           instance ID
│                           │
scheme                      type (always "agent")
```

| Part | Meaning | Example |
|------|---------|---------|
| `spiffe://` | Standard SPIFFE scheme | Always `spiffe://` |
| `agentauth.local` | Your trust domain | Configurable via `AA_TRUST_DOMAIN` |
| `agent` | Entity type | Always `agent` |
| `orch-001` | Which orchestrator launched it | Set by orchestrator |
| `task-analyze-q4` | What task this agent handles | Set in token request |
| `proc-abc123` | Unique process identifier | Auto-generated |

### Why SPIFFE IDs Matter

- **Unique per agent:** Two agents doing the same task get different IDs
- **Auditable:** Every action in the audit trail includes the agent's SPIFFE ID
- **Revocable:** You can revoke all tokens for a specific agent by its SPIFFE ID

---

## TTL (Time to Live)

### What Is TTL?

TTL is **how long a token lasts** before it automatically expires. It's measured in seconds.

| TTL Value | Duration | Good For |
|-----------|----------|----------|
| `30` | 30 seconds | Quick lookups, one-shot tasks |
| `300` | 5 minutes | Standard tasks (default) |
| `900` | 15 minutes | Longer processing jobs |

### Default and Maximum

- **Default TTL:** 300 seconds (5 minutes)
- **Maximum TTL:** 900 seconds (15 minutes)
- **Minimum TTL:** No minimum, but very short TTLs (<10s) may cause renewal issues

### Choosing the Right TTL

**Rule of thumb:** Set TTL to 2x the expected task duration, then use renewal for longer tasks.

```python
# Short task (expected: 10 seconds)
"ttl": 30

# Standard task (expected: 2-3 minutes)
"ttl": 300

# Longer task with renewal
"ttl": 300  # Start with 5 minutes, renew at 80% (4 minutes)
```

---

## Revocation

### What Is Revocation?

Revocation is **canceling a token before it expires**. Think of it as "deactivate that badge immediately."

### Four Levels of Revocation

| Level | What It Revokes | When to Use It |
|-------|----------------|---------------|
| **Token** | One specific token (by JTI) | "Cancel this one credential" |
| **Agent** | All tokens for one agent (by SPIFFE ID) | "This agent is compromised" |
| **Task** | All tokens for one task (by task ID) | "Abort this entire task" |
| **Chain** | All tokens in a delegation chain | "The root delegator is compromised" |

### Example: Revoking a Token

**Python:**
```python
# First, authenticate as admin
admin_resp = requests.post(f"{BROKER}/v1/admin/auth", json={
    "client_id": "admin",
    "client_secret": "your-admin-secret"
})
admin_token = admin_resp.json()["access_token"]

# Revoke a specific token by its JTI
requests.post(f"{BROKER}/v1/revoke",
    json={"level": "token", "target": "a1b2c3d4e5f6..."},
    headers={"Authorization": f"Bearer {admin_token}"}
)

# Revoke ALL tokens for an agent
requests.post(f"{BROKER}/v1/revoke",
    json={"level": "agent", "target": "spiffe://agentauth.local/agent/..."},
    headers={"Authorization": f"Bearer {admin_token}"}
)
```

**TypeScript:**
```typescript
// Authenticate as admin
const adminResp = await fetch(`${BROKER_URL}/v1/admin/auth`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ client_id: "admin", client_secret: "your-admin-secret" })
});
const { access_token: adminToken } = await adminResp.json();

// Revoke by agent
await fetch(`${BROKER_URL}/v1/revoke`, {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    Authorization: `Bearer ${adminToken}`
  },
  body: JSON.stringify({ level: "agent", target: "spiffe://agentauth.local/agent/..." })
});
```

---

## Delegation

### What Is Delegation?

Delegation lets one agent **give a limited version of its permissions** to another agent. The delegated token can only have the same or narrower scope — never broader.

### How It Works

```
Agent A (scope: read:data:*)
    │
    ├──delegates──▶ Agent B (scope: read:data:customers)  ← narrower, OK
    │
    └──delegates──▶ Agent C (scope: write:data:*)          ← different action, REJECTED
```

### Example

**Python:**
```python
# Agent A delegates to Agent B with narrower scope
resp = requests.post(f"{BROKER}/v1/delegate",
    json={
        "delegate_to": "agent-b",
        "scope": ["read:data:customers"],  # Narrower than Agent A's read:data:*
        "ttl": 60  # Shorter TTL too
    },
    headers={"Authorization": f"Bearer {agent_a_token}"}
)
delegated_token = resp.json()["access_token"]
# Give delegated_token to Agent B
```

**TypeScript:**
```typescript
const resp = await fetch(`${BROKER_URL}/v1/delegate`, {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    Authorization: `Bearer ${agentAToken}`
  },
  body: JSON.stringify({
    delegate_to: "agent-b",
    scope: ["read:data:customers"],
    ttl: 60
  })
});
const { access_token: delegatedToken } = await resp.json();
```

### Rules
- Maximum delegation depth: **5 hops**
- Scope can only **narrow** (never expand)
- Each delegation is **signed** and recorded in the audit trail
- Revoking a delegator revokes all downstream delegates

---

## Launch Tokens (The Bootstrap Problem)

### The Problem

How does an agent get its **first** credential if it doesn't have any credentials yet? This is called the "secret zero" problem.

### The Solution: Launch Tokens

An operator creates a **single-use, short-lived launch token** that the agent uses exactly once to register:

```
Operator ──creates──▶ Launch Token (lives 30 seconds, single-use)
                           │
                           ▼
Agent ──uses once──▶ Registers with broker ──▶ Gets a real token
                           │
                    Launch token consumed
                    (can never be reused)
```

### Why This Is Secure

- **Short-lived:** Default 30-second TTL — tiny window for theft
- **Single-use:** Even if intercepted, can only be used once
- **Scope-capped:** Limits what the agent can request
- **On-demand:** Created when needed, not pre-provisioned

> **If you use the sidecar**, you don't need to worry about launch tokens at all. The sidecar handles the bootstrap process automatically.

---

## Audit Trail

### What Is the Audit Trail?

Every action in AgentAuth is logged in a **tamper-evident audit trail**. Each event is hash-chained (like a mini blockchain) so you can detect if logs have been modified.

### What Gets Logged

| Event | When It Happens |
|-------|----------------|
| `token_issued` | Agent receives a new token |
| `token_renewed` | Agent renews a token |
| `token_released` | Agent releases a token (task complete) |
| `token_revoked` | Operator revokes a token |
| `scope_narrowed` | Delegation with narrower scope |
| `agent_denied` | Agent request was rejected |

### Querying the Audit Trail

**Using aactl (CLI):**
```bash
# All events for a specific agent
aactl audit events --agent-id "spiffe://agentauth.local/agent/orch/task/proc"

# All denied events (possible security issues)
aactl audit events --outcome denied

# Events in the last hour
aactl audit events --since 2026-02-27T14:00:00Z
```

**Using the API (Python):**
```python
admin_token = authenticate_as_admin()
events = requests.get(f"{BROKER}/v1/audit/events",
    params={"event_type": "token_revoked", "limit": 50},
    headers={"Authorization": f"Bearer {admin_token}"}
).json()

for event in events["events"]:
    print(f"{event['timestamp']} | {event['event_type']} | {event['agent_id']}")
```

---

## Scope Attenuation

### What Is Scope Attenuation?

Scope attenuation means **permissions can only get narrower, never wider**, as tokens are delegated from one agent to another.

```
Orchestrator: read:data:*  (broadest)
    └──▶ Research Agent: read:data:*  (same or narrower)
             └──▶ Writer Agent: read:data:reports  (narrower)
                      └──▶ Reviewer: read:data:reports  (same or narrower)
```

### Why It Matters

Without scope attenuation, a compromised agent could delegate `admin:*:*` to itself. With attenuation, the worst a compromised agent can do is delegate what it already has.

---

## Mutual Authentication

### What Is Mutual Authentication?

Mutual authentication means **both sides verify each other**. Instead of just the server checking the client, the client also checks the server.

AgentAuth supports a 3-step handshake where two agents can verify each other's identity:

```
Agent A                          Agent B
   │                                │
   ├── 1. "Here's my token,       │
   │      prove you're Agent B" ──▶│
   │                                │
   │◀── 2. "Here's my token,      │
   │      here's my signature" ────│
   │                                │
   ├── 3. Verify signature         │
   │      against Agent B's        │
   │      registered public key    │
   │                                │
   ✓ Both verified                 ✓
```

> **Note:** Mutual authentication is currently a Go API only, not exposed as HTTP endpoints. Most users won't need this — it's for advanced multi-agent communication scenarios.

---

## Next Steps

- [[Your First Agent (Python)]] — Build something with these concepts
- [[Your First Agent (TypeScript)]] — TypeScript version
- [[Developer Guide]] — Full developer integration guide
- [[Home]] — Back to the wiki home
