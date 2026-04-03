# Common Tasks

Step-by-step recipes for everyday AgentAuth workflows. Each task includes examples in both Python and TypeScript.

> **Audience:** Developers and Operators
>
> **Prerequisites:** [[What is AgentAuth?]] for concepts, AgentAuth running locally

---

## Quick Reference

| Task | Endpoint | Role | Method |
|------|----------|------|--------|
| Get a Token | `POST /v1/token` | Developer | Sidecar |
| Validate a Token | `POST /v1/token/validate` | Developer | Broker |
| Renew a Token | `POST /v1/token/renew` | Developer | Sidecar |
| Release a Token | `POST /v1/token/release` | Developer | Broker |
| Delegate a Token | `POST /v1/delegate` | Developer | Broker |
| Check Health | `GET /v1/health` | Anyone | Either |
| Authenticate as Admin | `POST /v1/admin/auth` | Operator | Broker |
| Create Launch Token | `POST /v1/admin/launch-tokens` | Operator | Broker |
| Revoke Tokens | `POST /v1/revoke` | Operator | Broker |
| Query Audit Trail | `GET /v1/audit/events` | Operator | Broker |

---

## Developer Tasks

### Get a Token

The most common operation. Ask the sidecar for a scoped, short-lived token.

**Python:**
```python
import requests

SIDECAR = "http://localhost:8081"

resp = requests.post(f"{SIDECAR}/v1/token", json={
    "agent_name": "data-processor",
    "scope": ["read:data:*"],
    "ttl": 300,
    "task_id": "task-analyze-q4"
})

data = resp.json()
token = data["access_token"]
print(f"Token expires in {data['expires_in']}s")
```

**TypeScript:**
```typescript
const resp = await fetch("http://localhost:8081/v1/token", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    agent_name: "data-processor",
    scope: ["read:data:*"],
    ttl: 300,
    task_id: "task-analyze-q4"
  })
});

const data = await resp.json();
const token = data.access_token;
```

**curl:**
```bash
curl -s -X POST http://localhost:8081/v1/token \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"data-processor","scope":["read:data:*"],"ttl":300}' \
  | python3 -m json.tool
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent_name` | string | Yes | Name for this agent |
| `scope` | string[] | Yes | Requested permissions |
| `ttl` | int | No | Token lifetime in seconds (default: 300, max: 900) |
| `task_id` | string | No | Associates token with a task |

---

### Validate a Token

Check whether a token is valid and inspect its claims. No authentication required.

**Python:**
```python
BROKER = "http://localhost:8080"

resp = requests.post(f"{BROKER}/v1/token/validate", json={"token": token})
result = resp.json()

if result["valid"]:
    claims = result["claims"]
    print(f"Agent: {claims['sub']}")
    print(f"Scope: {claims['scope']}")
    print(f"JTI:   {claims['jti']}")
else:
    print(f"Invalid: {result.get('error')}")
```

**TypeScript:**
```typescript
const resp = await fetch("http://localhost:8080/v1/token/validate", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ token })
});

const result = await resp.json();
if (result.valid) {
  console.log(`Agent: ${result.claims.sub}`);
  console.log(`Scope: ${result.claims.scope}`);
}
```

---

### Renew a Token

Extend a token's lifetime before it expires.

**Python:**
```python
resp = requests.post(f"{SIDECAR}/v1/token/renew",
    headers={"Authorization": f"Bearer {token}"}
)

if resp.ok:
    new_token = resp.json()["access_token"]
    print(f"Renewed! New TTL: {resp.json()['expires_in']}s")
```

**TypeScript:**
```typescript
const resp = await fetch("http://localhost:8081/v1/token/renew", {
  method: "POST",
  headers: { Authorization: `Bearer ${token}` }
});

if (resp.ok) {
  const data = await resp.json();
  const newToken = data.access_token;
}
```

> **Pro tip:** Renew at 80% of the TTL. For a 300-second token, renew at 240 seconds.

---

### Release a Token

Signal task completion. Optional but creates a clean audit trail entry.

**Python:**
```python
resp = requests.post(f"{BROKER}/v1/token/release",
    headers={"Authorization": f"Bearer {token}"}
)
# Returns 204 No Content on success
```

**TypeScript:**
```typescript
await fetch("http://localhost:8080/v1/token/release", {
  method: "POST",
  headers: { Authorization: `Bearer ${token}` }
});
```

