# Demo Agents (M12)

## Purpose

The demo agents are Python async clients that exercise the full AgentAuth lifecycle: challenge-response registration, scoped resource access, delegation with scope attenuation, and coordinated multi-agent workflows. They demonstrate the security model described in MVP Requirements Section 2.1 through a concrete ticket-resolution scenario where three agents collaborate with least-privilege credentials.

## Design decisions

**Dataclass-based agents over class inheritance**: Each agent is a `@dataclass` that extends `AgentBase`. Dataclasses provide clean construction, immutable-by-default fields, and work well with pytest fixtures. The `AgentBase` handles registration and resource calls; concrete agents only implement `run()`.

**Async httpx over requests**: All HTTP calls use `httpx.AsyncClient` for non-blocking I/O. This matches the resource server middleware pattern (M11) and allows future parallel agent execution. The `BrokerClient` wrapper accepts an optional `http_client` for test injection.

**Delegation token passthrough**: Agent C does not register with the broker. Instead it receives a delegation token from Agent B and uses it directly as a Bearer token. This demonstrates the least-privilege delegation model -- Agent C can only write tickets and send notifications, not read customer data.

**Mock transport for tests**: All 37 tests run against `httpx.MockTransport` handlers that return canned JSON. No live broker or resource server is required. Integration tests use a routing transport that dispatches by URL pattern (port 8080 for broker, anything else for resource server).

**Orchestrator as both library and CLI**: `run_demo()` is an async function suitable for programmatic use. The `__main__.py` wrapper provides a CLI with `--mode`, `--broker-url`, and `--launch-token` flags for manual runs.

## Architecture

```
Orchestrator
  |
  +-- Agent A (DataRetriever)
  |     register(read:Customers:12345) -> GET /customers/12345
  |     returns: customer_data
  |
  +-- Agent B (Analyzer)
  |     register(read:Customers, read:Orders, write:Tickets, invoke:Notifications)
  |     GET /orders/12345 -> analyze -> POST /v1/delegate (attenuated scope)
  |     returns: resolution, delegation_token
  |
  +-- Agent C (ActionTaker)
        uses delegation_token (no registration)
        PUT /tickets/789 -> POST /notifications/send
        returns: ticket_updated, notification_sent
```

## File layout

```
demo/
  agents/
    __init__.py
    __main__.py           -- CLI entrypoint (python -m agents)
    broker_client.py      -- async httpx wrapper for broker REST API
    agent_base.py         -- AgentBase: Ed25519 keys, registration, resource calls
    agent_retriever.py    -- Agent A: DataRetriever
    agent_analyzer.py     -- Agent B: Analyzer with delegation
    agent_action.py       -- Agent C: ActionTaker via delegated token
    orchestrator.py       -- run_demo() + CLI + DemoResult
    tests/
      __init__.py
      conftest.py         -- shared mock fixtures
      test_agent_base.py  -- BrokerClient + AgentBase tests (15)
      test_agent_retriever.py -- Agent A tests (3)
      test_agent_analyzer.py  -- Agent B tests (4)
      test_agent_action.py    -- Agent C tests (4)
      test_orchestrator.py    -- orchestrator wiring tests (4)
      test_integration.py     -- full pipeline tests (7)
```

## Running

```bash
cd demo
pip install -r requirements.txt

# Run all agent tests (no live services needed)
pytest agents/tests/ -v

# Run the demo in insecure mode (resource server must be running on :8090)
python -m agents --mode insecure --resource-url http://localhost:8090

# Run in secure mode (broker on :8080, resource server on :8090)
AA_SEED_TOKENS=true go run ./cmd/broker  # in another terminal
python -m agents --mode secure \
  --launch-token "$SEED_LAUNCH_TOKEN" \
  --broker-url http://localhost:8080 \
  --resource-url http://localhost:8090
```

## Scope model

| Agent | Registered Scopes | Resource Access |
|-------|------------------|-----------------|
| Agent A | `read:Customers:{id}` | GET /customers/{id} |
| Agent B | `read:Customers:{id}`, `read:Orders:{id}`, `write:Tickets:{tid}`, `invoke:Notifications` | GET /orders/{id} |
| Agent C | (delegated) `write:Tickets:{tid}`, `invoke:Notifications` | PUT /tickets/{tid}, POST /notifications/send |

Agent C's scope is strictly attenuated from Agent B's scope -- it cannot read customers or orders.
