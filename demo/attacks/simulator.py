"""Attack simulator -- runs all 5 attacks and produces a summary report."""

from __future__ import annotations

from dataclasses import dataclass, field

from attacks.accountability import accountability_check
from attacks.credential_theft import credential_theft_attack
from attacks.impersonation import impersonation_attack
from attacks.lateral_movement import lateral_movement_attack
from attacks.models import AttackResult
from attacks.privilege_escalation import escalation_attack


@dataclass
class SimulatorResult:
    """Aggregated result from running all 5 attack scenarios.

    Attributes:
        mode: "secure" or "insecure".
        results: Individual AttackResult for each scenario.
        meets_expectation: True when the mode's expected outcome is met.
            - insecure: all 5 attacks should succeed (proves the gap).
            - secure: all 5 attacks should be blocked (proves the fix).
    """

    mode: str
    results: list[AttackResult] = field(default_factory=list)

    @property
    def meets_expectation(self) -> bool:
        """Check whether outcomes match the expected demo story."""
        if self.mode == "insecure":
            return all(r.attack_succeeded for r in self.results)
        # Secure: none should succeed
        return all(not r.attack_succeeded for r in self.results)


async def run_all_attacks(
    mode: str,
    broker_url: str = "http://localhost:8080",
    resource_url: str = "http://localhost:8090",
    stolen_credential: str = "stolen-cred",
    agent_c_token: str = "agent-c-token",
    admin_token: str | None = None,
    shared_api_key: str = "shared-api-key",
) -> SimulatorResult:
    """Execute all 5 attack scenarios sequentially.

    Args:
        mode: "secure" or "insecure".
        broker_url: Base URL of the broker.
        resource_url: Base URL of the resource server.
        stolen_credential: Credential for the credential theft attack.
        agent_c_token: Token/key for the privilege escalation attack.
        admin_token: Admin token for audit queries (None in insecure mode).
        shared_api_key: Shared API key for impersonation in insecure mode.

    Returns:
        SimulatorResult with all 5 attack outcomes.
    """
    sim = SimulatorResult(mode=mode)

    # Attack 1: Credential Theft
    sim.results.append(
        await credential_theft_attack(stolen_credential, resource_url, mode)
    )

    # Attack 2: Lateral Movement
    sim.results.append(
        await lateral_movement_attack(stolen_credential, resource_url, mode)
    )

    # Attack 3: Agent Impersonation
    sim.results.append(
        await impersonation_attack(resource_url, mode, shared_api_key=shared_api_key)
    )

    # Attack 4: Privilege Escalation
    sim.results.append(
        await escalation_attack(agent_c_token, broker_url, resource_url, mode)
    )

    # Attack 5: Accountability
    sim.results.append(
        await accountability_check(broker_url, admin_token, mode)
    )

    return sim
