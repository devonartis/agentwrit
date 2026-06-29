# AgentWrit Demos

Two complete demo apps ship with the [Python SDK repo](https://github.com/devonartis/agentwrit-python) (`demo/` and `demo2/`). Both run against a live broker via Docker Compose and published images. Each spawns real broker agents with per-task scoped credentials so you can watch scope enforcement, delegation, and TTL expiry on real traffic.

## Run them

The Compose file lives in the SDK repo. It pulls three published images — the broker plus both demos — so nothing needs building:

| Service | Image | Port |
|---|---|---|
| Broker | `devonartis/agentwrit:latest` | 8080 |
| MedAssist | `devonartis/agentwrit-medassist:latest` | 5000 |
| Support Tickets | `devonartis/agentwrit-support-tickets:latest` | 5001 |

```bash
git clone https://github.com/devonartis/agentwrit-python.git
cd agentwrit-python

# Both demos drive an LLM to pick tools. Point at any OpenAI-compatible endpoint.
export LLM_API_KEY="sk-..."          # or a local vLLM / llama.cpp server via LLM_BASE_URL

docker compose up -d broker medassist          # MedAssist → http://localhost:5000
docker compose up -d broker support-tickets    # Support Tickets → http://localhost:5001
```

Each demo auto-registers itself with the broker on startup — no manual app setup. See [`demo/README.md`](https://github.com/devonartis/agentwrit-python/blob/main/demo/README.md) and [`demo2/README.md`](https://github.com/devonartis/agentwrit-python/blob/main/demo2/README.md) for scenario playbooks and code maps.

## MedAssist AI

A FastAPI clinical assistant. You ask a plain-language question about a patient; a local LLM picks tools (records, labs, billing, prescriptions); the app spawns broker agents on demand, each scoped to *one patient and one category*.

| What you'll see | What it proves |
|---|---|
| Agents spawn on demand per LLM tool call | Dynamic agent creation |
| Each agent scoped to one patient ID + category | Per-resource scope isolation |
| LLM asks for the wrong patient's data | Scope enforcement catches cross-boundary access |
| Clinical agent delegates to prescription agent | Delegation with scope attenuation |
| Tokens renew and release at end of encounter | Full lifecycle management |
| Dedicated audit tab | Hash-chained broker events |

## Support Ticket Zero-Trust

Flask + HTMX + SSE. Three LLM-driven agents (triage → knowledge → response) process customer tickets with broker-issued scoped credentials, streaming execution via SSE.

| What you'll see | What it proves |
|---|---|
| Anonymous tickets halt at triage | Identity gates the pipeline |
| `delete_account` / `send_external_email` in the LLM's tool list but not the agent's scope | Dangerous tools never execute — scope, not prompt, is the boundary |
| One scenario skips `release()` | A 5-second TTL dies on its own |

## Build your first agent without the demos

You can also run the broker and issue your first agent token directly. Follow the [Quick Start](../README.md#quick-start) or the [Getting Started walkthrough](getting-started-user.md) for the full curl-based registration flow.
