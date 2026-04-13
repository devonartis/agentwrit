# AgentWrit Documentation

AgentWrit is an open-source credential broker for AI agents. It issues short-lived, scope-attenuated tokens so agents operate with only the permissions their task requires — nothing more, nothing longer.

---

## The Book of AgentWrit

Start here. These pages explain what AgentWrit is, why it exists, and how every piece fits together.

| Page | What you'll learn |
|------|-------------------|
| [What Is AgentWrit?](agentwrit-explained.md) | The problem, the solution, and the three token types — no prior knowledge required |
| [Foundations](foundations.md) | What tokens are, why they beat API keys, and how JWTs work under the hood |
| [The Three Actors](roles.md) | Operator, Application, Agent — who holds what token and why |
| [Scopes and Permissions](scope-model.md) | The `action:resource:identifier` format, coverage rules, and the four enforcement points |
| [The Credential Lifecycle](credential-model.md) | Every credential's claims, TTLs, and how they flow through the attenuation chain |
| [Design Decisions](design-decisions.md) | Why we chose JWTs, Ed25519, SPIFFE, hash-chained audit, and everything else |

---

## Getting Started

Hands-on guides for each persona. Pick the one that matches your role.

| If you are... | Start here |
|---------------|-----------|
| **Just trying AgentWrit** to see how it works | [Your First Five Minutes](getting-started-user.md) |
| **Building an AI agent** in Python, TypeScript, or Go | [Getting Started: Developer](getting-started-developer.md) |
| **Deploying AgentWrit** in production | [Getting Started: Operator](getting-started-operator.md) |

---

## Guides

Deeper walkthroughs for specific tasks and patterns.

| Guide | What it covers |
|-------|---------------|
| [Common Tasks](common-tasks.md) | Token renewal, delegation, revocation, audit queries — the everyday operations |
| [Integration Patterns](integration-patterns.md) | Resource server validation, multi-agent orchestration, cloud federation |
| [Scenarios](scenarios.md) | End-to-end walkthroughs: data pipeline agent, customer service bot, CI/CD runner |
| [Troubleshooting](troubleshooting.md) | Common errors, what causes them, and how to fix them |

---

## Reference

Lookup documentation for endpoints, CLI commands, and internals.

| Reference | What it covers |
|-----------|---------------|
| [API Reference](api.md) | All 19 HTTP endpoints — request/response formats, error codes, rate limits |
| [CLI Reference (awrit)](awrit-reference.md) | Every `awrit` command with examples and output formats |
| [Architecture](architecture.md) | Internal package map, component diagrams, data flow |
| [Implementation Map](implementation-map.md) | Where every feature lives in the codebase — file paths, function names, test locations |
| [Concepts Deep Dive](concepts.md) | The full security pattern, industry context, and all eight components |

---

## Live Demos

See AgentWrit in action with the [Python SDK](python-sdk.md) demo applications:

| Demo | What it shows |
|------|-------------|
| **[MedAssist AI](demos.md)** | Healthcare multi-agent pipeline — clinical, prescription, and billing agents operating under strict scope isolation with LLM tool-calling, delegation, and per-patient scoping |
| **[Support Ticket Zero-Trust](demos.md)** | Three LLM-driven agents processing support tickets with broker-issued scoped credentials, streaming execution via SSE, and natural token expiry |

Both demos run against a real AgentWrit broker and show the full credential lifecycle: agent registration, scope enforcement, delegation, renewal, release, and revocation.

---

## Reading Order

If you're new, this path gets you productive fastest:

```
What Is AgentWrit?  →  Your First Five Minutes  →  Pick your persona guide
        ↓                                                    ↓
   Foundations  →  The Three Actors  →  Scopes  →  Common Tasks
```

If you're evaluating AgentWrit for your organization, start with [What Is AgentWrit?](agentwrit-explained.md) — it's written for people who aren't deeply technical.

If you're a security reviewer, start with [Concepts Deep Dive](concepts.md) and [Architecture](architecture.md).
