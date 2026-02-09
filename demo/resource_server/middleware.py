"""Token validation middleware for secure and insecure modes.

Secure mode: extracts Bearer token, calls broker POST /v1/token/validate
with the token and required scope. Returns 401/403 on failure.

Insecure mode: accepts any request with a non-empty API-Key header.
"""

from __future__ import annotations

import os
import re
from dataclasses import dataclass, field
from enum import Enum
from typing import Callable

import httpx
from fastapi import Request, Response
from starlette.middleware.base import BaseHTTPMiddleware, RequestResponseEndpoint
from starlette.responses import JSONResponse


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


# Paths that skip auth entirely.
_SKIP_PATHS = frozenset({"/health", "/docs", "/openapi.json", "/redoc"})

# Scope rules: (method, path_regex) -> scope template.
# Templates use {name} placeholders that are filled from regex named groups.
_SCOPE_RULES: list[tuple[str, re.Pattern, str]] = [
    ("GET", re.compile(r"^/customers/(?P<id>\d+)$"), "read:Customers:{id}"),
    ("GET", re.compile(r"^/orders/(?P<customer_id>\d+)$"), "read:Orders:{customer_id}"),
    ("PUT", re.compile(r"^/tickets/(?P<id>\d+)$"), "write:Tickets:{id}"),
    ("POST", re.compile(r"^/notifications/send$"), "invoke:Notifications"),
]


def _required_scope(method: str, path: str) -> str | None:
    """Determine the required scope for a request, or None if no rule matches."""
    for rule_method, pattern, template in _SCOPE_RULES:
        if method == rule_method:
            m = pattern.match(path)
            if m:
                return template.format(**m.groupdict())
    return None


def _problem_response(status: int, title: str, detail: str = "") -> JSONResponse:
    """Return an RFC 7807 problem+json response."""
    return JSONResponse(
        status_code=status,
        content={
            "type": f"urn:agentauth:resource:{status}",
            "title": title,
            "status": status,
            "detail": detail,
            "instance": "",
        },
        media_type="application/problem+json",
    )


class AuthMiddleware(BaseHTTPMiddleware):
    """Starlette middleware that enforces auth in both secure and insecure modes."""

    def __init__(
        self,
        app,
        mode: ServerMode = ServerMode.secure,
        broker_url: str | None = None,
        http_client: httpx.AsyncClient | None = None,
    ) -> None:
        super().__init__(app)
        self.mode = mode
        self.broker_url = broker_url or os.environ.get(
            "BROKER_URL", "http://localhost:8080"
        )
        # Allow injection of httpx client for testing.
        self._client = http_client

    async def dispatch(
        self, request: Request, call_next: RequestResponseEndpoint
    ) -> Response:
        """Check credentials before forwarding to the route handler."""
        # Skip auth for non-API paths.
        if request.url.path in _SKIP_PATHS:
            request.state.security = SecurityContext(authenticated=True, mode=self.mode)
            return await call_next(request)

        if self.mode == ServerMode.insecure:
            return await self._insecure_check(request, call_next)
        return await self._secure_check(request, call_next)

    async def _insecure_check(
        self, request: Request, call_next: RequestResponseEndpoint
    ) -> Response:
        """Insecure mode: accept any request with a non-empty API-Key header."""
        api_key = request.headers.get("api-key", "").strip()
        if not api_key:
            return _problem_response(
                401,
                "Missing API key",
                "Provide an API-Key header",
            )
        request.state.security = SecurityContext(
            authenticated=True,
            agent_id="insecure-api-key",
            mode=ServerMode.insecure,
        )
        return await call_next(request)

    async def _secure_check(
        self, request: Request, call_next: RequestResponseEndpoint
    ) -> Response:
        """Secure mode: validate Bearer token against the broker."""
        auth_header = request.headers.get("authorization", "")
        if not auth_header.startswith("Bearer "):
            return _problem_response(
                401,
                "Missing bearer token",
                "Provide an Authorization: Bearer <token> header",
            )
        token = auth_header[7:]

        required_scope = _required_scope(request.method, request.url.path)
        payload: dict = {"token": token}
        if required_scope:
            payload["required_scope"] = required_scope

        try:
            client = self._client or httpx.AsyncClient()
            try:
                resp = await client.post(
                    f"{self.broker_url}/v1/token/validate",
                    json=payload,
                    timeout=5.0,
                )
            finally:
                if self._client is None:
                    await client.aclose()
        except httpx.RequestError as exc:
            return _problem_response(
                502,
                "Broker unreachable",
                f"Could not reach broker at {self.broker_url}: {exc}",
            )

        if resp.status_code == 200:
            body = resp.json()
            request.state.security = SecurityContext(
                authenticated=True,
                agent_id=body.get("agent_id", ""),
                scopes=body.get("scope", []),
                mode=ServerMode.secure,
            )
            return await call_next(request)

        # Broker returned an error — forward the status.
        if resp.status_code == 403:
            return _problem_response(
                403,
                "Scope mismatch",
                f"Token does not grant required scope: {required_scope}",
            )
        return _problem_response(
            401,
            "Token validation failed",
            "The broker rejected the token",
        )
