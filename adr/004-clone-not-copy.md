# Decision 004: Clone from agentauth-internal, Not Copy

**Date:** 2026-03-29
**Status:** Final

## Decision

`git clone agentauth-internal agentauth-core` to preserve the full 412-commit incremental history. Then reset to the fork point.

## Why

The original `agentauth` repo was a file copy that lost all commit history. That made it impossible to understand why code was written a certain way, who changed what, or when bugs were introduced. History is not optional for a security project — reviewers need `git blame` to trace decisions.

## What it replaced

File copy (which is what created the original `agentauth` repo and caused the history loss in the first place).
