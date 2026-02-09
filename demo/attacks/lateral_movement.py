"""Attack 2: Lateral Movement -- agent tries to access resources outside its scope.

Insecure mode: Shared API key has no scope restrictions, all endpoints succeed.
Secure mode:   Token scoped to read:Customers:12345 cannot access orders/tickets/notifications.
"""

from __future__ import annotations

import httpx

from attacks.models import AttackResult


def _sanitize_error(exc: Exception) -> str:
    """Return a safe error string that never leaks URLs or tokens."""
    if isinstance(exc, httpx.HTTPStatusError):
        return f"HTTP {exc.response.status_code} from {exc.request.url.path}"
    return type(exc).__name__

# Endpoints that Agent A (read:Customers:12345) should NOT be able to reach.
LATERAL_TARGETS = [
    ("GET", "/orders/12345", None),
    ("PUT", "/tickets/789", {"status": "closed", "assignee": "rogue"}),
    ("POST", "/notifications/send", {"customer_id": 12345, "channel": "email", "message": "pwned"}),
]


async def lateral_movement_attack(
    agent_credential: str,
    resource_url: str,
    mode: str,
) -> AttackResult:
    """Attempt lateral movement from read:Customers to orders/tickets/notifications.

    Args:
        agent_credential: The API key (insecure) or Bearer token (secure).
        resource_url: Base URL of the resource server (no trailing slash).
        mode: "secure" or "insecure".

    Returns:
        AttackResult with per-endpoint attempt outcomes.
    """
    result = AttackResult(name="lateral_movement", mode=mode)

    if mode == "insecure":
        headers = {"API-Key": agent_credential}
    else:
        headers = {"Authorization": f"Bearer {agent_credential}"}

    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            for method, path, body in LATERAL_TARGETS:
                result.attempts += 1
                url = f"{resource_url}{path}"

                kwargs: dict = {"headers": headers}
                if body is not None:
                    kwargs["json"] = body

                resp = await client.request(method, url, **kwargs)

                if resp.status_code == 200:
                    result.successes += 1
                    result.details.append(
                        f"{method} {path}: ACCESS GRANTED (status {resp.status_code})"
                    )
                else:
                    result.blocked += 1
                    result.details.append(
                        f"{method} {path}: BLOCKED (status {resp.status_code})"
                    )
    except (httpx.ConnectError, httpx.TimeoutException) as exc:
        result.details.append(f"CONNECTION FAILED: {_sanitize_error(exc)}")
        return result

    return result
