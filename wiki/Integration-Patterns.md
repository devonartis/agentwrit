# Integration Patterns

Six proven patterns for integrating AgentAuth into production systems. Each pattern includes architecture diagrams, code examples in Python and TypeScript, and security analysis.

> **Audience:** Developers, architects, and platform engineers
>
> **Prerequisites:** [[Key Concepts Explained]], [[API Reference]]

---

## Table of Contents

1. [Multi-Agent Pipeline](#1-multi-agent-pipeline)
2. [Sidecar-Per-Microservice](#2-sidecar-per-microservice)
3. [Token Release as Task Completion](#3-token-release-as-task-completion)
4. [Delegation Chain with Scope Narrowing](#4-delegation-chain-with-scope-narrowing)
5. [Emergency Revocation Cascade](#5-emergency-revocation-cascade)
6. [BYOK Registration](#6-byok-registration)
7. [Security Checklist](#security-checklist)

---

## 1. Multi-Agent Pipeline

### Use Case

A sequential pipeline (e.g., research → write → review) where each agent gets progressively narrower access.

### Architecture

```
Orchestrator
    │
    ├─▶ Research Agent (scope: read:data:*)
    │       │
    │       └─▶ delegates ─▶ Writer Agent (scope: read:data:reports)
    │                           │
    │                           └─▶ delegates ─▶ Reviewer (scope: read:data:reports)
    │
    └── Each step: narrower scope, shorter TTL
```

### Python Example

```python
import requests
import os

SIDECAR = os.environ.get("AGENTAUTH_SIDECAR_URL", "http://localhost:8081")
BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "http://localhost:8080")

class PipelineAgent:
    """Base class for pipeline agents."""
    
    def __init__(self, name, token=None):
        self.name = name
        self.token = token
    
    def bootstrap(self, scope, task_id="pipeline-001"):
        """Get initial token from sidecar."""
        resp = requests.post(f"{SIDECAR}/v1/token", json={
            "agent_name": self.name,
            "scope": scope,
            "ttl": 300,
            "task_id": task_id
        })
        resp.raise_for_status()
        self.token = resp.json()["access_token"]
        return self
    
    def delegate(self, to_agent, scope, ttl=120):
        """Delegate narrower scope to another agent."""
        resp = requests.post(f"{BROKER}/v1/delegate",
            json={"delegate_to": to_agent, "scope": scope, "ttl": ttl},
            headers={"Authorization": f"Bearer {self.token}"}
        )
        resp.raise_for_status()
        return resp.json()["access_token"]
    
    def release(self):
        """Release token when done."""
        requests.post(f"{BROKER}/v1/token/release",
            headers={"Authorization": f"Bearer {self.token}"})


def run_pipeline():
    # Stage 1: Research Agent gets broad read access
    research = PipelineAgent("research-agent").bootstrap(["read:data:*"])
    # ... do research work ...
    
    # Stage 2: Delegate narrower scope to Writer
    writer_token = research.delegate("writer-agent", ["read:data:reports"], ttl=120)
    writer = PipelineAgent("writer-agent", token=writer_token)
    # ... do writing work ...
    
    # Stage 3: Delegate same-or-narrower scope to Reviewer
    reviewer_token = writer.delegate("reviewer-agent", ["read:data:reports"], ttl=60)
    reviewer = PipelineAgent("reviewer-agent", token=reviewer_token)
    # ... do review work ...
    
    # Clean up
    reviewer.release()
    writer.release()
    research.release()

run_pipeline()
```

### TypeScript Example

```typescript
const SIDECAR = process.env.AGENTAUTH_SIDECAR_URL || "http://localhost:8081";
const BROKER = process.env.AGENTAUTH_BROKER_URL || "http://localhost:8080";

class PipelineAgent {
  token: string | null = null;

  constructor(private name: string, token?: string) {
    if (token) this.token = token;
  }

  async bootstrap(scope: string[], taskId = "pipeline-001"): Promise<this> {
    const resp = await fetch(`${SIDECAR}/v1/token`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ agent_name: this.name, scope, ttl: 300, task_id: taskId }),
    });
    this.token = (await resp.json()).access_token;
    return this;
  }

  async delegate(toAgent: string, scope: string[], ttl = 120): Promise<string> {
    const resp = await fetch(`${BROKER}/v1/delegate`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${this.token}` },
      body: JSON.stringify({ delegate_to: toAgent, scope, ttl }),
    });
    return (await resp.json()).access_token;
  }

  async release(): Promise<void> {
    await fetch(`${BROKER}/v1/token/release`, {
      method: "POST",
      headers: { Authorization: `Bearer ${this.token}` },
    });
  }
}

async function runPipeline() {
  const research = await new PipelineAgent("research-agent").bootstrap(["read:data:*"]);
  const writerToken = await research.delegate("writer-agent", ["read:data:reports"], 120);
  const writer = new PipelineAgent("writer-agent", writerToken);
  const reviewerToken = await writer.delegate("reviewer-agent", ["read:data:reports"], 60);
  const reviewer = new PipelineAgent("reviewer-agent", reviewerToken);

  await reviewer.release();
  await writer.release();
  await research.release();
}
```

### Security Notes
- Scope **only narrows** down the chain — Writer can't escalate to `read:data:*`
- Revoking Research Agent automatically invalidates Writer and Reviewer
- Each delegation is signed and logged in the audit trail

---

## 2. Sidecar-Per-Microservice

### Use Case

Multiple services with different data access needs. Each service gets its own sidecar with a scope ceiling that prevents cross-domain access.

### Architecture

```
┌─ Customer Service ──────────┐  ┌─ Payment Service ───────────┐
│  App ──▶ Sidecar            │  │  App ──▶ Sidecar            │
│         (ceiling:read:pii:*)│  │         (ceiling:rw:billing:*)│
└─────────────────────────────┘  └─────────────────────────────┘
              │                              │
              └──────── Broker ──────────────┘
```

### Docker Compose

```yaml
services:
  broker:
    build: { context: ., target: broker }
    ports: ["8080:8080"]
    environment:
      - AA_ADMIN_SECRET=${AA_ADMIN_SECRET}

  sidecar-customer:
    build: { context: ., target: sidecar }
    ports: ["8081:8081"]
    environment:
      - AA_BROKER_URL=http://broker:8080
      - AA_ADMIN_SECRET=${AA_ADMIN_SECRET}
      - AA_SIDECAR_SCOPE_CEILING=read:pii:*
      - AA_SIDECAR_PORT=8081

  sidecar-payment:
    build: { context: ., target: sidecar }
    ports: ["8082:8082"]
    environment:
      - AA_BROKER_URL=http://broker:8080
      - AA_ADMIN_SECRET=${AA_ADMIN_SECRET}
      - AA_SIDECAR_SCOPE_CEILING=read:billing:*,write:billing:*
      - AA_SIDECAR_PORT=8082
```

---

## 3. Token Release as Task Completion

### Use Case

Use token release as a reliable signal that a task is done. The audit trail records the exact moment of completion.

### Python Example

```python
import time

def run_task_with_completion_signal(agent_name, scope, task_id):
    """Run a task and use token release as the completion signal."""
    # Get token
    resp = requests.post(f"{SIDECAR}/v1/token", json={
        "agent_name": agent_name,
        "scope": scope,
        "task_id": task_id,
        "ttl": 300
    })
    token = resp.json()["access_token"]
    start_time = time.time()
    
    try:
        # Do your work here
        result = do_actual_work(token)
        return result
    finally:
        # ALWAYS release the token, even if the task fails
        elapsed = time.time() - start_time
        requests.post(f"{BROKER}/v1/token/release",
            headers={"Authorization": f"Bearer {token}"})
        print(f"Task {task_id} completed in {elapsed:.1f}s")
```

### TypeScript Example

```typescript
async function runTaskWithCompletion(agentName: string, scope: string[], taskId: string) {
  const resp = await fetch(`${SIDECAR}/v1/token`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ agent_name: agentName, scope, task_id: taskId, ttl: 300 }),
  });
  const token = (await resp.json()).access_token;

  try {
    return await doActualWork(token);
  } finally {
    await fetch(`${BROKER}/v1/token/release`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
  }
}
```

---

## 4. Delegation Chain with Scope Narrowing

### Use Case

Deep agent hierarchies (up to 5 hops) where each level gets progressively narrower access.

```
Agent A (read:data:*)
  └─▶ Agent B (read:data:reports)       ← narrower
       └─▶ Agent C (read:data:reports)  ← same or narrower
            └─▶ Agent D (read:data:reports)
                 └─▶ Agent E (read:data:reports)  ← max depth (5)
