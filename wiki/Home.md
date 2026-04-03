# Welcome to AgentAuth

**AgentAuth** is a security tool that gives AI agents temporary, limited-access passes instead of permanent master keys. Think of it like a visitor badge system for AI — each agent gets a badge that only works for a short time and only opens specific doors.

---

## Why Does This Exist?

Imagine you hire a temp worker. Would you give them:
- **A)** A master key to every room in the building, forever?
- **B)** A visitor badge that only opens the rooms they need, and expires when their shift ends?

Obviously **B**. But in most AI systems today, agents get option A — long-lived API keys with broad access. If an agent gets compromised, the attacker has the keys to everything.

**AgentAuth fixes this** by giving each agent a short-lived, limited-scope credential that expires in minutes.

---

## How It Works (The Simple Version)

```
1. Agent asks for a badge     →  "I need to read customer data for 5 minutes"
2. AgentAuth checks the rules →  "You're allowed to read customer data. Here's your badge."
3. Agent uses the badge        →  Shows badge to access customer database
4. Badge expires automatically →  After 5 minutes, the badge stops working
```

That's it. No permanent keys. No broad access. No cleanup needed.

---

## Key Ideas (In Plain English)

| Concept | What It Means | Real-World Analogy |
|---------|--------------|-------------------|
| **Token** | A temporary pass for an agent | A visitor badge |
| **Scope** | What the pass allows | "This badge opens Room 101 only" |
| **Broker** | The system that issues passes | The security desk |
| **Sidecar** | A helper that handles passes for your agent | A receptionist who gets the badge for you |
| **TTL** | How long the pass lasts (default: 5 minutes) | "Badge expires at 3:00 PM" |
| **Revocation** | Canceling a pass early | "Deactivate that visitor's badge immediately" |

---

## Quick Start (Get Running in 2 Minutes)

### What You Need
- [Docker](https://docs.docker.com/get-docker/) installed on your computer
- A terminal (Command Prompt, Terminal, or PowerShell)

### Step 1: Start AgentAuth

```bash
# Copy this entire block and paste it into your terminal
git clone https://github.com/devonartis/agentauth-core.git
cd agentauth-core
AA_ADMIN_SECRET="my-super-secret-key-change-me" docker compose up -d
```

> **What just happened?** You started two services:
> - **Broker** on port 8080 — the "security desk" that issues tokens
> - **Sidecar** on port 8081 — the "receptionist" that makes getting tokens easy

### Step 2: Get Your First Token

```bash
curl -s -X POST http://localhost:8081/v1/token \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "my-first-agent",
    "scope": ["read:data:*"],
    "ttl": 300
  }' | python3 -m json.tool
```

> **What just happened?** You asked the sidecar for a token that:
> - Identifies your agent as `my-first-agent`
> - Allows reading data (`read:data:*`)
> - Expires in 300 seconds (5 minutes)

You should see a response like:
```json
{
    "access_token": "eyJhbGciOi...(long string)...",
    "expires_in": 300,
    "scope": ["read:data:*"],
    "agent_id": "spiffe://agentauth.local/agent/...",
    "token_type": "Bearer"
}
```

**Congratulations!** You just issued your first ephemeral agent credential!

### Step 3: Clean Up

```bash
docker compose down
```

---

## Wiki Pages

### For Beginners
- [[What is AgentAuth?]] — Detailed explanation with diagrams
- [[Your First Agent (Python)]] — Build a simple agent step by step
- [[Your First Agent (TypeScript)]] — Same tutorial in TypeScript
- [[Key Concepts Explained]] — Every concept explained simply

### For Developers
- [[Developer Guide]] — Integrate AgentAuth into your applications
- [[Common Tasks]] — Step-by-step recipes for everyday workflows
- [[Integration Patterns]] — Real-world architecture patterns
- [[API Reference]] — Complete endpoint documentation

### For Operators
- [[Operator Guide]] — Deploy and manage AgentAuth
- [[Sidecar Deployment]] — Configure and deploy sidecars
- [[aactl CLI Reference]] — Command-line tool for operators
- [[Configuration Reference]] — All environment variables

### Reference
- [[Architecture]] — System design and components
- [[Troubleshooting]] — Fix common errors
- [[Security]] — Security model and threat analysis
- [[Known Issues]] — Current limitations and workarounds
- [[Contributing]] — How to contribute to AgentAuth
- [[Changelog]] — Version history
