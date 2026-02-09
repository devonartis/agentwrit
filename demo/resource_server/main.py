"""FastAPI resource server — simulated customer database for AgentAuth demos.

Usage:
    python -m resource_server.main                        # default: secure mode
    python -m resource_server.main --mode insecure        # insecure mode
    RESOURCE_SERVER_MODE=insecure python -m resource_server.main
"""

from __future__ import annotations

import argparse
import os

import uvicorn
from fastapi import FastAPI
from fastapi.responses import JSONResponse

from resource_server.middleware import ServerMode
from resource_server.routes import router


def create_app(mode: ServerMode | None = None) -> FastAPI:
    """Build and return the FastAPI application.

    Args:
        mode: Operating mode. If None, reads from RESOURCE_SERVER_MODE env var
              (default "secure").
    """
    if mode is None:
        raw = os.environ.get("RESOURCE_SERVER_MODE", "secure").lower()
        mode = ServerMode(raw)

    app = FastAPI(
        title="AgentAuth Demo Resource Server",
        version="0.1.0",
        description="Simulated customer database API for AgentAuth demos.",
    )
    app.state.mode = mode

    app.include_router(router)

    @app.get("/health")
    def health() -> dict:
        """Health check endpoint."""
        return {"status": "healthy", "mode": mode.value}

    return app


def main() -> None:
    """CLI entrypoint — parse args and run uvicorn."""
    parser = argparse.ArgumentParser(description="AgentAuth Demo Resource Server")
    parser.add_argument(
        "--mode",
        choices=["secure", "insecure"],
        default=None,
        help="Operating mode (default: RESOURCE_SERVER_MODE env var or 'secure')",
    )
    parser.add_argument("--port", type=int, default=8090, help="Port to listen on")
    parser.add_argument("--host", default="0.0.0.0", help="Host to bind to")
    args = parser.parse_args()

    mode = ServerMode(args.mode) if args.mode else None
    app = create_app(mode=mode)
    uvicorn.run(app, host=args.host, port=args.port, log_level="info")


if __name__ == "__main__":
    main()
