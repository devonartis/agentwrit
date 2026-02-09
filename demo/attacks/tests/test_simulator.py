"""Integration tests for the attack simulator."""

from __future__ import annotations

import httpx
import pytest

from attacks.models import AttackResult
from attacks.simulator import SimulatorResult, run_all_attacks


# -- mock transports -------------------------------------------------------


def _insecure_resource_transport():
    """Mock: insecure resource server returns 200 for any API-Key request."""

    def handler(request: httpx.Request) -> httpx.Response:
        if request.headers.get("api-key"):
            return httpx.Response(200, json={"ok": True, "data": "accessible"})
        return httpx.Response(401, json={"title": "Missing API key"})

    return httpx.MockTransport(handler)


def _secure_transport():
    """Mock: secure broker + resource server that enforces scopes.

    Routes by host/path:
    - broker /v1/delegate -> 403 (scope attenuation violation)
    - broker /v1/audit/events -> 200 with events
    - resource /customers/12345 -> 200 (scoped token matches)
    - resource /customers/* (other) -> 403 (scope mismatch)
    - resource /orders/* -> 403
    - resource /tickets/* -> 403
    - resource /notifications/* -> 403
    - resource (fake bearer) -> 401
    """

    audit_events = [
        {
            "event_type": "resource_accessed",
            "agent_id": "spiffe://agentauth.local/agent/orch1/task1/inst1",
            "action": "read:Customers:12345",
            "timestamp": "2026-01-15T10:30:00Z",
        },
    ]

    def handler(request: httpx.Request) -> httpx.Response:
        path = request.url.path
        host = str(request.url.host)
        auth = request.headers.get("authorization", "")

        # Broker endpoints
        if "broker" in host or request.url.port == 8080:
            if path == "/v1/delegate" and request.method == "POST":
                if not auth.startswith("Bearer "):
                    return httpx.Response(401, json={"title": "Missing bearer"})
                return httpx.Response(
                    403, json={"title": "Scope attenuation violation", "status": 403}
                )
            if path == "/v1/audit/events" and request.method == "GET":
                if not auth.startswith("Bearer "):
                    return httpx.Response(401, json={"title": "Missing bearer"})
                return httpx.Response(200, json={"events": audit_events})
            return httpx.Response(404, json={"title": "Not found"})

        # Resource server endpoints (secure mode)
        if not auth.startswith("Bearer "):
            return httpx.Response(401, json={"title": "Missing bearer token"})

        # Impersonation: random hex tokens are always rejected
        # Real scoped tokens: only customers/12345 passes
        token = auth[7:]

        # Credential theft + lateral: "stolen-cred" token has expired (5-min TTL)
        if token == "stolen-cred":
            return httpx.Response(
                401, json={"title": "Token expired", "status": 401}
            )

        # Escalation: agent-c-token has write:Tickets + invoke:Notifications only
        if token == "agent-c-token":
            return httpx.Response(403, json={"title": "Scope mismatch", "status": 403})

        # Unknown tokens (impersonation attack fake tokens) -> 401
        return httpx.Response(
            401, json={"title": "Token validation failed", "status": 401}
        )

    return httpx.MockTransport(handler)


def _patched_client_class(transport):
    class Patched(httpx.AsyncClient):
        def __init__(self, **kw):
            kw["transport"] = transport
            super().__init__(**kw)

    return Patched


# -- insecure mode integration tests ----------------------------------------


class TestSimulatorInsecure:
    """All 5 attacks should succeed in insecure mode (proves the gap)."""

    @pytest.mark.asyncio
    async def test_all_attacks_succeed(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_insecure_resource_transport())
        )

        sim = await run_all_attacks(
            mode="insecure",
            broker_url="http://test-broker:8080",
            resource_url="http://test-resource:8090",
            stolen_credential="shared-api-key",
            agent_c_token="shared-api-key",
            admin_token=None,
            shared_api_key="shared-api-key",
        )

        assert isinstance(sim, SimulatorResult)
        assert sim.mode == "insecure"
        assert len(sim.results) == 5

        for r in sim.results:
            assert isinstance(r, AttackResult)
            assert r.attack_succeeded is True, f"{r.name} should succeed in insecure"

        assert sim.meets_expectation is True

    @pytest.mark.asyncio
    async def test_result_names(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_insecure_resource_transport())
        )

        sim = await run_all_attacks(
            mode="insecure",
            broker_url="http://test-broker:8080",
            resource_url="http://test-resource:8090",
        )

        names = [r.name for r in sim.results]
        assert names == [
            "credential_theft",
            "lateral_movement",
            "impersonation",
            "privilege_escalation",
            "accountability",
        ]


# -- secure mode integration tests ------------------------------------------


class TestSimulatorSecure:
    """All 5 attacks should be blocked in secure mode (proves the fix)."""

    @pytest.mark.asyncio
    async def test_all_attacks_blocked(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        sim = await run_all_attacks(
            mode="secure",
            broker_url="http://test-broker:8080",
            resource_url="http://test-resource:8090",
            stolen_credential="stolen-cred",
            agent_c_token="agent-c-token",
            admin_token="admin-token-123",
        )

        assert isinstance(sim, SimulatorResult)
        assert sim.mode == "secure"
        assert len(sim.results) == 5

        for r in sim.results:
            assert isinstance(r, AttackResult)
            assert r.attack_succeeded is False, f"{r.name} should be blocked in secure"

        assert sim.meets_expectation is True

    @pytest.mark.asyncio
    async def test_credential_theft_all_expired(self, monkeypatch) -> None:
        """Credential theft in secure mode: expired token blocks all 5."""
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        sim = await run_all_attacks(
            mode="secure",
            broker_url="http://test-broker:8080",
            resource_url="http://test-resource:8090",
            stolen_credential="stolen-cred",
            agent_c_token="agent-c-token",
            admin_token="admin-token-123",
        )

        cred_theft = sim.results[0]
        assert cred_theft.name == "credential_theft"
        assert cred_theft.attempts == 5
        assert cred_theft.successes == 0
        assert cred_theft.blocked == 5
        assert cred_theft.attack_succeeded is False


# -- SimulatorResult tests --------------------------------------------------


class TestSimulatorResult:
    """Test the SimulatorResult dataclass behavior."""

    def test_insecure_meets_expectation_all_succeed(self) -> None:
        sim = SimulatorResult(
            mode="insecure",
            results=[
                AttackResult(name="a", mode="insecure", successes=1),
                AttackResult(name="b", mode="insecure", successes=3),
            ],
        )
        assert sim.meets_expectation is True

    def test_insecure_fails_if_one_blocked(self) -> None:
        sim = SimulatorResult(
            mode="insecure",
            results=[
                AttackResult(name="a", mode="insecure", successes=1),
                AttackResult(name="b", mode="insecure", successes=0, blocked=1),
            ],
        )
        assert sim.meets_expectation is False

    def test_secure_meets_expectation_all_blocked(self) -> None:
        sim = SimulatorResult(
            mode="secure",
            results=[
                AttackResult(name="a", mode="secure", successes=0, blocked=5),
                AttackResult(name="b", mode="secure", successes=0, blocked=3),
            ],
        )
        assert sim.meets_expectation is True

    def test_secure_fails_if_one_succeeds(self) -> None:
        sim = SimulatorResult(
            mode="secure",
            results=[
                AttackResult(name="a", mode="secure", successes=0, blocked=5),
                AttackResult(name="b", mode="secure", successes=1),
            ],
        )
        assert sim.meets_expectation is False
