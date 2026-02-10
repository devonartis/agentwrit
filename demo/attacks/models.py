"""Shared data models for attack scenarios."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class AttackResult:
    """Outcome of a single attack scenario run.

    Attributes:
        name: Machine-readable attack identifier (e.g. "credential_theft").
        mode: Server mode the attack ran against ("secure" or "insecure").
        attempts: Total number of requests the attacker made.
        successes: How many requests succeeded (attacker got data/access).
        blocked: How many requests were blocked (401/403).
        details: Human-readable log of each step for demo output.
    """

    name: str
    mode: str
    attempts: int = 0
    successes: int = 0
    blocked: int = 0
    details: list[str] = field(default_factory=list)

    @property
    def attack_succeeded(self) -> bool:
        """True if the attacker got at least one unauthorized success."""
        return self.successes > 0
