"""Tests for Attack 3: Agent Impersonation."""

from __future__ import annotations

import httpx
import pytest

from attacks.impersonation import IMPERSONATION_TARGETS, impersonation_attack
from attacks.models import AttackResult


# -- helpers ----------------------------------------------------------------


def _insecure_transport():
    """Mock: insecure mode accepts any API-Key."""

    def handler(request: httpx.Request) -> httpx.Response:
        if request.headers.get("api-key"):
            return httpx.Response(200, json={"ok": True})
        return httpx.Response(401, json={"title": "Missing API key"})

    return httpx.MockTransport(handler)


def _secure_transport():
    """Mock: secure mode rejects fake/unknown Bearer tokens (401)."""

    def handler(request: httpx.Request) -> httpx.Response:
        auth = request.headers.get("authorization", "")
        if not auth.startswith("Bearer "):
            return httpx.Response(401, json={"title": "Missing bearer token"})

        # Broker rejects the forged token
        return httpx.Response(
            401,
            json={
                "type": "urn:agentauth:resource:401",
                "title": "Token validation failed",
                "status": 401,
                "detail": "The broker rejected the token",
            },
        )

    return httpx.MockTransport(handler)


def _patched_client_class(transport):
    class Patched(httpx.AsyncClient):
        def __init__(self, **kw):
            kw["transport"] = transport
            super().__init__(**kw)

    return Patched


# -- insecure mode tests ----------------------------------------------------


class TestImpersonationInsecure:
    """Insecure mode: shared API key makes rogue indistinguishable."""

    @pytest.mark.asyncio
    async def test_rogue_accepted_everywhere(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_insecure_transport())
        )

        result = await impersonation_attack(
            resource_url="http://test-resource",
            mode="insecure",
        )

        assert result.name == "impersonation"
        assert result.mode == "insecure"
        assert result.attempts == 2
        assert result.successes == 2
        assert result.blocked == 0
        assert result.attack_succeeded is True

    @pytest.mark.asyncio
    async def test_details_show_impersonation_accepted(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_insecure_transport())
        )

        result = await impersonation_attack(
            resource_url="http://test-resource",
            mode="insecure",
        )

        for detail in result.details:
            assert "IMPERSONATION ACCEPTED" in detail


# -- secure mode tests ------------------------------------------------------


class TestImpersonationSecure:
    """Secure mode: fake token rejected by broker."""

    @pytest.mark.asyncio
    async def test_rogue_rejected_everywhere(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        result = await impersonation_attack(
            resource_url="http://test-resource",
            mode="secure",
        )

        assert result.attempts == 2
        assert result.successes == 0
        assert result.blocked == 2
        assert result.attack_succeeded is False

    @pytest.mark.asyncio
    async def test_details_show_rejected(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        result = await impersonation_attack(
            resource_url="http://test-resource",
            mode="secure",
        )

        for detail in result.details:
            assert "REJECTED" in detail

    @pytest.mark.asyncio
    async def test_rejection_is_401(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        result = await impersonation_attack(
            resource_url="http://test-resource",
            mode="secure",
        )

        for detail in result.details:
            assert "401" in detail


# -- target list tests ------------------------------------------------------


class TestImpersonationTargets:
    """Verify the impersonation targets."""

    def test_two_targets(self) -> None:
        assert len(IMPERSONATION_TARGETS) == 2

    def test_targets_include_customers_and_orders(self) -> None:
        paths = [path for _, path in IMPERSONATION_TARGETS]
        assert "/customers/12345" in paths
        assert "/orders/12345" in paths
