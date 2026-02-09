"""Async HTTP client wrapping the AgentAuth broker API."""

from __future__ import annotations

import logging
from dataclasses import dataclass

import httpx

logger = logging.getLogger(__name__)

_TIMEOUT = 5.0


@dataclass
class BrokerClient:
    """Thin wrapper around the broker REST endpoints.

    Parameters
    ----------
    broker_url:
        Base URL of the broker (no trailing slash).
    http_client:
        Optional pre-built ``httpx.AsyncClient`` (useful for tests).
    """

    broker_url: str = "http://localhost:8080"
    http_client: httpx.AsyncClient | None = None

    # -- internal helpers ----------------------------------------------------

    async def _client(self) -> httpx.AsyncClient:
        if self.http_client is not None:
            return self.http_client
        return httpx.AsyncClient()

    async def _request(
        self,
        method: str,
        path: str,
        *,
        json: dict | None = None,
        bearer: str | None = None,
    ) -> dict:
        """Issue a request and return the decoded JSON body.

        Raises ``httpx.HTTPStatusError`` on 4xx/5xx responses.
        """
        headers: dict[str, str] = {}
        if bearer:
            headers["Authorization"] = f"Bearer {bearer}"

        client = await self._client()
        owned = self.http_client is None
        try:
            resp = await client.request(
                method,
                f"{self.broker_url}{path}",
                json=json,
                headers=headers,
                timeout=_TIMEOUT,
            )
            resp.raise_for_status()
            return resp.json()
        finally:
            if owned:
                await client.aclose()

    # -- public API ----------------------------------------------------------

    async def get_challenge(self) -> str:
        """GET /v1/challenge -- returns the hex nonce string."""
        data = await self._request("GET", "/v1/challenge")
        return data["nonce"]

    async def register(
        self,
        launch_token: str,
        nonce: str,
        public_key_jwk: dict,
        signature_b64url: str,
        orch_id: str,
        task_id: str,
        scopes: list[str],
    ) -> dict:
        """POST /v1/register -- returns registration response with access_token."""
        body = {
            "launch_token": launch_token,
            "nonce": nonce,
            "agent_public_key": public_key_jwk,
            "signature": signature_b64url,
            "orchestration_id": orch_id,
            "task_id": task_id,
            "requested_scope": scopes,
        }
        return await self._request("POST", "/v1/register", json=body)

    async def validate_token(self, admin_token: str, token: str, scope: str = "") -> dict:
        """POST /v1/token/validate -- validate a token (requires admin Bearer)."""
        body: dict = {"token": token}
        if scope:
            body["required_scope"] = scope
        return await self._request("POST", "/v1/token/validate", json=body, bearer=admin_token)

    async def delegate(
        self,
        bearer_token: str,
        delegator_token: str,
        target_agent_id: str,
        scopes: list[str],
        max_ttl: int = 300,
    ) -> dict:
        """POST /v1/delegate -- create a delegation token.

        ``bearer_token`` authenticates with the auth middleware;
        ``delegator_token`` is placed in the request body for the DelegSvc.
        """
        body = {
            "delegator_token": delegator_token,
            "target_agent_id": target_agent_id,
            "delegated_scope": scopes,
            "max_ttl": max_ttl,
        }
        return await self._request("POST", "/v1/delegate", json=body, bearer=bearer_token)

    async def revoke(
        self,
        admin_token: str,
        level: str,
        target_id: str,
        reason: str = "",
    ) -> dict:
        """POST /v1/revoke -- revoke a token/agent/task/chain."""
        body = {
            "level": level,
            "target_id": target_id,
            "reason": reason,
        }
        return await self._request("POST", "/v1/revoke", json=body, bearer=admin_token)
