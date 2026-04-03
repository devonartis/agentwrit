# aactl CLI Reference

Complete reference for `aactl`, the operator command-line tool for managing the AgentAuth broker.

> **Audience:** Operators and administrators
>
> **Prerequisites:** [[Operator Guide]] for setup

---

## Setup

### Install

```bash
cd /path/to/agentauth-core
go build -o aactl ./cmd/aactl
```

### Configure

```bash
export AACTL_BROKER_URL="http://localhost:8080"
export AACTL_ADMIN_SECRET="your-admin-secret"
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output raw JSON instead of formatted tables |

---

## Commands

### audit events

Query the audit trail.

```bash
aactl audit events [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--agent-id` | string | Filter by agent SPIFFE ID |
| `--task-id` | string | Filter by task ID |
| `--event-type` | string | Filter by type: `token_issued`, `token_revoked`, `scope_narrowed`, `agent_denied` |
| `--since` | string | Events after this time (RFC3339) |
| `--until` | string | Events before this time (RFC3339) |
| `--outcome` | string | `success` or `denied` |
| `--limit` | int | Max results (default 100) |
| `--offset` | int | Pagination offset |

**Examples:**

```bash
# All recent events
aactl audit events

# Failed authorization attempts
aactl audit events --outcome denied

# Events for a specific agent
aactl audit events --agent-id "spiffe://agentauth.local/agent/orch/task/proc"

# Revocations in the last hour
aactl audit events --event-type token_revoked --since 2026-02-27T14:00:00Z

# Paginate
aactl audit events --limit 100 --offset 0
aactl audit events --limit 100 --offset 100

# JSON for scripting
aactl audit events --json | jq '.events | length'
```

---

### token release

Release (self-revoke) an agent token.

```bash
aactl token release --token <jwt>
```

| Flag | Type | Description |
|------|------|-------------|
| `--token` | string | The agent JWT to release (required) |

**Example:**
```bash
aactl token release --token "$AGENT_TOKEN"
```

> **Note:** Uses the token itself for authentication (not the admin secret). Idempotent — releasing an already-released token succeeds.

---

### revoke

Revoke tokens at various levels.

```bash
aactl revoke --level <level> --target <target>
```

| Flag | Type | Description |
|------|------|-------------|
| `--level` | string | `token`, `agent`, `task`, or `chain` (required) |
| `--target` | string | JTI, SPIFFE ID, or task ID (required) |

**Examples:**

```bash
# Revoke one token by JTI
aactl revoke --level token --target "jti-abc123"

# Revoke all tokens for an agent
aactl revoke --level agent --target "spiffe://agentauth.local/agent/orch/task/proc"

# Revoke all tokens for a task
aactl revoke --level task --target "task-456"

# Revoke entire delegation chain
aactl revoke --level chain --target "spiffe://agentauth.local/agent/root"

# Check result in JSON
aactl revoke --level agent --target "spiffe://..." --json
```

**Output:**
```
REVOKED  LEVEL  TARGET                                    COUNT
true     agent  spiffe://agentauth.local/agent/orch/...   3
```

---

### sidecars list

List all registered sidecars.

```bash
aactl sidecars list
```

**Output:**
```
ID               SCOPES                    STATUS  CREATED
sidecar-prod-1   admin:read, admin:write   active  2026-02-20T08:15:00Z
sidecar-stage-1  read                      active  2026-02-15T10:00:00Z
```

```bash
# JSON for scripting
aactl sidecars list --json | jq '.sidecars | length'

# Find sidecars with admin scopes
aactl sidecars list --json | jq '.sidecars[] | select(.scope_ceiling | contains(["admin:read"]))'
```

---

### sidecars ceiling get

Get a sidecar's scope ceiling.

```bash
aactl sidecars ceiling get <sidecar-id>
```

**Example:**
```bash
aactl sidecars ceiling get sidecar-prod-1
# Output:
# SIDECAR ID       SCOPE CEILING
# sidecar-prod-1   admin:read admin:write read

aactl sidecars ceiling get sidecar-prod-1 --json | jq '.scope_ceiling'
```

---

### sidecars ceiling set

Update a sidecar's scope ceiling. **Narrowing revokes excess tokens automatically.**

```bash
aactl sidecars ceiling set <sidecar-id> --scopes <scope1>,<scope2>,...
```

| Flag | Type | Description |
|------|------|-------------|
| `--scopes` | string | Comma-separated scope ceiling (required) |

**Examples:**

```bash
# Widen ceiling
aactl sidecars ceiling set sidecar-prod-1 --scopes admin:read,admin:write,read,write

# Narrow ceiling (will revoke excess tokens!)
aactl sidecars ceiling set sidecar-prod-1 --scopes read
```

**Output:**
```
OLD CEILING           NEW CEILING  NARROWED  REVOKED  REVOKED COUNT
admin:read,admin:write,read  read        true      true     5
```

---

## Common Workflows

### Incident Response

```bash
# 1. Check what the agent did
aactl audit events --agent-id "spiffe://..." --since 2026-02-27T10:00:00Z

# 2. Revoke the agent
aactl revoke --level agent --target "spiffe://..."

# 3. Revoke delegation chain
aactl revoke --level chain --target "spiffe://..."

# 4. Verify revocation
aactl audit events --event-type token_revoked --since 2026-02-27T10:00:00Z
```

### Compliance Audit

```bash
# Export all events for a time period
aactl audit events --since 2026-02-01T00:00:00Z --until 2026-03-01T00:00:00Z --json > feb-audit.json

# Count events by type
cat feb-audit.json | jq '.events | group_by(.event_type) | map({type: .[0].event_type, count: length})'
```

### Sidecar Rotation

```bash
# List current sidecars
aactl sidecars list

# Narrow old sidecar (revokes tokens)
aactl sidecars ceiling set old-sidecar --scopes ""

# Verify new sidecar is healthy
curl https://new-sidecar:8081/v1/health
```

---

## Next Steps

- [[Operator Guide]] — Full operator guide
- [[Sidecar Deployment]] — Sidecar deployment patterns
- [[Troubleshooting]] — Fix common errors
