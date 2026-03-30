# Real-World Scenarios — The 8 Components in Production

> **Purpose:** Show how the 8 Ephemeral Agent Credentialing components work together in real production deployments. Each scenario is a real use case with real API calls.
>
> **Audience:** Developers building agent systems, architects evaluating AgentAuth, security reviewers verifying coverage.
>
> **Prerequisites:** [Concepts](concepts.md) for component definitions, [Implementation Map](implementation-map.md) for code tracing.

---

## Scenario 1: Financial Services — Loan Document Processing Pipeline

A bank runs an AI pipeline that reads loan applications, extracts data, scores risk, and writes decisions. Three agents, each with different permissions, process documents in sequence. No agent should be able to access more than its specific task requires.

### The Agents

| Agent | Job | Required Scope | Why Limited |
|-------|-----|---------------|-------------|
| Document Reader | Extract text from uploaded PDFs | `read:documents:loans` | Should never write or delete documents |
| Risk Scorer | Run credit model against extracted data | `read:data:credit`, `read:documents:loans` | Needs both data sources but should never write |
| Decision Writer | Write the final approval/denial | `write:decisions:loans` | Should never read raw credit data |

### How the 8 Components Protect This Pipeline

**Setup (one-time):**

```bash
# Operator registers the loan-processing app
aactl app register \
  --name loan-pipeline \
  --scopes "read:documents:loans,read:data:credit,write:decisions:loans"

# Returns: client_id=lp-a1b2c3, client_secret=... (save this)
```

**Runtime (every pipeline run):**

```python
import base64, binascii, requests
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

BROKER = "https://broker.internal:8080"

class LoanPipeline:
    def __init__(self, client_id, client_secret):
        self.broker = BROKER
        self.client_id = client_id
        self.client_secret = client_secret

    def run(self, loan_application_id):
        # ━━━ Component 2: Short-Lived Token ━━━
        # App authenticates and gets a token (default 1800s TTL)
        app_token = self._app_auth()

        # ━━━ Component 1: Ephemeral Identity ━━━
        # Each agent gets its own SPIFFE identity and scoped token
        reader_token = self._spawn_agent(
            app_token, "doc-reader",
            scope=["read:documents:loans"],
            task_id=f"loan-{loan_application_id}",
        )

        scorer_token = self._spawn_agent(
            app_token, "risk-scorer",
            scope=["read:data:credit", "read:documents:loans"],
            task_id=f"loan-{loan_application_id}",
        )

        writer_token = self._spawn_agent(
            app_token, "decision-writer",
            scope=["write:decisions:loans"],
            task_id=f"loan-{loan_application_id}",
        )

        # ━━━ Component 3: Zero-Trust Enforcement ━━━
        # Each agent uses ONLY its token. The broker validates every request.
        # If the reader tries to write, the broker returns 403.
        doc_text = self._read_document(reader_token, loan_application_id)
        risk_score = self._score_risk(scorer_token, doc_text)
        self._write_decision(writer_token, loan_application_id, risk_score)

        # ━━━ Component 4: Automatic Expiration ━━━
        # Agents release their tokens when done (task completion signal)
        for token in [reader_token, scorer_token, writer_token]:
            requests.post(f"{self.broker}/v1/token/release",
                headers={"Authorization": f"Bearer {token}"})

        # ━━━ Component 5: Immutable Audit ━━━
        # Every action above was recorded:
        # - 3x agent_registered (one per agent)
        # - 3x token_issued
        # - 3x token_released
        # - All with task_id=loan-{id} for correlation

    def _app_auth(self):
        resp = requests.post(f"{self.broker}/v1/app/auth", json={
            "client_id": self.client_id,
            "client_secret": self.client_secret,
        })
        return resp.json()["access_token"]

    def _spawn_agent(self, app_token, agent_name, scope, task_id):
        """Create launch token (Component 2) + register agent (Component 1)."""
        # App creates a launch token via the app route
        lt_resp = requests.post(f"{self.broker}/v1/app/launch-tokens",
            headers={"Authorization": f"Bearer {app_token}"},
            json={
                "agent_name": agent_name,
                "allowed_scope": scope,
                "max_ttl": 300,  # 5 minutes max
                "ttl": 30,       # launch token expires in 30s
                "single_use": True,
            })
        launch_token = lt_resp.json()["launch_token"]

        # Agent generates Ed25519 keypair and registers
        private_key = Ed25519PrivateKey.generate()
        pub_b64 = base64.b64encode(
            private_key.public_key().public_bytes_raw()
        ).decode()

        challenge = requests.get(f"{self.broker}/v1/challenge")
        nonce = challenge.json()["nonce"]

        sig_b64 = base64.b64encode(
            private_key.sign(binascii.unhexlify(nonce))
        ).decode()

        reg = requests.post(f"{self.broker}/v1/register", json={
            "launch_token": launch_token,
            "nonce": nonce,
            "public_key": pub_b64,
            "signature": sig_b64,
            "orch_id": "loan-pipeline",
            "task_id": task_id,
            "requested_scope": scope,
        })
        return reg.json()["access_token"]
```

