# Sidecar Observability Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace all 15 raw `fmt.Printf/Println` calls in the sidecar with structured `obs` logging, add Prometheus metrics, and enhance the health endpoint — aligned with the security pattern's Component 5 (Immutable Audit Logging).

**Architecture:** Reuse the broker's `internal/obs` package for structured logging only. Sidecar Prometheus metrics live in a NEW `cmd/sidecar/metrics.go` — each binary owns its own metrics. The sidecar uses module string `"SIDECAR"` with component-level granularity (`MAIN`, `BOOTSTRAP`, `RENEWAL`, `REGISTRY`, `TOKEN`, `HEALTH`).

**Tech Stack:** Go 1.24, `internal/obs` package, `prometheus/client_golang` (already a dependency)

---

## Context

### Current State

- 15 raw `fmt.Printf/Println` calls across 5 sidecar files
- `AA_SIDECAR_LOG_LEVEL` loaded in `config.go` but never wired to anything
- Zero Prometheus metrics in sidecar (broker has 8)
- Security pattern Component 5 requires structured, correlated audit logging

### Three Audiences

| Audience | Needs |
|----------|-------|
| Coding agents | Clear error responses (already handled via HTTP status + JSON) |
| Developers | Structured logs for local debugging, health endpoint details |
| Admins | Prometheus metrics, log levels, audit events |

### Why Approach A (Direct Reuse)

- `internal/obs` is already accessible to `cmd/sidecar/` (same Go module)
- Identical log format: `[AA:SIDECAR:OK] 2026-02-14T... | BOOTSTRAP | broker ready`
- Single `obs.Configure()` call wires `AA_SIDECAR_LOG_LEVEL`
- Zero code duplication, no format drift
- Sidecar Prometheus metrics live in `cmd/sidecar/metrics.go` (not in `obs`) — each binary owns its own metrics

---

## Design

### 1. Structured Logging

Wire `obs.Configure(cfg.LogLevel)` at sidecar startup. Replace every `fmt.Printf` with the appropriate `obs` call.

**Component names:** `MAIN`, `BOOTSTRAP`, `RENEWAL`, `REGISTRY`, `TOKEN`, `HEALTH`

| Current call | File | Replacement | Level |
|---|---|---|---|
| `fmt.Printf("[sidecar] starting...")` | main.go:27 | `obs.Ok("SIDECAR", "MAIN", "starting", ...)` | OK |
| `fmt.Fprintln(os.Stderr, "FATAL: AA_ADMIN_SECRET...")` | main.go:19 | `obs.Fail("SIDECAR", "MAIN", "AA_ADMIN_SECRET must be set")` | FAIL |
| `fmt.Fprintln(os.Stderr, "FATAL: AA_SIDECAR_SCOPE_CEILING...")` | main.go:23 | `obs.Fail("SIDECAR", "MAIN", "AA_SIDECAR_SCOPE_CEILING must be set")` | FAIL |
| `fmt.Fprintf(os.Stderr, "FATAL: bootstrap failed...")` | main.go:31 | `obs.Fail("SIDECAR", "MAIN", "bootstrap failed", err.Error())` | FAIL |
| `fmt.Printf("[sidecar] renewal goroutine started...")` | main.go:39 | `obs.Ok("SIDECAR", "MAIN", "renewal goroutine started", ...)` | OK |
| `fmt.Printf("[sidecar] ready on %s...")` | main.go:53 | `obs.Ok("SIDECAR", "MAIN", "ready", "addr="+addr, ...)` | OK |
| `fmt.Println("[sidecar] shutting down...")` | main.go:60 | `obs.Ok("SIDECAR", "MAIN", "shutting down")` | OK |
| `fmt.Fprintf(os.Stderr, "FATAL: %v...")` | main.go:65 | `obs.Fail("SIDECAR", "MAIN", "listen failed", err.Error())` | FAIL |
| `fmt.Println("[sidecar] broker is ready")` | bootstrap.go:91 | `obs.Ok("SIDECAR", "BOOTSTRAP", "broker ready")` | OK |
| `fmt.Println("[sidecar] admin authenticated")` | bootstrap.go:98 | `obs.Ok("SIDECAR", "BOOTSTRAP", "admin authenticated")` | OK |
| `fmt.Println("[sidecar] activation token created")` | bootstrap.go:105 | `obs.Ok("SIDECAR", "BOOTSTRAP", "activation token created")` | OK |
| `fmt.Println("[sidecar] sidecar activated")` | bootstrap.go:112 | `obs.Ok("SIDECAR", "BOOTSTRAP", "sidecar activated")` | OK |
| `fmt.Printf("[sidecar] renewal failed...")` | renewal.go:42 | `obs.Warn("SIDECAR", "RENEWAL", "renewal failed", ...)` | WARN |
| `fmt.Println("[sidecar] token expired...")` | renewal.go:46 | `obs.Warn("SIDECAR", "RENEWAL", "token expired, marking unhealthy")` | WARN |
| `fmt.Printf("[sidecar] token renewed...")` | renewal.go:65 | `obs.Trace("SIDECAR", "RENEWAL", "token renewed", ...)` | TRACE |
| `fmt.Printf("[sidecar] lazy-registered agent...")` | handler.go:184 | `obs.Ok("SIDECAR", "REGISTRY", "agent registered", ...)` | OK |
| `fmt.Printf("[sidecar] BYOK registered agent...")` | register_handler.go:157 | `obs.Ok("SIDECAR", "REGISTRY", "BYOK agent registered", ...)` | OK |

