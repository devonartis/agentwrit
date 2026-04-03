# Your First Agent (TypeScript)

Build a working AI agent with AgentAuth in 15 minutes using TypeScript/Node.js.

> **What you'll build:** A TypeScript agent that gets a temporary credential, uses it, renews it, and releases it.
>
> **What you'll learn:** The complete token lifecycle in TypeScript.
>
> **Prerequisites:** Node.js 18+, Docker, and a terminal.

---

## Step 1: Start AgentAuth

```bash
git clone https://github.com/devonartis/agentauth-core.git
cd agentauth-core
AA_ADMIN_SECRET="my-super-secret-key-change-me" docker compose up -d
```

Verify it's running:
```bash
curl http://localhost:8081/v1/health
```

---

## Step 2: Set Up Your Project

```bash
mkdir my-agent && cd my-agent
npm init -y
npm install node-fetch@3
```

> **Note:** We use `node-fetch` v3 for ESM support. If you're using Node 18+, the built-in `fetch` works too — just skip the import.

---

## Step 3: Write Your Agent

Create `agent.mjs` (note the `.mjs` extension for ES modules):

```typescript
/**
 * My First AgentAuth Agent (TypeScript/Node.js)
 * =============================================
 * Demonstrates the complete token lifecycle:
 * 1. Get a token from the sidecar
 * 2. Validate the token with the broker
 * 3. Renew the token
 * 4. Release the token when done
 */

// Use built-in fetch (Node 18+) or import node-fetch
// import fetch from 'node-fetch';  // Uncomment if using node-fetch

const SIDECAR_URL = "http://localhost:8081";
const BROKER_URL = "http://localhost:8080";

// ── Step 1: Get a Token ─────────────────────────────────────────────

async function getToken() {
  console.log("📋 Requesting a token from the sidecar...");

  const response = await fetch(`${SIDECAR_URL}/v1/token`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      agent_name: "my-first-ts-agent",      // A name for your agent
      scope: ["read:data:*"],                // What you want access to
      ttl: 300,                              // 5 minutes
      task_id: "tutorial-ts-001"             // Optional: tag for audit trail
    }),
  });

  if (!response.ok) {
    const error = await response.text();
    console.error(`   Error: ${response.status} - ${error}`);
    return null;
  }

  const data = await response.json();

  console.log("   Token received!");
  console.log(`   Agent ID:    ${data.agent_id}`);
  console.log(`   Scope:       ${data.scope.join(", ")}`);
  console.log(`   Expires in:  ${data.expires_in} seconds`);
  console.log(`   Token type:  ${data.token_type}`);
  console.log();

  return data;
}

// ── Step 2: Use the Token ───────────────────────────────────────────

async function useToken(token) {
  console.log("🔍 Validating the token against the broker...");

  const response = await fetch(`${BROKER_URL}/v1/token/validate`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });

  const data = await response.json();

  if (data.valid) {
    const claims = data.claims;
    console.log("   Token is VALID!");
    console.log(`   Subject:  ${claims.sub}`);
    console.log(`   Scope:    ${claims.scope.join(", ")}`);
    console.log(`   Issued:   ${new Date(claims.iat * 1000).toISOString()}`);
    console.log(`   Expires:  ${new Date(claims.exp * 1000).toISOString()}`);
    console.log(`   Token ID: ${claims.jti}`);
  } else {
    console.log(`   Token is INVALID: ${data.error || "unknown"}`);
  }

  console.log();
  return data.valid;
}

// ── Step 3: Renew the Token ─────────────────────────────────────────

async function renewToken(token) {
  console.log("🔄 Renewing the token...");

  const response = await fetch(`${SIDECAR_URL}/v1/token/renew`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
  });

  if (response.ok) {
    const data = await response.json();
    console.log(`   Renewed! New expiry: ${data.expires_in} seconds`);
    console.log();
    return data.access_token;
  } else {
    console.log(`   Renewal failed: ${response.status} - ${await response.text()}`);
    console.log();
    return null;
  }
}

// ── Step 4: Release the Token ───────────────────────────────────────

async function releaseToken(token) {
  console.log("🏁 Releasing the token (task complete)...");

  const response = await fetch(`${BROKER_URL}/v1/token/release`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
  });

  if (response.status === 204) {
    console.log("   Token released successfully!");
  } else if (response.status === 401) {
    console.log("   Token was already expired or revoked");
  } else {
    console.log(`   Release failed: ${response.status}`);
  }

  console.log();
}

// ── Main ────────────────────────────────────────────────────────────

async function main() {
  console.log("=".repeat(60));
  console.log("  My First AgentAuth Agent (TypeScript)");
  console.log("=".repeat(60));
  console.log();

  // Step 1: Get a token
  const tokenData = await getToken();
  if (!tokenData) {
    console.log("Failed to get token. Is AgentAuth running?");
    console.log("Try: AA_ADMIN_SECRET='my-super-secret-key-change-me' docker compose up -d");
    return;
  }

  let token = tokenData.access_token;

  // Step 2: Use the token
  const isValid = await useToken(token);
  if (!isValid) {
    console.log("Token validation failed!");
    return;
  }

  // Simulate doing some work...
  console.log("⏳ Simulating work (2 seconds)...");
  await new Promise((resolve) => setTimeout(resolve, 2000));
  console.log();

  // Step 3: Renew the token
  const newToken = await renewToken(token);
  if (newToken) {
    token = newToken;
  }

  // Step 4: Release when done
  await releaseToken(token);

  console.log("=".repeat(60));
  console.log("  Done! Your first agent lifecycle is complete.");
  console.log("=".repeat(60));
}

main().catch(console.error);
```