**What each component did:**

| Component | What happened | Where in code |
|-----------|--------------|---------------|
| 1. Ephemeral Identity | Each agent got `spiffe://agentauth.local/agent/loan-pipeline/loan-12345/{unique}` | `identity/id_svc.go:Register()` |
| 2. Short-Lived Tokens | App token: 1800s. Agent tokens: 300s max. Launch tokens: 30s. | `token/tkn_svc.go:Issue()` |
| 3. Zero-Trust | Every API call validated by `ValMw`. Reader can't write. Writer can't read credit data. | `authz/val_mw.go:Wrap()` |
| 4. Expiration & Revocation | Agents released tokens on completion. If the pipeline crashes, tokens expire in 5 min. | `handler/release_hdl.go`, `token/tkn_claims.go:Validate()` |
| 5. Audit Trail | 9+ events recorded with `task_id=loan-12345` — full pipeline trace. | `audit/audit_log.go:Record()` |
| 6. Mutual Auth | Not used here (single pipeline, agents don't talk to each other). | — |
| 7. Delegation | Not used here (no agent-to-agent delegation needed). | — |
| 8. Observability | `agentauth_tokens_issued_total{scope="read:documents:loans"}` incremented. Health check shows all events recorded. | `obs/obs.go` metrics |

---

## Scenario 2: DevOps — Production Deployment with Delegation

A deployment orchestrator spawns agents to deploy code. The lead agent has broad access but delegates narrow scopes to worker agents. If something goes wrong, the operator revokes the entire delegation chain.

### The Agents

| Agent | Job | Scope | Created By |
|-------|-----|-------|-----------|
| Deploy Orchestrator | Coordinates the deployment | `write:deploy:*`, `read:config:*` | App via launch token |
| Config Reader | Reads environment config | `read:config:production` | Delegated by Orchestrator |
| Deployer | Pushes code to production | `write:deploy:web-service` | Delegated by Orchestrator |

### How It Works

```python
# ━━━ Component 7: Delegation Chain ━━━
# The orchestrator delegates narrowed scope to workers

# Orchestrator has: write:deploy:*, read:config:*
# It delegates ONLY what each worker needs

# Config reader gets narrowed scope
config_reader = requests.post(f"{BROKER}/v1/delegate",
    headers={"Authorization": f"Bearer {orchestrator_token}"},
    json={
        "delegate_to": "spiffe://agentauth.local/agent/deploy/task-42/config-reader",
        "scope": ["read:config:production"],  # narrowed from read:config:*
        "ttl": 120,
    })
config_reader_token = config_reader.json()["access_token"]
# delegation_chain now has 1 entry: the orchestrator

# Deployer gets narrowed scope
deployer = requests.post(f"{BROKER}/v1/delegate",
    headers={"Authorization": f"Bearer {orchestrator_token}"},
    json={
        "delegate_to": "spiffe://agentauth.local/agent/deploy/task-42/deployer",
        "scope": ["write:deploy:web-service"],  # narrowed from write:deploy:*
        "ttl": 120,
    })
deployer_token = deployer.json()["access_token"]

# ━━━ Component 3: Zero-Trust ━━━
# Config reader can ONLY read production config.
# It cannot deploy. It cannot read staging config.
# Deployer can ONLY deploy web-service.
# It cannot read config. It cannot deploy other services.
```

**Emergency revocation (Component 4):**

```bash
# Something goes wrong — operator revokes the ENTIRE delegation chain
# This invalidates the orchestrator AND all delegated tokens

curl -X POST https://broker.internal:8080/v1/revoke \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "level": "chain",
    "target": "spiffe://agentauth.local/agent/deploy/task-42/orchestrator"
  }'

# All three agents (orchestrator, config reader, deployer) are now revoked.
# Their next API call returns 401 immediately.
```

**What each component did:**

| Component | What happened |
|-----------|--------------|
| 1. Ephemeral Identity | Orchestrator registered via challenge-response. Workers got identities via delegation. |
| 2. Short-Lived Tokens | Delegated tokens: 120s TTL. Orchestrator: 300s. |
| 3. Zero-Trust | Every request validated. Scope boundaries enforced. Config reader can't deploy. |
| 4. Revocation | Chain-level revocation killed all 3 agents in one call. |
| 5. Audit Trail | `delegation_created` events trace the full chain. `token_revoked` records the emergency. |
| 6. Mutual Auth | Not used (workers trust the broker, not each other). |
| 7. Delegation | Orchestrator → Config Reader (narrowed). Orchestrator → Deployer (narrowed). Max depth 5. |
| 8. Observability | `agentauth_tokens_revoked_total{level="chain"}` incremented. Prometheus alert fires. |

---

## Scenario 3: Healthcare — Patient Record Access with Full Audit

A hospital runs AI agents that assist clinicians. Each agent accesses a specific patient's records for a specific consultation. The compliance team needs a complete audit trail of every access, and agents must be individually revocable if compromised.

### How the 8 Components Apply

```python
# ━━━ Component 1: Ephemeral Identity ━━━
# Each consultation gets its own agent with a unique SPIFFE ID
# spiffe://hospital.health/agent/ehr-system/consultation-789/agent-abc123

agent_token = spawn_agent(
    app_token,
    agent_name="clinical-assistant",
    scope=["read:patient:patient-456"],  # THIS patient only
    task_id="consultation-789",
    orch_id="ehr-system",
)

# ━━━ Component 3: Zero-Trust ━━━
# The agent can read patient-456's records.
# It CANNOT read patient-457. The scope is patient-specific.
# read:patient:patient-456 does NOT cover read:patient:patient-457

# ━━━ Component 2: Short-Lived Token ━━━
# Token expires in 300 seconds (5 minutes).
# A consultation that runs long must renew:
renewed = requests.post(f"{BROKER}/v1/token/renew",
    headers={"Authorization": f"Bearer {agent_token}"})
agent_token = renewed.json()["access_token"]
# Old token is immediately revoked (Component 4)

# ━━━ Component 4: Revocation ━━━
# Consultation ends — agent releases its token
requests.post(f"{BROKER}/v1/token/release",
    headers={"Authorization": f"Bearer {agent_token}"})

# If the agent is compromised mid-consultation:
# Operator can revoke just THIS agent without affecting others
requests.post(f"{BROKER}/v1/revoke",
    headers={"Authorization": f"Bearer {admin_token}"},
    json={"level": "agent", "target": agent_spiffe_id})
# Or revoke all agents for this consultation:
requests.post(f"{BROKER}/v1/revoke",
    headers={"Authorization": f"Bearer {admin_token}"},
    json={"level": "task", "target": "consultation-789"})

# ━━━ Component 5: Audit Trail ━━━
# Compliance query: "who accessed patient-456's records?"
events = requests.get(f"{BROKER}/v1/audit/events",
    headers={"Authorization": f"Bearer {admin_token}"},
    params={
        "task_id": "consultation-789",
        "event_type": "resource_accessed",
    })
# Returns hash-chained, tamper-evident audit events
# Each event has: timestamp, agent_id, task_id, outcome, detail

# ━━━ Component 8: Observability ━━━
# Prometheus dashboard shows:
# - agentauth_active_agents: 0 (consultation complete)
# - agentauth_tokens_issued_total{scope="read:patient:patient-456"}: 1
# - agentauth_audit_events_total: growing
# - agentauth_request_duration_seconds: SLA compliance
```

### Compliance Summary

| Requirement | How AgentAuth Delivers |
|------------|----------------------|
| Patient-specific access control | Scope: `read:patient:patient-456` (Component 3) |
| Time-limited access | 300s token TTL, auto-expiry (Component 2) |
| Individual agent accountability | Unique SPIFFE ID per consultation (Component 1) |
| Revocability | Agent, task, or chain-level revocation (Component 4) |
| Complete audit trail | Hash-chained events with task_id correlation (Component 5) |
| Tamper evidence | SHA-256 hash chain — broken links detectable (Component 5) |
| Monitoring | Prometheus metrics + health endpoint (Component 8) |

---

## Component Coverage Across Scenarios

| Component | Scenario 1 (Finance) | Scenario 2 (DevOps) | Scenario 3 (Healthcare) |
|-----------|---------------------|--------------------|-----------------------|
| 1. Ephemeral Identity | 3 agents with unique SPIFFE IDs | 1 registered + 2 delegated | 1 agent per consultation |
| 2. Short-Lived Tokens | 300s agent, 30s launch, 1800s app | 120s delegated, 300s orchestrator | 300s with renewal |
| 3. Zero-Trust | Reader can't write, Writer can't read credit | Config reader can't deploy | Agent can only read one patient |
| 4. Expiration & Revocation | Token release on completion | Chain-level emergency revocation | Agent + task-level revocation |
| 5. Audit Trail | 9+ events per pipeline run | Delegation + revocation events | Compliance-ready with task_id |
| 6. Mutual Auth | — | — | — |
| 7. Delegation | — | Scope narrowing to workers | — |
| 8. Observability | Metrics per scope label | Revocation metrics + alerts | SLA monitoring via histograms |

Component 6 (Mutual Auth) is implemented as a Go API but not used in these HTTP-based scenarios. It applies when agents need to verify each other's identity directly — for example, two agents in a mesh network that need to exchange data without going through the broker.