```

### Rules

| Rule | Detail |
|------|--------|
| Max depth | 5 hops |
| Scope direction | Narrower only (never wider) |
| Revocation | Revoking a parent revokes all children |
| Chain integrity | SHA-256 chain hash in each token |

---

## 5. Emergency Revocation Cascade

### Use Case

An agent is compromised. Revoke everything associated with it immediately.

### Python Example

```python
def emergency_revoke(admin_token, compromised_agent_id, task_id=None):
    """Emergency revocation procedure."""
    headers = {"Authorization": f"Bearer {admin_token}"}
    
    # Step 1: Revoke the agent (all its tokens)
    resp = requests.post(f"{BROKER}/v1/revoke",
        json={"level": "agent", "target": compromised_agent_id},
        headers=headers)
    agent_result = resp.json()
    print(f"Agent revoked: {agent_result['count']} tokens")
    
    # Step 2: Revoke the delegation chain (all downstream delegates)
    resp = requests.post(f"{BROKER}/v1/revoke",
        json={"level": "chain", "target": compromised_agent_id},
        headers=headers)
    chain_result = resp.json()
    print(f"Chain revoked: {chain_result['count']} tokens")
    
    # Step 3: If task-level revocation needed
    if task_id:
        resp = requests.post(f"{BROKER}/v1/revoke",
            json={"level": "task", "target": task_id},
            headers=headers)
        task_result = resp.json()
        print(f"Task revoked: {task_result['count']} tokens")
    
    # Step 4: Audit what the agent did
    events = requests.get(f"{BROKER}/v1/audit/events",
        params={"agent_id": compromised_agent_id, "limit": 100},
        headers=headers).json()
    
    print(f"\nAudit trail ({len(events['events'])} events):")
    for event in events["events"]:
        print(f"  {event['timestamp']} | {event['event_type']} | {event.get('detail', '')[:60]}")
