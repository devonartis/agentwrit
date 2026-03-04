# Phase 1b: App-Scoped Launch Tokens

**Status:** Spec
**Priority:** P0 — enables apps to create agents within their ceiling
**Effort estimate:** 1 day
**Depends on:** Phase 1a (app registration, app auth, app JWT)
**Architecture doc:** `../.plans/CoWork-Architecture-Direct-Broker.md`

---

## Overview: What We're Building and Why

AgentAuth is transforming from a sidecar-dependent system (where the admin master key is copied into every deployment) into an app-centric architecture where each application has its own identity and scoped credentials. Phase 1a (the previous phase) created the `AppRecord` data model and let apps authenticate with the broker using `client_id` + `client_secret`.

**But authentication alone isn't enough.** In AgentAuth, an agent needs a *launch token* to register — launch tokens are single-use credentials that authorize an agent to go through the Ed25519 challenge-response flow and receive a JWT. Today, only the admin can create launch tokens. After Phase 1a, apps can authenticate... but they still need to ask the operator for every launch token. That defeats the purpose of giving apps their own credentials.

**Phase 1b closes this gap.** It lets apps create their own launch tokens within their scope ceiling. The scope ceiling is the hard permission boundary set during app registration — an app with `read:weather:*` can create launch tokens for `read:weather:current` but never for `write:weather:anything`. This is the same scope attenuation principle that already governs agent tokens and delegation chains.

**This phase also establishes the traceability chain: App → Launch Token → Agent.** When an app creates a launch token, the token carries the app's `app_id`. When an agent registers using that token, the agent record inherits the `app_id`. This means every agent in the system can be traced back to the app that created it — critical for the app-level revocation coming in Phase 1c.

**What changes:** The launch token endpoint accepts app JWTs (not just admin JWTs), scope ceiling enforcement on app-created launch tokens, `app_id` field added to launch token records and agent records.

**What stays the same:** The Ed25519 challenge-response agent registration flow is completely untouched. Launch tokens work the same way — they're just created by apps now instead of only by admins. Admin-created launch tokens continue to work exactly as before.

---

## Problem Statement

After Phase 1a, apps can authenticate with the broker and receive a scoped JWT — but that JWT can't do anything yet. The only way to create launch tokens (which agents need to register) is through admin credentials. This means developers still need the operator to create every launch token, defeating the purpose of giving apps their own credentials.

Phase 1b closes this gap: apps create their own launch tokens, scoped within their ceiling, without involving the operator.

---

## Goals

1. Apps can create launch tokens within their own scope ceiling — no operator involvement needed
2. The scope ceiling is enforced: apps cannot create tokens with broader permissions than they were granted
3. Agents registered through app-created launch tokens are traceable back to the originating app
4. The admin launch token flow continues to work unchanged (backward compatible)

---

## Non-Goals

1. **App-level revocation** — revoking all tokens for an app (Phase 1c)
2. **`app_id` in agent JWT claims** — adding app attribution to issued JWTs (Phase 1c)
3. **Separate API route for apps** — apps use the existing launch token endpoint, just with different auth
4. **SDK wrapping** — the Python SDK simplifies this in Phase 3
5. **Agent management by apps** — apps can create launch tokens but don't manage agents directly yet

---

## User Stories

### Developer Stories

1. **As a developer with app credentials**, I want to create a launch token for my agent so that I can register the agent with the broker without asking the operator for a token.

2. **As a developer**, I want the broker to reject my launch token request if I ask for scopes outside my app's ceiling so that I know immediately what my app is allowed to do.

3. **As a developer**, I want agents I register to be linked to my app so that audit trails and management operations know which agents belong to which app.

### Operator Stories

4. **As an operator**, I want to see which app created a launch token so that I can trace agent registrations back to the responsible app.

5. **As an operator**, I want the broker to enforce each app's scope ceiling on launch token creation so that developers can't escalate their own permissions.

6. **As an operator**, I want admin-created launch tokens to continue working exactly as before so that existing workflows aren't broken.

### Security Stories

7. **As a security reviewer**, I want scope attenuation enforced at the launch token level so that an app with `read:weather:*` cannot create a launch token granting `write:weather:*`.

8. **As a security reviewer**, I want every agent record to carry the `app_id` of the app that created it so that compromise investigations can identify all agents belonging to a compromised app.

---

## What Needs to Be Done

### 1. Extend the Launch Token Endpoint to Accept App JWTs

The existing launch token creation endpoint currently only accepts admin-authenticated requests. It needs to also accept requests from app-authenticated users (carrying `app:launch-tokens:*` scope). Both admin and app callers should use the same endpoint — the difference is how the scope ceiling is enforced.

### 2. Enforce App Scope Ceiling on Launch Token Creation

When an app creates a launch token, the requested scopes must be a subset of the app's registered scope ceiling. If the app asks for scopes outside its ceiling, the request fails with a clear error explaining what's allowed. Admin callers skip this check (admins have no ceiling).

### 3. Link Launch Tokens to Apps

Launch token records need to carry the `app_id` of the app that created them. Admin-created tokens have no `app_id` (backward compatible). This field is the bridge between "which app" and "which agent" — because agents register using launch tokens.

### 4. Flow App Identity Through Agent Registration

When an agent registers using an app-created launch token, the agent record should inherit the `app_id` from the launch token. This creates a traceable chain: App → Launch Token → Agent. Existing agents (registered before this change) have no `app_id`.

### 5. Audit Trail Attribution

Audit events for launch token creation and agent registration should include the `app_id` when the action was triggered by an app-authenticated user. This makes "what did weather-bot do?" queries return the full chain from app auth through agent registration.

---

## Success Criteria

- An app can authenticate → create a launch token → use it to register an agent (complete flow)
- Launch token scopes cannot exceed the app's ceiling (403 on violation)
- Agent records are traceable to the originating app
- Admin can still create launch tokens without any app — no regressions
- Existing agents and tokens without `app_id` continue to function normally
- All operations appear in the audit trail with app attribution where applicable

---

## Testing Workflow

> **Before writing any test code**, extract the user stories from the `## User Stories` section above into a standalone file:
> `tests/phase-1b-user-stories.md`
>
> This is required by the project workflow (CLAUDE.md). The coding agent writes user stories first, saves them to `tests/`, then writes test code against them. Do not skip this step.
