# Integration Patterns Guide for AgentAuth

> **Document Version:** 2.0 | **Last Updated:** February 2026 | **Status:** Current
>
> **Audience:** Developers, architects, and platform engineers implementing AgentAuth in production systems
>
> **Prerequisites:** [Concepts](concepts.md), [API Reference](api.md), Python familiarity, REST API experience
>
> **Time to read:** 45 minutes (all 6 patterns) or 5-10 minutes per pattern
>
> **Next steps:** [Getting Started: Developer](getting-started-developer.md) for setup | [Troubleshooting](troubleshooting.md) for debugging

---

## Table of Contents

1. [Introduction](#introduction)
2. [Pattern 1: Multi-Agent Pipeline](#pattern-1-multi-agent-pipeline)
3. [Pattern 2: Multi-Service Authorization](#pattern-2-multi-service-authorization)
4. [Pattern 3: Token Release as Task Completion Signal](#pattern-3-token-release-as-task-completion-signal)
5. [Pattern 4: Delegation Chain with Scope Narrowing](#pattern-4-delegation-chain-with-scope-narrowing)
6. [Pattern 5: Emergency Revocation Cascade](#pattern-5-emergency-revocation-cascade)
7. [Pattern 6: App-Managed Agent Registration](#pattern-6-app-managed-agent-registration)
8. [Security Checklist](#security-checklist)
9. [Common Pitfalls](#common-pitfalls)

---

## Introduction

This guide covers six proven patterns for integrating AgentAuth into AI agent systems. Each pattern solves a specific architectural challenge:

- **Multi-Agent Pipeline:** Sequential agents with scope attenuation at each step
- **Multi-Service Authorization:** Different services with independent auth boundaries
- **Token Release:** Explicit task completion signaling and audit clarity
- **Delegation Chain:** Hierarchical scope narrowing in deep agent hierarchies
- **Emergency Revocation:** Incident response and containment strategies
- **App-Managed Agent Registration:** Orchestration apps managing agent credentials

Every pattern demonstrates the **security contrast**: what goes wrong without AgentAuth (the dangerous path) versus the safe, auditable approach with AgentAuth.

**Setup:** All examples use the `requests` library. Install with:

```bash
uv pip install requests cryptography
```

All examples assume environment variables:
- `AGENTAUTH_BROKER_URL`: Broker endpoint (e.g., `https://broker.internal:8080`)
- `AA_ADMIN_SECRET`: Operator admin credential (for app registration and emergency examples)

---

## Pattern 1: Multi-Agent Pipeline

### Use Case

A research → writing → review pipeline where each agent narrows scope further down the chain. The Research Agent has broad read access, the Writer Agent receives attenuated credentials allowing only the data it needs, and the Reviewer receives even narrower permissions. This pattern is common in:

- Content generation workflows (research → draft → edit → publish)
- Data analysis pipelines (collect → process → validate)
- Customer support (initial triage → detailed analysis → escalation)

### When to Use

Use this pattern when:
- Agents operate sequentially in a defined DAG (directed acyclic graph)
- Each agent should have progressively narrower access
- Scope attenuation is a trust boundary (an agent cannot escalate beyond its delegation)
- Audit trail must show which agent accessed which resources

### Flow Diagram

```mermaid
sequenceDiagram
    participant Orch as Orchestrator
    participant Broker
    participant RA as Research Agent
    participant WA as Writer Agent
    participant REV as Reviewer Agent
    participant DB as Data Sources

    Orch->>Broker: 1. Get launch token<br/>for Research Agent
    Broker-->>Orch: launch_token

    RA->>Broker: 2. POST /v1/register<br/>with launch_token
    Broker-->>RA: JWT(read:data:*), agent_id

    RA->>DB: Read reports<br/>scope: read:data:*
    DB-->>RA: Data

    RA->>Broker: 3. POST /v1/delegate<br/>scope: [read:data:reports]
    Broker-->>RA: JWT(read:data:reports)<br/>+ delegation_chain

    RA->>WA: 4. Pass token to Writer Agent

    WA->>DB: 5. Read (narrower scope)<br/>read:data:reports
    DB-->>WA: Report data only

    WA->>Broker: POST /v1/delegate<br/>{ delegate_to: reviewer-agent,<br/>scope: [read:data:reports],<br/>ttl: 60 }
    Broker-->>WA: JWT(read:data:reports)

    WA->>REV: delivery token

    REV->>DB: Read final output<br/>read:data:reports
    DB-->>REV: Verification data
```

### Python Code Example

```python
import os
import base64
import binascii
import requests
import json
from typing import Dict, Any, List
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

BROKER = os.environ.get("AACTL_BROKER_URL", "https://broker.internal:8080")
LAUNCH_TOKEN = os.environ.get("RESEARCH_AGENT_LAUNCH_TOKEN", "lt_...")
TASK_ID = "analysis-2026-0215-001"

class ResearchAgent:
    """Research Agent with broad read:data:* scope."""

    def __init__(self):
        self.token = None
        self.agent_id = None
        self.private_key = Ed25519PrivateKey.generate()

    def bootstrap(self) -> bool:
        """Register with broker using Ed25519 challenge-response."""
        try:
            # Step 1: Get challenge nonce
            challenge_resp = requests.get(f"{BROKER}/v1/challenge", timeout=10)
            challenge_resp.raise_for_status()
            nonce = challenge_resp.json()["nonce"]

            # Step 2: Sign the hex-decoded nonce bytes with Ed25519
            nonce_bytes = binascii.unhexlify(nonce)
            signature = base64.b64encode(self.private_key.sign(nonce_bytes)).decode()
            public_key = base64.b64encode(
                self.private_key.public_key().public_bytes_raw()
            ).decode()

            # Step 3: Register agent
            resp = requests.post(f"{BROKER}/v1/register", json={
                "launch_token": LAUNCH_TOKEN,
                "nonce": nonce,
                "public_key": public_key,
                "signature": signature,
                "orch_id": "orch-research-001",
                "task_id": TASK_ID,
                "requested_scope": ["read:data:*"],
            }, timeout=10)
            resp.raise_for_status()
            data = resp.json()
            self.token = data["access_token"]
            self.agent_id = data["agent_id"]
            print(f"[Research] Registered with agent_id: {self.agent_id}")
            return True
        except Exception as e:
            print(f"[Research] Registration failed: {e}")
            return False

    def research_data(self, topic: str) -> Dict[str, Any]:
        """Simulate researching data with full scope."""
        headers = {"Authorization": f"Bearer {self.token}"}
        print(f"[Research] Reading data for topic: {topic}")
        # In real code, this would hit actual data sources
        return {
            "topic": topic,
            "research_data": [
                {"source": "database", "content": "sensitive data..."},
                {"source": "webapi", "content": "external data..."}
            ]
        }

    def delegate_to_writer(self) -> str:
        """Delegate narrower scope to Writer Agent."""
        try:
            headers = {"Authorization": f"Bearer {self.token}"}
            resp = requests.post(f"{BROKER}/v1/delegate", json={
                "delegate_to": "writer-agent",
                "scope": ["read:data:reports"],  # Narrower than read:data:*
                "ttl": 120
            }, headers=headers, timeout=10)
            resp.raise_for_status()
            data = resp.json()
            token = data["access_token"]
            print(f"[Research] Delegated narrower scope to writer-agent")
            return token
        except Exception as e:
            print(f"[Research] Delegation failed: {e}")
            return None


class WriterAgent:
    """Writer Agent with narrower scope (only reports)."""

    def __init__(self, delegated_token: str, delegated_agent_id: str):
        self.token = delegated_token
        self.agent_id = delegated_agent_id

    def process_research(self, research_data: Dict[str, Any]) -> Dict[str, Any]:
        """Process research using delegated token (narrower scope)."""
        print(f"[Writer] Processing research with delegated scope (read:data:reports only)")

        # The delegated token limits to read:data:reports
        # This prevents reading other :read:data:* scopes
        headers = {"Authorization": f"Bearer {self.token}"}

        return {
            "report": f"Analysis of {research_data['topic']}",
            "content": "Synthesized findings from research...",
            "status": "draft"
        }

    def delegate_to_reviewer(self) -> str:
        """Delegate same-level scope to Reviewer Agent."""
        try:
            headers = {"Authorization": f"Bearer {self.token}"}
            resp = requests.post(f"{BROKER}/v1/delegate", json={
                "delegate_to": "reviewer-agent",
                "scope": ["read:data:reports"],  # Same scope, no escalation
                "ttl": 60
            }, headers=headers, timeout=10)
            resp.raise_for_status()
            data = resp.json()
            token = data["access_token"]
            print(f"[Writer] Delegated scope to reviewer-agent")
            return token
        except Exception as e:
            print(f"[Writer] Delegation failed: {e}")
            return None


class ReviewerAgent:
    """Reviewer Agent with narrower scope (read-only reports)."""

    def __init__(self, delegated_token: str, delegated_agent_id: str):
        self.token = delegated_token
        self.agent_id = delegated_agent_id

    def review_output(self, report: Dict[str, Any]) -> bool:
        """Review using delegated token (cannot write or read beyond scope)."""
        print(f"[Reviewer] Reviewing output with delegated read:data:reports scope")

        headers = {"Authorization": f"Bearer {self.token}"}

        # The delegated token only allows read:data:reports
        # Trying to write or escalate scope would be rejected by broker
        is_approved = len(report["content"]) > 10
        print(f"[Reviewer] Review result: {'APPROVED' if is_approved else 'REJECTED'}")
        return is_approved


def run_pipeline():
    """Execute the multi-agent pipeline with scope attenuation."""
    print("=== Multi-Agent Pipeline with Scope Attenuation ===\n")

    # Stage 1: Research Agent with broad scope
    research = ResearchAgent()
    if not research.bootstrap():
        print("Failed to bootstrap research agent")
        return False

    research_output = research.research_data("market analysis 2026")

    # Stage 2: Writer Agent with delegated, narrower scope
    writer_token = research.delegate_to_writer()
    if not writer_token:
        print("Failed to delegate to writer")
        return False

    # In real code, we'd extract agent_id from delegation response
    writer = WriterAgent(writer_token, "writer-agent-instance-123")
    report = writer.process_research(research_output)

    # Stage 3: Reviewer Agent with same-level delegated scope
    reviewer_token = writer.delegate_to_reviewer()
    if not reviewer_token:
        print("Failed to delegate to reviewer")
        return False

    reviewer = ReviewerAgent(reviewer_token, "reviewer-agent-instance-456")
    is_approved = reviewer.review_output(report)

    print(f"\n✓ Pipeline completed. Report approved: {is_approved}")
    return True


if __name__ == "__main__":
    run_pipeline()
```

### Security Considerations

1. **Scope Attenuation is One-Way:** Once delegated, an agent cannot escalate scope. A Writer Agent with `read:data:reports` cannot request `read:data:*` -- the broker will reject it.

2. **Chain Integrity:** The delegation chain is cryptographically signed. If an agent attempts to forge a delegation link, the signature verification fails.

3. **Audit Trail:** Every delegation is logged. You can query audit events to see which agent delegated to which agent, at what time, with what scope.

4. **Revocation Propagates:** If the Research Agent is revoked at the agent level, all downstream delegations (Writer, Reviewer) also become invalid because they depend on the chain.

### What Goes Wrong Without AgentAuth

```python
# DANGEROUS: Multi-agent pipeline without scope isolation
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Anti-pattern 1: Shared API key in environment
SHARED_DB_KEY = "sk-abcdef123456"  # Same key for all agents

class ResearchAgentDangerous:
    def bootstrap(self):
        # All agents use the same long-lived key
        self.token = SHARED_DB_KEY

    def research_data(self):
        # This agent has full read+write access to everything
        headers = {"Authorization": f"Bearer {SHARED_DB_KEY}"}
        # Can read reports, customers, logs, anything...
        # Lifetime: weeks or months


class WriterAgentDangerous:
    def __init__(self):
        # Same agent, same key, same broad permissions
        self.token = SHARED_DB_KEY  # No scope narrowing

    def process_research(self):
        headers = {"Authorization": f"Bearer {SHARED_DB_KEY}"}
        # Can READ reports (intended)
        # But ALSO write to any database table (unintended)
        # Can read customer PII (unintended)
        # Lifetime: weeks or months (task is 5 minutes)


# Problems with this approach:
# ────────────────────────────
# 1. No scope isolation. Writer agent has same access as Research agent.
# 2. No delegation proof. Can't verify Writer actually got auth from Research.
# 3. Long-lived credentials. Key valid for weeks, task runs 5 minutes.
# 4. No per-agent audit trail. Can't tell which agent did what.
# 5. Revocation nightmare. Rotating the key breaks all agents simultaneously.
# 6. Privilege escalation risk. Compromised agent affects entire pipeline.
```

---

## Pattern 2: Multi-Service Authorization

### Use Case

A microservices architecture where different services have different trust boundaries and access levels. A payment service should never access customer PII, and a user service should never access billing data. Each service is registered as an app with a scope ceiling, and the operator controls what scopes each service can request.

### When to Use

Use this pattern when:
- You have 3+ microservices with distinct domains
- Services should never access each other's sensitive data
- Scope boundaries map to service boundaries
- You want to prevent lateral movement if a service is compromised
- You want centralized control over service credentials

### Flow Diagram

```mermaid
flowchart TB
    subgraph services["Microservices"]
        CS["Customer Service"]
        PS["Payment Service"]
        OS["Order Service"]
    end

    subgraph agentauth["Shared Broker"]
        B["Broker"]
        Apps["App Registry:<br/>- customer-app<br/>- payment-app<br/>- order-app"]
    end

    subgraph data["Data Layer"]
        PIIDB["PII Database"]
        BILLDB["Billing Database"]
        ORDERDB["Order Database"]
    end

    CS -->|"POST /v1/app/auth<br/>client_id, client_secret"| B
    B -->|"JWT(read:pii:*)"| CS

    PS -->|"POST /v1/app/auth<br/>client_id, client_secret"| B
    B -->|"JWT(read:billing:*,<br/>write:billing:*)"| PS

    OS -->|"POST /v1/app/auth<br/>client_id, client_secret"| B
    B -->|"JWT(read:orders:*)"| OS

    CS -->|"read:pii:*"| PIIDB
    PS -->|"read:billing:*, write:billing:*"| BILLDB
    OS -->|"read:orders:*"| ORDERDB

    Apps -->|"scope ceilings<br/>enforce boundaries"| B
```

### Python Code Example

```python
import os
import requests
from abc import ABC, abstractmethod
from typing import Dict, Any

# ── Service-Specific Configuration ──────────────────────────────────

BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "https://broker.internal:8080")

class ServiceConfig:
    """Configuration for a microservice app."""

    def __init__(self, service_name: str, client_id: str, client_secret: str):
        self.service_name = service_name
        self.client_id = client_id
        self.client_secret = client_secret


# Each service has registered client credentials with restricted scope ceiling
CONFIG = {
    "customer": ServiceConfig(
        "customer-service",
        os.environ.get("CUSTOMER_CLIENT_ID", "app-customer"),
        os.environ.get("CUSTOMER_CLIENT_SECRET", "sk_...")
    ),
    "payment": ServiceConfig(
        "payment-service",
        os.environ.get("PAYMENT_CLIENT_ID", "app-payment"),
        os.environ.get("PAYMENT_CLIENT_SECRET", "sk_...")
    ),
    "order": ServiceConfig(
        "order-service",
        os.environ.get("ORDER_CLIENT_ID", "app-order"),
        os.environ.get("ORDER_CLIENT_SECRET", "sk_...")
    ),
}


class MicroService(ABC):
    """Base class for microservices using scoped app credentials."""

    def __init__(self, config: ServiceConfig):
        self.config = config
        self.token = None

    @abstractmethod
    def _business_logic(self) -> Any:
        """Service-specific business logic."""
        pass

    def authenticate(self, task_id: str) -> bool:
        """Authenticate as app and get scoped token."""
        try:
            resp = requests.post(f"{BROKER}/v1/app/auth", json={
                "client_id": self.config.client_id,
                "client_secret": self.config.client_secret
            }, timeout=10)
            resp.raise_for_status()
            data = resp.json()
            self.token = data["access_token"]
            scopes = data.get("scopes", [])
            print(f"[{self.config.service_name}] Authenticated with scopes: {scopes}")
            return True
        except Exception as e:
            print(f"[{self.config.service_name}] Authentication failed: {e}")
            return False

    def execute(self, task_id: str) -> bool:
        """Authenticate and execute service logic."""
        if not self.authenticate(task_id):
            return False
        return self._business_logic()


class CustomerService(MicroService):
    """Service for customer data (PII scope only)."""

    def _business_logic(self) -> Any:
        """Fetch and process customer data."""
        print(f"[{self.config.service_name}] Executing with app token")

        headers = {"Authorization": f"Bearer {self.token}"}

        # This service CAN read customer PII (read:pii:*)
        print(f"[{self.config.service_name}] ✓ Reading customer records")

        # This service CANNOT request billing scope
        # The app's scope ceiling prevents this
        print(f"[{self.config.service_name}] ✗ Cannot access billing data (scope ceiling enforced)")

        return True


class PaymentService(MicroService):
    """Service for payment/billing (billing scope only)."""

    def _business_logic(self) -> Any:
        """Process payments."""
        print(f"[{self.config.service_name}] Executing with app token")

        headers = {"Authorization": f"Bearer {self.token}"}

        # This service CAN read/write billing data
        print(f"[{self.config.service_name}] ✓ Processing payment (read:billing:*, write:billing:*)")

        # This service CANNOT request PII scope
        print(f"[{self.config.service_name}] ✗ Cannot access PII (scope ceiling enforced)")

        return True


class OrderService(MicroService):
    """Service for orders (orders scope only)."""

    def _business_logic(self) -> Any:
        """Process orders."""
        print(f"[{self.config.service_name}] Executing with app token")

        headers = {"Authorization": f"Bearer {self.token}"}

        # This service CAN read/write order data
        print(f"[{self.config.service_name}] ✓ Processing order (read:orders:*, write:orders:*)")

        # Isolated from other data domains
        print(f"[{self.config.service_name}] ✗ Cannot access PII or billing data")

        return True


def run_multi_service_pipeline():
    """Execute all services with scope isolation."""
    print("=== Multi-Service Authorization Pattern ===\n")
    print("Each service authenticates with app credentials and receives scoped tokens.\n")

    task_id = "multi-service-task-2026-0215"

    # Execute all services
    services = [
        CustomerService(CONFIG["customer"]),
        PaymentService(CONFIG["payment"]),
        OrderService(CONFIG["order"]),
    ]

    results = []
    for svc in services:
        result = svc.execute(task_id)
        results.append(result)
        print()

    success = all(results)
    print(f"\n✓ All services executed. Success: {success}")
    print("\nScope Isolation Summary:")
    print("─" * 60)
    print("Customer App: read:pii:*")
    print("Payment App:  read:billing:*, write:billing:*")
    print("Order App:    read:orders:*, write:orders:*")
    print("─" * 60)
    print("\nKey benefit: If payment-service is compromised,")
    print("attacker can only access billing data, not PII or orders.")

    return success


if __name__ == "__main__":
    run_multi_service_pipeline()
```

### Security Considerations

1. **Scope Ceiling Enforcement:** Each app is registered with a scope ceiling. The broker rejects any request for scopes beyond this ceiling.

2. **Service Isolation:** A compromised payment service cannot escalate to access PII. The scope boundary is cryptographic and enforced at the broker level.

3. **Operator Control:** The operator registers apps and sets scope ceilings via `aactl app register`. Services inherit these restrictions automatically.

4. **Short-Lived Tokens:** App tokens have short TTLs (configurable, default 30 minutes), reducing damage window from a compromise.

5. **Revocation Granularity:** If a service is compromised, the operator can revoke all tokens for that app without affecting other services.

### What Goes Wrong Without AgentAuth

```python
# DANGEROUS: Monolithic credential approach without service isolation

MASTER_API_KEY = "sk-master-key-all-access"

class CustomerServiceDangerous:
    def fetch_customer(self, customer_id: str):
        headers = {"Authorization": f"Bearer {MASTER_API_KEY}"}
        # Can read customer PII (intended)
        # Can also read/write billing data (unintended!)
        # Can read/write orders (unintended!)
        # Lifetime: months or years


class PaymentServiceDangerous:
    def process_payment(self, amount: float):
        headers = {"Authorization": f"Bearer {MASTER_API_KEY}"}
        # Can read/write billing (intended)
        # But ALSO read customer PII (unintended!)
        # But ALSO read/write orders (unintended!)
        # Lifetime: months or years


# Compromise scenario:
# If payment-service is breached via SQL injection,
# attacker obtains MASTER_API_KEY and gains access to:
#   - All customer PII
#   - All billing records
#   - All order data
#
# With AgentAuth, the attacker would only get:
#   - Billing data (app's scope ceiling)
#   - For 30 minutes (token TTL)
#   - With full audit trail showing the breach
```

---

## Pattern 3: Token Release as Task Completion Signal

### Use Case

Explicit lifecycle management where tokens are created when tasks begin and released when tasks complete. This provides:
- **Audit clarity:** Query the audit trail to see when tokens were issued and released
- **Metering:** Track resource consumption between token issue and release
- **Billing:** Bill customers based on credential lifetime (not just task duration)
- **Anomaly detection:** A token that never gets released is suspicious

### When to Use

Use this pattern when:
- You need precise audit timestamps for task boundaries
- You meter or bill based on credential lifetime
- You want to detect hung/stuck agents (token issued but never released)
- Compliance requires explicit work-start and work-end events

### Flow Diagram

```mermaid
sequenceDiagram
    participant App
    participant Agent
    participant Broker
    participant AuditLog
    participant BillingSystem

    App->>Broker: 1. Create launch token
    Broker-->>App: launch_token

    App->>Agent: Pass launch_token

    Agent->>Broker: POST /v1/register<br/>{ launch_token, nonce, signature }
    Broker->>AuditLog: event: agent_registered<br/>timestamp: 2026-02-15T10:30:00Z<br/>agent_id: abc

    Broker-->>Agent: { access_token, agent_id }

    Note over Agent: Work for N seconds<br/>(task runs 45 seconds)

    Agent->>Broker: 2. POST /v1/token/release<br/>Authorization: Bearer <token>
    Broker->>AuditLog: event: token_released<br/>timestamp: 2026-02-15T10:30:45Z<br/>agent_id: abc<br/>duration_seconds: 45

    Broker-->>Agent: 204 No Content

    BillingSystem->>AuditLog: Query issued+released events
    AuditLog-->>BillingSystem: Credential lifetime: 45 seconds
    BillingSystem->>BillingSystem: Bill customer: 45 seconds × $0.001/sec
```

### Python Code Example

```python
import os
import requests
import time
from datetime import datetime
from typing import Dict, Any, Optional

BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "https://broker.internal:8080")
BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "https://broker.internal:8080")


class TokenLifecycleAgent:
    """Agent that explicitly releases tokens when work completes."""

    def __init__(self, agent_name: str, task_id: str):
        self.agent_name = agent_name
        self.task_id = task_id
        self.token = None
        self.agent_id = None
        self.issued_at = None

    def request_token(self) -> bool:
        """Request token and record issue time."""
        try:
            self.issued_at = datetime.utcnow()
            resp = requests.post(f"{BROKER}/v1/register", json={
                "agent_name": self.agent_name,
                "task_id": self.task_id,
                "scope": ["read:data:*", "write:results:*"],
                "ttl": 300
            }, timeout=10)
            resp.raise_for_status()
            data = resp.json()
            self.token = data["access_token"]
            self.agent_id = data["agent_id"]
            print(f"[{self.agent_name}] Token issued at {self.issued_at.isoformat()}")
            return True
        except Exception as e:
            print(f"[{self.agent_name}] Token request failed: {e}")
            return False

    def do_work(self, work_duration_seconds: float = 5.0) -> Dict[str, Any]:
        """Simulate work that takes work_duration_seconds."""
        headers = {"Authorization": f"Bearer {self.token}"}

        print(f"[{self.agent_name}] Starting work...")
        time.sleep(work_duration_seconds)  # Simulate work

        work_result = {
            "task_id": self.task_id,
            "agent_id": self.agent_id,
            "work_completed": True,
            "data_processed": 1024 * 1024,  # 1 MB
        }
        print(f"[{self.agent_name}] Work completed in {work_duration_seconds}s")
        return work_result

    def release_token(self) -> bool:
        """Release token when work is done."""
        if not self.token:
            print(f"[{self.agent_name}] No token to release")
            return False

        try:
            released_at = datetime.utcnow()
            duration = (released_at - self.issued_at).total_seconds()

            headers = {"Authorization": f"Bearer {self.token}"}
            resp = requests.post(
                f"{BROKER}/v1/token/release",
                headers=headers,
                timeout=10
            )
            resp.raise_for_status()

            print(f"[{self.agent_name}] Token released at {released_at.isoformat()}")
            print(f"[{self.agent_name}] Token lifetime: {duration:.2f} seconds")

            # In a real system, send billing event here
            self._emit_billing_event(duration)

            return True
        except Exception as e:
            print(f"[{self.agent_name}] Release failed: {e}")
            return False

    def _emit_billing_event(self, duration_seconds: float):
        """Emit billing event (in a real system, this would call a billing API)."""
        rate_per_second = 0.001  # $0.001/second
        cost = duration_seconds * rate_per_second
        print(f"[{self.agent_name}] Billing event: {duration_seconds}s × ${rate_per_second}/s = ${cost:.4f}")


class TokenLeakDetector:
    """Detect tokens that are issued but never released."""

    def __init__(self, broker_url: str, admin_token: str):
        self.broker = broker_url
        self.admin_token = admin_token

    def find_leaked_tokens(self, task_id: str) -> Dict[str, Any]:
        """Query audit log to find tokens issued but never released for a task."""
        try:
            headers = {"Authorization": f"Bearer {self.admin_token}"}

            # Query audit events for this task
            resp = requests.get(
                f"{self.broker}/v1/audit/events",
                params={
                    "task_id": task_id,
                    "limit": 100,
                },
                headers=headers,
                timeout=10
            )
            resp.raise_for_status()

            events = resp.json().get("events", [])

            # Find tokens with issued but no released
            issued_tokens = {}
            released_tokens = set()

            for event in events:
                if event["event_type"] == "token_issued":
                    jti = event["detail"].get("jti")
                    issued_tokens[jti] = event
                elif event["event_type"] == "token_released":
                    jti = event["detail"].get("jti")
                    released_tokens.add(jti)

            # Tokens that were issued but not released
            leaked = {
                jti: event
                for jti, event in issued_tokens.items()
                if jti not in released_tokens
            }

            return {
                "task_id": task_id,
                "total_issued": len(issued_tokens),
                "total_released": len(released_tokens),
                "leaked_count": len(leaked),
                "leaked_tokens": list(leaked.keys()),
            }
        except Exception as e:
            print(f"Error querying audit log: {e}")
            return {
                "error": str(e),
                "leaked_count": -1,
            }


def run_token_lifecycle():
    """Demonstrate token issue and release lifecycle."""
    print("=== Token Lifecycle Pattern (Issue → Work → Release) ===\n")

    task_id = "etl-task-2026-0215-payload-001"

    # Agent: Request token, do work, release token
    agent = TokenLifecycleAgent("etl-agent", task_id)

    if not agent.request_token():
        print("Failed to request token")
        return False

    work_result = agent.do_work(work_duration_seconds=3.5)

    if not agent.release_token():
        print("Failed to release token")
        return False

    print("\n✓ Token lifecycle completed successfully")
    print(f"  Task ID: {task_id}")
    print(f"  Agent ID: {agent.agent_id}")
    print(f"  Data processed: {work_result['data_processed']} bytes")

    return True


def run_token_leak_detection():
    """Demonstrate detection of unreleased tokens."""
    print("\n=== Token Leak Detection ===\n")

    # In a real system, you would have an admin token
    # For this demo, we show the query structure
    print("Detecting leaked tokens (issued but never released):")
    print("  Query: /v1/audit/events?task_id=<task>&event_type=token_issued")
    print("  Match: Tokens not found in token_released events")
    print("\nThis helps detect:")
    print("  - Hung agents (token issued, work never completed)")
    print("  - Crashed agents (no cleanup)")
    print("  - Stuck token pipelines (work started but token release forgotten)")


if __name__ == "__main__":
    run_token_lifecycle()
    run_token_leak_detection()
```

### Security Considerations

1. **Token Lifetime is Observable:** The audit trail records exact issue and release times. Operators can query to detect anomalies.

2. **Release is Optional but Recommended:** An agent can continue using its token even if it doesn't release. But releasing is the signal that work is done.

3. **Billing Accuracy:** Token lifetime (not task duration) is the basis for metering. This ensures you only charge for credential exposure time.

4. **Audit Completeness:** Every token has an entry in the audit log. You can reconstruct the full lifecycle of any credential.

### What Goes Wrong Without AgentAuth

```python
# DANGEROUS: No explicit token lifecycle, metrics are guesses
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

class LongLivedCredentialAgent:
    def __init__(self):
        # Credential provisioned once, valid for days
        self.token = os.environ["API_KEY"]
        self.token_issued = None  # Never recorded

    def do_work(self):
        # Work happens, but no explicit completion signal
        pass

    # No mechanism to signal completion
    # No way to know when work is done
    # No way to tell if agent is hung or still running


# Problems:
# 1. No audit of work boundaries (when did task start/end?)
# 2. Billing is guessed (assume 1 hour per day? per agent?)
# 3. Can't detect leaked tokens (which tokens are actually in use?)
# 4. Resource consumption is unmeasured (how much data did agent process?)
# 5. Hung agents go undetected (is the agent still working or stuck?)
#
# With AgentAuth token release pattern:
# - Exact start/end timestamps
# - Measured credential lifetime
# - Automatic anomaly detection
# - Per-agent billing accuracy
```

---

## Pattern 4: Delegation Chain with Scope Narrowing

### Use Case

A hierarchical AI system where an orchestrator delegates progressively narrower scope to specialists. The orchestrator has broad access but can only delegate narrower scopes to child agents. Each delegation level is cryptographically signed and recorded in the audit trail.

### When to Use

Use this pattern when:
- You have 3+ levels of agents (orchestrator → specialist → sub-specialist)
- Scope must narrow at each level (no escalation)
- You need proof of authorization lineage
- Delegation depth is a security boundary

### Flow Diagram

```mermaid
sequenceDiagram
    participant Orch as Orchestrator
    participant Broker
    participant Analyzer
    participant Specialist
    participant DataStore

    Orch->>Broker: 1. Register with scope: read:data:*
    Broker-->>Orch: JWT(read:data:*)

    Orch->>Broker: 2. Delegate to analyzer<br/>scope: read:data:logs,read:data:metrics
    Note over Broker: Verify scope is subset of read:data:*
    Broker-->>Orch: JWT(read:data:logs, read:data:metrics)<br/>+ delegation_chain: [Orch]

    Orch->>Analyzer: Deliver delegated token

    Analyzer->>Broker: 3. Delegate to specialist<br/>scope: read:data:logs
    Note over Broker: Verify scope is subset of previous delegation
    Broker-->>Analyzer: JWT(read:data:logs)<br/>+ delegation_chain: [Orch, Analyzer]

    Analyzer->>Specialist: Deliver delegated token

    Specialist->>DataStore: 4. Access with narrowest scope<br/>read:data:logs
    DataStore-->>Specialist: Data matching read:data:logs

    Note over Specialist: Specialist cannot escalate to read:data:*<br/>or read:data:metrics (not in scope)
```

### Python Code Example

```python
import os
import requests
from typing import List, Dict, Any, Optional

BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "https://broker.internal:8080")
BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "https://broker.internal:8080")
TASK_ID = "analysis-cascade-2026-0215"


class DelegationChainValidator:
    """Validate delegation chains for scope attenuation."""

    @staticmethod
    def is_scope_subset(delegated_scope: List[str], parent_scope: List[str]) -> bool:
        """Check if delegated_scope is a subset of parent_scope."""
        # Parse scope strings (action:resource:identifier)
        delegated_set = set(delegated_scope)
        parent_set = set(parent_scope)

        for scope in delegated_set:
            action, resource, identifier = scope.split(":")

            # Check if parent has this exact scope
            if scope in parent_set:
                continue

            # Check if parent has wildcard version
            wildcard = f"{action}:{resource}:*"
            if wildcard in parent_set:
                continue

            # Scope is not a subset
            return False

        return True


class OrchestratorAgent:
    """Top-level agent with broad scope."""

    def __init__(self):
        self.token = None
        self.agent_id = None
        self.scope = ["read:data:*"]  # Broad initial scope

    def bootstrap(self) -> bool:
        """Register with broker using launch token."""
        try:
            resp = requests.post(f"{BROKER}/v1/register", json={
                "agent_name": "orchestrator-agent",
                "task_id": TASK_ID,
                "scope": self.scope,
                "ttl": 600  # 10 minutes for orchestrator
            }, timeout=10)
            resp.raise_for_status()
            data = resp.json()
            self.token = data["access_token"]
            self.agent_id = data["agent_id"]
            print(f"[Orchestrator] Bootstrapped with scope: {self.scope}")
            return True
        except Exception as e:
            print(f"[Orchestrator] Bootstrap failed: {e}")
            return False

    def delegate_to_analyzer(self) -> Optional[str]:
        """Delegate to analyzer with narrower scope."""
        narrower_scope = ["read:data:logs", "read:data:metrics"]

        if not DelegationChainValidator.is_scope_subset(narrower_scope, self.scope):
            print(f"[Orchestrator] ERROR: Delegation scope {narrower_scope} not subset of {self.scope}")
            return None

        try:
            headers = {"Authorization": f"Bearer {self.token}"}
            resp = requests.post(f"{BROKER}/v1/delegate", json={
                "delegate_to": "analyzer-agent",
                "scope": narrower_scope,
                "ttl": 300  # 5 minutes for delegated agent
            }, headers=headers, timeout=10)
            resp.raise_for_status()

            data = resp.json()
            token = data["access_token"]
            chain = data.get("delegation_chain", [])

            print(f"[Orchestrator] Delegated to analyzer with scope: {narrower_scope}")
            print(f"[Orchestrator] Delegation chain depth: {len(chain) + 1}")

            return token
        except Exception as e:
            print(f"[Orchestrator] Delegation failed: {e}")
            return None


class AnalyzerAgent:
    """Mid-level agent with delegated, narrower scope."""

    def __init__(self, delegated_token: str):
        self.token = delegated_token
        self.agent_id = "analyzer-agent-instance"
        self.scope = ["read:data:logs", "read:data:metrics"]

    def analyze(self) -> Dict[str, Any]:
        """Perform analysis with available scope."""
        headers = {"Authorization": f"Bearer {self.token}"}

        print(f"[Analyzer] Analyzing data with scope: {self.scope}")

        # Can read logs and metrics
        analysis = {
            "logs_analyzed": 1000,
            "metrics_collected": 500,
            "anomalies_found": 15,
        }

        print(f"[Analyzer] Analysis complete: {analysis}")
        return analysis

    def delegate_to_specialist(self) -> Optional[str]:
        """Delegate further to specialist with even narrower scope."""
        narrower_scope = ["read:data:logs"]  # Only logs, drop metrics

        if not DelegationChainValidator.is_scope_subset(narrower_scope, self.scope):
            print(f"[Analyzer] ERROR: Cannot escalate scope to {narrower_scope}")
            return None

        try:
            headers = {"Authorization": f"Bearer {self.token}"}
            resp = requests.post(f"{BROKER}/v1/delegate", json={
                "delegate_to": "specialist-agent",
                "scope": narrower_scope,
                "ttl": 120  # 2 minutes for specialist
            }, headers=headers, timeout=10)
            resp.raise_for_status()

            data = resp.json()
            token = data["access_token"]
            chain = data.get("delegation_chain", [])

            print(f"[Analyzer] Delegated to specialist with scope: {narrower_scope}")
            print(f"[Analyzer] Delegation chain depth: {len(chain) + 1}")

            return token
        except Exception as e:
            print(f"[Analyzer] Delegation failed: {e}")
            return None


class SpecialistAgent:
    """Leaf-level agent with most narrow scope."""

    def __init__(self, delegated_token: str):
        self.token = delegated_token
        self.agent_id = "specialist-agent-instance"
        self.scope = ["read:data:logs"]  # Most narrow scope

    def specialize(self) -> Dict[str, Any]:
        """Perform specialized analysis with narrowest scope."""
        headers = {"Authorization": f"Bearer {self.token}"}

        print(f"[Specialist] Performing specialized task with scope: {self.scope}")

        # Can ONLY read logs, cannot access metrics
        result = {
            "log_anomalies_identified": 7,
            "severity_high": 2,
            "severity_medium": 5,
        }

        print(f"[Specialist] Specialized analysis: {result}")

        # Attempt to escalate (would fail)
        print(f"[Specialist] Attempting to access metrics (not in scope)...")
        print(f"[Specialist] ✗ Request rejected: scope insufficient for read:data:metrics")

        return result


def run_delegation_cascade():
    """Execute a 3-level delegation cascade."""
    print("=== Delegation Chain with Scope Narrowing ===\n")
    print("Orchestrator (read:data:*)")
    print("  ↓ delegates narrower scope")
    print("Analyzer (read:data:logs, read:data:metrics)")
    print("  ↓ delegates even narrower scope")
    print("Specialist (read:data:logs)")
    print()

    # Level 1: Orchestrator bootstraps
    orch = OrchestratorAgent()
    if not orch.bootstrap():
        print("Failed to bootstrap orchestrator")
        return False

    # Level 2: Orchestrator delegates to Analyzer
    analyzer_token = orch.delegate_to_analyzer()
    if not analyzer_token:
        print("Failed to delegate to analyzer")
        return False

    analyzer = AnalyzerAgent(analyzer_token)
    analysis = analyzer.analyze()

    # Level 3: Analyzer delegates to Specialist
    specialist_token = analyzer.delegate_to_specialist()
    if not specialist_token:
        print("Failed to delegate to specialist")
        return False

    specialist = SpecialistAgent(specialist_token)
    result = specialist.specialize()

    print("\n✓ Delegation cascade completed")
    print(f"  Orchestrator → Analyzer → Specialist")
    print(f"  Scope narrowed at each level (no escalation possible)")
    print(f"  All actions recorded in audit trail with chain signatures")

    return True


if __name__ == "__main__":
    run_delegation_cascade()
```

### Security Considerations

1. **Scope Attenuation is Enforced:** The broker validates that delegated scope is a subset of the delegator's scope. Escalation attempts are rejected.

2. **Chain Depth Limit:** Maximum delegation depth is 5. This prevents infinite chains that could obscure authorization lineage.

3. **Cryptographic Proof:** Each delegation step signs the chain. Forging delegation links would invalidate the signature.

4. **Revocation Propagates Downward:** Revoking the orchestrator at chain-level invalidates all downstream delegations.

### What Goes Wrong Without AgentAuth

```python
# DANGEROUS: Delegation without scope narrowing or proof
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

class OrchestratorDangerous:
    def __init__(self):
        # Broad access
        self.token = "master-key-all-resources"

    def delegate_to_analyzer(self):
        # Just pass the token along
        return self.token  # NO scope narrowing


class AnalyzerDangerous:
    def __init__(self, token):
        # Receives same broad token
        self.token = token  # Still has read:data:*

    def delegate_to_specialist(self):
        # Cannot narrow scope (no mechanism)
        return self.token  # Specialist ALSO gets read:data:*


# Problems:
# 1. No scope attenuation. Analyzer and Specialist have same access as Orchestrator.
# 2. No delegation proof. Can't verify Specialist was authorized by Analyzer.
# 3. No chain signing. A rogue agent could forge delegation claims.
# 4. No depth limit. Could create infinite delegation chains.
# 5. No escalation prevention. Specialist could claim to have broader scope.
#
# With AgentAuth delegation pattern:
# - Each level has narrower scope enforced by broker
# - Cryptographic proof of authorization lineage
# - Chain signatures prevent forgery
# - Depth limit prevents infinite chains
# - Escalation is cryptographically impossible
```

---

## Pattern 5: Emergency Revocation Cascade

### Use Case

When a security incident occurs, operators need to immediately invalidate compromised credentials at varying blast radii. A single malicious agent might only require revoking one token, but a major vulnerability might require revoking all tokens across a chain or task.

### When to Use

Use this pattern when:
- You need incident response capabilities
- Scope of compromise varies (single agent, entire task, whole chain)
- You need to revoke without redeploying agents
- Audit trail must show revocation events

### Flow Diagram

```mermaid
flowchart TB
    subgraph incident["Security Incident"]
        E["Suspicious activity detected:<br/>Unusual data access,<br/>Failed auth attempts,<br/>Policy violations"]
    end

    subgraph operator["Operator Actions"]
        Auth["1. Authenticate with AA_ADMIN_SECRET"]
        Assess["2. Assess blast radius"]
        Decide{"3. Choose revocation level"}
    end

    subgraph revocation["Revocation Levels"]
        TokenRev["Token Level<br/>Single token by JTI"]
        AgentRev["Agent Level<br/>All tokens for SPIFFE ID"]
        TaskRev["Task Level<br/>All tokens for task_id"]
        ChainRev["Chain Level<br/>All delegated tokens from root"]
    end

    subgraph broker["Broker Action"]
        UpdateRevList["Update revocation list"]
        LogEvent["Log revocation event"]
        Notify["Notify all resource servers"]
    end

    subgraph effect["Effect"]
        Invalid["All tokens at chosen level<br/>become permanently invalid"]
    end

    E --> Auth
    Auth --> Assess
    Assess --> Decide

    Decide -->|"Isolated incident"| TokenRev
    Decide -->|"Agent compromised"| AgentRev
    Decide -->|"Data poisoning"| TaskRev
    Decide -->|"Privilege escalation"| ChainRev

    TokenRev --> UpdateRevList
    AgentRev --> UpdateRevList
    TaskRev --> UpdateRevList
    ChainRev --> UpdateRevList

    UpdateRevList --> LogEvent
    LogEvent --> Notify
    Notify --> Invalid

    style TokenRev fill:#e8f5e9
    style AgentRev fill:#fff3e0
    style TaskRev fill:#fce4ec
    style ChainRev fill:#f3e5f5
```

### Python Code Example

```python
import os
import requests
from typing import Dict, Any, Optional
from enum import Enum

BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "https://broker.internal:8080")
ADMIN_SECRET = os.environ.get("AA_ADMIN_SECRET")


class RevocationLevel(Enum):
    """Four levels of revocation with increasing blast radius."""
    TOKEN = "token"      # Single token
    AGENT = "agent"      # All tokens for an agent
    TASK = "task"        # All tokens for a task
    CHAIN = "chain"      # All delegated tokens from root


class IncidentAnalyzer:
    """Assess incident and recommend revocation level."""

    @staticmethod
    def analyze_incident(incident_type: str) -> RevocationLevel:
        """Recommend revocation level based on incident type."""

        incident_map = {
            "token_leaked": RevocationLevel.TOKEN,
            "agent_compromised": RevocationLevel.AGENT,
            "data_poisoned": RevocationLevel.TASK,
            "privilege_escalation": RevocationLevel.CHAIN,
        }

        return incident_map.get(incident_type, RevocationLevel.TOKEN)


class EmergencyRevocationManager:
    """Manage emergency revocation operations."""

    def __init__(self, broker_url: str, admin_secret: str):
        self.broker = broker_url
        self.admin_secret = admin_secret
        self.admin_token = None

    def authenticate(self) -> bool:
        """Authenticate as operator using admin secret."""
        try:
            resp = requests.post(f"{self.broker}/v1/admin/auth", json={
                "secret": self.admin_secret
            }, timeout=10)
            resp.raise_for_status()

            data = resp.json()
            self.admin_token = data["access_token"]
            print("[Revocation] Operator authenticated")
            return True
        except Exception as e:
            print(f"[Revocation] Authentication failed: {e}")
            return False

    def revoke_token(self, jti: str, reason: str) -> bool:
        """Revoke a single token by JTI (minimal blast radius)."""
        return self._revoke(
            level=RevocationLevel.TOKEN,
            target=jti,
            reason=reason
        )

    def revoke_agent(self, agent_id: str, reason: str) -> bool:
        """Revoke all tokens for an agent (agent-level blast radius)."""
        return self._revoke(
            level=RevocationLevel.AGENT,
            target=agent_id,
            reason=reason
        )

    def revoke_task(self, task_id: str, reason: str) -> bool:
        """Revoke all tokens for a task (task-level blast radius)."""
        return self._revoke(
            level=RevocationLevel.TASK,
            target=task_id,
            reason=reason
        )

    def revoke_chain(self, root_agent_id: str, reason: str) -> bool:
        """Revoke delegation chain from root (chain-level blast radius)."""
        return self._revoke(
            level=RevocationLevel.CHAIN,
            target=root_agent_id,
            reason=reason
        )

    def _revoke(self, level: RevocationLevel, target: str, reason: str) -> bool:
        """Execute revocation at specified level."""
        if not self.admin_token:
            print("[Revocation] Not authenticated")
            return False

        try:
            headers = {"Authorization": f"Bearer {self.admin_token}"}

            resp = requests.post(f"{self.broker}/v1/revoke", json={
                "level": level.value,
                "target": target,
                "reason": reason,
            }, headers=headers, timeout=10)
            resp.raise_for_status()

            print(f"[Revocation] Revoked at {level.value} level")
            print(f"[Revocation] Target: {target}")
            print(f"[Revocation] Reason: {reason}")
            return True
        except Exception as e:
            print(f"[Revocation] Revocation failed: {e}")
            return False


class AuditQueryManager:
    """Query audit trail for revocation verification."""

    def __init__(self, broker_url: str, admin_token: str):
        self.broker = broker_url
        self.admin_token = admin_token

    def verify_revocation(self, target: str, level: str) -> Dict[str, Any]:
        """Verify that tokens were revoked."""
        try:
            headers = {"Authorization": f"Bearer {self.admin_token}"}

            # Query recent revocation events
            resp = requests.get(f"{self.broker}/v1/audit/events", json={
                "event_type": "token_revoked",
                "limit": 10,
            }, headers=headers, timeout=10)
            resp.raise_for_status()

            events = resp.json().get("events", [])

            # Find matching revocation
            matching = [e for e in events if target in str(e)]

            return {
                "target": target,
                "level": level,
                "revocation_found": len(matching) > 0,
                "events": matching,
            }
        except Exception as e:
            print(f"[Audit] Query failed: {e}")
            return {"error": str(e)}


def run_incident_response():
    """Demonstrate emergency revocation scenarios."""
    print("=== Emergency Revocation Cascade ===\n")

    if not ADMIN_SECRET:
        print("ERROR: AA_ADMIN_SECRET not set")
        print("In production, this is provided by operator authentication")
        return False

    mgr = EmergencyRevocationManager(BROKER, ADMIN_SECRET)

    # Authenticate
    if not mgr.authenticate():
        return False

    print("\n--- Incident Scenario 1: Token Leaked ---")
    print("A single token was exposed in error logs")
    print("Blast radius: Minimal (only one token)")
    mgr.revoke_token(
        jti="token-abc123",
        reason="Token leaked in error logs"
    )

    print("\n--- Incident Scenario 2: Agent Compromised ---")
    print("An agent was compromised via prompt injection")
    print("Blast radius: Agent-level (all tokens for that agent)")
    mgr.revoke_agent(
        agent_id="spiffe://agentauth.local/agent/orch-456/task-789/a1b2c3d4",
        reason="Agent compromised via prompt injection attack"
    )

    print("\n--- Incident Scenario 3: Data Poisoning ---")
    print("An entire task was tainted with poisoned data")
    print("Blast radius: Task-level (all agents in task)")
    mgr.revoke_task(
        task_id="analysis-2026-0215",
        reason="Data poisoning detected: invalid input values"
    )

    print("\n--- Incident Scenario 4: Privilege Escalation ---")
    print("Delegation chain was exploited for escalation")
    print("Blast radius: Chain-level (all delegated tokens from root)")
    mgr.revoke_chain(
        root_agent_id="spiffe://agentauth.local/agent/orch-456/task-789/root",
        reason="Delegation chain exploited for privilege escalation"
    )

    print("\n✓ Revocation scenarios completed")
    print("\nKey benefits:")
    print("  - Targeted revocation (not all-or-nothing)")
    print("  - Immediate effect (no wait for token expiration)")
    print("  - Audit trail shows what was revoked and why")
    print("  - Operators maintain control without redeployment")

    return True


def demonstrate_revocation_effect():
    """Show what happens when a revoked token is used."""
    print("\n=== Effect of Revocation ===\n")

    print("Scenario: An agent tries to use a revoked token")
    print()

    print("Agent request:")
    print("  GET /api/data")
    print("  Authorization: Bearer <revoked-token>")
    print()

    print("Broker validation pipeline:")
    print("  ✓ Extract Bearer token")
    print("  ✓ Verify EdDSA signature")
    print("  ✓ Check expiration")
    print("  ✗ Check revocation → REVOKED")
    print()

    print("Response:")
    print("  HTTP 403 Forbidden")
    print("  {")
    print("    'status': 403,")
    print("    'title': 'Forbidden',")
    print("    'detail': 'Token has been revoked',")
    print("    'error_code': 'token_revoked'")
    print("  }")
    print()

    print("Effect: Agent cannot proceed. No data accessed.")
    print("Duration: Immediate (no wait for natural expiration)")
    print("Reversal: Only by reissuing token from broker")


if __name__ == "__main__":
    run_incident_response()
    demonstrate_revocation_effect()
```

### Security Considerations

1. **Four Levels of Control:** Token, agent, task, and chain revocation provide granular control. Choose the narrowest blast radius appropriate to the incident.

2. **Immediate Effect:** Revocation is effective immediately on the next request validation. No waiting for token expiration.

3. **No Token Reissue:** Once revoked, a token cannot be reissued without re-registering the agent. This prevents accidental reuse.

4. **Audit Trail:** Every revocation is logged with reason, timestamp, and operator ID. This provides accountability.

5. **Idempotent:** Revoking an already-revoked token is safe (succeeds with no error).

### What Goes Wrong Without AgentAuth

```python
# DANGEROUS: Incident response without revocation mechanism
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Anti-pattern: Long-lived keys with no revocation
SHARED_DB_KEY = "sk-master-prod-database"  # Valid for 6 months

class IncidentResponseWithoutRevocation:
    def respond_to_compromise(self, compromised_agent: str):
        print(f"Agent {compromised_agent} has been compromised!")

        # Option 1: Rotate the shared key
        print("Rotating database key...")
        # PROBLEM: This breaks ALL agents, not just the compromised one
        # Breaks 50+ agents still doing legitimate work

        # Option 2: Wait for key expiration
        print("Waiting for key to expire in 6 months...")
        # PROBLEM: Attacker has 6 months to exfiltrate data

        # Option 3: Update firewall rules
        print("Adding firewall rule to block agent IP...")
        # PROBLEM: Agent could move to different IP
        # Legitimate traffic from that IP is also blocked


# With AgentAuth emergency revocation:
# 1. Agent revocation: Invalidates only compromised agent's tokens
# 2. Task revocation: If needed, invalidate all agents in a task
# 3. Chain revocation: If needed, invalidate entire delegation tree
# 4. Immediate effect: Active in milliseconds, not hours or months
# 5. No collateral damage: Other agents continue working
```

---

## Pattern 6: App-Managed Agent Registration

### Use Case

Advanced use cases where an orchestration app manages agent registration with the broker. Useful for:
- Apps that manage multiple agents in production
- Agents requiring app-controlled credential lifecycle
- Decentralized agent networks with central control
- Compliance scenarios requiring agent-controlled key material

### When to Use

Use this pattern when:
- An app must create launch tokens for agents it manages
- You need fine-grained control over agent credential lifetime
- You require audit trail of which app created which agents
- You have multiple apps managing overlapping agent populations

### Flow Diagram

```mermaid
sequenceDiagram
    participant App
    participant Broker
    participant Agent
    participant Resource

    Note over App: 1. Authenticate as app<br/>with client credentials

    App->>Broker: POST /v1/app/auth
    Broker-->>App: app_token (short-lived)

    Note over App: 2. Create launch token for agent

    App->>Broker: POST /v1/admin/launch-tokens<br/>Bearer app_token<br/>{ agent_name, scope, ttl }

    Broker-->>App: { launch_token, expires_at }

    Note over App: 3. Deliver launch token to agent

    App->>Agent: launch_token

    Note over Agent: 4. Register using launch token

    Agent->>Broker: GET /v1/challenge
    Broker-->>Agent: nonce

    Agent->>Broker: POST /v1/register<br/>{ launch_token, nonce, signature }
    Broker-->>Agent: { agent_id, access_token, expires_in }

    Agent->>Resource: GET /api/data<br/>Authorization: Bearer <token>
    Resource-->>Agent: Data
```

### Python Code Example

```python
import os
import requests
import secrets
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ed25519
from cryptography.hazmat.backends import default_backend
from typing import Dict, Any, Optional

BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "https://broker.internal:8080")
ADMIN_SECRET = os.environ.get("AA_ADMIN_SECRET")
TASK_ID = "byok-task-2026-0215"


class Ed25519KeyManager:
    """Manage Ed25519 keypairs for BYOK registration."""

    def __init__(self):
        self.private_key = None
        self.public_key = None

    def generate_keypair(self) -> bool:
        """Generate a new Ed25519 keypair locally."""
        try:
            self.private_key = ed25519.Ed25519PrivateKey.generate()
            self.public_key = self.private_key.public_key()

            print("[BYOK] Generated Ed25519 keypair")
            print(f"[BYOK] Public key (hex): {self._public_key_hex()[:32]}...")
            return True
        except Exception as e:
            print(f"[BYOK] Key generation failed: {e}")
            return False

    def _public_key_hex(self) -> str:
        """Export public key as hex string."""
        public_bytes = self.public_key.public_bytes(
            encoding=serialization.Encoding.Raw,
            format=serialization.PublicFormat.Raw
        )
        return public_bytes.hex()

    def _public_key_pem(self) -> str:
        """Export public key as PEM string."""
        public_bytes = self.public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo
        )
        return public_bytes.decode("utf-8")

    def sign_nonce(self, nonce: str) -> str:
        """Sign a nonce (hex string) with the private key."""
        nonce_bytes = bytes.fromhex(nonce)
        signature = self.private_key.sign(nonce_bytes)
        return signature.hex()


class OperatorLaunchTokenIssuer:
    """Operator issues single-use launch tokens for BYOK agents."""

    def __init__(self, broker_url: str, admin_secret: str):
        self.broker = broker_url
        self.admin_secret = admin_secret
        self.admin_token = None

    def authenticate(self) -> bool:
        """Authenticate as operator."""
        try:
            resp = requests.post(f"{self.broker}/v1/admin/auth", json={
                "secret": self.admin_secret
            }, timeout=10)
            resp.raise_for_status()
            data = resp.json()
            self.admin_token = data["access_token"]
            print("[Operator] Authenticated with broker")
            return True
        except Exception as e:
            print(f"[Operator] Authentication failed: {e}")
            return False

    def issue_launch_token(self, agent_name: str, allowed_scope: str) -> Optional[str]:
        """Issue a single-use launch token for an agent."""
        if not self.admin_token:
            print("[Operator] Not authenticated")
            return None

        try:
            headers = {"Authorization": f"Bearer {self.admin_token}"}
            resp = requests.post(f"{self.broker}/v1/admin/launch-tokens", json={
                "agent_name": agent_name,
                "allowed_scope": allowed_scope.split(","),
                "max_ttl": 300,
                "single_use": True,
                "ttl": 60  # Launch token expires in 60 seconds
            }, headers=headers, timeout=10)
            resp.raise_for_status()

            data = resp.json()
            token = data["launch_token"]
            print(f"[Operator] Issued launch token for {agent_name}")
            print(f"[Operator] Launch token: {token[:16]}...")
            print(f"[Operator] Expires in: {data.get('expires_in', 60)} seconds")
            return token
        except Exception as e:
            print(f"[Operator] Launch token issuance failed: {e}")
            return None


class BYOKAgent:
    """Agent that registers using its own Ed25519 keypair (BYOK)."""

    def __init__(self, agent_name: str, launch_token: str):
        self.agent_name = agent_name
        self.launch_token = launch_token
        self.keys = Ed25519KeyManager()
        self.token = None
        self.agent_id = None

    def register(self, scope: str) -> bool:
        """Register with BYOK: generate keys, sign challenge, register."""

        # Step 1: Generate keypair locally
        if not self.keys.generate_keypair():
            return False

        # Step 2: Get challenge nonce
        nonce = self._get_challenge()
        if not nonce:
            return False

        # Step 3: Sign nonce with private key (private key never leaves agent)
        signature = self.keys.sign_nonce(nonce)

        # Step 4: Register with broker
        return self._register_with_broker(scope, nonce, signature)

    def _get_challenge(self) -> Optional[str]:
        """Get a challenge nonce from the broker."""
        try:
            resp = requests.get(f"{BROKER}/v1/challenge", timeout=10)
            resp.raise_for_status()
            data = resp.json()
            nonce = data["nonce"]
            print(f"[BYOK] Got challenge nonce: {nonce[:16]}...")
            return nonce
        except Exception as e:
            print(f"[BYOK] Challenge request failed: {e}")
            return None

    def _register_with_broker(self, scope: str, nonce: str, signature: str) -> bool:
        """Register the agent and obtain access token."""
        try:
            resp = requests.post(f"{BROKER}/v1/register", json={
                "launch_token": self.launch_token,
                "nonce": nonce,
                "public_key": self.keys._public_key_hex(),
                "signature": signature,
                "orch_id": "orch-standalone-byok",
                "task_id": TASK_ID,
                "requested_scope": [scope]
            }, timeout=10)
            resp.raise_for_status()

            data = resp.json()
            self.token = data["access_token"]
            self.agent_id = data["agent_id"]

            print(f"[BYOK] Registered successfully")
            print(f"[BYOK] Agent ID: {self.agent_id}")
            print(f"[BYOK] Access token: {self.token[:32]}...")
            return True
        except Exception as e:
            print(f"[BYOK] Registration failed: {e}")
            return False

    def use_token(self) -> bool:
        """Use the token to access a resource (simulate)."""
        if not self.token:
            print("[BYOK] No token available")
            return False

        headers = {"Authorization": f"Bearer {self.token}"}
        print(f"[BYOK] Using token to access protected resource")
        print(f"[BYOK] Headers: Authorization: Bearer <token>")

        # In a real system, this would call an actual API
        print(f"[BYOK] ✓ Request successful (simulated)")
        return True


def run_byok_flow():
    """Demonstrate BYOK registration flow."""
    print("=== BYOK (Bring Your Own Key) Registration ===\n")

    if not ADMIN_SECRET:
        print("ERROR: AA_ADMIN_SECRET not set")
        print("Operator needs admin credentials to issue launch tokens")
        return False

    # Operator phase: Issue launch token
    print("=== Operator Phase: Issue Launch Token ===\n")
    issuer = OperatorLaunchTokenIssuer(BROKER, ADMIN_SECRET)
    if not issuer.authenticate():
        return False

    launch_token = issuer.issue_launch_token(
        agent_name="standalone-byok-agent",
        allowed_scope="read:data:*,write:results:*"
    )

    if not launch_token:
        return False

    # Agent phase: Register with BYOK
    print("\n=== Agent Phase: BYOK Registration ===\n")

    agent = BYOKAgent("standalone-byok-agent", launch_token)

    if not agent.register(scope="read:data:*"):
        return False

    # Agent uses token
    print("\n=== Agent Phase: Use Token ===\n")

    if not agent.use_token():
        return False

    print("\n✓ BYOK flow completed successfully")
    print("\nKey security properties:")
    print("  - Private key never leaves agent")
    print("  - Broker never sees private key")
    print("  - Launch token is single-use")
    print("  - Identity proven via Ed25519 signature")

    return True


def demonstrate_byok_key_management():
    """Show local key management best practices."""
    print("\n=== BYOK Key Management Best Practices ===\n")

    print("1. Key Generation (local, isolated):")
    print("   - Generate Ed25519 keypair on agent startup")
    print("   - Never share private key with broker or network")
    print("   - Store in secure enclave if available")

    print("\n2. Nonce Signing (local):")
    print("   - Receive nonce from broker")
    print("   - Sign nonce with private key locally")
    print("   - Send signature + public key to broker (never private key)")

    print("\n3. Key Rotation:")
    print("   - Issue new launch token for agent")
    print("   - Agent generates new keypair")
    print("   - Register with new public key")
    print("   - Old credentials expire naturally")

    print("\n4. Key Lifecycle:")
    print("   - Generate: Agent startup (or on-demand)")
    print("   - Register: Challenge-response flow")
    print("   - Use: Include in /v1/register and /v1/delegate calls")
    print("   - Cleanup: Delete when agent terminates")

    print("\nComparison:")
    print("  App-Managed Path:")
    print("    ✓ Central control over credentials")
    print("    ✓ Audit trail of agent creation")
    print("    ✗ Requires app infrastructure")

    print("\n  Direct Agent Registration Path:")
    print("    ✓ Agent controls credentials directly")
    print("    ✓ Independent agent management")
    print("    ✗ More complex (requires cryptography)")
    print("    ✗ Requires operational discipline")


if __name__ == "__main__":
    run_byok_flow()
    demonstrate_byok_key_management()
```

### Security Considerations

1. **Private Key Custody:** The agent's private key never leaves the agent. The broker never sees it. This is the core security property of BYOK.

2. **Nonce Signing:** Nonces are single-use (30-second TTL). Signing prevents replay attacks.

3. **Launch Token Single-Use:** The launch token can only be used once. After registration, it's consumed and cannot be reused.

4. **Ed25519 Signature Verification:** The broker verifies the Ed25519 signature over the nonce. Invalid signatures are rejected immediately.

5. **Key Rotation:** New keypairs should be generated for each task or on a schedule. Compromised keys affect only credentials signed with that key.

### What Goes Wrong Without AgentAuth

```python
# DANGEROUS: Manual key management without standards
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Anti-pattern 1: Sharing private keys in environment
os.environ["AGENT_PRIVATE_KEY"] = "-----BEGIN PRIVATE KEY-----\n..."
# PROBLEM: Private keys in environment variables are read by:
#   - Docker inspect
#   - /proc/[pid]/environ
#   - System logs
#   - Crash dumps


# Anti-pattern 2: No nonce verification
class UnsafeRegistration:
    def register(self):
        # Sending public key but no proof of private key
        requests.post("https://broker/register", json={
            "public_key": "...",
            # Missing: signature over nonce
        })
        # PROBLEM: Anyone could register with anyone else's public key


# Anti-pattern 3: Reusing keys indefinitely
# Generated once, used for months
AGENT_KEY = ed25519.Ed25519PrivateKey.generate()  # One-time setup
# PROBLEM: If compromised, valid for months
# With AgentAuth: Keys rotated per-task, valid for minutes


# With AgentAuth BYOK:
# - Nonce challenge proves key possession without exposing it
# - Private keys stay local and never transmitted
# - Single-use launch tokens prevent registration replay
# - Keys rotated frequently (per-task)
```

---

## Security Checklist

When implementing AgentAuth patterns, verify:

### Credentials and Tokens

- [ ] Agents never store tokens in environment variables (request at runtime)
- [ ] Tokens expire within minutes (default 300 seconds)
- [ ] Private keys (BYOK) never leave the agent
- [ ] Launch tokens are single-use and expire quickly (default 30 seconds)
- [ ] Admin secrets are only used on secure operator machines, never in agents

### Scope and Delegation

- [ ] Scope ceilings are enforced at the broker level
- [ ] Delegation always narrows scope (never escalates)
- [ ] Delegation depth is limited (maximum 5 hops)
- [ ] Scope format is validated: `action:resource:identifier`
- [ ] Wildcard scope (`*`) is used narrowly and intentionally

### Revocation and Incidents

- [ ] Revocation levels are understood (token, agent, task, chain)
- [ ] Operator can revoke without redeploying agents
- [ ] Revocation is tested regularly in incident drills
- [ ] Audit trail captures all revocation events with reason
- [ ] Revoked tokens cannot be reissued (must re-register)

### Audit and Compliance

- [ ] Audit events are logged for all token operations
- [ ] Hash chaining makes audit trail tamper-evident
- [ ] Audit trail is retained per compliance requirements
- [ ] PII is sanitized before logging
- [ ] Query audit trail to verify credential lifecycle

### Deployment and Operations

- [ ] App scope ceilings match the union of all expected scopes
- [ ] Broker is deployed as high-availability, critical infrastructure
- [ ] Audit database is separated from main application database
- [ ] Admin secret rotation is scheduled and tested
- [ ] Monitoring alerts on revocation events

---

## Common Pitfalls

### Pitfall 1: Overly Broad Scope Ceilings

**Problem:** Setting `scope_ceiling: *:*:*` defeats the purpose of scope isolation.

**Solution:** Define scope ceilings narrowly. For a payment microservice, use `read:billing:*,write:billing:*`, not `*:*:*`.

### Pitfall 2: Not Releasing Tokens

**Problem:** Agents request tokens but never call `/v1/token/release`. This makes audit unclear and prevents billing by credential lifetime.

**Solution:** Always release tokens when work is done. Use try/finally to ensure release even if work fails.

### Pitfall 3: Overly Permissive App Scope Ceilings

**Problem:** Registering apps with broad scope ceilings (e.g., `*:*:*`) defeats service isolation.

**Solution:** Register each app with narrow scope ceilings. A payment app should have `read:billing:*,write:billing:*`, not `*:*:*`.

### Pitfall 4: Delegation Loops

**Problem:** Attempting to create circular delegation chains (A delegates to B delegates back to A).

**Solution:** Delegation is always downward in a hierarchy. Ensure your agent topology is acyclic.

### Pitfall 5: Ignoring Audit Trail

**Problem:** Logging token events but never querying the audit trail for incidents.

**Solution:** Establish regular audit reviews. Query for anomalies: tokens never released, unusual scope requests, revocation spikes.

### Pitfall 6: Storing Tokens at Rest

**Problem:** Caching tokens in database or disk for "fast lookup".

**Solution:** Tokens are ephemeral. Request fresh tokens on each task. If caching is necessary, store in memory with short TTL.

---

## Next Steps

- **[Getting Started: Developer](getting-started-developer.md)** — Implement your first agent in 15 minutes
- **[API Reference](api.md)** — Complete endpoint documentation
- **[Troubleshooting](troubleshooting.md)** — Debug common issues
- **[Common Tasks](common-tasks.md)** — Step-by-step workflows

---

**Questions?** See [Troubleshooting](troubleshooting.md) or check recent examples in `/docs/examples/`.
