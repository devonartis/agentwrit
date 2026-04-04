# Mission

AgentAuth is an open-source credential broker for AI agents. It issues short-lived, scope-attenuated tokens so agents operate with only the permissions their task requires — nothing more, nothing longer.

## Core Principles

- **Pattern-driven.** Every feature traces to the [Ephemeral Agent Credentialing v1.3](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.3.md) security pattern. If it's not in the pattern, it doesn't belong in core.
- **Security by default.** Tokens expire in minutes. Permissions are scoped to one task. Revocation works at four levels. The audit trail is tamper-evident. No shortcuts.
- **Pluggable architecture.** Enterprise modules (HITL, OIDC, cloud federation, MCP) plug in via interfaces. Zero add-on code in this repo.
- **Minimal dependencies.** Ed25519, JWT, hash-chain, scope enforcement, revocation — all Go stdlib. 5 direct dependencies total.
- **Open-core model.** This repo becomes open-source. Enterprise add-ons live in separate private repos.

## What AgentAuth Is NOT

- Not an OAuth provider (it issues its own ephemeral tokens, not OAuth tokens)
- Not a service mesh (no sidecars — they were removed in Phase 0)
- Not a secrets manager (it issues credentials, not stores them)

## Who Reads Our Artifacts

- **Executives** read test evidence to decide if a release is safe. Write banners they can understand without asking an engineer.
- **Manual QA testers** follow test evidence to verify features. They may not be deeply technical. Write steps they can follow.
- **Security reviewers** audit the code and evidence for vulnerabilities. Be precise about what was tested and what wasn't.
- **Contributors** read CLAUDE.md, MEMORY.md, and docs/ to onboard. Keep them current.
