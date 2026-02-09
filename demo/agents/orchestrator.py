"""Demo Orchestrator -- drives the full Agent A -> B -> C workflow."""

from __future__ import annotations

import argparse
import asyncio
import logging
import time
from dataclasses import dataclass, field

import httpx

from agents.agent_action import ActionTaker
from agents.agent_analyzer import Analyzer
from agents.agent_retriever import DataRetriever
from agents.broker_client import BrokerClient
from resource_server.middleware import ServerMode


def _sanitize_error(exc: Exception) -> str:
    """Return a safe error string that never leaks URLs or tokens."""
    if isinstance(exc, httpx.HTTPStatusError):
        return f"HTTP {exc.response.status_code} from {exc.request.url.path}"
    return type(exc).__name__

logger = logging.getLogger(__name__)


@dataclass
class AgentResult:
    """Timing and outcome for a single agent step."""

    agent_name: str
    success: bool
    elapsed_ms: float
    detail: str = ""


@dataclass
class DemoResult:
    """Aggregate outcome of the full demo workflow."""

    mode: str
    agents: list[AgentResult] = field(default_factory=list)
    total_time_ms: float = 0.0
    success: bool = False


async def run_demo(
    mode: ServerMode,
    ticket_id: int = 789,
    customer_id: int = 12345,
    broker_url: str = "http://localhost:8080",
    resource_url: str = "http://localhost:8090",
    launch_token: str = "",
    admin_token: str = "",
    insecure_api_key: str = "dev-key",
) -> DemoResult:
    """Execute the full 3-agent demo.

    Parameters
    ----------
    mode:
        ``ServerMode.secure`` or ``ServerMode.insecure``.
    ticket_id / customer_id:
        Domain objects for the scenario.
    broker_url / resource_url:
        Addresses of the running services.
    launch_token:
        Seed launch token (secure mode only).
    admin_token:
        Seed admin token (secure mode only -- unused today but reserved).
    insecure_api_key:
        Shared key for insecure mode.
    """
    result = DemoResult(mode=mode.value)
    broker = BrokerClient(broker_url=broker_url)
    orch_id = "demo-orch"
    task_id = f"ticket-{ticket_id}"
    t_start = time.monotonic()

    # ── Agent A: DataRetriever ─────────────────────────────────────────
    agent_a = DataRetriever(
        name="Agent-A",
        broker=broker,
        resource_url=resource_url,
        mode=mode,
        insecure_api_key=insecure_api_key,
    )
    t0 = time.monotonic()
    try:
        customer_data = await agent_a.run(
            customer_id=customer_id,
            launch_token=launch_token,
            orch_id=orch_id,
            task_id=task_id,
        )
        result.agents.append(AgentResult(
            agent_name="Agent-A",
            success=True,
            elapsed_ms=(time.monotonic() - t0) * 1000,
            detail=f"Retrieved customer #{customer_id}",
        ))
    except (httpx.HTTPStatusError, httpx.ConnectError, httpx.TimeoutException, ValueError, RuntimeError) as exc:
        result.agents.append(AgentResult(
            agent_name="Agent-A", success=False,
            elapsed_ms=(time.monotonic() - t0) * 1000,
            detail=_sanitize_error(exc),
        ))
        result.total_time_ms = (time.monotonic() - t_start) * 1000
        return result

    # ── Agent B: Analyzer ──────────────────────────────────────────────
    agent_b = Analyzer(
        name="Agent-B",
        broker=broker,
        resource_url=resource_url,
        mode=mode,
        insecure_api_key=insecure_api_key,
    )
    t0 = time.monotonic()
    # Agent C's SPIFFE ID (constructed; in secure mode Agent B delegates to it)
    agent_c_id = (
        f"spiffe://agentauth.local/agent/{orch_id}/{task_id}/agent-c"
        if mode == ServerMode.secure
        else ""
    )
    try:
        analysis = await agent_b.run(
            customer_data=customer_data,
            customer_id=customer_id,
            ticket_id=ticket_id,
            launch_token=launch_token,
            orch_id=orch_id,
            task_id=task_id,
            agent_c_id=agent_c_id,
        )
        result.agents.append(AgentResult(
            agent_name="Agent-B",
            success=True,
            elapsed_ms=(time.monotonic() - t0) * 1000,
            detail=analysis.resolution_text[:120],
        ))
    except (httpx.HTTPStatusError, httpx.ConnectError, httpx.TimeoutException, ValueError, RuntimeError) as exc:
        result.agents.append(AgentResult(
            agent_name="Agent-B", success=False,
            elapsed_ms=(time.monotonic() - t0) * 1000,
            detail=_sanitize_error(exc),
        ))
        result.total_time_ms = (time.monotonic() - t_start) * 1000
        return result

    # ── Agent C: ActionTaker ───────────────────────────────────────────
    agent_c = ActionTaker(
        name="Agent-C",
        broker=broker,
        resource_url=resource_url,
        mode=mode,
        insecure_api_key=insecure_api_key,
    )
    t0 = time.monotonic()
    try:
        action_result = await agent_c.run(
            ticket_id=ticket_id,
            customer_id=customer_id,
            resolution=analysis.resolution_text,
            delegation_token=analysis.delegation_token,
            orch_id=orch_id,
            task_id=task_id,
        )
        result.agents.append(AgentResult(
            agent_name="Agent-C",
            success=True,
            elapsed_ms=(time.monotonic() - t0) * 1000,
            detail=(
                f"ticket_updated={action_result.ticket_updated}, "
                f"notification_sent={action_result.notification_sent}"
            ),
        ))
    except (httpx.HTTPStatusError, httpx.ConnectError, httpx.TimeoutException, ValueError, RuntimeError) as exc:
        result.agents.append(AgentResult(
            agent_name="Agent-C", success=False,
            elapsed_ms=(time.monotonic() - t0) * 1000,
            detail=_sanitize_error(exc),
        ))
        result.total_time_ms = (time.monotonic() - t_start) * 1000
        return result

    result.total_time_ms = (time.monotonic() - t_start) * 1000
    result.success = all(a.success for a in result.agents)
    return result


