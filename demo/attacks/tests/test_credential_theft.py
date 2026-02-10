"""Tests for Attack 1: Credential Theft."""

from __future__ import annotations

import httpx
import pytest

from attacks.credential_theft import CUSTOMER_IDS, credential_theft_attack
from attacks.models import AttackResult


# -- helpers ----------------------------------------------------------------


def _insecure_transport():
    """Mock transport: insecure mode accepts every request with 200."""

    def handler(request: httpx.Request) -> httpx.Response:
        # Any request with API-Key header gets through
        if request.headers.get("api-key"):
            return httpx.Response(
                200, json={"customer_id": "12345", "name": "Acme Corp"}
            )
        return httpx.Response(401, json={"title": "Missing API key"})

    return httpx.MockTransport(handler)


def _secure_transport(allowed_customer_id: int = 12345):
    """Mock transport: secure mode allows only one customer, blocks others."""

    def handler(request: httpx.Request) -> httpx.Response:
        auth = request.headers.get("authorization", "")
        if not auth.startswith("Bearer "):
            return httpx.Response(401, json={"title": "Missing bearer token"})

        # Parse the customer ID from the URL path
        path = request.url.path
        # /customers/{id}
        parts = path.strip("/").split("/")
        if len(parts) == 2 and parts[0] == "customers":
            cid = int(parts[1])
            if cid == allowed_customer_id:
                return httpx.Response(
                    200, json={"customer_id": str(cid), "name": "Acme Corp"}
                )
            return httpx.Response(
                403,
                json={
                    "type": "urn:agentauth:resource:403",
                    "title": "Scope mismatch",
                    "status": 403,
                },
            )

        return httpx.Response(404, json={"title": "Not found"})

    return httpx.MockTransport(handler)


def _patched_client_class(transport):
    """Return an AsyncClient subclass that injects the mock transport."""

    class Patched(httpx.AsyncClient):
        def __init__(self, **kw):
            kw["transport"] = transport
            super().__init__(**kw)

    return Patched


# -- insecure mode tests ----------------------------------------------------


class TestCredentialTheftInsecure:
    """Insecure mode: stolen API key accesses ALL customers."""

    @pytest.mark.asyncio
    async def test_all_customers_accessible(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_insecure_transport())
        )

        result = await credential_theft_attack(
            stolen_credential="shared-api-key-123",
            resource_url="http://test-resource",
            mode="insecure",
        )

        assert isinstance(result, AttackResult)
        assert result.name == "credential_theft"
        assert result.mode == "insecure"
        assert result.attempts == 5
        assert result.successes == 5
        assert result.blocked == 0
        assert result.attack_succeeded is True
        assert len(result.details) == 5

    @pytest.mark.asyncio
    async def test_details_show_access_granted(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_insecure_transport())
        )

        result = await credential_theft_attack(
            stolen_credential="shared-api-key-123",
            resource_url="http://test-resource",
            mode="insecure",
        )

        for detail in result.details:
            assert "ACCESS GRANTED" in detail


# -- secure mode tests ------------------------------------------------------


class TestCredentialTheftSecure:
    """Secure mode: token scoped to one customer; 4 of 5 blocked."""

    @pytest.mark.asyncio
    async def test_only_scoped_customer_accessible(self, monkeypatch) -> None:
        """Edge case: token scoped to one customer (not expired).

        In the demo's expected flow, stolen tokens are expired (TTL passed),
        so all 5 customers are blocked (attack_succeeded=False). This test
        covers the intermediate case where the token is still valid but scoped,
        resulting in 1/5 success -- which counts as attack_succeeded=True.
        The integration test in test_simulator.py uses the expired-token path.
        """
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport(12345))
        )

        result = await credential_theft_attack(
            stolen_credential="scoped-token-for-12345",
            resource_url="http://test-resource",
            mode="secure",
        )

        assert result.attempts == 5
        assert result.successes == 1
        assert result.blocked == 4
        # Still technically succeeded once (the originally allowed customer)
        assert result.attack_succeeded is True

    @pytest.mark.asyncio
    async def test_expired_token_blocks_all(self, monkeypatch) -> None:
        """If the token is expired, all 5 requests are blocked."""

        def handler(request: httpx.Request) -> httpx.Response:
            return httpx.Response(
                401,
                json={
                    "type": "urn:agentauth:resource:401",
                    "title": "Token expired",
                    "status": 401,
                },
            )

        monkeypatch.setattr(
            httpx,
            "AsyncClient",
            _patched_client_class(httpx.MockTransport(handler)),
        )

        result = await credential_theft_attack(
            stolen_credential="expired-token",
            resource_url="http://test-resource",
            mode="secure",
        )

        assert result.attempts == 5
        assert result.successes == 0
        assert result.blocked == 5
        assert result.attack_succeeded is False

    @pytest.mark.asyncio
    async def test_details_show_blocked(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport(12345))
        )

        result = await credential_theft_attack(
            stolen_credential="scoped-token-for-12345",
            resource_url="http://test-resource",
            mode="secure",
        )

        blocked_details = [d for d in result.details if "BLOCKED" in d]
        granted_details = [d for d in result.details if "ACCESS GRANTED" in d]
        assert len(blocked_details) == 4
        assert len(granted_details) == 1


# -- model tests -----------------------------------------------------------


class TestAttackResultModel:
    """Basic AttackResult dataclass behavior."""

    def test_defaults(self) -> None:
        r = AttackResult(name="test", mode="secure")
        assert r.attempts == 0
        assert r.successes == 0
        assert r.blocked == 0
        assert r.details == []
        assert r.attack_succeeded is False

    def test_attack_succeeded_true(self) -> None:
        r = AttackResult(name="test", mode="insecure", successes=1)
        assert r.attack_succeeded is True

    def test_attack_succeeded_false(self) -> None:
        r = AttackResult(name="test", mode="secure", successes=0, blocked=5)
        assert r.attack_succeeded is False
