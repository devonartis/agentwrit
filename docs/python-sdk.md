# AgentWrit Python SDK

The Python client for the broker is live: **[`agentwrit`](https://pypi.org/project/agentwrit/) v0.3.0** on PyPI (MIT, Python 3.10+), source at [`devonartis/agentwrit-python`](https://github.com/devonartis/agentwrit-python). It wraps the broker's Ed25519 challenge-response registration flow into simple calls — you don't manage nonces, signatures, or token renewal manually.

```bash
uv add agentwrit          # or: pip install agentwrit
```

The SDK pulls in `httpx` and `cryptography` automatically.

## Five lines

```python
from agentwrit import AgentWritApp, validate

# App authenticates with its client_id/client_secret (issued by the broker operator).
app = AgentWritApp(broker_url, client_id, client_secret)

# Create an ephemeral agent scoped to one task.
agent = app.create_agent("my-service", "task-1", ["read:data:customer-7291"])

# Use the token at any resource server.
httpx.get("https://api/customers/7291", headers=agent.bearer_header)

# Any service can verify the token against the broker.
validate(app.broker_url, agent.access_token)

# Done — the token dies at the broker.
agent.release()
```

## API surface

**`AgentWritApp(broker_url, client_id, client_secret)`** — the developer's entry point. One app, many agents.

- `create_agent(orch_id, task_id, requested_scope, *, private_key=None, max_ttl=300, label=None)` → `Agent`

**`Agent`** — an ephemeral, per-task principal. Its authority can only narrow from the app's ceiling.

- `bearer_header` → `{"Authorization": "Bearer …"}` for resource-server calls
- `access_token` → the raw JWT
- `renew()` — extend the token; the old one is immediately revoked
- `release()` — revoke the token at the broker
- `delegate(...)` — derive a further-attenuated token for a sub-task

**Module helpers**

- `validate(broker_url, token)` → `ValidateResult` — any service can verify a token
- `scope_is_subset(requested, allowed)` → `bool` — check attenuation before asking

## Current status

- **v0.3.0** — 15 acceptance tests passing against a live broker
- Full agent lifecycle: register, renew, delegate, release
- Structured RFC 7807 error exceptions (`AuthorizationError`, `TransportError`, …)
- **Synchronous** — on FastAPI/Starlette/Sanic, wrap calls in `asyncio.to_thread(...)`

## Without the SDK

You can use the broker's HTTP API directly from any language. See the [Getting Started walkthrough](getting-started-user.md) for the full curl-based registration flow, or the [API Reference](api.md) for all endpoints.
