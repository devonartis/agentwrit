"""In-memory state manager for the demo dashboard.

Tracks demo execution state, SSE subscribers, and accumulated events.
Thread-safe via asyncio.Lock for concurrent SSE clients.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
from datetime import datetime, timezone


@dataclass
class DashboardState:
    """Shared mutable state for a single dashboard session.

    Attributes:
        running: True while a demo/attack run is in progress.
        mode: Current demo mode ("insecure" or "secure"), or None if idle.
        demo_result: Serialized DemoResult dict after demo completes.
        attack_results: List of serialized AttackResult dicts after attacks complete.
        events: Chronological list of all published events.
    """

    running: bool = False
    mode: str | None = None
    demo_result: dict | None = None
    attack_results: list[dict] | None = None
    events: list[dict] = field(default_factory=list)
    _subscribers: list[asyncio.Queue] = field(default_factory=list, repr=False)
    _lock: asyncio.Lock = field(default_factory=asyncio.Lock, repr=False)

    async def publish(self, event: dict) -> None:
        """Append *event* to the log and push it to all SSE subscribers."""
        if "timestamp" not in event:
            event["timestamp"] = datetime.now(timezone.utc).isoformat()
        async with self._lock:
            self.events.append(event)
            for queue in self._subscribers:
                try:
                    queue.put_nowait(event)
                except asyncio.QueueFull:
                    pass  # drop if subscriber is slow

    async def subscribe(self) -> asyncio.Queue:
        """Register a new SSE subscriber and return its event queue."""
        queue: asyncio.Queue = asyncio.Queue(maxsize=256)
        async with self._lock:
            self._subscribers.append(queue)
        return queue

    async def unsubscribe(self, queue: asyncio.Queue) -> None:
        """Remove a subscriber queue."""
        async with self._lock:
            try:
                self._subscribers.remove(queue)
            except ValueError:
                pass

    async def reset(self) -> None:
        """Clear all state back to initial idle."""
        async with self._lock:
            self.running = False
            self.mode = None
            self.demo_result = None
            self.attack_results = None
            self.events.clear()
            # Drain and remove all subscriber queues.
            for queue in self._subscribers:
                while not queue.empty():
                    try:
                        queue.get_nowait()
                    except asyncio.QueueEmpty:
                        break
            self._subscribers.clear()
