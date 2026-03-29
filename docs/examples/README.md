# Real-World Examples

Multi-agent workflow examples showing AgentAuth in action. Each example demonstrates how to manage ephemeral agent credentials in production systems:

- **Operator setup** (brief) -- one-time broker deployment with app and launch token configuration. Your operator does this before developers write code.
- **Developer agent code** (main content) -- pure Python agent code that authenticates directly with the broker. This is what developers write.
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

Each example shows the operator setup and developer agent code for a production scenario.

### As the Operator

The operator section shows one-time infrastructure setup:

1. Deploy the broker as a centralized service with `AA_ADMIN_SECRET` configured
2. Create an app registration via `aactl app register` (or admin API)
3. Create launch tokens for each agent workflow via `aactl` or the admin API
4. Distribute the broker URL and launch tokens to developers (or the orchestrator that creates agents)
5. Handle revocation and audit queries when needed

Apps and launch tokens are centralized -- operators manage them once and developers receive credentials for use.

### As the Developer

The developer section shows agent code that:

1. Gets a launch token from the operator (or orchestrator)
2. Performs challenge-response registration with the broker to get a JWT
3. Uses that JWT (Bearer token) to access resources
4. Handles renewal, delegation, and error cases

Developers call the broker directly (`AGENTAUTH_BROKER_URL`) via standard HTTP and JWT.

## Environment Variables

All example code reads the broker URL from environment variables:

```bash
# Broker URL (required for all agents)
export AGENTAUTH_BROKER_URL="https://agentauth.internal.company.com"

# Operator only: admin secret (never shared with developers)
export AA_ADMIN_SECRET=your-secret-here
```

## Local Development

For local development and testing, you can run the broker with Docker Compose:

```bash
export AA_ADMIN_SECRET=your-secret-here
./scripts/stack_up.sh

# Override URL for local development
export AGENTAUTH_BROKER_URL="http://localhost:8080"

# Install Python dependencies
uv pip install requests cryptography
```

This is for local testing only. In production, the broker is deployed as a centralized service behind your organization's internal DNS or service mesh.
