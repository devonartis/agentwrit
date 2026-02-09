"""Attack 1: Credential Theft -- stolen credential reuse after agent terminates.

Insecure mode: Shared API key never expires, grants access to ALL customers.
Secure mode:   Scoped token only allows the original customer; others get 403.
"""

from __future__ import annotations

import httpx

from attacks.models import AttackResult

CUSTOMER_IDS = [12345, 12346, 12347, 12348, 12349]


async def credential_theft_attack(
    stolen_credential: str,
    resource_url: str,
    mode: str,
) -> AttackResult:
    """Attempt to access all 5 customer records using a stolen credential.

    Args:
        stolen_credential: The API key (insecure) or Bearer token (secure)
            that the attacker obtained after the legitimate agent terminated.
        resource_url: Base URL of the resource server (no trailing slash).
        mode: "secure" or "insecure".

    Returns:
        AttackResult with per-customer attempt outcomes.
    """
    result = AttackResult(name="credential_theft", mode=mode)

    if mode == "insecure":
        headers = {"API-Key": stolen_credential}
    else:
        headers = {"Authorization": f"Bearer {stolen_credential}"}

    async with httpx.AsyncClient() as client:
        for cid in CUSTOMER_IDS:
            result.attempts += 1
            url = f"{resource_url}/customers/{cid}"
            resp = await client.get(url, headers=headers)

            if resp.status_code == 200:
                result.successes += 1
                result.details.append(
                    f"Customer {cid}: ACCESS GRANTED (status {resp.status_code})"
                )
            else:
                result.blocked += 1
                result.details.append(
                    f"Customer {cid}: BLOCKED (status {resp.status_code})"
                )

    return result
