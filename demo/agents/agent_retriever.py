"""Agent A -- DataRetriever: fetches customer data from the resource server."""

from __future__ import annotations

import logging
from dataclasses import dataclass

from agents.agent_base import AgentBase
from resource_server.middleware import ServerMode

logger = logging.getLogger(__name__)


@dataclass
class DataRetriever(AgentBase):
    """Retrieves customer data for a given customer ID.

    In secure mode, the agent registers with the broker to obtain a
    scoped token before calling the resource server.  In insecure mode
    it uses a shared API key and skips registration.
    """

    async def run(
        self,
        customer_id: int,
        launch_token: str = "",
        orch_id: str = "",
        task_id: str = "",
    ) -> dict:
        """Execute the data-retrieval workflow.

        Returns the customer data dict from the resource server.
        """
        if self.mode == ServerMode.secure:
            logger.info("[%s] Registering with broker...", self.name)
            await self.register(
                launch_token=launch_token,
                orch_id=orch_id,
                task_id=task_id,
                scopes=[f"read:Customers:{customer_id}"],
            )

        logger.info("[%s] Fetching customer #%d...", self.name, customer_id)
        data = await self.call_resource("GET", f"/customers/{customer_id}")
        logger.info("[%s] Got customer data", self.name)
        return data