# ── CLI entrypoint ─────────────────────────────────────────────────────────

def _print_summary(result: DemoResult) -> None:
    """Print a human-readable summary table."""
    print(f"\n{'='*60}")
    print(f"  AgentAuth Demo - mode={result.mode}")
    print(f"{'='*60}")
    for agent in result.agents:
        status = "OK" if agent.success else "FAIL"
        print(f"  [{status}] {agent.agent_name:10s} {agent.elapsed_ms:8.1f}ms  {agent.detail}")
    print(f"{'─'*60}")
    overall = "SUCCESS" if result.success else "FAILURE"
    print(f"  {overall} - total {result.total_time_ms:.1f}ms")
    print(f"{'='*60}\n")


def main() -> None:
    parser = argparse.ArgumentParser(description="AgentAuth Demo Orchestrator")
    parser.add_argument(
        "--mode", choices=["secure", "insecure"], default="insecure",
        help="Operating mode (default: insecure)",
    )
    parser.add_argument("--broker-url", default="http://localhost:8080")
    parser.add_argument("--resource-url", default="http://localhost:8090")
    parser.add_argument("--launch-token", default="", help="Seed launch token (secure mode)")
    parser.add_argument("--admin-token", default="", help="Seed admin token (secure mode)")
    parser.add_argument("--ticket-id", type=int, default=789)
    parser.add_argument("--customer-id", type=int, default=12345)
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(name)s %(levelname)s %(message)s",
    )

    mode = ServerMode.secure if args.mode == "secure" else ServerMode.insecure
    result = asyncio.run(run_demo(
        mode=mode,
        ticket_id=args.ticket_id,
        customer_id=args.customer_id,
        broker_url=args.broker_url,
        resource_url=args.resource_url,
        launch_token=args.launch_token,
        admin_token=args.admin_token,
    ))
    _print_summary(result)


if __name__ == "__main__":
    main()
