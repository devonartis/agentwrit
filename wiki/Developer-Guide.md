# Developer Guide

The complete guide for developers integrating AgentAuth into AI agent applications. Covers both Python and TypeScript with production-ready patterns.

> **Audience:** Developers building AI agents
>
> **Prerequisites:** [[What is AgentAuth?]] and [[Key Concepts Explained]]
>
> **Time:** 30 minutes for the full guide, or jump to the section you need

---

## Table of Contents

1. [Choose Your Integration Path](#choose-your-integration-path)
2. [The Simple Path: Sidecar-Managed Keys](#the-simple-path-sidecar-managed-keys)
3. [The BYOK Path: Bring Your Own Keys](#the-byok-path-bring-your-own-keys)
4. [Enforcing Scopes in Your Resource Server](#enforcing-scopes-in-your-resource-server)
5. [Token Renewal Strategies](#token-renewal-strategies)
6. [Error Handling](#error-handling)
7. [Production Checklist](#production-checklist)

---

## Choose Your Integration Path

| | Sidecar Path (Recommended) | BYOK Path (Advanced) |
|---|---|---|
| **Complexity** | 1 HTTP call | ~50 lines of crypto code |
| **Key management** | Sidecar handles it | You manage keys |
| **Best for** | Most applications | Custom key rotation, HSM integration |
| **Setup** | Deploy a sidecar | Just a broker |

**If you're not sure, use the sidecar path.** It's simpler, safer, and handles edge cases for you.

---

## The Simple Path: Sidecar-Managed Keys

### Python

```python
"""
Complete sidecar integration example.
The sidecar handles key generation, challenge-response, and registration.
"""
import requests
import os

SIDECAR = os.environ.get("AGENTAUTH_SIDECAR_URL", "http://localhost:8081")

def get_agent_token(agent_name: str, scope: list[str], 
                    ttl: int = 300, task_id: str = None) -> dict:
    """Get a scoped token from the sidecar."""
    payload = {
        "agent_name": agent_name,
        "scope": scope,
        "ttl": ttl,
    }
    if task_id:
        payload["task_id"] = task_id

    resp = requests.post(f"{SIDECAR}/v1/token", json=payload, timeout=10)
    resp.raise_for_status()
    return resp.json()


# Usage
token_data = get_agent_token(
    agent_name="data-processor",
    scope=["read:data:customers"],
    ttl=300,
    task_id="task-process-q4"
)

token = token_data["access_token"]
print(f"Agent: {token_data['agent_id']}")
print(f"Scope: {token_data['scope']}")
print(f"TTL:   {token_data['expires_in']}s")
```

### TypeScript

```typescript
const SIDECAR = process.env.AGENTAUTH_SIDECAR_URL || "http://localhost:8081";

interface TokenResponse {
  access_token: string;
  expires_in: number;
  scope: string[];
  agent_id: string;
  token_type: string;
}

async function getAgentToken(
  agentName: string,
  scope: string[],
  ttl = 300,
  taskId?: string
): Promise<TokenResponse> {
  const payload: Record<string, unknown> = {
    agent_name: agentName,
    scope,
    ttl,
  };
  if (taskId) payload.task_id = taskId;

  const resp = await fetch(`${SIDECAR}/v1/token`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  if (!resp.ok) {
    throw new Error(`Token request failed: ${resp.status} - ${await resp.text()}`);
  }

  return resp.json() as Promise<TokenResponse>;
}

// Usage
const tokenData = await getAgentToken("data-processor", ["read:data:customers"], 300, "task-process-q4");
console.log(`Agent: ${tokenData.agent_id}`);
```

---

## The BYOK Path: Bring Your Own Keys

For advanced use cases where you manage your own Ed25519 keys.

### Python

```python
"""
BYOK (Bring Your Own Key) integration.
You generate and manage the Ed25519 key pair yourself.
"""
import requests
import base64
import os
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "http://localhost:8080")

def byok_register(broker: str, agent_name: str, scope: list[str],
                  launch_token: str, task_id: str = "default",
                  orch_id: str = "default") -> dict:
    """Register an agent using your own Ed25519 keys."""
    
    # 1. Generate Ed25519 key pair
    private_key = Ed25519PrivateKey.generate()
    public_key = private_key.public_key()
    
    # 2. Get challenge nonce from broker
    challenge = requests.get(f"{broker}/v1/challenge", timeout=5)
    challenge.raise_for_status()
    nonce_hex = challenge.json()["nonce"]
    
    # 3. Sign the nonce bytes (NOT the hex string!)
    #    This is the #1 mistake developers make.
    nonce_bytes = bytes.fromhex(nonce_hex)
    signature = private_key.sign(nonce_bytes)
    
    # 4. Encode public key and signature as base64
    pub_b64 = base64.b64encode(
        public_key.public_bytes(Encoding.Raw, PublicFormat.Raw)
    ).decode()
    sig_b64 = base64.b64encode(signature).decode()
    
    # 5. Register with the broker
    resp = requests.post(f"{broker}/v1/register", json={
        "agent_name": agent_name,
        "public_key": pub_b64,
        "signature": sig_b64,
        "nonce": nonce_hex,
        "launch_token": launch_token,
        "task_id": task_id,
        "orch_id": orch_id,
        "requested_scope": scope,
    }, timeout=10)
    resp.raise_for_status()
    
    return resp.json()

# Usage (you need a launch token from your operator)
result = byok_register(
    broker=BROKER,
    agent_name="byok-agent",
    scope=["read:data:*"],
    launch_token="your-launch-token-here",
    task_id="byok-task-001"
)
print(f"Registered! Token: {result['access_token'][:30]}...")
```

### Common BYOK Mistakes

| Mistake | What Happens | Fix |
|---------|-------------|-----|
| Sign hex string instead of bytes | `401: nonce signature verification failed` | Use `bytes.fromhex(nonce_hex)` |
| Use DER/PEM encoded public key | `401: invalid Ed25519 public key: wrong key size` | Use `Encoding.Raw, PublicFormat.Raw` |
| Wait too long after challenge | `401: nonce not found or expired` | Register within 30 seconds of challenge |

---

## Enforcing Scopes in Your Resource Server

When your resource server receives a request with an AgentAuth token, it must verify the token and check scopes.

### Python (Flask)

```python
from flask import Flask, request, jsonify
import requests

app = Flask(__name__)
BROKER = "http://localhost:8080"

def validate_token(token: str) -> dict | None:
    """Validate a token and return its claims."""
    resp = requests.post(f"{BROKER}/v1/token/validate", json={"token": token})
    data = resp.json()
    return data["claims"] if data.get("valid") else None

def require_scope(required: str):
    """Decorator to enforce scope on an endpoint."""
    def decorator(f):
        def wrapper(*args, **kwargs):
            auth = request.headers.get("Authorization", "")
            if not auth.startswith("Bearer "):
                return jsonify({"error": "Missing token"}), 401
            
            token = auth[7:]  # Strip "Bearer "
            claims = validate_token(token)
            
            if not claims:
                return jsonify({"error": "Invalid token"}), 401
            
            # Check scope
            req_action, req_resource, req_id = required.split(":")
            has_scope = any(
                _scope_matches(s, req_action, req_resource, req_id)
                for s in claims["scope"]
            )
            
            if not has_scope:
                return jsonify({"error": "Insufficient scope"}), 403
            
            request.agent_claims = claims
            return f(*args, **kwargs)
        wrapper.__name__ = f.__name__
        return wrapper
    return decorator

def _scope_matches(scope: str, action: str, resource: str, identifier: str) -> bool:
    s_action, s_resource, s_id = scope.split(":")
    return (s_action == action or s_action == "*") and \
           (s_resource == resource or s_resource == "*") and \
           (s_id == identifier or s_id == "*")

@app.route("/customers")
@require_scope("read:data:customers")
def get_customers():
    agent = request.agent_claims["sub"]
    return jsonify({"customers": [...], "accessed_by": agent})
```

### TypeScript (Express)

```typescript
import express, { Request, Response, NextFunction } from "express";

const BROKER = "http://localhost:8080";

interface AgentClaims {
  sub: string;
  scope: string[];
  exp: number;
  jti: string;
}

// Extend Request to include agent claims
declare global {
  namespace Express {
    interface Request {
      agentClaims?: AgentClaims;
    }
  }
}

async function validateToken(token: string): Promise<AgentClaims | null> {
  const resp = await fetch(`${BROKER}/v1/token/validate`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
  const data = await resp.json();
  return data.valid ? data.claims : null;
}

function scopeMatches(scope: string, action: string, resource: string, id: string): boolean {
  const [sAction, sResource, sId] = scope.split(":");
  return (sAction === action || sAction === "*") &&
         (sResource === resource || sResource === "*") &&
         (sId === id || sId === "*");
}

function requireScope(required: string) {
  return async (req: Request, res: Response, next: NextFunction) => {
    const auth = req.headers.authorization || "";
    if (!auth.startsWith("Bearer ")) {
      return res.status(401).json({ error: "Missing token" });
    }

    const token = auth.slice(7);
    const claims = await validateToken(token);

    if (!claims) {
      return res.status(401).json({ error: "Invalid token" });
    }

    const [action, resource, id] = required.split(":");
    const hasScope = claims.scope.some(s => scopeMatches(s, action, resource, id));

    if (!hasScope) {
      return res.status(403).json({ error: "Insufficient scope" });
    }

    req.agentClaims = claims;
    next();
  };
}

const app = express();

app.get("/customers", requireScope("read:data:customers"), (req, res) => {
  res.json({ customers: [], accessed_by: req.agentClaims!.sub });
});

app.listen(3000);
```

---

## Token Renewal Strategies

### Strategy 1: Renew at 80% TTL (Recommended)

**Python:**
```python
import time
import threading

class TokenManager:
    def __init__(self, sidecar_url, agent_name, scope, ttl=300):
        self.sidecar_url = sidecar_url
        self.agent_name = agent_name
        self.scope = scope
        self.ttl = ttl
        self.token = None
        self._stop = threading.Event()

    def start(self):
        self._acquire()
        threading.Thread(target=self._loop, daemon=True).start()

    def _acquire(self):
        resp = requests.post(f"{self.sidecar_url}/v1/token", json={
            "agent_name": self.agent_name,
            "scope": self.scope,
            "ttl": self.ttl
        }, timeout=10)
        self.token = resp.json()["access_token"]

    def _loop(self):
        while not self._stop.wait(self.ttl * 0.8):
            try:
                resp = requests.post(f"{self.sidecar_url}/v1/token/renew",
                    headers={"Authorization": f"Bearer {self.token}"}, timeout=5)
                if resp.ok:
                    self.token = resp.json()["access_token"]
                else:
                    self._acquire()
            except Exception:
                self._acquire()

    def stop(self):
        self._stop.set()
```

### Strategy 2: Renew on Demand

For short tasks, skip the loop and just renew when you need to:

```python
def make_request_with_token(url, token, sidecar_url):
    """Make a request, renewing the token if it's expired."""
    resp = requests.get(url, headers={"Authorization": f"Bearer {token}"})
    
    if resp.status_code == 401:
        # Token expired, renew it
        renewed = requests.post(f"{sidecar_url}/v1/token/renew",
            headers={"Authorization": f"Bearer {token}"})
        if renewed.ok:
            token = renewed.json()["access_token"]
            resp = requests.get(url, headers={"Authorization": f"Bearer {token}"})
    
    return resp, token
```

---

## Error Handling

All broker errors follow the RFC 7807 format:

```json
{
  "type": "urn:agentauth:error:scope_violation",
  "title": "Forbidden",
  "status": 403,
  "detail": "requested scope exceeds allowed scope",
  "request_id": "a1b2c3d4e5f67890",
  "hint": "requested scope must be a subset of allowed scope"
}
```

### Python Error Handler

```python
class AgentAuthError(Exception):
    def __init__(self, status, error_code, detail, request_id, hint=None):
        self.status = status
        self.error_code = error_code
        self.detail = detail
        self.request_id = request_id
        self.hint = hint
        super().__init__(f"{status} {error_code}: {detail}")

def handle_response(resp):
    if resp.ok:
        return resp.json()
    
    error = resp.json()
    raise AgentAuthError(
        status=error.get("status", resp.status_code),
        error_code=error.get("error_code", "unknown"),
        detail=error.get("detail", "Unknown error"),
        request_id=error.get("request_id", "none"),
        hint=error.get("hint")
    )

# Usage
try:
    data = handle_response(requests.post(...))
except AgentAuthError as e:
    print(f"Error: {e.detail}")
    if e.hint:
        print(f"Hint: {e.hint}")
    print(f"Request ID for support: {e.request_id}")
```

### TypeScript Error Handler

```typescript
class AgentAuthError extends Error {
  constructor(
    public status: number,
    public errorCode: string,
    public detail: string,
    public requestId: string,
    public hint?: string
  ) {
    super(`${status} ${errorCode}: ${detail}`);
  }
}

async function handleResponse<T>(resp: Response): Promise<T> {
  if (resp.ok) return resp.json() as Promise<T>;

  const error = await resp.json();
  throw new AgentAuthError(
    error.status || resp.status,
    error.error_code || "unknown",
    error.detail || "Unknown error",
    error.request_id || "none",
    error.hint
  );
}

// Usage
try {
  const data = await handleResponse<TokenResponse>(await fetch(...));
} catch (e) {
  if (e instanceof AgentAuthError) {
    console.error(`Error: ${e.detail}`);
    if (e.hint) console.error(`Hint: ${e.hint}`);
    console.error(`Request ID: ${e.requestId}`);
  }
}
```

---

## Production Checklist

- [ ] **Use the sidecar** — Don't call the broker directly from application code
- [ ] **Set appropriate TTLs** — 2x expected task duration, renew at 80%
- [ ] **Handle errors** — Implement retry with exponential backoff
- [ ] **Validate tokens** — Resource servers must validate tokens, not just check presence
- [ ] **Check scopes** — Don't just check if token is valid; verify it has the right scope
- [ ] **Release tokens** — Call `/v1/token/release` when tasks complete
- [ ] **Log request IDs** — Include `request_id` from errors in your logs for debugging
- [ ] **Use TLS in production** — Set `AA_TLS_MODE=tls` or `AA_TLS_MODE=mtls`
- [ ] **Use UDS for sidecar** — Set `AA_SOCKET_PATH` to avoid network exposure
- [ ] **Monitor sidecar health** — Check `GET /v1/health` and `GET /v1/metrics`

---

## Next Steps

- [[Common Tasks]] — Step-by-step recipes
- [[Integration Patterns]] — Architecture patterns for production
- [[API Reference]] — Complete endpoint documentation
- [[Troubleshooting]] — Fix common errors