---

### Delegate to Another Agent

Give another agent a narrower version of your permissions.

**Python:**
```python
resp = requests.post(f"{BROKER}/v1/delegate",
    json={
        "delegate_to": "writer-agent",
        "scope": ["read:data:reports"],  # Must be narrower than your scope
        "ttl": 60                        # Shorter TTL is good practice
    },
    headers={"Authorization": f"Bearer {my_token}"}
)

delegated_token = resp.json()["access_token"]
# Deliver this token to the other agent
```

**TypeScript:**
```typescript
const resp = await fetch("http://localhost:8080/v1/delegate", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    Authorization: `Bearer ${myToken}`
  },
  body: JSON.stringify({
    delegate_to: "writer-agent",
    scope: ["read:data:reports"],
    ttl: 60
  })
});

const { access_token: delegatedToken } = await resp.json();
```

---

## Operator Tasks

### Authenticate as Admin

Get an admin token to manage the broker.

**Python:**
```python
import os

BROKER = "http://localhost:8080"
ADMIN_SECRET = os.environ["AA_ADMIN_SECRET"]

resp = requests.post(f"{BROKER}/v1/admin/auth", json={
    "client_id": "admin",
    "client_secret": ADMIN_SECRET
})

admin_token = resp.json()["access_token"]
```

**curl:**
```bash
curl -s -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"client_id\":\"admin\",\"client_secret\":\"$AA_ADMIN_SECRET\"}" \
  | python3 -m json.tool
```

> **Important:** Cache this token (it lasts 300 seconds). Don't re-authenticate for every request.

---

### Create a Launch Token

Create a single-use token for agent bootstrap.

**Python:**
```python
resp = requests.post(f"{BROKER}/v1/admin/launch-tokens",
    json={
        "agent_name": "new-agent",
        "allowed_scope": ["read:data:*"],
        "max_ttl": 300,
        "single_use": True,
        "ttl": 30  # Launch token expires in 30 seconds
    },
    headers={"Authorization": f"Bearer {admin_token}"}
)

launch_token = resp.json()["launch_token"]
print(f"Launch token (use within 30s): {launch_token}")
```

---

### Revoke Tokens

Cancel tokens at various levels.

**Python:**
```python
# Revoke a specific token
requests.post(f"{BROKER}/v1/revoke",
    json={"level": "token", "target": "jti-of-the-token"},
    headers={"Authorization": f"Bearer {admin_token}"}
)

# Revoke all tokens for an agent
requests.post(f"{BROKER}/v1/revoke",
    json={"level": "agent", "target": "spiffe://agentauth.local/agent/..."},
    headers={"Authorization": f"Bearer {admin_token}"}
)

# Revoke all tokens for a task
requests.post(f"{BROKER}/v1/revoke",
    json={"level": "task", "target": "task-123"},
    headers={"Authorization": f"Bearer {admin_token}"}
)

# Revoke an entire delegation chain
requests.post(f"{BROKER}/v1/revoke",
    json={"level": "chain", "target": "spiffe://agentauth.local/agent/root-agent/..."},
    headers={"Authorization": f"Bearer {admin_token}"}
)
```

**aactl:**
```bash
aactl revoke --level agent --target "spiffe://agentauth.local/agent/..."
```

---

### Query Audit Trail

Investigate what happened.

**Python:**
```python
# Get recent events
events = requests.get(f"{BROKER}/v1/audit/events",
    params={"limit": 50},
    headers={"Authorization": f"Bearer {admin_token}"}
).json()

for event in events["events"]:
    print(f"{event['timestamp']} | {event['event_type']:20s} | {event.get('agent_id', 'N/A')}")
```

**aactl:**
```bash
# All events
aactl audit events

# Filter by type
aactl audit events --event-type token_revoked

# Filter by agent
aactl audit events --agent-id "spiffe://agentauth.local/agent/..."

# Filter by time range
aactl audit events --since 2026-02-27T10:00:00Z --until 2026-02-27T12:00:00Z

# Show only failures
aactl audit events --outcome denied

# JSON output for scripts
aactl audit events --json
```

---

## Next Steps

- [[Integration Patterns]] — Architecture patterns for production
- [[API Reference]] — Complete endpoint documentation
- [[Troubleshooting]] — Fix common errors
- [[aactl CLI Reference]] — Full CLI documentation
