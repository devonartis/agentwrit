# Real-World Examples

Multi-agent workflow examples showing AgentAuth in action. Each example is structured with clear role separation:

- **Operator setup** (brief) -- one-time sidecar deployment with scope ceiling configuration. Your operator does this before developers write code.
- **Developer agent code** (main content) -- pure Python agent code that talks to the sidecar. This is what developers write.
- **Dangerous path** -- the same workflow without AgentAuth, showing real consequences.
- **Security comparison table**

## Examples

| Example | Agents | Key Risk | Key AgentAuth Feature |
|---------|--------|----------|----------------------|
| [Data Pipeline](data-pipeline.md) | Research + Writer + Review | Unauthorized data access | Scope attenuation, delegation |
| [Code Generation](code-generation.md) | Planning + Coder + Test | Supply chain attacks | Branch-scoped write access |
| [Customer Support](customer-support.md) | Triage + Knowledge + Response | PII breach, GDPR/SOC 2 | PII-scoped access, compliance audit |
| [DevOps Automation](devops-automation.md) | Monitor + Remediation + Notification | Infrastructure destruction | Surgical write scopes, no-delete enforcement |

## How to Read These

Each example has two roles and two deployment models:

### Deployment Models

**Default (sidecar-managed) -- 95% of use cases.** The operator deploys a sidecar with a scope ceiling -- the maximum permissions any agent can request through this sidecar. Developers call `POST /v1/token` on the sidecar and get back a scoped JWT. The sidecar handles admin auth, launch token creation, Ed25519 key generation, and challenge-response registration -- all transparently behind that single call. This is like AWS IAM: the operator creates an IAM role with a permission boundary (the sidecar scope ceiling), and the developer assumes the role to get temporary credentials (the scoped JWT).

**Advanced (operator-managed) -- for per-agent scope ceiling isolation.** When different agents need isolated scope ceilings (e.g., one group must never be able to request PII scopes), the operator deploys multiple sidecars with different ceilings. In rare cases where the operator needs per-agent launch tokens with individual scope ceilings distinct from any sidecar ceiling, the operator creates launch tokens manually via the broker's admin API.

### As the Operator

The operator section is brief. The operator:

1. Deploys the broker and sidecar as centralized services with `AA_ADMIN_SECRET` and `AA_SIDECAR_SCOPE_CEILING` configured
2. Gives developers the sidecar URL (`AGENTAUTH_SIDECAR_URL`) and tells them their allowed scopes (the scope ceiling)
3. Handles revocation and audit queries when needed (these operations talk to the broker via `AGENTAUTH_BROKER_URL`)

The scope ceiling is the union of all scopes any agent might need through this sidecar. Each agent requests only what it needs -- the sidecar enforces that requests stay within the ceiling.

### As the Developer

The developer section is the main content. It assumes you received a sidecar URL and your allowed scopes from your operator. The developer:

1. Calls `POST /v1/token` on the sidecar with `agent_name`, `task_id`, and `scope`
2. Gets back a scoped JWT
3. Uses that token to access resources
4. Handles renewal, delegation, and error cases

Developers use `AGENTAUTH_SIDECAR_URL` to talk to the sidecar. Developers never deal with launch tokens, admin secrets, or broker URLs.

## Environment Variables

All example code reads service URLs from environment variables:

```bash
# Operator: broker URL for admin operations
export AGENTAUTH_BROKER_URL="https://agentauth.internal.company.com"

# Developer: sidecar URL for agent operations
export AGENTAUTH_SIDECAR_URL="https://sidecar.internal.company.com"

# Operator only: admin secret (never shared with developers)
export AA_ADMIN_SECRET=your-secret-here
```

## Local Development

For local development and testing, you can run the full stack with Docker Compose:

```bash
export AA_ADMIN_SECRET=your-secret-here
./scripts/stack_up.sh

# Override URLs for local development
export AGENTAUTH_BROKER_URL="http://localhost:8080"
export AGENTAUTH_SIDECAR_URL="http://localhost:8081"

# Install Python dependencies
pip install requests cryptography
```

This is for local testing only. In production, the broker and sidecar are deployed as centralized services behind your organization's internal DNS or service mesh.
