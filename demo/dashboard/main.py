"""FastAPI dashboard app -- web UI for visualizing AgentAuth demos.

Usage:
    python -m dashboard.main
    python -m dashboard.main --port 8070
"""

from __future__ import annotations

import argparse
import asyncio
import logging
from dataclasses import asdict
from pathlib import Path

import uvicorn
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates
from starlette.requests import Request

from dashboard.routes import router
from dashboard.state import DashboardState

logger = logging.getLogger(__name__)

_BASE_DIR = Path(__file__).resolve().parent
_TEMPLATE_DIR = _BASE_DIR / "templates"
_STATIC_DIR = _BASE_DIR / "static"


def _sanitize_error(exc: Exception) -> str:
    """Return a safe error string that never leaks URLs or tokens."""
    return type(exc).__name__


async def _default_demo_runner(state: DashboardState, mode: str) -> None:
    """Run demo orchestrator then attack simulator, publishing events."""
    from resource_server.middleware import ServerMode

    server_mode = ServerMode.secure if mode == "secure" else ServerMode.insecure

    # Publish start event.
    await state.publish({
        "type": "status",
        "data": {"message": "Demo started", "mode": mode},
    })

    # -- Demo orchestrator --
    try:
        from agents.orchestrator import run_demo

        demo_result = await run_demo(mode=server_mode)
        state.demo_result = asdict(demo_result)

        for agent in demo_result.agents:
            await state.publish({
                "type": "agent_event",
                "data": {
                    "agent_name": agent.agent_name,
                    "success": agent.success,
                    "elapsed_ms": agent.elapsed_ms,
                    "detail": agent.detail,
                },
            })
    except (ValueError, RuntimeError, OSError) as exc:
        logger.error("Demo run failed: %s", _sanitize_error(exc))
        await state.publish({
            "type": "status",
            "data": {"message": "Demo failed", "error": _sanitize_error(exc)},
        })
        state.running = False
        return

    # -- Attack simulator --
    await state.publish({
        "type": "status",
        "data": {"message": "Running attacks", "mode": mode},
    })

    try:
        from attacks.simulator import run_all_attacks

        sim_result = await run_all_attacks(mode=mode)
        state.attack_results = [asdict(r) for r in sim_result.results]

        for attack in sim_result.results:
            await state.publish({
                "type": "attack_result",
                "data": {
                    "name": attack.name,
                    "attack_succeeded": attack.attack_succeeded,
                    "attempts": attack.attempts,
                    "successes": attack.successes,
                    "blocked": attack.blocked,
                    "details": attack.details,
                },
            })
    except (ValueError, RuntimeError, OSError) as exc:
        logger.error("Attack simulation failed: %s", _sanitize_error(exc))
        await state.publish({
            "type": "status",
            "data": {"message": "Attacks failed", "error": _sanitize_error(exc)},
        })

    # -- Done --
    state.running = False
    await state.publish({
        "type": "status",
        "data": {"message": "Demo complete", "mode": mode},
    })


def create_app(
    broker_url: str | None = None,
    resource_url: str | None = None,
    http_client=None,
    demo_runner=None,
) -> FastAPI:
    """Build and return the dashboard FastAPI application.

    Args:
        broker_url: Override broker URL (unused directly; demo sub-components
                    pick up their own defaults).
        resource_url: Override resource server URL.
        http_client: Optional httpx client for testing.
        demo_runner: Optional coroutine ``(state, mode) -> None`` that drives
                     the demo. Defaults to the real orchestrator + attack sim.
    """
    app = FastAPI(
        title="AgentAuth Demo Dashboard",
        version="0.1.0",
        description="Web-based dashboard for visualizing AgentAuth demos.",
    )

    # Shared mutable state.
    app.state.dashboard = DashboardState()
    app.state.demo_runner = demo_runner or _default_demo_runner

    # Template engine.
    templates = Jinja2Templates(directory=str(_TEMPLATE_DIR))
    app.state.templates = templates

    # Static files (CSS, JS).
    if _STATIC_DIR.is_dir():
        app.mount("/static", StaticFiles(directory=str(_STATIC_DIR)), name="static")

    # Routes.
    app.include_router(router)

    # Root -- serve dashboard HTML.
    @app.get("/")
    async def index(request: Request):
        return templates.TemplateResponse("index.html", {"request": request})

    # Health.
    @app.get("/health")
    def health() -> dict:
        return {"status": "healthy", "service": "dashboard"}

    return app


def main() -> None:
    """CLI entrypoint -- parse args and run uvicorn."""
    parser = argparse.ArgumentParser(description="AgentAuth Demo Dashboard")
    parser.add_argument("--port", type=int, default=8070, help="Port to listen on")
    parser.add_argument("--host", default="0.0.0.0", help="Host to bind to")
    args = parser.parse_args()

    app = create_app()
    uvicorn.run(app, host=args.host, port=args.port, log_level="info")


if __name__ == "__main__":
    main()