---

## Step 4: Run It

```bash
node agent.mjs
```

You should see the same lifecycle as the Python version — get, validate, renew, release.

---

## TypeScript Version (With Types)

If you're using TypeScript, create `agent.ts`:

```typescript
// Types for AgentAuth responses
interface TokenResponse {
  access_token: string;
  expires_in: number;
  scope: string[];
  agent_id: string;
  token_type: string;
}

interface ClaimSet {
  iss: string;
  sub: string;
  exp: number;
  iat: number;
  jti: string;
  scope: string[];
  task_id?: string;
  orch_id?: string;
}

interface ValidationResponse {
  valid: boolean;
  claims?: ClaimSet;
  error?: string;
  detail?: string;
}

const SIDECAR_URL = "http://localhost:8081";
const BROKER_URL = "http://localhost:8080";

async function getToken(): Promise<TokenResponse | null> {
  const response = await fetch(`${SIDECAR_URL}/v1/token`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      agent_name: "my-typed-agent",
      scope: ["read:data:*"],
      ttl: 300,
    }),
  });

  if (!response.ok) return null;
  return (await response.json()) as TokenResponse;
}

async function validateToken(token: string): Promise<boolean> {
  const response = await fetch(`${BROKER_URL}/v1/token/validate`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });

  const data = (await response.json()) as ValidationResponse;
  
  if (data.valid && data.claims) {
    console.log(`Valid! Agent: ${data.claims.sub}`);
    console.log(`Scope: ${data.claims.scope.join(", ")}`);
    console.log(`Expires: ${new Date(data.claims.exp * 1000).toISOString()}`);
  }
  
  return data.valid;
}

async function renewToken(token: string): Promise<string | null> {
  const response = await fetch(`${SIDECAR_URL}/v1/token/renew`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!response.ok) return null;
  const data = (await response.json()) as TokenResponse;
  return data.access_token;
}

async function releaseToken(token: string): Promise<void> {
  await fetch(`${BROKER_URL}/v1/token/release`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
  });
}

// Usage
async function main(): Promise<void> {
  const tokenData = await getToken();
  if (!tokenData) throw new Error("Failed to get token");

  let token = tokenData.access_token;
  
  await validateToken(token);
  
  const renewed = await renewToken(token);
  if (renewed) token = renewed;
  
  await releaseToken(token);
  console.log("Done!");
}

main();
```

---

## Auto-Renewal Class (TypeScript)

For long-running agents:

```typescript
class TokenManager {
  private token: string | null = null;
  private timer: NodeJS.Timeout | null = null;

  constructor(
    private sidecarUrl: string,
    private agentName: string,
    private scope: string[],
    private ttl: number = 300
  ) {}

  async start(): Promise<string> {
    await this.acquire();
    this.scheduleRenewal();
    return this.token!;
  }

  getToken(): string {
    if (!this.token) throw new Error("Token not acquired. Call start() first.");
    return this.token;
  }

  async stop(): Promise<void> {
    if (this.timer) clearTimeout(this.timer);
    if (this.token) {
      await fetch(`${BROKER_URL}/v1/token/release`, {
        method: "POST",
        headers: { Authorization: `Bearer ${this.token}` },
      });
    }
    this.token = null;
  }

  private async acquire(): Promise<void> {
    const resp = await fetch(`${this.sidecarUrl}/v1/token`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        agent_name: this.agentName,
        scope: this.scope,
        ttl: this.ttl,
      }),
    });
    const data = (await resp.json()) as TokenResponse;
    this.token = data.access_token;
  }

  private scheduleRenewal(): void {
    // Renew at 80% of TTL
    const renewalMs = this.ttl * 0.8 * 1000;
    this.timer = setTimeout(async () => {
      try {
        const resp = await fetch(`${this.sidecarUrl}/v1/token/renew`, {
          method: "POST",
          headers: { Authorization: `Bearer ${this.token}` },
        });
        if (resp.ok) {
          const data = (await resp.json()) as TokenResponse;
          this.token = data.access_token;
        } else {
          await this.acquire();
        }
        this.scheduleRenewal();
      } catch {
        await this.acquire();
        this.scheduleRenewal();
      }
    }, renewalMs);
  }
}

// Usage:
const manager = new TokenManager(SIDECAR_URL, "long-running-agent", ["read:data:*"]);
await manager.start();

// Use manager.getToken() in your API calls
const headers = { Authorization: `Bearer ${manager.getToken()}` };

// When done:
await manager.stop();
```

---

## Next Steps

- [[Key Concepts Explained]] — Understand scopes, SPIFFE IDs, and delegation
- [[Common Tasks]] — Recipes for real-world workflows
- [[Developer Guide]] — Full developer integration guide
- [[Integration Patterns]] — Architecture patterns for production
