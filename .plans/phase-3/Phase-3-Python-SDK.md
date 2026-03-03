# Phase 3: Python SDK

**Status:** Spec
**Priority:** P1 — developer experience, makes Token Proxy truly optional
**Effort estimate:** 3-5 days
**Depends on:** Phase 1a (app auth), Phase 1b (app-scoped launch tokens)
**Architecture doc:** `../.plans/CoWork-Architecture-Direct-Broker.md`

---

## Overview: What We're Building and Why

After Phases 1a-2, apps can register, authenticate, create agents, get audited, and be revoked — all without the master key. But for a developer to actually USE this, they need to: (1) authenticate the app with the broker, (2) create a launch token, (3) generate an Ed25519 keypair, (4) request a challenge nonce, (5) sign the nonce, (6) register the agent with the signed nonce, (7) handle token caching, and (8) renew tokens before they expire. That's 8 steps involving cryptography, HTTP calls, and state management.

**Phase 3 wraps all of this into one function call: `client.get_token(agent_name, scope)`.** The Python SDK handles app authentication, launch token creation, Ed25519 key generation, challenge-response, token caching, and automatic renewal — invisibly. The developer never sees nonces, keypairs, or renewal logic.

This is the same simplification the Token Proxy was built to provide — but as a library instead of infrastructure. The developer gets `pip install agentauth` instead of "deploy a sidecar." This makes the Token Proxy genuinely optional: it still has value for infrastructure-level circuit breaking and caching, but it's no longer the only path to a simple developer experience.

**What's built:** A pip-installable Python package (`agentauth`) with `AgentAuthClient` for token acquisition, caching, renewal, validation, revocation, and delegation. Custom exception hierarchy for actionable errors. Retry with exponential backoff. Ed25519 keys generated in memory (never persisted).

**What stays the same:** The SDK uses the exact same broker API as the sidecar and raw HTTP. No new broker endpoints. No special treatment. The broker doesn't know or care whether the caller is an SDK, a sidecar, or a curl command.

---

## Problem Statement

Today, the only way for a developer to interact with the broker is either through the Token Proxy (sidecar) or by hand-crafting HTTP calls. The sidecar wraps 5+ broker calls into a single `/v1/token` call — but it requires deploying infrastructure. Hand-crafting HTTP means the developer needs to understand Ed25519 challenge-response, nonce handling, scope formatting, and token renewal.

A Python SDK gives developers the same simplicity as the sidecar (`client.get_token(scope=[...])`) without requiring infrastructure deployment. This makes the Token Proxy genuinely optional.

---

## Goals

1. Developers can get an agent token in one function call: `client.get_token(agent_name, scope)`
2. The SDK handles the entire flow internally: app auth → launch token → challenge-response → Ed25519 → JWT
3. Token caching and automatic renewal happen transparently
4. The SDK is a pip-installable Python package (`pip install agentauth`)
5. Error messages are clear and actionable — developers should never need to read broker internals

---

## Non-Goals

1. **Go SDK** — Go developers can use the broker API directly (same language as the broker)
2. **JavaScript/TypeScript SDK** — future consideration based on demand
3. **CLI replacement** — the SDK is for programmatic use, not operator workflows
4. **Token Proxy replacement** — the proxy still has value for infrastructure-level caching and circuit breaking
5. **Offline token validation** — requires JWKS endpoint (Phase 4)

---

## User Stories

### Developer Stories

1. **As a Python developer**, I want to initialize a client with `broker_url`, `client_id`, and `client_secret` so that I can start using AgentAuth with 3 lines of setup.

2. **As a developer**, I want `client.get_token(agent_name="my-agent", scope=["read:data:*"])` to return a valid JWT so that I don't need to understand Ed25519, nonces, or challenge-response.

3. **As a developer**, I want the SDK to cache my agent's token and automatically renew it before expiry so that I don't need to implement token lifecycle management.

4. **As a developer**, I want the SDK to retry failed requests with exponential backoff so that transient broker issues don't crash my application.

