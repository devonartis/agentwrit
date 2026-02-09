"""Attack 5: No Accountability -- can we identify which agent acted?

Insecure mode: No broker audit trail; agents are anonymous behind shared keys.
Secure mode:   Full audit trail with SPIFFE IDs, timestamps, scopes per action.
"""

from __future__ import annotations

import httpx

from attacks.models import AttackResult


def _sanitize_error(exc: Exception) -> str:
    """Return a safe error string that never leaks URLs or tokens."""
    if isinstance(exc, httpx.HTTPStatusError):
        return f"HTTP {exc.response.status_code} from {exc.request.url.path}"
    return type(exc).__name__


async def accountability_check(
    broker_url: str,
    admin_token: str | None,
    mode: str,
) -> AttackResult:
    """Check whether agent actions can be attributed after the fact.

    In secure mode, queries the broker's audit endpoint to find events
    with per-agent SPIFFE ID attribution.

    In insecure mode, there is no broker-issued token and therefore no
    audit trail -- the attacker's actions cannot be traced to an agent.

    Args:
        broker_url: Base URL of the broker.
        admin_token: Admin Bearer token for audit access (None in insecure mode).
        mode: "secure" or "insecure".

    Returns:
        AttackResult where `attack_succeeded` means the attacker evaded attribution.
    """
    result = AttackResult(name="accountability", mode=mode)

    if mode == "insecure":
        # No broker audit to query -- agents used shared API keys.
        result.attempts = 1
        result.successes = 1  # "success" = attacker evaded attribution
        result.details.append(
            "No audit trail available: agents used shared API keys with no identity"
        )
        result.details.append(
            "Cannot determine which agent accessed customer records"
        )
        return result

    # Secure mode: query the broker's audit endpoint.
    result.attempts = 1
    headers = {"Authorization": f"Bearer {admin_token}"}

    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            resp = await client.get(
                f"{broker_url}/v1/audit/events",
                headers=headers,
            )

            if resp.status_code == 200:
                body = resp.json()
                events = body if isinstance(body, list) else body.get("events", [])

                if events:
                    # Found audit events with attribution
                    result.blocked = 1  # attacker's evasion was blocked
                    result.details.append(
                        f"Audit trail found: {len(events)} event(s) with agent attribution"
                    )
                    for evt in events[:5]:  # Show up to 5 events
                        agent_id = evt.get("agent_id", evt.get("subject", "unknown"))
                        action = evt.get("action", evt.get("event_type", "unknown"))
                        result.details.append(
                            f"  Event: agent={agent_id}, action={action}"
                        )
                else:
                    result.successes = 1
                    result.details.append(
                        "Audit endpoint returned empty: no events recorded"
                    )
            else:
                result.successes = 1
                result.details.append(
                    f"Audit query failed (status {resp.status_code}): cannot verify attribution"
                )
    except (httpx.ConnectError, httpx.TimeoutException) as exc:
        result.details.append(f"CONNECTION FAILED: {_sanitize_error(exc)}")
        return result

    return result
