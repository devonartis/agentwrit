# AgentWrit Python SDK

The Python SDK is live on [PyPI](https://pypi.org/project/agentwrit/) and [GitHub](https://github.com/devonartis/agentwrit-python).

## What it does

The Python SDK wraps the broker's Ed25519 challenge-response registration flow into simple function calls. You don't need to manage nonces, signatures, or token renewal manually.

```python
from agentwrit import AgentWritApp

agent = AgentWritApp(broker_url="http://localhost:8080").register(
    launch_token=LAUNCH_TOKEN,
    task_id="read-customer-42",
    requested_scope=["read:data:customers:42"],
)

# Use the token at your resource server
response = httpx.get(url, headers=agent.bearer_header)

# Done — release the credential
agent.release()
```

## Install

```bash
pip install agentwrit
```

## Current status

- Full agent lifecycle: register, renew, delegate, release
- Scope checking and validation helpers
- 11 CI gates including bandit and pip-audit
- Live demos: MedAssist AI healthcare pipeline, Support Ticket zero-trust agents

## Demos

See [Live Demos](demos.md) for working examples that run against a real broker — including multi-agent LLM pipelines with scope isolation, delegation, and per-patient scoping.

## Links

- **PyPI:** [pypi.org/project/agentwrit](https://pypi.org/project/agentwrit/)
- **GitHub:** [devonartis/agentwrit-python](https://github.com/devonartis/agentwrit-python)
- **Docker Hub:** [devonartis/agentwrit-medassist](https://hub.docker.com/r/devonartis/agentwrit-medassist) (MedAssist demo)

## Using the broker directly

You can also use the broker's HTTP API directly from any language. See the [Getting Started walkthrough](getting-started-user.md) for the full curl-based registration flow, or the [API Reference](api.md) for all endpoints.
