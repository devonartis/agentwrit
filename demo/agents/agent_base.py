"""Base class for AgentAuth demo agents with registration and resource access."""

from __future__ import annotations

import base64
import logging
from dataclasses import dataclass, field

import httpx
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

from agents.broker_client import BrokerClient
from resource_server.middleware import ServerMode

logger = logging.getLogger(__name__)


def _b64url(data: bytes) -> str:
    """Base64url-encode *data* with no padding (RFC 7515)."""
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


@dataclass
class RegistrationResult:
    """Result returned by :meth:`AgentBase.register`."""

    agent_instance_id: str
    access_token: str
    expires_in: int
    refresh_after: int


@dataclass
class AgentBase:
    """Common foundation shared by every demo agent.

    Handles ephemeral key generation, broker registration, and
    authenticated calls to the resource server.
    """

    name: str
    broker: BrokerClient
    resource_url: str = "http://localhost:8090"
    mode: ServerMode = ServerMode.secure
    insecure_api_key: str = "dev-key"

    # -- state filled by register() -----------------------------------------
    agent_instance_id: str = field(default="", init=False)
    access_token: str = field(default="", init=False)
    _private_key: Ed25519PrivateKey | None = field(default=None, init=False, repr=False)

    # -- registration --------------------------------------------------------

    async def register(
        self,
        launch_token: str,
        orch_id: str,
        task_id: str,
        scopes: list[str],
    ) -> RegistrationResult:
        """Run the full challenge-response registration flow.

        1. Generate an ephemeral Ed25519 key pair.
        2. Obtain a nonce from the broker.
        3. Sign the nonce.
        4. POST /v1/register with the proof.
        5. Store the resulting credentials.
        """
        # 1 -- key generation
        self._private_key = Ed25519PrivateKey.generate()
        pub = self._private_key.public_key()
        pub_bytes = pub.public_bytes(Encoding.Raw, PublicFormat.Raw)
        jwk = {"kty": "OKP", "crv": "Ed25519", "x": _b64url(pub_bytes)}

        # 2 -- challenge
        nonce = await self.broker.get_challenge()
        logger.info("[%s] obtained nonce %s...", self.name, nonce[:16])

        # 3 -- sign
        nonce_bytes = bytes.fromhex(nonce)
        signature = self._private_key.sign(nonce_bytes)
        sig_b64url = _b64url(signature)

        # 4 -- register
        resp = await self.broker.register(
            launch_token=launch_token,
            nonce=nonce,
            public_key_jwk=jwk,
            signature_b64url=sig_b64url,
            orch_id=orch_id,
            task_id=task_id,
            scopes=scopes,
        )

        # 5 -- store credentials
        self.agent_instance_id = resp["agent_instance_id"]
        self.access_token = resp["access_token"]

        result = RegistrationResult(
            agent_instance_id=self.agent_instance_id,
            access_token=self.access_token,
            expires_in=resp["expires_in"],
            refresh_after=resp["refresh_after"],
        )
        logger.info("[%s] registered as %s", self.name, self.agent_instance_id)
        return result

    # -- resource server calls -----------------------------------------------

    def _headers(self) -> dict[str, str]:
        """Return auth headers appropriate for the current mode."""
        if self.mode == ServerMode.secure:
            return {"Authorization": f"Bearer {self.access_token}"}
        return {"API-Key": self.insecure_api_key}

    async def call_resource(
        self,
        method: str,
        path: str,
        json: dict | None = None,
    ) -> dict:
        """Make an authenticated HTTP call to the resource server.

        Returns the decoded JSON response body.
        Raises ``httpx.HTTPStatusError`` on non-2xx responses.
        """
        url = f"{self.resource_url}{path}"
        headers = self._headers()
        logger.info("[%s] %s %s", self.name, method, path)

        async with httpx.AsyncClient() as client:
            resp = await client.request(
                method, url, json=json, headers=headers, timeout=5.0,
            )
            resp.raise_for_status()
            data = resp.json()

        logger.info("[%s] %s %s -> %d", self.name, method, path, resp.status_code)
        return data
