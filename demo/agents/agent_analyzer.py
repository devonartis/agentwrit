"""Agent B -- Analyzer: examines customer + orders and delegates to Agent C."""

from __future__ import annotations

import logging
from dataclasses import dataclass, field

from agents.agent_base import AgentBase
from resource_server.middleware import ServerMode

logger = logging.getLogger(__name__)


@dataclass
class AnalyzerResult:
    """Outcome produced by the Analyzer agent."""

    resolution_text: str
    orders: list[dict] = field(default_factory=list)
    delegation_token: str = ""
    delegation_depth: int = 0


@dataclass
class Analyzer(AgentBase):
    """Analyses customer data and order history, then delegates to Agent C.

    In secure mode, it registers with read scopes for the customer and
    orders, performs a simulated analysis, and creates a delegation token
    with attenuated write/invoke scope for the action-taker agent.
    """

    async def run(
        self,
        customer_data: dict,
        customer_id: int,
        ticket_id: int,
        launch_token: str = "",
        orch_id: str = "",
        task_id: str = "",
        agent_c_id: str = "",
    ) -> AnalyzerResult:
        """Execute the analysis workflow.

        Parameters
        ----------
        customer_data:
            Customer record returned by Agent A.
        customer_id:
            Customer identifier used for order lookup.
        ticket_id:
            Ticket identifier used for scope delegation.
        launch_token / orch_id / task_id:
            Broker registration parameters (secure mode only).
        agent_c_id:
            SPIFFE ID of Agent C (for delegation target).
        """
        if self.mode == ServerMode.secure:
            logger.info("[%s] Registering with broker...", self.name)
            await self.register(
                launch_token=launch_token,
                orch_id=orch_id,
                task_id=task_id,
                scopes=[
                    f"read:Customers:{customer_id}",
                    f"read:Orders:{customer_id}",
                    f"write:Tickets:{ticket_id}",
                    "invoke:Notifications",
                ],
            )

        # Fetch orders
        logger.info("[%s] Fetching orders for customer #%d...", self.name, customer_id)
        orders_resp = await self.call_resource("GET", f"/orders/{customer_id}")
        orders = orders_resp.get("orders", [orders_resp])

        # Simulated analysis
        tier = customer_data.get("tier", "standard")
        resolution = (
            f"Resolution for customer #{customer_id} (tier={tier}): "
            f"{len(orders)} order(s) reviewed. "
            f"Recommend closing ticket #{ticket_id} with priority "
            f"{'high' if tier == 'enterprise' else 'normal'}."
        )
        logger.info("[%s] Analysis complete: %s", self.name, resolution[:80])

        # Delegation (secure mode only)
        delegation_token = ""
        delegation_depth = 0
        if self.mode == ServerMode.secure and agent_c_id:
            logger.info("[%s] Delegating to Agent C (%s)...", self.name, agent_c_id)
            deleg_resp = await self.broker.delegate(
                bearer_token=self.access_token,
                delegator_token=self.access_token,
                target_agent_id=agent_c_id,
                scopes=[f"write:Tickets:{ticket_id}", "invoke:Notifications"],
            )
            delegation_token = deleg_resp["delegation_token"]
            delegation_depth = deleg_resp["delegation_depth"]
            logger.info("[%s] Delegation token issued (depth=%d)", self.name, delegation_depth)

        return AnalyzerResult(
            resolution_text=resolution,
            orders=orders,
            delegation_token=delegation_token,
            delegation_depth=delegation_depth,
        )
