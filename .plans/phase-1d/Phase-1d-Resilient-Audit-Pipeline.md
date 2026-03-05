# Phase 1d: Resilient Audit Pipeline

**Status:** Spec
**Priority:** P1 — pattern compliance, operational resilience
**Effort estimate:** 1 day
**Depends on:** Phase 1c (audit hardening, `VerifyChain`, new audit fields)
**Architecture doc:** `../.plans/CoWork-Architecture-Direct-Broker.md`
**Pattern requirement:** Ephemeral Agent Credentialing v1.2, Component 5 — "Implement local event queuing when the central logging service is unavailable, then replay them when connectivity is restored. Operations never fail solely due to logging unavailability."
**NIST analysis:** `../.plans/nist-submission/NIST-Recommendations-vs-Implementation.md` (item 42)

---

## Overview: What We're Building and Why

The Ephemeral Agent Credentialing pattern v1.2 requires that agent operations never fail solely because the audit logging service is unavailable. Today, AgentAuth writes audit events inline with request handling — the `AuditLog.Record()` call happens synchronously during each operation. If the SQLite write fails, the behavior is undefined: the operation may succeed without an audit record, or it may fail entirely depending on where the error is handled.

This is a pattern violation. The pattern explicitly states: "Implement local event queuing when the central logging service is unavailable, then replay them when connectivity is restored."

**Phase 1d makes the audit pipeline resilient.** It decouples event creation from event persistence by introducing a local event queue between the handler and the store. When the store is healthy, events flow through immediately (no perceptible latency change). When the store is unavailable, events queue locally and replay automatically when connectivity is restored. Agent operations never block on audit writes.

This is a separate phase from Phase 1c because it changes HOW events are written (the write pipeline architecture), not WHAT gets written (fields, queries, integrity). Phase 1c adds `original_principal`, `intermediate_agents`, `VerifyChain`, and read-only access — all of which change the audit data model and query surface. Phase 1d changes the plumbing underneath without touching the data model.

**What changes:** Audit event recording becomes asynchronous via a bounded in-memory queue. A background worker drains the queue to SQLite. Queue overflow triggers a local file fallback. A replay mechanism processes the fallback file on recovery.

**What stays the same:** All audit event fields unchanged. All query endpoints unchanged. Hash chain integrity unchanged — events are still hash-chained in order, just potentially with a small delay between creation and persistence. `VerifyChain` (Phase 1c) works identically.

---

## Problem Statement

AgentAuth's audit pipeline has a single point of failure: the SQLite write. If the database file is locked, the disk is full, or the write fails for any reason, audit events are silently lost or operations fail. The pattern requires that operations continue and events are buffered for later persistence.

This matters in production: container restarts, volume issues, SQLite lock contention under high concurrency, and disk pressure are all realistic failure modes. An audit system that only works when everything is healthy provides false confidence.

---

## Goals

1. Agent operations (registration, token issuance, delegation, revocation) never fail because the audit store is unavailable
2. Audit events are buffered in a bounded in-memory queue when the store is unavailable
3. When the queue reaches capacity, events overflow to a local file (not lost)
4. When the store recovers, buffered and overflowed events are replayed in order
5. The hash chain remains valid after replay — events are chained in creation order, not persistence order
6. Operators can monitor queue depth and overflow status

---

## Non-Goals

1. **External logging service integration** — Splunk, Elasticsearch, CloudWatch forwarding (future spec)
2. **Distributed audit log** — multi-broker audit aggregation (future spec)
3. **Guaranteed delivery with acknowledgment** — we accept that a hard crash during queue drain may lose in-flight events. The file fallback mitigates this for store-down scenarios, but a process kill during the queue→file write is an accepted edge case
4. **Changing the audit data model** — Phase 1c handles fields and queries, Phase 1d handles the write path only

---

## User Stories

### Operator Stories

1. **As an operator**, I want agent operations to continue working when the audit database is temporarily unavailable so that a disk issue or SQLite lock doesn't cause a system-wide outage.

2. **As an operator**, I want audit events to be buffered and replayed automatically when the database recovers so that I don't lose audit trail coverage during an outage.

