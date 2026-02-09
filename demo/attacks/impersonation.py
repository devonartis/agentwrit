"""Attack 3: Agent Impersonation -- rogue agent pretends to be legitimate.

Insecure mode: Same shared API key works for anyone; rogue is indistinguishable.
Secure mode:   Fake/forged token is rejected by broker validation (401).
"""

from __future__ import annotations

import secrets

import httpx

from attacks.models import AttackResult

# Resources the impersonator tries to access.
IMPERSONATION_TARGETS = [
    ("GET", "/customers/12345"),
    ("GET", "/orders/12345"),
]


async def impersonation_attack(
    resource_url: str,
    mode: str,
    shared_api_key: str = "shared-api-key",
) -> AttackResult:
    """Attempt to access resources using a forged identity.

    In insecure mode, the rogue uses the same shared API key as legitimate
    agents -- demonstrating that anyone with the key is accepted.

    In secure mode, the rogue fabricates a Bearer token (random hex) which
    the resource server rejects when it validates against the broker.

    Args:
        resource_url: Base URL of the resource server (no trailing slash).
        mode: "secure" or "insecure".
        shared_api_key: API key used in insecure mode (same one legit agents use).

    Returns:
        AttackResult with per-endpoint attempt outcomes.
    """
    result = AttackResult(name="impersonation", mode=mode)

    if mode == "insecure":
        headers = {"API-Key": shared_api_key}
    else:
        # Rogue forges a fake Bearer token
        fake_token = secrets.token_hex(32)
        headers = {"Authorization": f"Bearer {fake_token}"}

    async with httpx.AsyncClient() as client:
        for method, path in IMPERSONATION_TARGETS:
            result.attempts += 1
            url = f"{resource_url}{path}"
            resp = await client.request(method, url, headers=headers)

            if resp.status_code == 200:
                result.successes += 1
                result.details.append(
                    f"{method} {path}: IMPERSONATION ACCEPTED (status {resp.status_code})"
                )
            else:
                result.blocked += 1
                result.details.append(
                    f"{method} {path}: REJECTED (status {resp.status_code})"
                )

    return result
