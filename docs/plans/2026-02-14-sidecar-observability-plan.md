# Sidecar Observability Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace all 15 raw `fmt.Printf/Println` calls in the sidecar with structured `obs` logging, add Prometheus metrics in a dedicated `metrics.go`, and enhance the health endpoint.

**Architecture:** Reuse `internal/obs` for log format only (`Ok/Warn/Fail/Trace`). Sidecar Prometheus metrics live in a NEW `cmd/sidecar/metrics.go` — each binary owns its own metrics. Health handler gets registry count, last renewal timestamp, and uptime.

**Tech Stack:** Go 1.24, `internal/obs` (logging), `prometheus/client_golang` + `promauto` (metrics)

---

## Pre-Flight

Before starting any task, verify the project builds and tests pass:

```bash
cd /Users/divineartis/proj/agentAuth
go build ./...
go test ./cmd/sidecar/... -count=1
```

Expected: all pass, zero `fmt.Printf` warnings from linter.

---

### Task 1: Create `cmd/sidecar/metrics.go` — Prometheus Metrics

This is the foundation. All other tasks depend on these metric vars existing.

**Files:**
- Create: `cmd/sidecar/metrics.go`
- Create: `cmd/sidecar/metrics_test.go`

**Step 1: Write the test**

Create `cmd/sidecar/metrics_test.go`:

```go
package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestSidecarMetricsRegistered(t *testing.T) {
	// Verify all sidecar metrics are registered with the default registry.
	// promauto registers automatically, so we just verify the vars are non-nil
	// and can be collected without panic.
	metrics := []prometheus.Collector{
		SidecarBootstrapTotal,
		SidecarRenewalsTotal,
		SidecarTokenExchangesTotal,
		SidecarScopeDenialsTotal,
		SidecarAgentsRegistered,
		SidecarRequestDuration,
	}
	for i, m := range metrics {
		if m == nil {
			t.Errorf("metric %d is nil", i)
		}
	}
}

func TestRecordSidecarBootstrap(t *testing.T) {
	// Should not panic.
	RecordBootstrap("success")
	RecordBootstrap("failure")
}

func TestRecordSidecarRenewal(t *testing.T) {
	RecordRenewal("success")
	RecordRenewal("failure")
}

func TestRecordSidecarExchange(t *testing.T) {
	RecordExchange("success")
	RecordExchange("failure")
}

func TestRecordSidecarScopeDenial(t *testing.T) {
	RecordScopeDenial()
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./cmd/sidecar/... -run TestSidecarMetrics -count=1
```

Expected: FAIL — `SidecarBootstrapTotal` undefined.

**Step 3: Write the implementation**

Create `cmd/sidecar/metrics.go`:

```go
package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// Sidecar Prometheus Metrics
//
// Each binary owns its own metrics. Broker metrics live in internal/obs.
// Sidecar metrics live here, prefixed "agentauth_sidecar_".
// ---------------------------------------------------------------------------

// SidecarBootstrapTotal counts bootstrap attempts (success/failure).
var SidecarBootstrapTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_sidecar_bootstrap_total",
	Help: "Total sidecar bootstrap attempts",
}, []string{"status"})

// SidecarRenewalsTotal counts token renewal outcomes (success/failure).
var SidecarRenewalsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_sidecar_renewals_total",
	Help: "Total sidecar token renewal attempts",
}, []string{"status"})

// SidecarTokenExchangesTotal counts agent token exchanges (success/failure).
var SidecarTokenExchangesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_sidecar_token_exchanges_total",
	Help: "Total agent token exchanges via sidecar",
}, []string{"status"})

// SidecarScopeDenialsTotal counts scope ceiling enforcement denials.
var SidecarScopeDenialsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "agentauth_sidecar_scope_denials_total",
	Help: "Total scope ceiling enforcement denials",
})

// SidecarAgentsRegistered tracks the current number of registered agents.
var SidecarAgentsRegistered = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "agentauth_sidecar_agents_registered",
	Help: "Number of currently registered agents in sidecar memory",
})

// SidecarRequestDuration observes per-endpoint HTTP latency in seconds.
var SidecarRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "agentauth_sidecar_request_duration_seconds",
	Help:    "Sidecar HTTP request duration in seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"endpoint"})

// ---------------------------------------------------------------------------
// Convenience helpers — thin wrappers so call sites stay clean.
// ---------------------------------------------------------------------------

// RecordBootstrap increments the bootstrap counter with the given status.
func RecordBootstrap(status string) {
	SidecarBootstrapTotal.WithLabelValues(status).Inc()
}

// RecordRenewal increments the renewal counter with the given status.
func RecordRenewal(status string) {
	SidecarRenewalsTotal.WithLabelValues(status).Inc()
}

// RecordExchange increments the token exchange counter with the given status.
func RecordExchange(status string) {
	SidecarTokenExchangesTotal.WithLabelValues(status).Inc()
}

// RecordScopeDenial increments the scope denial counter.
func RecordScopeDenial() {
	SidecarScopeDenialsTotal.Inc()
}
```

