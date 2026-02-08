# Observability Module (M08)

## Purpose

The observability module provides two broker-wide capabilities:
- A centralized RFC 7807 problem response factory for consistent API error payloads.
- Prometheus metrics primitives and endpoint exposure for runtime visibility.

This module standardizes how failures are emitted and how operational behavior is measured.

## Design decisions

### Centralized RFC 7807 writer

`internal/obs.WriteProblem` is the single writer for `application/problem+json` payloads. Handlers and middleware pass only status/type/title and avoid duplicating JSON response logic.

### Process-global Prometheus collectors

Metrics are registered once via `RegisterMetrics()` and reused through helper functions:
- `RecordIssuance`
- `RecordValidation`
- `SetRevocationCacheHitRatio`
- `RecordClockSkew`
- `RecordDelegationDepth`
- `RecordAnomalyRevocation`
- `SetHeartbeatMissRate`

The one-time registration model avoids duplicate collector panics and keeps handler wiring simple.

Current production call sites:
- `RecordIssuance` in `TknSvc.Issue`
- `RecordValidation` in `ValHdl` and `ValMw`
- `SetRevocationCacheHitRatio` in `RevSvc.IsRevoked` (rolling hit ratio)
- `RecordClockSkew` in `TknSvc.Verify` when skew tolerance is used
- `RecordDelegationDepth` in `DelegSvc.Delegate`
- `RecordAnomalyRevocation` in `HeartbeatMgr.sweep` on auto-revocation
- `SetHeartbeatMissRate` in `HeartbeatMgr.sweep`

### Health status classification

`HealthHdl` reports:
- broker status (`healthy`, `degraded`, `unhealthy`)
- version
- uptime seconds
- component map (`sqlite`, `redis`)

HTTP status behavior:
- `200` when broker status is `healthy`
- `503` when broker status is `degraded` or `unhealthy`

Current behavior:
- sqlite is required (`nil` store => `unhealthy`)
- redis is optional for current broker topology (`nil`/empty config => treated healthy)
- configured redis endpoint is probed with a short PING exchange

### Metrics endpoint behavior

`MetricsHdl` exposes Prometheus text format at `GET /v1/metrics` using `promhttp.Handler()`. Non-GET methods are rejected with `405`.

## Metrics inventory

- `aa_token_issuance_duration_ms` (histogram)
- `aa_validation_decision_total{decision="allow|deny"}` (counter vec)
- `aa_revocation_cache_hit_ratio` (gauge)
- `aa_clock_skew_detected_total` (counter)
- `aa_delegation_chain_depth` (histogram)
- `aa_anomaly_revocation_triggered_total` (counter)
- `aa_heartbeat_miss_rate` (gauge)
