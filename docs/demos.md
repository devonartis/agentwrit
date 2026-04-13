# AgentWrit Demos

> **Coming soon.** The demo applications ship with the Python SDK and will be available when [`devonartis/agentwrit-python`](https://github.com/devonartis/agentwrit-python) goes public.

## MedAssist AI

A FastAPI web app where a local LLM dynamically creates broker agents with per-patient scoped credentials. You enter a patient ID and a plain-language request. The LLM chooses which tools to call, and the app creates agents with only the scopes those tools need — for that specific patient.

| What you'll see | What it proves |
|---|---|
| Agents spawn on demand per LLM tool call | Dynamic agent creation |
| Each agent scoped to one patient ID | Per-resource scope isolation |
| LLM asks for wrong patient's data | Scope enforcement catches cross-boundary access |
| Clinical agent delegates to prescription agent | Delegation with scope attenuation |
| Tokens renew and release at end of encounter | Full lifecycle management |
| Dedicated audit tab | Hash-chained broker events |

## Support Ticket Zero-Trust

Three LLM-driven agents process support tickets with broker-issued scoped credentials, streaming execution via SSE, and natural token expiry.

## In the meantime

You can follow the [Quick Start](../README.md#quick-start) to run the broker and issue your first agent token in five minutes. The [Getting Started walkthrough](getting-started-user.md) covers the full registration flow with curl.

## Get notified

Watch this repo or [file an issue](https://github.com/devonartis/agentwrit/issues) to be notified when demos are available.
