"""Token validation middleware for secure and insecure modes.

Secure mode: extracts Bearer token and validates against broker's /v1/token/validate.
Insecure mode: accepts any request with a non-empty API-Key header.

This file is a stub for T01; full implementation arrives in T02 (M11-T02).
"""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum


class ServerMode(str, Enum):
    """Operating mode for the resource server."""

    secure = "secure"
    insecure = "insecure"


@dataclass
class SecurityContext:
    """Result of auth check — passed to route handlers via request state."""

    authenticated: bool = False
    agent_id: str = ""
    scopes: list[str] = field(default_factory=list)
    mode: ServerMode = ServerMode.insecure