**Step 4: Run tests**

```bash
go test ./cmd/sidecar/... -run TestSidecarMetrics -count=1
go test ./cmd/sidecar/... -run TestRecord -count=1
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add cmd/sidecar/metrics.go cmd/sidecar/metrics_test.go
git commit -m "feat(sidecar): add Prometheus metrics in dedicated metrics.go"
```

---

### Task 2: Add `count()` to Agent Registry

The health endpoint needs to report how many agents are registered. The registry currently has no count method.

**Files:**
- Modify: `cmd/sidecar/registry.go`
- Modify: `cmd/sidecar/registry_test.go`

**Step 1: Write the test**

Add to `cmd/sidecar/registry_test.go`:

```go
func TestAgentRegistry_Count(t *testing.T) {
	reg := newAgentRegistry()

	if got := reg.count(); got != 0 {
		t.Errorf("empty registry count = %d, want 0", got)
	}

	reg.store("agent-a", &agentEntry{spiffeID: "spiffe://test/a"})
	if got := reg.count(); got != 1 {
		t.Errorf("after 1 store, count = %d, want 1", got)
	}

	reg.store("agent-b", &agentEntry{spiffeID: "spiffe://test/b"})
	if got := reg.count(); got != 2 {
		t.Errorf("after 2 stores, count = %d, want 2", got)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./cmd/sidecar/... -run TestAgentRegistry_Count -count=1
```

Expected: FAIL — `reg.count undefined`.

**Step 3: Write the implementation**

Add to `cmd/sidecar/registry.go` after the `store` method:

```go
// count returns the number of registered agents.
func (r *agentRegistry) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}
```

**Step 4: Run tests**

```bash
go test ./cmd/sidecar/... -run TestAgentRegistry -count=1
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add cmd/sidecar/registry.go cmd/sidecar/registry_test.go
git commit -m "feat(sidecar): add count() to agent registry"
```

---

### Task 3: Add `lastRenewal` and `startTime` to Sidecar State

The health endpoint needs last renewal timestamp and uptime. These fields belong on `sidecarState` in `bootstrap.go`.

**Files:**
- Modify: `cmd/sidecar/bootstrap.go` (sidecarState struct and setToken)
- Modify: `cmd/sidecar/state_test.go` (if it exists) or create test

**Step 1: Write the test**

