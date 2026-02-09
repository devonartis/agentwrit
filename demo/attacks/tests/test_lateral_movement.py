"""Tests for Attack 2: Lateral Movement."""

from __future__ import annotations

import httpx
import pytest

from attacks.lateral_movement import LATERAL_TARGETS, lateral_movement_attack
from attacks.models import AttackResult


# -- helpers ----------------------------------------------------------------


def _insecure_transport():
    """Mock: insecure mode returns 200 for every request with API-Key."""

    def handler(request: httpx.Request) -> httpx.Response:
        if not request.headers.get("api-key"):
            return httpx.Response(401, json={"title": "Missing API key"})
        return httpx.Response(200, json={"ok": True})

    return httpx.MockTransport(handler)


def _secure_transport():
    """Mock: secure mode returns 403 for all lateral targets (scope mismatch)."""

    def handler(request: httpx.Request) -> httpx.Response:
        auth = request.headers.get("authorization", "")
        if not auth.startswith("Bearer "):
            return httpx.Response(401, json={"title": "Missing bearer token"})

        # Token is scoped to read:Customers:12345 -- nothing else passes
        return httpx.Response(
            403,
            json={
                "type": "urn:agentauth:resource:403",
                "title": "Scope mismatch",
                "status": 403,
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


class TestLateralMovementInsecure:
    """Insecure mode: shared API key lets the attacker move freely."""

    @pytest.mark.asyncio
    async def test_all_endpoints_accessible(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_insecure_transport())
        )

        result = await lateral_movement_attack(
            agent_credential="shared-api-key",
            resource_url="http://test-resource",
            mode="insecure",
        )

        assert result.name == "lateral_movement"
        assert result.mode == "insecure"
        assert result.attempts == 3
        assert result.successes == 3
        assert result.blocked == 0
        assert result.attack_succeeded is True

    @pytest.mark.asyncio
    async def test_details_all_granted(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_insecure_transport())
        )

        result = await lateral_movement_attack(
            agent_credential="shared-api-key",
            resource_url="http://test-resource",
            mode="insecure",
        )

        for detail in result.details:
            assert "ACCESS GRANTED" in detail


# -- secure mode tests ------------------------------------------------------


class TestLateralMovementSecure:
    """Secure mode: scoped token blocks all lateral access."""

    @pytest.mark.asyncio
    async def test_all_endpoints_blocked(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        result = await lateral_movement_attack(
            agent_credential="scoped-token-customers-only",
            resource_url="http://test-resource",
            mode="secure",
        )

        assert result.attempts == 3
        assert result.successes == 0
        assert result.blocked == 3
        assert result.attack_succeeded is False

    @pytest.mark.asyncio
    async def test_details_all_blocked(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        result = await lateral_movement_attack(
            agent_credential="scoped-token-customers-only",
            resource_url="http://test-resource",
            mode="secure",
        )

        for detail in result.details:
            assert "BLOCKED" in detail

    @pytest.mark.asyncio
    async def test_blocked_status_is_403(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        result = await lateral_movement_attack(
            agent_credential="scoped-token-customers-only",
            resource_url="http://test-resource",
            mode="secure",
        )

        for detail in result.details:
            assert "403" in detail


# -- target list tests ------------------------------------------------------


class TestLateralTargets:
    """Verify the attack targets the expected endpoints."""

    def test_three_lateral_targets(self) -> None:
        assert len(LATERAL_TARGETS) == 3

    def test_targets_cover_orders_tickets_notifications(self) -> None:
        paths = [path for _, path, _ in LATERAL_TARGETS]
        assert "/orders/12345" in paths
        assert "/tickets/789" in paths
        assert "/notifications/send" in paths