5. **As a developer**, I want `client.validate_token(token)` to check if a token is still valid so that I can verify tokens received from other agents.

6. **As a developer**, I want `client.revoke_token(token)` to self-revoke a token so that my agent can clean up when it's done.

7. **As a developer**, I want clear error messages like "Scope 'write:data:*' exceeds your app's ceiling of ['read:data:*']" so that I can fix permission issues without reading broker logs.

8. **As a developer**, I want `client.delegate(to_agent, scope, ttl)` to create a delegation token so that my agent can grant limited permissions to other agents.

### Operator Stories

9. **As an operator**, I want the SDK to use the same broker API as every other client so that I can monitor, rate-limit, and audit SDK traffic the same way.

10. **As an operator**, I want the SDK to respect rate limits (429 responses) gracefully so that one developer's SDK usage doesn't impact others.

### Security Stories

11. **As a security reviewer**, I want the SDK to generate Ed25519 keypairs in memory (never persisted to disk) so that agent private keys are truly ephemeral.

12. **As a security reviewer**, I want the SDK to never log or expose the `client_secret` so that credentials don't leak through application logs.

---

## What Needs to Be Done

### 1. Package Structure

A Python package named `agentauth` with the following public interface:

```
agentauth/
├── client.py          # AgentAuthClient - main entry point
├── token.py           # Token caching and lifecycle
├── crypto.py          # Ed25519 key generation and signing
├── errors.py          # Custom exception hierarchy
└── __init__.py        # Exports AgentAuthClient
```

Installable via: `pip install agentauth`

### 2. Core Client

`AgentAuthClient` initialized with `broker_url`, `client_id`, `client_secret`. On initialization:

- Authenticates the app with the broker (`POST /v1/app/auth`)
- Stores the app JWT (handles renewal when it expires)

### 3. Token Acquisition Flow

`client.get_token(agent_name, scope)` handles the complete flow:

1. Create a launch token (using the app JWT)
2. Generate an Ed25519 keypair in memory
3. Request a challenge nonce from the broker
4. Sign the nonce with the private key
5. Register the agent with the signed nonce
6. Return the resulting JWT

All of this is invisible to the developer. They call one function and get a token.

### 4. Token Caching and Renewal

- Tokens are cached per agent_name + scope combination
- Before a cached token expires (configurable buffer, default 30 seconds), the SDK renews it automatically
- If renewal fails, the SDK re-registers the agent (full flow)
- Cache is in-memory only (no disk persistence of tokens)

### 5. Error Handling

Custom exception hierarchy:
- `AgentAuthError` — base
- `AuthenticationError` — client_id/secret wrong, app inactive
- `ScopeCeilingError` — requested scope exceeds ceiling
- `RateLimitError` — 429 from broker, includes retry-after
- `BrokerUnavailableError` — broker unreachable after retries
- `TokenExpiredError` — token expired and renewal failed

### 6. Retry and Backoff

- Configurable retry count (default 3)
- Exponential backoff: 1s → 2s → 4s
- Respects `Retry-After` header on 429 responses
- No retry on 4xx errors (except 429)

### 7. Delegation Support

`client.delegate(token, to_agent, scope, ttl)` wraps the `POST /v1/delegate` endpoint. Returns a delegation token with a narrowed scope for the target agent.

### 8. Documentation

- README with quickstart (3 steps: install, configure, get token)
- API reference for all public methods
- Error handling guide
- Examples: basic usage, delegation, multi-agent

---

## Success Criteria

- Developer can get a token in 3 lines: init client → get_token → use token
- Full flow works: app auth → launch token → Ed25519 challenge → JWT
- Token caching prevents unnecessary broker calls
- Automatic renewal keeps tokens fresh without developer intervention
- Ed25519 keys are ephemeral (in-memory only)
- `client_secret` never appears in logs or error messages
- All broker errors are translated to clear, actionable Python exceptions
- Rate limiting is respected with proper backoff
- Package installable via pip
- Works with Python 3.9+
