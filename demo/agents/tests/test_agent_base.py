"""Tests for BrokerClient and AgentBase."""

from __future__ import annotations

import base64

import httpx
import pytest

from agents.agent_base import AgentBase, RegistrationResult, _b64url
from agents.broker_client import BrokerClient
from resource_server.middleware import ServerMode
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat


# ── helpers ────────────────────────────────────────────────────────────────


def _pad_b64url(s: str) -> str:
    """Add base64 padding so stdlib can decode."""
    return s + "=" * (-len(s) % 4)


# ── Ed25519 key / JWK ─────────────────────────────────────────────────────


class TestEdKeysAndJwk:
    """Verify Ed25519 key generation produces a valid JWK and signable key."""

    def test_b64url_encoding(self) -> None:
        raw = b"\x00\x01\x02\xff"
        encoded = _b64url(raw)
        # No padding characters
        assert "=" not in encoded
        # Roundtrip
        decoded = base64.urlsafe_b64decode(_pad_b64url(encoded))
        assert decoded == raw

    def test_ed25519_key_gen_produces_32_byte_pub(self) -> None:
        priv = Ed25519PrivateKey.generate()
        pub_bytes = priv.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
        assert len(pub_bytes) == 32

    def test_nonce_signing_roundtrip(self) -> None:
        """Sign a hex nonce and verify with the matching public key."""
        priv = Ed25519PrivateKey.generate()
        pub = priv.public_key()
        nonce_hex = "ab" * 32
        nonce_bytes = bytes.fromhex(nonce_hex)
        sig = priv.sign(nonce_bytes)
        # Verify should not raise
        pub.verify(sig, nonce_bytes)

    def test_jwk_format(self) -> None:
        priv = Ed25519PrivateKey.generate()
        pub_bytes = priv.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
        jwk = {"kty": "OKP", "crv": "Ed25519", "x": _b64url(pub_bytes)}
        assert jwk["kty"] == "OKP"
        assert jwk["crv"] == "Ed25519"
        decoded_x = base64.urlsafe_b64decode(_pad_b64url(jwk["x"]))
        assert decoded_x == pub_bytes


# ── BrokerClient ───────────────────────────────────────────────────────────


class TestBrokerClient:
    """Verify BrokerClient sends correct HTTP requests via mock transport."""

    @pytest.mark.asyncio
    async def test_get_challenge(self, mock_broker_client: BrokerClient) -> None:
        nonce = await mock_broker_client.get_challenge()
        assert nonce == "ab" * 32

    @pytest.mark.asyncio
    async def test_register(self, mock_broker_client: BrokerClient) -> None:
        resp = await mock_broker_client.register(
            launch_token="lt-123",
            nonce="ab" * 32,
            public_key_jwk={"kty": "OKP", "crv": "Ed25519", "x": "AAAA"},
            signature_b64url="c2ln",
            orch_id="orch1",
            task_id="task1",
            scopes=["read:Customers:*"],
        )
        assert resp["agent_instance_id"].startswith("spiffe://")
        assert resp["access_token"] == "mock-access-token"

    @pytest.mark.asyncio
    async def test_validate_token(self, mock_broker_client: BrokerClient) -> None:
        resp = await mock_broker_client.validate_token(
            admin_token="admin-tok",
            token="some-tok",
            scope="read:Customers:*",
        )
        assert resp["valid"] is True

    @pytest.mark.asyncio
    async def test_delegate(self, mock_broker_client: BrokerClient) -> None:
        resp = await mock_broker_client.delegate(
            bearer_token="bearer-tok",
            delegator_token="deleg-tok",
            target_agent_id="spiffe://agentauth.local/agent/o/t/i2",
            scopes=["read:Customers:12345"],
            max_ttl=120,
        )
        assert resp["delegation_token"] == "mock-deleg-token"
        assert resp["delegation_depth"] == 1

    @pytest.mark.asyncio
    async def test_revoke(self, mock_broker_client: BrokerClient) -> None:
        resp = await mock_broker_client.revoke(
            admin_token="admin-tok",
            level="token",
            target_id="some-jti",
            reason="test revocation",
        )
        assert resp["revoked"] is True


# ── AgentBase ──────────────────────────────────────────────────────────────


class TestAgentBaseRegistration:
    """Test the full registration flow with mocked broker."""

    @pytest.mark.asyncio
    async def test_register_stores_credentials(self, mock_agent: AgentBase) -> None:
        result = await mock_agent.register(
            launch_token="lt-123",
            orch_id="orch1",
            task_id="task1",
            scopes=["read:Customers:*"],
        )
        assert isinstance(result, RegistrationResult)
        assert result.agent_instance_id.startswith("spiffe://")
        assert mock_agent.agent_instance_id == result.agent_instance_id
        assert mock_agent.access_token == "mock-access-token"

    @pytest.mark.asyncio
    async def test_register_generates_ephemeral_key(self, mock_agent: AgentBase) -> None:
        await mock_agent.register("lt", "o", "t", ["s"])
        assert mock_agent._private_key is not None


class TestAgentBaseHeaders:
    """Test auth header generation for secure and insecure modes."""

    @pytest.mark.asyncio
    async def test_secure_headers(self, mock_agent: AgentBase) -> None:
        mock_agent.access_token = "tok-abc"
        headers = mock_agent._headers()
        assert headers == {"Authorization": "Bearer tok-abc"}

    @pytest.mark.asyncio
    async def test_insecure_headers(self, insecure_agent: AgentBase) -> None:
        headers = insecure_agent._headers()
        assert headers == {"API-Key": "test-key"}


class TestAgentBaseResourceCalls:
    """Test call_resource sends correct method/headers."""

    @pytest.mark.asyncio
    async def test_call_resource_secure(self, mock_agent: AgentBase, monkeypatch) -> None:
        mock_agent.access_token = "tok-xyz"

        captured: dict = {}

        def handler(request: httpx.Request) -> httpx.Response:
            captured["method"] = request.method
            captured["url"] = str(request.url)
            captured["auth"] = request.headers.get("authorization", "")
            return httpx.Response(200, json={"ok": True})

        mock_agent.resource_url = "http://test-resource"

        class PatchedClient(httpx.AsyncClient):
            def __init__(self, **kw):
                kw["transport"] = httpx.MockTransport(handler)
                kw["base_url"] = "http://test-resource"
                super().__init__(**kw)

        monkeypatch.setattr(httpx, "AsyncClient", PatchedClient)
        data = await mock_agent.call_resource("GET", "/customers/123")

        assert data == {"ok": True}
        assert captured["method"] == "GET"
        assert captured["auth"] == "Bearer tok-xyz"

    @pytest.mark.asyncio
    async def test_call_resource_insecure(self, insecure_agent: AgentBase, monkeypatch) -> None:
        captured: dict = {}

        def handler(request: httpx.Request) -> httpx.Response:
            captured["api_key"] = request.headers.get("api-key", "")
            return httpx.Response(200, json={"ok": True})

        class PatchedClient(httpx.AsyncClient):
            def __init__(self, **kw):
                kw["transport"] = httpx.MockTransport(handler)
                kw["base_url"] = "http://test-resource"
                super().__init__(**kw)

        monkeypatch.setattr(httpx, "AsyncClient", PatchedClient)
        await insecure_agent.call_resource("PUT", "/tickets/789")

        assert captured["api_key"] == "test-key"
