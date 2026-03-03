# Phase 2: Activation Token Bootstrap (No More Master Key in Proxies)

**Status:** Spec
**Priority:** P0 — stops master key spread
**Effort estimate:** 1 day
**Depends on:** Phase 1a (app registration with client credentials)
**Architecture doc:** `../.plans/CoWork-Architecture-Direct-Broker.md`

---

## Overview: What We're Building and Why

The root cause of AgentAuth's security problem is the admin master key spreading to every deployment. Phases 1a-1c gave apps their own credentials and a complete lifecycle — but the Token Proxy (sidecar) still bootstraps with the master key (`AA_ADMIN_SECRET`). Every sidecar deployment is still a copy of the most powerful credential in the system.

**Phase 2 is the master key containment phase.** It replaces the sidecar's master key bootstrap with a single-use activation token tied to a specific app. The operator registers an app (Phase 1a), generates an activation token for it, and gives the developer that token instead of the master key. The sidecar consumes the token on first startup and gets scoped credentials — the master key never leaves the operator's environment.

Think of the analogy: instead of giving every delivery person a copy of the building master key, you give each one a single-use entry code that works once, grants access to one floor, and expires after an hour.

After this phase, the master key exists in exactly 2 places: the broker's config and the operator's vault. The core security promise of the initiative is delivered.

**What changes:** Token Proxy accepts `AA_ACTIVATION_TOKEN` environment variable, activation tokens are tied to specific apps, sidecar records carry `app_id`, deprecation warning on `AA_ADMIN_SECRET` usage.

**What stays the same:** The sidecar continues to work. Existing deployments using `AA_ADMIN_SECRET` are backward compatible (deprecated, not removed). Agent registration through the sidecar is unchanged.

---

## Problem Statement

Today, every Token Proxy (sidecar) deployment requires the admin master key (`AA_ADMIN_SECRET`) in its configuration. This means the master key is copied to every environment where a proxy runs — dev, staging, production, each app's deployment. If any proxy is compromised, the attacker has the master key to the entire system.

Phase 1a gave apps their own credentials, but the proxy still uses the master key for its initial bootstrap. Phase 2 replaces this: the proxy bootstraps with a single-use activation token tied to an app, not the master key.

---

## Goals

1. Token Proxy can bootstrap using an activation token instead of the admin master key
2. Activation tokens are single-use, scoped to an app, and time-limited
3. The master key only exists in two places: the broker's config and the operator's vault
4. Existing `AA_ADMIN_SECRET`-based proxy bootstrap still works (deprecated, with warning)

---

## Non-Goals

1. **Removing `AA_ADMIN_SECRET` support entirely** — it stays for backward compatibility, just deprecated
2. **Automatic proxy re-bootstrap** — if the proxy loses its credentials, the operator generates a new activation token
3. **Multi-proxy per app** — each proxy deployment gets its own activation token
4. **Proxy-to-proxy communication** — proxies don't talk to each other

---

## User Stories

### Operator Stories

1. **As an operator**, I want to generate an activation token for a proxy deployment so that I never have to put the master key in a proxy's config file.

2. **As an operator**, I want the activation token to be single-use so that if it's intercepted, it can't be reused after the proxy has consumed it.

3. **As an operator**, I want the activation token to be tied to a specific app so that the proxy only gets credentials scoped to that app's ceiling.

4. **As an operator**, I want `aactl app activate --id APP_ID` to generate an activation token I can give to the developer for their proxy deployment.

5. **As an operator**, I want to see a deprecation warning when a proxy bootstraps with `AA_ADMIN_SECRET` so that I know which deployments still use the old method.

### Developer Stories

6. **As a developer**, I want to start my Token Proxy with `AA_ACTIVATION_TOKEN=...` instead of `AA_ADMIN_SECRET=...` so that I don't need to handle the master key.

7. **As a developer**, I want the proxy to behave identically regardless of how it bootstrapped so that my agent code doesn't need to know or care about the bootstrap method.

### Security Stories

8. **As a security reviewer**, I want to verify that the master key is not present in any proxy deployment so that a proxy compromise doesn't escalate to full system compromise.

9. **As a security reviewer**, I want activation tokens to have a short TTL (default 1 hour) so that unused tokens expire quickly.

10. **As a security reviewer**, I want every proxy bootstrap (activation or legacy) to generate an audit event so that I can track how each proxy was provisioned.

---

## What Needs to Be Done

### 1. App-Scoped Activation Token Generation

When an operator registers an app (Phase 1a), they can also generate an activation token for a proxy deployment. The activation token is a single-use JWT that carries:

- The app's `app_id` and scope ceiling
- A short TTL (default 1 hour, configurable)
- An audience claim identifying it as an activation token
- A JTI for single-use tracking

This builds on the existing sidecar activation flow (`POST /v1/admin/sidecar-activations`) but ties the activation to a specific app instead of being generic.

### 2. Token Proxy Accepts `AA_ACTIVATION_TOKEN`

The Token Proxy (sidecar) binary needs a new environment variable: `AA_ACTIVATION_TOKEN`. On startup:

- If `AA_ACTIVATION_TOKEN` is set, the proxy calls `POST /v1/sidecar/activate` with the activation token
- The broker validates the token (signature, expiry, single-use, app linkage)
- The broker returns scoped credentials tied to the app
- The proxy stores these credentials and operates normally

If `AA_ADMIN_SECRET` is set instead, the proxy uses the existing bootstrap flow but logs a deprecation warning: "Using AA_ADMIN_SECRET is deprecated. Use AA_ACTIVATION_TOKEN instead."

If both are set, `AA_ACTIVATION_TOKEN` takes precedence.

### 3. Link Sidecar to App

When a proxy activates with an app-scoped activation token, the resulting sidecar record should carry the `app_id`. This means:

- Audit events from the proxy are attributable to the app
- The operator can see which proxies belong to which apps
- Revoking an app (Phase 1c) also affects its proxies

### 4. aactl Command

`aactl app activate --id APP_ID [--ttl 1h]` — generates an activation token for a proxy deployment. Displays the token and reminds the operator it's single-use.

### 5. Deprecation Warning in Proxy Logs

When the proxy starts with `AA_ADMIN_SECRET`, emit a clear warning:
```
WARNING: Bootstrapping with AA_ADMIN_SECRET is deprecated and will be removed in a future version.
Use AA_ACTIVATION_TOKEN instead. See docs/operations.md for migration steps.
```

---

## Success Criteria

- Proxy starts successfully with `AA_ACTIVATION_TOKEN` (no master key needed)
- Activation token is consumed on first use — second attempt fails
- Activation token expires after TTL — late attempt fails
- Proxy bootstrapped via activation token operates identically to master-key-bootstrapped proxy
- Sidecar record carries `app_id` from activation token
- Proxy started with `AA_ADMIN_SECRET` logs deprecation warning but still works
- Audit trail shows which bootstrap method was used for each proxy
- `aactl app activate` generates and displays the activation token