Add to existing `cmd/sidecar/state_test.go` (or create if it doesn't exist):

```go
func TestSidecarState_LastRenewal(t *testing.T) {
	st := &sidecarState{startTime: time.Now()}
	st.setToken("tok1", 300)

	lr := st.getLastRenewal()
	if lr.IsZero() {
		t.Error("lastRenewal should be set after setToken")
	}
	if time.Since(lr) > 1*time.Second {
		t.Error("lastRenewal should be recent")
	}
}

func TestSidecarState_StartTime(t *testing.T) {
	now := time.Now()
	st := &sidecarState{startTime: now}

	if got := st.getStartTime(); !got.Equal(now) {
		t.Errorf("getStartTime() = %v, want %v", got, now)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./cmd/sidecar/... -run TestSidecarState_LastRenewal -count=1
```

Expected: FAIL — `startTime` field or `getLastRenewal` undefined.

**Step 3: Write the implementation**

In `cmd/sidecar/bootstrap.go`, update the `sidecarState` struct and its methods:

Add two fields to the struct:
```go
type sidecarState struct {
	mu           sync.RWMutex
	sidecarToken string
	sidecarID    string
	expiresIn    int
	healthy      bool
	lastRenewal  time.Time
	startTime    time.Time
}
```

Update `setToken` to record renewal time:
```go
func (s *sidecarState) setToken(token string, expiresIn int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sidecarToken = token
	s.expiresIn = expiresIn
	s.healthy = true
	s.lastRenewal = time.Now()
}
```

Add two new getters:
```go
// getLastRenewal returns when the token was last renewed (read-locked).
func (s *sidecarState) getLastRenewal() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastRenewal
}

// getStartTime returns the sidecar start time (immutable after init, but
// read-locked for consistency).
func (s *sidecarState) getStartTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startTime
}
```

In the `bootstrap` function, set `startTime`:
```go
st := &sidecarState{sidecarID: resp.sidecarID, startTime: time.Now()}
```

**Step 4: Run tests**

```bash
go test ./cmd/sidecar/... -run TestSidecarState -count=1
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add cmd/sidecar/bootstrap.go cmd/sidecar/state_test.go
git commit -m "feat(sidecar): add lastRenewal and startTime to sidecar state"
```

---

### Task 4: Wire `obs.Configure` + Replace `fmt` in `main.go`

This is the main wiring task. Replace all 8 `fmt.Printf/Println/Fprintf` calls in `main.go` with `obs` calls, wire the log level config, and add the `/v1/metrics` route.

**Files:**
- Modify: `cmd/sidecar/main.go`

**Step 1: Apply the changes**

The updated `main.go`:

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/your-module/internal/obs"
)
```

Replace imports — keep `"fmt"` and `"os"` for the two `os.Exit` calls (obs.Fail doesn't exit).

Replace each fmt call:

1. Line 15-20 (config validation):
```go
if cfg.AdminSecret == "" {
    obs.Fail("SIDECAR", "MAIN", "AA_ADMIN_SECRET must be set")
    os.Exit(1)
}
if len(cfg.ScopeCeiling) == 0 {
    obs.Fail("SIDECAR", "MAIN", "AA_SIDECAR_SCOPE_CEILING must be set")
    os.Exit(1)
}
```

2. Line 27 (starting):
```go
obs.Ok("SIDECAR", "MAIN", "starting", "broker="+cfg.BrokerURL, "scope_ceiling="+strings.Join(cfg.ScopeCeiling, ","))
```

3. Line 31 (bootstrap failed):
```go
obs.Fail("SIDECAR", "MAIN", "bootstrap failed", err.Error())
os.Exit(1)
```

4. Line 39 (renewal started):
```go
obs.Ok("SIDECAR", "MAIN", "renewal goroutine started", fmt.Sprintf("buffer=%.0f%%", cfg.RenewalBuffer*100))
```

5. Add metrics route in the mux block:
```go
mux.Handle("/v1/metrics", promhttp.Handler())
```

6. Line 53 (ready):
```go
obs.Ok("SIDECAR", "MAIN", "ready", "addr="+addr, "sidecar_id="+state.sidecarID)
```

7. Line 60 (shutting down):
```go
obs.Ok("SIDECAR", "MAIN", "shutting down")
```

8. Line 65 (listen failed):
```go
obs.Fail("SIDECAR", "MAIN", "listen failed", err.Error())
os.Exit(1)
```

Add `obs.Configure(cfg.LogLevel)` as the very first line after `cfg := loadConfig()`:
```go
cfg := loadConfig()
obs.Configure(cfg.LogLevel)
```

**Step 2: Run tests**

```bash
go build ./cmd/sidecar/...
go test ./cmd/sidecar/... -count=1
```

Expected: builds clean, all tests pass. Zero `fmt.Printf` calls remain in main.go (the `fmt.Sprintf` in the renewal buffer message is fine — it's formatting a value, not printing).

**Step 3: Verify no fmt.Print left in main.go**

```bash
grep -n 'fmt\.Print' cmd/sidecar/main.go
```

Expected: no output (or only `fmt.Sprintf` which is acceptable).

**Step 4: Commit**

```bash
git add cmd/sidecar/main.go
git commit -m "feat(sidecar): wire obs.Configure + replace fmt.Print in main.go"
```

---

### Task 5: Replace `fmt` in `bootstrap.go`

Replace all 4 `fmt.Println` calls in the bootstrap function with `obs.Ok` calls. Add bootstrap metric recording.

**Files:**
- Modify: `cmd/sidecar/bootstrap.go`

**Step 1: Apply the changes**

Replace import `"fmt"` with `"github.com/your-module/internal/obs"` (keep `"fmt"` if still needed for `fmt.Errorf`).

Line 91: `fmt.Println("[sidecar] broker is ready")` →
```go
obs.Ok("SIDECAR", "BOOTSTRAP", "broker ready")
```

Line 98: `fmt.Println("[sidecar] admin authenticated")` →
```go
obs.Ok("SIDECAR", "BOOTSTRAP", "admin authenticated")
```

Line 105: `fmt.Println("[sidecar] activation token created")` →
```go
obs.Ok("SIDECAR", "BOOTSTRAP", "activation token created")
```

Line 112: `fmt.Println("[sidecar] sidecar activated")` →
```go
obs.Ok("SIDECAR", "BOOTSTRAP", "sidecar activated", "sidecar_id="+resp.sidecarID)
RecordBootstrap("success")
```

At the top of `bootstrap()`, add failure recording on error return. Wrap the function's error returns to record failure:

After `st.setToken(...)`, before returning, the success metric is already recorded above. For failures, add `RecordBootstrap("failure")` before each error return in `bootstrap()`. Simplest approach — add a single defer:

```go
func bootstrap(bc *brokerClient, cfg sidecarConfig) (st *sidecarState, err error) {
	defer func() {
		if err != nil {
			RecordBootstrap("failure")
		}
	}()
	// ... rest of function unchanged
```

**Step 2: Run tests**

```bash
go build ./cmd/sidecar/...
go test ./cmd/sidecar/... -count=1
```

Expected: all pass.

**Step 3: Verify no fmt.Println left**

```bash
grep -n 'fmt\.Println\|fmt\.Printf' cmd/sidecar/bootstrap.go
```

Expected: no output.

**Step 4: Commit**

```bash
git add cmd/sidecar/bootstrap.go
git commit -m "feat(sidecar): replace fmt.Print in bootstrap.go with obs logging"
```

---

### Task 6: Replace `fmt` in `renewal.go`

Replace all 3 `fmt.Printf/Println` calls with `obs` logging. Add renewal metric recording. Update `lastRenewal` timestamp (already handled by `setToken`).

**Files:**
- Modify: `cmd/sidecar/renewal.go`

**Step 1: Apply the changes**

Replace import `"fmt"` with `"github.com/your-module/internal/obs"` (keep `"fmt"` only if `fmt.Sprintf` is needed — check if it is).

Line 42: `fmt.Printf("[sidecar] renewal failed: %v (retry in %v)\n", err, backoff)` →
```go
obs.Warn("SIDECAR", "RENEWAL", "renewal failed", err.Error(), "retry_in="+backoff.String())
RecordRenewal("failure")
```

Line 46: `fmt.Println("[sidecar] token expired, marking unhealthy")` →
```go
obs.Warn("SIDECAR", "RENEWAL", "token expired, marking unhealthy")
```

Line 65: `fmt.Printf("[sidecar] token renewed, next in %v\n", sleepDur)` →
```go
obs.Trace("SIDECAR", "RENEWAL", "token renewed", "next_in="+sleepDur.String())
RecordRenewal("success")
```

**Step 2: Run tests**

```bash
go build ./cmd/sidecar/...
go test ./cmd/sidecar/... -run TestStartRenewal -count=1
```

Expected: all pass.

**Step 3: Verify no fmt.Print left**

```bash
grep -n 'fmt\.Print' cmd/sidecar/renewal.go
```

Expected: no output.

**Step 4: Commit**

```bash
git add cmd/sidecar/renewal.go
git commit -m "feat(sidecar): replace fmt.Print in renewal.go with obs logging"
```

---

### Task 7: Replace `fmt` in `handler.go` + Add Exchange/Denial Metrics

Replace the 1 `fmt.Printf` call in `lazyRegister`. Add metric increments for token exchange success/failure and scope denial. Enhance the health handler response with `agents_registered`, `last_renewal`, `uptime_seconds`.

**Files:**
- Modify: `cmd/sidecar/handler.go`

**Step 1: Apply the changes**

**In `lazyRegister` (line 184):**

```go
obs.Ok("SIDECAR", "REGISTRY", "agent registered", "agent="+agentName, "agent_id="+agentID)
```

**In `tokenHandler.ServeHTTP`** — add metric increments:

After scope check fails (line 76, the `writeError` for scope ceiling):
```go
if !scopeIsSubset(req.Scope, h.scopeCeiling) {
    RecordScopeDenial()
    obs.Warn("SIDECAR", "TOKEN", "scope ceiling exceeded", "requested="+strings.Join(req.Scope, ","), "ceiling="+strings.Join(h.scopeCeiling, ","))
    writeError(w, http.StatusForbidden, "requested scope exceeds sidecar ceiling")
    return
}
```

After broker exchange fails (line 100):
```go
if err != nil {
    RecordExchange("failure")
    writeError(w, http.StatusBadGateway, "broker token exchange failed: "+err.Error())
    return
}
```

After successful exchange (before writeJSON on line 104):
```go
RecordExchange("success")
```

**In `healthHandler.ServeHTTP`** — enhance the response. The handler needs access to the registry and must be wired with it. Update the struct:

```go
type healthHandler struct {
    state        *sidecarState
    scopeCeiling []string
    registry     *agentRegistry
}

func newHealthHandler(state *sidecarState, ceiling []string, registry *agentRegistry) *healthHandler {
    return &healthHandler{
        state:        state,
        scopeCeiling: ceiling,
        registry:     registry,
    }
}
```

Update `ServeHTTP` response body:

```go
resp := map[string]any{
    "status":           status,
    "broker_connected": connected,
    "healthy":          healthy,
    "scope_ceiling":    h.scopeCeiling,
}

if h.registry != nil {
    resp["agents_registered"] = h.registry.count()
}

if h.state != nil {
    if lr := h.state.getLastRenewal(); !lr.IsZero() {
        resp["last_renewal"] = lr.Format(time.RFC3339)
    }
    if st := h.state.getStartTime(); !st.IsZero() {
        resp["uptime_seconds"] = time.Since(st).Seconds()
    }
}

writeJSON(w, httpStatus, resp)
```

**In `main.go`** — update `newHealthHandler` call to pass registry:

```go
mux.Handle("/v1/health", newHealthHandler(state, cfg.ScopeCeiling, registry))
```

Add `"time"` import to `handler.go` if not already present.

**Step 2: Run tests**

```bash
go build ./cmd/sidecar/...
go test ./cmd/sidecar/... -count=1
```

Expected: all pass. Some tests may need updating if they call `newHealthHandler` with old signature — fix those.

**Step 3: Verify no fmt.Print left in handler.go**

```bash
grep -n 'fmt\.Print' cmd/sidecar/handler.go
```

Expected: no output.

**Step 4: Commit**

```bash
git add cmd/sidecar/handler.go cmd/sidecar/main.go
git commit -m "feat(sidecar): add exchange/denial metrics + enhance health endpoint"
```

---

### Task 8: Replace `fmt` in `register_handler.go` + Agent Gauge

Replace the 1 `fmt.Printf` call. Increment the agents_registered gauge on successful BYOK registration.

**Files:**
- Modify: `cmd/sidecar/register_handler.go`

**Step 1: Apply the changes**

Line 157: `fmt.Printf("[sidecar] BYOK registered agent %s → %s\n", req.AgentName, agentID)` →
```go
obs.Ok("SIDECAR", "REGISTRY", "BYOK agent registered", "agent="+req.AgentName, "agent_id="+agentID)
SidecarAgentsRegistered.Inc()
```

Also increment the gauge in `handler.go`'s `resolveAgent` after `h.registry.store(...)`:
```go
SidecarAgentsRegistered.Inc()
```

Replace `"fmt"` import with `"github.com/your-module/internal/obs"` in register_handler.go (keep `"fmt"` only if used elsewhere in the file — check).

**Step 2: Run tests**

```bash
go build ./cmd/sidecar/...
go test ./cmd/sidecar/... -count=1
```

Expected: all pass.

**Step 3: Verify no fmt.Print left in register_handler.go**

```bash
grep -n 'fmt\.Print' cmd/sidecar/register_handler.go
```

Expected: no output.

**Step 4: Commit**

```bash
git add cmd/sidecar/register_handler.go cmd/sidecar/handler.go
git commit -m "feat(sidecar): replace fmt.Print in register_handler.go + agent gauge"
```

---

### Task 9: Final Verification — Zero fmt.Print + Full Test Pass

**Step 1: Verify zero fmt.Printf/Println calls remain in sidecar**

```bash
grep -rn 'fmt\.Println\|fmt\.Printf\|fmt\.Fprintf' cmd/sidecar/*.go | grep -v '_test.go'
```

Expected: no output (or only `fmt.Errorf` / `fmt.Sprintf` which are value formatting, not printing).

**Step 2: Full test suite**

```bash
go test ./... -count=1 -race
```

Expected: all pass with race detection.

**Step 3: Lint**

```bash
golangci-lint run ./... 2>/dev/null || echo "lint not installed, skipping"
```

**Step 4: Gates**

```bash
./scripts/gates.sh task
```

Expected: all gates pass.

**Step 5: Commit any remaining fixes, then tag**

If any fixes were needed, commit them. Then run gates one final time.

---

### Task 10: Update Documentation

Update docs to reflect the new observability features.

**Files:**
- Modify: `docs/DEVELOPER_GUIDE.md` — add sidecar logging and metrics section
- Modify: `docs/USER_GUIDE.md` — add sidecar log level and metrics endpoint
- Modify: `CHANGELOG.md` — add entries under `[Unreleased]`

**CHANGELOG entries to add under `### Added`:**

```markdown
- **Sidecar Observability**: Structured logging via `internal/obs` package — replaces all 15 raw `fmt.Printf` calls with leveled, structured log lines (`[AA:SIDECAR:LEVEL] TIMESTAMP | COMPONENT | MESSAGE | context`)
- **Sidecar Observability**: `AA_SIDECAR_LOG_LEVEL` now wired (was loaded but unused) — supports `quiet`, `standard`, `verbose`, `trace`
- **Sidecar Observability**: 6 Prometheus metrics in dedicated `cmd/sidecar/metrics.go`: bootstrap, renewals, token exchanges, scope denials, agents registered, request duration
- **Sidecar Observability**: `GET /v1/metrics` endpoint on sidecar for Prometheus scraping
- **Sidecar Observability**: Health endpoint now reports `agents_registered`, `last_renewal`, `uptime_seconds`
```

**Commit:**

```bash
git add docs/DEVELOPER_GUIDE.md docs/USER_GUIDE.md CHANGELOG.md
git commit -m "docs(sidecar): add observability — structured logging + Prometheus metrics"
```

---

## Summary of New/Modified Files

| File | Status | Purpose |
|------|--------|---------|
| `cmd/sidecar/metrics.go` | **NEW** | 6 Prometheus metrics + convenience helpers |
| `cmd/sidecar/metrics_test.go` | **NEW** | Metric registration tests |
| `cmd/sidecar/main.go` | Modified | Wire obs.Configure, replace 8 fmt calls, add /v1/metrics |
| `cmd/sidecar/bootstrap.go` | Modified | Replace 4 fmt calls, add state fields, bootstrap metric |
| `cmd/sidecar/renewal.go` | Modified | Replace 3 fmt calls, renewal metrics |
| `cmd/sidecar/handler.go` | Modified | Replace 1 fmt call, exchange/denial metrics, health enhancement |
| `cmd/sidecar/register_handler.go` | Modified | Replace 1 fmt call, agent gauge |
| `cmd/sidecar/registry.go` | Modified | Add count() method |
| `cmd/sidecar/registry_test.go` | Modified | count() test |
| `cmd/sidecar/state_test.go` | Modified | lastRenewal/startTime tests |
| `docs/DEVELOPER_GUIDE.md` | Modified | Sidecar observability docs |
| `docs/USER_GUIDE.md` | Modified | Log level + metrics endpoint docs |
| `CHANGELOG.md` | Modified | Unreleased entries |

## Important Notes for Implementer

1. **Module path**: Replace `github.com/your-module` with the actual module path from `go.mod`. Check with: `head -1 go.mod`
2. **`fmt` import cleanup**: After replacing `fmt.Printf` calls, check if `"fmt"` is still needed in each file. Remove unused imports or the build will fail.
3. **Test signature changes**: `newHealthHandler` gains a third parameter (`registry`). Any existing test that calls it needs updating.
4. **`bootstrap.go` has sidecarState**: There is no separate `state.go` file. The struct lives in `bootstrap.go`.
5. **promauto auto-registers**: No need to call `prometheus.MustRegister()` — `promauto` does it automatically on package init.