```

### aactl Commands

```bash
# Step 1: Revoke the agent
aactl revoke --level agent --target "spiffe://agentauth.local/agent/..."

# Step 2: Revoke the chain
aactl revoke --level chain --target "spiffe://agentauth.local/agent/..."

# Step 3: Audit
aactl audit events --agent-id "spiffe://agentauth.local/agent/..." --json
```

---

## 6. BYOK Registration

### Use Case

You want full control over key material — for HSM integration, custom key rotation, or compliance requirements.

See [[Developer Guide]] for the complete BYOK code example.

---

## Security Checklist

Before going to production, verify:

- [ ] **Sidecar scope ceilings** are as narrow as possible
- [ ] **TLS/mTLS** is enabled for broker-sidecar communication
- [ ] **UDS mode** is used for sidecar-app communication (no TCP exposure)
- [ ] **AA_ADMIN_SECRET** is a strong random value (not a default)
- [ ] **Token TTLs** are appropriate (not longer than needed)
- [ ] **Audit trail** is being monitored for denied events
- [ ] **Revocation procedures** are documented and tested
- [ ] **Delegation depth** is limited to what you actually need
- [ ] **Error handling** includes retry with exponential backoff
- [ ] **Token renewal** happens at 80% of TTL, not at expiry

---

## Next Steps

- [[Sidecar Deployment]] — Deploy sidecars in Docker, Kubernetes, or bare metal
- [[Troubleshooting]] — Fix common errors
- [[Security]] — Full security model and threat analysis
