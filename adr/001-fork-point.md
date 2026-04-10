# Decision 001: Fork Point is 2c5194e (TD-006)

**Date:** 2026-03-29
**Status:** Final

## Decision

Fork agentauth-core from commit `2c5194e` (TD-006: Per-App JWT TTL), not `3f9639f` (Phase 1C-alpha).

## Why

Phase 1C-alpha bakes `hitl_scopes` into the app data model — store, service, handler, CLI (4 source files, 3 tests). TD-006 has identical core + app functionality with zero HITL references. Verified with `grep -ri "hitl|approval" internal/ cmd/` returning nothing at `2c5194e`.

Starting from a clean fork avoids contamination removal. Every HITL reference that leaks into core is a line we have to find, understand, and delete — and a risk we miss one.

## What it replaced

The original plan was to fork from the latest commit and strip HITL. Starting clean was less work and less risk.
