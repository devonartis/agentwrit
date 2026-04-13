# AgentWrit Python SDK

> **Coming soon.** The Python SDK is in active development and will be published as an open-source repo at [`devonartis/agentwrit-python`](https://github.com/devonartis/agentwrit-python) once it's ready.

## What it does

The Python SDK wraps the broker's Ed25519 challenge-response registration flow into simple function calls. You don't need to manage nonces, signatures, or token renewal manually.

```python
from agentauth import AgentAuthApp

agent = AgentAuthApp(broker_url="http://localhost:8080").register(
    launch_token=LAUNCH_TOKEN,
    task_id="read-customer-42",
    requested_scope=["read:data:customers:42"],
)

# Use the token at your resource server
response = httpx.get(url, headers=agent.bearer_header)

# Done — release the credential
agent.release()
```

## Current status

- **v0.3.0** — 15 acceptance tests passing against a live broker
- Full agent lifecycle: register, renew, delegate, release
- Scope checking and validation helpers
- `pip install agentauth` *(PyPI rename to `agentwrit` pending)*

## In the meantime

You can use the broker's HTTP API directly from any language. See the [Getting Started walkthrough](getting-started-user.md) for the full curl-based registration flow, or the [API Reference](api.md) for all endpoints.

## Get notified

Watch this repo or [file an issue](https://github.com/devonartis/agentwrit/issues) to be notified when the SDK goes public.
