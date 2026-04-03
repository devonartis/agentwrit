# What is AgentAuth?

AgentAuth is a **credential broker for AI agents**. It solves one big problem: **how do you give an AI agent access to things without giving it the keys to the kingdom?**

---

## The Problem (Why You Need This)

Let's say you build an AI agent that needs to:
- Read customer records from a database
- Call a payment API
- Write results to a file store

The common approach today? Give the agent an API key or service account with access to everything. That key lives in an environment variable, lasts forever, and works everywhere.

**What could go wrong?**

```
Your AI agent has:
  ✗ A permanent API key (never expires)
  ✗ Access to ALL databases (not just the one it needs)
  ✗ The same key as every other agent (no individual identity)
  ✗ No audit trail (which agent did what?)

If compromised:
  ✗ Attacker has full access to everything
  ✗ Key works until someone manually rotates it
  ✗ No way to know which agent was compromised
  ✗ Revoking the key breaks ALL agents
```

This is like giving every employee the same master key that never expires. If one person loses their key, you have to change every lock in the building.

---

## The Solution (What AgentAuth Does)

AgentAuth replaces permanent keys with **temporary, limited-scope tokens**:

```
Your AI agent has:
  ✓ A temporary token (expires in 5 minutes)
  ✓ Access to ONLY what it needs (read customer data, nothing else)
  ✓ Its own unique identity (spiffe://agentauth.local/agent/...)
  ✓ Full audit trail (every action is logged)

If compromised:
  ✓ Token expires in minutes — attacker's window is tiny
  ✓ Only the compromised agent's resources are at risk
  ✓ You can revoke just that one agent's access
  ✓ Other agents keep working normally
```

---

## The Architecture (How the Pieces Fit)

AgentAuth has three main pieces:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Your Agent │────▶│   Sidecar   │────▶│   Broker    │
│  (your code)│     │  (helper)   │     │  (security  │
│             │◀────│             │◀────│   desk)     │
└─────────────┘     └─────────────┘     └─────────────┘
       │                                       │
       ▼                                       ▼
┌─────────────┐                        ┌─────────────┐
│  Resource   │                        │ Audit Trail │
│  Server     │                        │  (logs)     │
└─────────────┘                        └─────────────┘
```

### The Broker (The Security Desk)
The broker is the brain of AgentAuth. It:
- Issues tokens to agents
- Validates tokens when resource servers check them
- Manages revocation (canceling tokens)
- Keeps an audit trail of everything

**Default port:** 8080

### The Sidecar (The Receptionist)
The sidecar is a helper that sits next to your application. It:
- Handles all the crypto (key generation, signatures) so you don't have to
- Gets tokens from the broker on your behalf
- Renews tokens before they expire
- Makes the whole process as simple as one HTTP call

**Default port:** 8081

### Your Agent (Your Code)
Your agent just needs to:
1. Ask the sidecar for a token (one HTTP request)
2. Use the token to access resources (put it in the `Authorization` header)
3. That's it. The sidecar handles renewal and expiration.

---

## The Token Lifecycle (Step by Step)

Here's what happens when your agent needs a credential:

### Step 1: Agent Requests a Token

Your agent sends a simple HTTP request to the sidecar:

**Python:**
```python
import requests

response = requests.post("http://localhost:8081/v1/token", json={
    "agent_name": "my-agent",
    "scope": ["read:data:customers"],
    "ttl": 300  # 5 minutes
})

token_data = response.json()
token = token_data["access_token"]
print(f"Got token! Expires in {token_data['expires_in']} seconds")
```

**TypeScript:**
```typescript
const response = await fetch("http://localhost:8081/v1/token", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    agent_name: "my-agent",
    scope: ["read:data:customers"],
    ttl: 300
  })
});

const tokenData = await response.json();
const token = tokenData.access_token;
console.log(`Got token! Expires in ${tokenData.expires_in} seconds`);
```

### Step 2: Agent Uses the Token

Include the token in your API calls:

**Python:**
```python
# Use the token to access a protected resource
headers = {"Authorization": f"Bearer {token}"}
data = requests.get("https://api.example.com/customers", headers=headers)
```

**TypeScript:**
```typescript
const data = await fetch("https://api.example.com/customers", {
  headers: { Authorization: `Bearer ${token}` }
});
```

### Step 3: Token Expires Automatically

After 5 minutes (or whatever TTL you set), the token stops working. No cleanup needed. If your agent needs more time, it can renew before expiration:

**Python:**
```python
# Renew before the token expires
renewed = requests.post("http://localhost:8081/v1/token/renew",
    headers={"Authorization": f"Bearer {token}"}
)
new_token = renewed.json()["access_token"]
```

**TypeScript:**
```typescript
const renewed = await fetch("http://localhost:8081/v1/token/renew", {
  method: "POST",
  headers: { Authorization: `Bearer ${token}` }
});
const newToken = (await renewed.json()).access_token;
```

---

## Scopes (What the Token Allows)

Scopes follow the pattern `action:resource:identifier`:

| Scope | Meaning |
|-------|---------|
| `read:data:customers` | Read customer data only |
| `read:data:*` | Read any data |
| `write:data:orders` | Write to orders only |
| `read:data:*` + `write:data:orders` | Read any data, write only orders |

**Scopes can only get narrower, never wider.** If your agent has `read:data:*`, it can delegate `read:data:customers` to another agent — but it can never escalate to `write:data:*`.

---

## SPIFFE IDs (Agent Identity)

Every agent gets a unique identity in the format:

```
spiffe://agentauth.local/agent/{orchestrator}/{task}/{instance}
```

For example:
```
spiffe://agentauth.local/agent/orch-001/task-analyze-q4/proc-abc123
```

This tells you:
- **Domain:** `agentauth.local` (your trust domain)
- **Orchestrator:** `orch-001` (who launched this agent)
- **Task:** `task-analyze-q4` (what task this agent is doing)
- **Instance:** `proc-abc123` (this specific agent process)

---

## When Should You Use AgentAuth?

**Good fit:**
- Multi-agent AI systems where agents need privileged access
- Short-lived tasks (minutes, not hours)
- Compliance requirements for least-privilege access
- Systems where you need to know exactly which agent did what

**Not a good fit:**
- Agents that run for hours or days (use credential rotation instead)
- Agents that only access public, non-sensitive resources
- Fully offline environments with no network access

---

## Next Steps

- **Hands-on:** [[Your First Agent (Python)]] — build a working agent in 15 minutes
- **Hands-on:** [[Your First Agent (TypeScript)]] — same tutorial in TypeScript
- **Concepts:** [[Key Concepts Explained]] — deeper dive into every concept
- **Home:** [[Home]] — back to the main page
