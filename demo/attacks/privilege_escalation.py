"""Attack 4: Privilege Escalation -- agent tries to expand its own scope.

Insecure mode: Shared API key = full access; no delegation system to abuse.
Secure mode:   Delegation denied (scope attenuation) + direct access denied (scope mismatch).
"""

from __future__ import annotations

import httpx

from attacks.models import AttackResult


async def escalation_attack(
    agent_credential: str,
    broker_url: str,
    resource_url: str,
    mode: str,
) -> AttackResult:
    """Attempt privilege escalation via delegation abuse and direct access.

    Step 1: Try to delegate a broader scope (write:Customers:*) to self
            via POST /v1/delegate.
    Step 2: Try direct access to GET /customers/12345 (outside Agent C's scope).

    Args:
        agent_credential: API key (insecure) or Bearer token (secure).
        broker_url: Base URL of the broker (for delegation).
        resource_url: Base URL of the resource server.
        mode: "secure" or "insecure".

    Returns:
        AttackResult with outcomes from escalation attempts.
    """
    result = AttackResult(name="privilege_escalation", mode=mode)

    if mode == "insecure":
        resource_headers = {"API-Key": agent_credential}
    else:
        resource_headers = {"Authorization": f"Bearer {agent_credential}"}

    async with httpx.AsyncClient() as client:
        # Step 1: Attempt scope escalation via delegation
        if mode == "secure":
            result.attempts += 1
            deleg_body = {
                "delegator_token": agent_credential,
                "target_agent_id": "spiffe://agentauth.local/agent/orch1/task1/rogue",
                "delegated_scope": ["write:Customers:*"],
                "max_ttl": 300,
            }
            resp = await client.post(
                f"{broker_url}/v1/delegate",
                json=deleg_body,
                headers={"Authorization": f"Bearer {agent_credential}"},
            )
            if resp.status_code in (200, 201):
                result.successes += 1
                result.details.append(
                    f"DELEGATE write:Customers:*: ESCALATION GRANTED (status {resp.status_code})"
                )
            else:
                result.blocked += 1
                result.details.append(
                    f"DELEGATE write:Customers:*: DENIED (status {resp.status_code})"
                )
        else:
            # In insecure mode there is no delegation system.
            result.details.append(
                "DELEGATE: SKIPPED (no delegation system in insecure mode)"
            )

        # Step 2: Direct access to resource outside scope
        result.attempts += 1
        resp = await client.get(
            f"{resource_url}/customers/12345",
            headers=resource_headers,
        )
        if resp.status_code == 200:
            result.successes += 1
            result.details.append(
                f"GET /customers/12345: ACCESS GRANTED (status {resp.status_code})"
            )
        else:
            result.blocked += 1
            result.details.append(
                f"GET /customers/12345: BLOCKED (status {resp.status_code})"
            )

    return result
