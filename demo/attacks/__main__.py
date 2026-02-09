"""CLI entrypoint: python -m attacks [--mode secure|insecure] [--broker-url] [--resource-url]."""

from __future__ import annotations

import argparse
import asyncio
import sys

from attacks.simulator import SimulatorResult, run_all_attacks


# -- ANSI colors ------------------------------------------------------------

_GREEN = "\033[32m"
_RED = "\033[31m"
_YELLOW = "\033[33m"
_BOLD = "\033[1m"
_RESET = "\033[0m"


def _color(text: str, code: str) -> str:
    return f"{code}{text}{_RESET}"


# -- output formatting ------------------------------------------------------


def _print_results(sim: SimulatorResult) -> None:
    """Print a formatted table of attack results."""
    mode_label = _color(sim.mode.upper(), _BOLD)
    print(f"\n{'='*60}")
    print(f" Attack Simulator Results  --  mode: {mode_label}")
    print(f"{'='*60}\n")

    header = f"{'Attack':<25} {'Attempts':>8} {'Success':>8} {'Blocked':>8}  {'Verdict'}"
    print(header)
    print("-" * len(header.expandtabs()))

    for r in sim.results:
        if sim.mode == "insecure":
            # In insecure mode, attack succeeding is the EXPECTED (gap demo)
            verdict = _color("EXPLOITED", _RED) if r.attack_succeeded else _color("SAFE", _GREEN)
        else:
            # In secure mode, attack blocked is the EXPECTED (fix demo)
            verdict = _color("BLOCKED", _GREEN) if not r.attack_succeeded else _color("EXPLOITED", _RED)

        print(
            f"{r.name:<25} {r.attempts:>8} {r.successes:>8} {r.blocked:>8}  {verdict}"
        )

    print()
    if sim.meets_expectation:
        msg = "All outcomes match expected demo story."
        print(_color(f"  PASS: {msg}", _GREEN))
    else:
        msg = "Some outcomes deviate from expected demo story!"
        print(_color(f"  FAIL: {msg}", _RED))
    print()


# -- CLI --------------------------------------------------------------------


def main() -> int:
    """Parse args and run the attack simulator."""
    parser = argparse.ArgumentParser(
        description="AgentAuth Attack Simulator -- gap vs. fix demo"
    )
    parser.add_argument(
        "--mode",
        choices=["secure", "insecure"],
        default="insecure",
        help="Resource server mode (default: insecure)",
    )
    parser.add_argument(
        "--broker-url",
        default="http://localhost:8080",
        help="Broker base URL (default: http://localhost:8080)",
    )
    parser.add_argument(
        "--resource-url",
        default="http://localhost:8090",
        help="Resource server base URL (default: http://localhost:8090)",
    )
    parser.add_argument(
        "--stolen-credential",
        default="stolen-cred",
        help="Credential for theft attack",
    )
    parser.add_argument(
        "--agent-c-token",
        default="agent-c-token",
        help="Agent C token for escalation attack",
    )
    parser.add_argument(
        "--admin-token",
        default=None,
        help="Admin token for audit queries (secure mode)",
    )
    parser.add_argument(
        "--shared-api-key",
        default="shared-api-key",
        help="Shared API key for insecure mode",
    )
    args = parser.parse_args()

    sim = asyncio.run(
        run_all_attacks(
            mode=args.mode,
            broker_url=args.broker_url,
            resource_url=args.resource_url,
            stolen_credential=args.stolen_credential,
            agent_c_token=args.agent_c_token,
            admin_token=args.admin_token,
            shared_api_key=args.shared_api_key,
        )
    )

    _print_results(sim)
    return 0 if sim.meets_expectation else 1


if __name__ == "__main__":
    sys.exit(main())
