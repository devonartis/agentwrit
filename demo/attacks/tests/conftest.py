"""Shared fixtures for attack simulator tests."""

from __future__ import annotations

import sys
from pathlib import Path

# Ensure demo/ is on sys.path so `from attacks.xxx` imports work.
_demo_dir = str(Path(__file__).resolve().parent.parent.parent)
if _demo_dir not in sys.path:
    sys.path.insert(0, _demo_dir)
