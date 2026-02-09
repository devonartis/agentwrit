"""Dashboard route handlers: SSE stream, demo control, and status endpoints."""

from __future__ import annotations

import asyncio
import json
import logging
from dataclasses import asdict

from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse, StreamingResponse

from dashboard.state import DashboardState

logger = logging.getLogger(__name__)

router = APIRouter()


def _sanitize_error(exc: Exception) -> str:
    """Return a safe error string that never leaks URLs or tokens."""
    return type(exc).__name__


# ---------------------------------------------------------------------------
# SSE stream
# ---------------------------------------------------------------------------


async def _event_generator(state: DashboardState):
    """Yield SSE-formatted events from a subscriber queue."""
    queue = await state.subscribe()
    try:
        while True:
            try:
                event = await asyncio.wait_for(queue.get(), timeout=30.0)
                yield f"data: {json.dumps(event)}\n\n"
            except asyncio.TimeoutError:
                # Send keep-alive comment to prevent connection timeout.
                yield ": keepalive\n\n"
    except asyncio.CancelledError:
        pass
    finally:
        await state.unsubscribe(queue)


@router.get("/events/stream")
async def sse_stream(request: Request):
    """Server-Sent Events endpoint for real-time dashboard updates."""
    state: DashboardState = request.app.state.dashboard
    return StreamingResponse(
        _event_generator(state),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )


# ---------------------------------------------------------------------------
# Demo control
# ---------------------------------------------------------------------------


@router.post("/demo/run")
async def run_demo_endpoint(request: Request):
    """Start a demo+attack run as a background task."""
    state: DashboardState = request.app.state.dashboard
    if state.running:
        return JSONResponse(
            status_code=409,
            content={"status": "error", "detail": "Demo already running"},
        )

    body = await request.json()
    mode = body.get("mode", "insecure")
    if mode not in ("insecure", "secure"):
        return JSONResponse(
            status_code=400,
            content={"status": "error", "detail": "mode must be 'insecure' or 'secure'"},
        )

    state.running = True
    state.mode = mode

    runner = request.app.state.demo_runner

    async def _safe_runner():
        try:
            await runner(state, mode)
        except Exception as exc:
            logger.error("Runner failed: %s", _sanitize_error(exc))
            await state.publish({"type": "status", "data": {"message": "Demo failed", "error": _sanitize_error(exc)}})
            state.running = False

    state._task = asyncio.create_task(_safe_runner())

    return {"status": "started", "mode": mode}


@router.post("/demo/reset")
async def reset_demo(request: Request):
    """Clear all dashboard state."""
    state: DashboardState = request.app.state.dashboard
    await state.reset()
    return {"status": "reset"}


@router.get("/demo/status")
async def demo_status(request: Request):
    """Return the current dashboard state snapshot."""
    state: DashboardState = request.app.state.dashboard
    return {
        "running": state.running,
        "mode": state.mode,
        "demo_result": state.demo_result,
        "attack_results": state.attack_results,
    }