### 2. Sidecar Prometheus Metrics

NEW file `cmd/sidecar/metrics.go` — sidecar owns its own metrics (not monolithic `obs`):

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| `agentauth_sidecar_bootstrap_total` | CounterVec | `status` | Bootstrap attempts (success/failure) |
| `agentauth_sidecar_renewals_total` | CounterVec | `status` | Token renewal outcomes (success/failure) |
| `agentauth_sidecar_token_exchanges_total` | CounterVec | `status` | Agent token exchanges via sidecar |
| `agentauth_sidecar_scope_denials_total` | Counter | — | Scope ceiling enforcement denials |
| `agentauth_sidecar_agents_registered` | Gauge | — | Currently registered agents in memory |
| `agentauth_sidecar_request_duration_seconds` | HistogramVec | `endpoint` | Per-endpoint latency |

### 3. Metrics Endpoint

Add `GET /v1/metrics` to sidecar mux using `promhttp.Handler()` — same pattern as broker's `cmd/broker/main.go`.

### 4. Health Endpoint Enhancement

Current health response already includes `status`, `sidecar_id`, `scope_ceiling`. Add:

- `agents_registered` (int) — count from agent registry
- `last_renewal` (RFC3339 timestamp) — from sidecar state
- `uptime_seconds` (float64) — time since startup

### 5. Audit Event Logging (Security Pattern Alignment)

The sidecar doesn't maintain its own audit store — the broker owns audit. But sidecar structured logs serve as a correlated local audit trail.

Key events logged at OK level (always visible unless `quiet`):
- Bootstrap complete (with sidecar_id)
- Agent registered (lazy or BYOK, with agent_id)
- Token exchanged (with agent_id, scope)
- Renewal success

Key events logged at WARN level:
- Scope ceiling denial (with requested scope, ceiling)
- Renewal failure (with error, retry backoff)
- Token expired

---

## Files to Touch

| File | Change |
|------|--------|
| `cmd/sidecar/metrics.go` | **NEW**: All 6 sidecar Prometheus metrics + convenience helpers |
| `cmd/sidecar/main.go` | Wire `obs.Configure()`, replace 8 fmt calls, add `/v1/metrics` route |
| `cmd/sidecar/bootstrap.go` | Replace 4 fmt calls, add `lastRenewal`/`startTime` to state, increment bootstrap metric |
| `cmd/sidecar/renewal.go` | Replace 3 fmt calls, increment renewal metric, update lastRenewal |
| `cmd/sidecar/handler.go` | Replace 1 fmt call, add exchange/denial metric calls, enhance health response |
| `cmd/sidecar/register_handler.go` | Replace 1 fmt call, increment agents_registered gauge |
| `cmd/sidecar/registry.go` | Add `count()` method |

## What This Does NOT Include (YAGNI)

- No separate sidecar audit log store (broker owns audit)
- No anomaly detection (pattern says "optional but recommended" — defer)
- No log rotation (handled by container runtime / systemd)
- No custom log format — reuses broker's proven format exactly
- No request-ID middleware for sidecar (defer to Phase 3 if needed)
