# Audit Module (M05)

## Purpose

The audit module provides an immutable audit trail for all security-relevant broker operations. Every credential issuance, access decision, and revocation is recorded as a hash-chained event, enabling tamper detection and forensic analysis.

## Design decisions

**In-memory over SQLite**: The audit store uses an in-memory `[]AuditEvt` with `sync.RWMutex`, consistent with the project's zero-dependency pattern. A single-process broker with no external storage keeps deployment simple. Migration to durable storage (SQLite, PostgreSQL) requires only replacing the storage layer inside `AuditLog`.

**Inject over AuditMw**: Audit events are emitted directly from domain components (`ValMw`, `RevokeHdl`, `RegHdl`) rather than through a generic HTTP middleware. Reason: audit events need domain-specific fields (agent_id, scope, denial_reason, delegation depth) that a middleware cannot capture without duplicating request parsing logic. The trade-off is slightly more coupling, but the events are richer and more accurate.

**SHA-256 hash chain**: Each event stores `prev_hash` (previous event's hash) and `event_hash` (SHA-256 of all fields + prev_hash). Pipe-delimited concatenation is deterministic without canonical JSON. Tampering with any field invalidates the hash and all subsequent events in the chain.

**Optional audit injection**: All constructors accept `*audit.AuditLog` as a nilable parameter. When nil, no audit events are emitted. This keeps unit tests fast and decoupled from the audit subsystem.

## Event schema

| Field | Type | Description |
|-------|------|-------------|
| `event_id` | string | 16-byte crypto/rand hex identifier |
| `event_type` | string | One of: `credential_issued`, `access_granted`, `access_denied`, `token_revoked`, `delegation_created`, `delegation_revoked`, `anomaly_detected` |
| `timestamp` | string | ISO 8601 UTC |
| `agent_instance_id` | string | SPIFFE ID of the agent |
| `task_id` | string | Task identifier |
| `orchestration_id` | string | Orchestration identifier |
| `resource` | string | Resource path or identifier |
| `action` | string | HTTP method or action description |
| `outcome` | string | Result: `granted`, `denied`, `revoked`, `issued` |
| `denial_reason` | string | Reason for denial (omitted if not denied) |
| `delegation_depth` | int | Depth in delegation chain |
| `delegation_chain_hash` | string | SHA-256 hash of delegation chain |
| `prev_hash` | string | Hash of previous event in chain |
| `event_hash` | string | SHA-256 hash of this event |

## Sanitization rules

Before storage, events are sanitized:
- **Email addresses** are replaced with `[REDACTED_EMAIL]`
- **Phone numbers** are replaced with `[REDACTED_PHONE]`
- **Customer IDs** (6+ digit numeric patterns) are hashed with SHA-256 to `[CID:<hex>]`, preserving forensic linking across events

Sanitization targets the `Resource` and `Action` fields where PII may appear.

## Read aggregation

`AggregateReads` merges consecutive `access_granted` read events for the same agent+resource into summary events (e.g., `read (x100)`). Write events and failures are never aggregated.

## Querying

`GET /v1/audit/events` supports:

| Parameter | Description |
|-----------|-------------|
| `agent_id` | Filter by agent SPIFFE ID |
| `task_id` | Filter by task identifier |
| `orchestration_id` | Filter by orchestration ID |
| `event_type` | Filter by event type |
| `from` | ISO 8601 lower bound (inclusive) |
| `to` | ISO 8601 upper bound (inclusive) |
| `limit` | Page size (default 100, max 1000) |
| `offset` | Pagination offset |

Response includes `events`, `total`, and `next_offset` for cursor-based pagination.
