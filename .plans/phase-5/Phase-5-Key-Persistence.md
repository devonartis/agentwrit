# Phase 5: Key Persistence

**Status:** Spec
**Priority:** P1 — broker restart resilience
**Effort estimate:** 1-2 days
**Depends on:** None (independent), but completes Phase 4 (JWKS)
**Architecture doc:** `../.plans/CoWork-Architecture-Direct-Broker.md`

---

## Overview: What We're Building and Why

AgentAuth generates a new Ed25519 signing keypair every time the broker starts. This means every broker restart is a system-wide reset: all existing tokens become invalid (the new key can't verify signatures from the old key), all agents must re-register, and any cached JWKS keys (Phase 4) are wrong. A routine broker update, a container restart, or a crash causes the equivalent of "everyone log in again."

**Phase 5 persists the signing key across restarts.** On first startup, the broker generates an Ed25519 keypair and saves it to a configured file path. On subsequent startups, it loads the existing key. Tokens issued before a restart remain valid after restart. Agents don't need to re-register. The JWKS endpoint (Phase 4) returns the same key before and after restart.

Think of it this way: today, the broker is like a bank that changes all its locks every time the power flickers — everyone needs new keys. After Phase 5, the locks persist through power cycles, and key rotation is an explicit operator decision, not an accident.

**What's built:** Key file persistence (PEM-encoded Ed25519 private key), configurable key path (`AA_KEY_PATH`), optional encryption at rest (`AA_KEY_ENCRYPT`), automatic key generation on first start, key loading on subsequent starts, explicit key rotation command (`aactl key rotate`), audit events for key lifecycle.

**What stays the same:** Token format unchanged, JWT signing logic unchanged, all endpoints unchanged. The only difference is WHERE the signing key comes from (file instead of freshly generated).

---

## Problem Statement

Today, the broker generates a new Ed25519 signing keypair every time it starts. This means:

- Every token issued before a restart becomes invalid (signature verification fails with new key)
- Every agent must re-register after a broker restart
- The JWKS endpoint (Phase 4) becomes unreliable because cached keys are invalidated on restart
- Any broker restart is effectively a system-wide outage for all active agents

For development and testing this is tolerable. For production, it's not.

---

## Goals

1. The broker's Ed25519 signing key persists across restarts
2. Tokens issued before a restart remain valid after restart
3. Agents don't need to re-register after a broker restart
4. The JWKS endpoint returns the same key before and after restart
5. The persisted key is protected (encrypted at rest or file-permission restricted)

---

## Non-Goals

1. **Key rotation** — scheduled key rotation with dual-key support (future enhancement)
2. **HSM integration** — hardware security module for key storage (enterprise feature)
3. **Key backup/recovery** — disaster recovery for lost keys (separate concern)
4. **Multi-broker key sharing** — distributed deployment with shared keys (future)
5. **Backward compatibility with ephemeral keys** — once persisted, the broker always uses the persisted key

---

## User Stories

### Operator Stories

1. **As an operator**, I want the broker to survive a restart without invalidating all active tokens so that routine maintenance doesn't cause a system-wide outage.

2. **As an operator**, I want to configure where the signing key is stored (file path or database) so that I can choose the persistence mechanism appropriate for my environment.

3. **As an operator**, I want the stored key to be encrypted or file-permission-protected so that an attacker with filesystem access can't extract the signing key.

4. **As an operator**, I want the broker to generate a new key on first startup (no key file exists) and load the existing key on subsequent startups so that initial setup is automatic.

5. **As an operator**, I want the option to force key regeneration (`--rotate-key` flag) so that I can manually rotate the signing key when needed.

### Developer Stories

6. **As a developer**, I want my cached tokens to remain valid after a broker restart so that my agents don't need to re-authenticate and re-register whenever the broker is updated.

7. **As a developer using the SDK**, I want the SDK's automatic renewal to work seamlessly across broker restarts so that my application never notices a restart happened.

### Security Stories

8. **As a security reviewer**, I want the signing key stored with restrictive file permissions (0600) so that only the broker process can read it.

9. **As a security reviewer**, I want the option to encrypt the key file with a passphrase so that the key is protected even if the file is copied.

10. **As a security reviewer**, I want a clear audit event when the broker loads a persisted key vs generates a new one so that key lifecycle is traceable.

---

## What Needs to Be Done

### 1. Key Persistence Mechanism

On first startup (no key file exists):
- Generate Ed25519 keypair (same as today)
- Save the private key to a configured file path
- Log audit event: "signing key generated and persisted"

On subsequent startups (key file exists):
- Load the private key from the file
- Derive the public key from it
- Log audit event: "signing key loaded from file"
- Verify the key is valid (sign + verify a test message)

### 2. Configuration

New environment variables:
- `AA_KEY_PATH` — file path for the signing key (default: `./agentauth.key`)
- `AA_KEY_ENCRYPT` — whether to encrypt the key file (default: `false`)
- `AA_KEY_PASSPHRASE` — passphrase for encryption (required if `AA_KEY_ENCRYPT=true`)

### 3. Key File Format

The key file should use a standard format:
- PEM-encoded Ed25519 private key (PKCS#8)
- If encrypted: PEM with ENCRYPTED PRIVATE KEY header
- File permissions set to 0600 on creation

### 4. Force Key Rotation

A command or flag to force key regeneration:
- `aactl key rotate` or broker flag `--rotate-key`
- Generates a new keypair
- Overwrites the key file
- All tokens signed with the old key become invalid (this is intentional — operator chose to rotate)
- Audit event: "signing key rotated"

### 5. Graceful Key Rotation (Future-Ready)

While full dual-key rotation is out of scope, the architecture should be future-ready:
- The JWKS endpoint (Phase 4) already returns a `keys` array (plural)
- The `kid` claim in JWTs identifies which key signed the token
- A future Phase could add: generate new key → sign new tokens with new key → verify with both keys for a grace period → remove old key

---

## Success Criteria

- Broker restart: tokens issued before restart are valid after restart
- Broker restart: agents don't need to re-register
- Broker restart: JWKS endpoint returns the same key
- First startup: key generated and saved automatically
- Key file has restrictive permissions (0600)
- `AA_KEY_PATH` configures key location
- Force rotation: `--rotate-key` regenerates the key (with clear warning)
- Audit events for key generation, loading, and rotation
- Existing tests still pass (key behavior is transparent to the rest of the system)

---

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Key file stolen → attacker can forge tokens | High | File permissions (0600), optional encryption, monitor for unauthorized access |
| Key file corrupted → broker can't start | Medium | Broker detects invalid key and offers to regenerate (with warning about invalidating tokens) |
| Encryption passphrase lost → can't load key | Medium | Document passphrase management in operations guide |
| Key rotation without warning → all tokens invalidated | Low | Rotation requires explicit flag, logs clear warning, generates audit event |

---

## Testing Workflow

> **Before writing any test code**, extract the user stories from the `## User Stories` section above into a standalone file:
> `tests/phase-5-user-stories.md`
>
> This is required by the project workflow (CLAUDE.md). The coding agent writes user stories first, saves them to `tests/`, then writes test code against them. Do not skip this step.