3. **As an operator**, I want to see the current audit queue depth and overflow status so that I can detect when the audit pipeline is degraded before it becomes critical.

4. **As an operator**, I want `aactl audit status` to show pipeline health: queue depth, overflow file size, last successful write, and last error.

### Security Stories

5. **As a security reviewer**, I want the audit hash chain to remain valid after events are replayed from a queue or overflow file so that the tamper-proof property is preserved regardless of write timing.

6. **As a security reviewer**, I want overflow events written to a local file (not just held in memory) so that a process restart during a store outage doesn't lose buffered audit events.

7. **As a security reviewer**, I want the replay mechanism to detect and skip duplicate events (by event ID) so that a partial replay + recovery doesn't create duplicate entries in the audit log.

---

## What Needs to Be Done

### 1. Bounded In-Memory Event Queue

Replace the synchronous `AuditLog.Record()` call path with a channel-based queue:

```go
type AuditPipeline struct {
    queue     chan *AuditEvent  // bounded channel, default capacity 10,000
    store     AuditStore       // the SQLite-backed store
    overflow  *OverflowFile    // file-based fallback
    hashChain *HashChain       // maintains hash chain state
}
```

`Record()` becomes non-blocking: it assigns the event an ID and hash (maintaining chain order), then sends it to the channel. If the channel is full, it writes to the overflow file. It never blocks the caller.

Configuration: `AA_AUDIT_QUEUE_SIZE` (default 10000), `AA_AUDIT_OVERFLOW_PATH` (default `./data/audit-overflow.jsonl`).

### 2. Background Drain Worker

A single goroutine drains the queue to the SQLite store. On write failure:

- Log the error
- Move the event to the overflow file
- Continue draining (don't block on a single failed write)
- Track consecutive failures — after 10 consecutive failures, stop attempting writes and drain entirely to overflow
- Retry store writes every 5 seconds when in overflow mode

### 3. Overflow File

A JSONL (JSON Lines) file where each line is a serialized `AuditEvent`. Append-only. Events include their pre-computed hash chain values so replay preserves chain integrity.

The overflow file is:
- Created on first overflow event
- Appended to during store outage
- Replayed on recovery (store becomes writable again)
- Renamed to `.replayed` after successful replay (not deleted — evidence of the outage)

### 4. Replay Mechanism

When the drain worker detects the store is writable again (successful write after a failure period):

1. Pause queue draining
2. Open the overflow file
3. Read events in order
4. For each event, check if the event ID already exists in the store (skip duplicates)
5. Write to store
6. After all events are replayed, rename overflow file to `audit-overflow-{timestamp}.replayed`
7. Resume queue draining

### 5. Hash Chain Ordering

The hash chain must be computed at event creation time (in `Record()`), not at persistence time. This means the `AuditPipeline` holds the `prevHash` state and assigns hashes synchronously before enqueuing. The queue and overflow file carry pre-computed hashes. This ensures the chain is ordered by creation time regardless of when events are persisted.

### 6. Health Endpoint and CLI

Expose pipeline health via `GET /v1/audit/status`:

```json
{
  "queue_depth": 42,
  "queue_capacity": 10000,
  "overflow_active": false,
  "overflow_file_size_bytes": 0,
  "last_successful_write": "2026-03-05T12:00:00Z",
  "last_error": null,
  "consecutive_failures": 0
}
```

CLI: `aactl audit status`

---

## Success Criteria

- Agent registration succeeds when SQLite is temporarily unavailable (file locked or disk full)
- Audit events are queued and persisted once SQLite recovers
- The hash chain is valid after replay (`GET /v1/audit/verify` passes)
- No duplicate events after replay
- Overflow file is created when queue is full
- Overflow file events are replayed in order on recovery
- `aactl audit status` shows queue depth and overflow status
- Under normal operation (store healthy), latency impact is negligible (<1ms added)
- Existing audit query endpoints work identically — the change is in the write path only

---

## Testing Workflow

> **Before writing any test code**, extract the user stories from the `## User Stories` section above into a standalone file:
> `tests/phase-1d/user-stories.md`
>
> This is required by the project workflow (CLAUDE.md). The coding agent writes user stories first, saves them to `tests/`, then writes test code against them. Do not skip this step.
