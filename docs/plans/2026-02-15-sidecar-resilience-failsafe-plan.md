# Sidecar Resilience: Failsafe Mode — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add circuit breaker, cached token fallback, and bootstrap retry to the sidecar so it degrades gracefully when the broker is temporarily unreachable.

**Architecture:** Standalone circuit breaker in `cmd/sidecar/circuitbreaker.go` with sliding-window failure tracking, three states (Closed/Open/Probing), background health probe, and cached token serving. Bootstrap changes from fail-fast exit to retry-with-backoff. HTTP server starts before bootstrap completes.

**Tech Stack:** Go stdlib (`sync`, `time`), Prometheus (`promauto`), existing `obs` logging package

**Design doc:** `docs/plans/2026-02-15-sidecar-phase3-failsafe-design.md`

---

### Task 1: Add Circuit Breaker Config Vars

**Files:**
- Modify: `cmd/sidecar/config.go`
- Test: `go test ./cmd/sidecar/... -run TestLoadConfig` (manual verification — no config tests exist yet)

**Step 1: Add fields to sidecarConfig struct**

Add these fields after `RenewalBuffer float64` (line 15 of `config.go`):

```go
CBWindow      int     // sliding window duration in seconds
CBThreshold   float64 // failure rate 0.0-1.0 to trip circuit
CBProbeInterval int   // seconds between health probes when open
CBMinRequests int     // min requests in window before tripping
```

**Step 2: Load the new env vars in loadConfig()**

Add after the `cfg.RenewalBuffer` block (after line 31):

```go
cfg.CBWindow = envOrInt("AA_SIDECAR_CB_WINDOW", 30)
cfg.CBThreshold = envOrFloat("AA_SIDECAR_CB_THRESHOLD", 0.5, 0.0, 1.0)
cfg.CBProbeInterval = envOrInt("AA_SIDECAR_CB_PROBE_INTERVAL", 5)
cfg.CBMinRequests = envOrInt("AA_SIDECAR_CB_MIN_REQUESTS", 5)
```

**Step 3: Add helper functions**

Add `envOrInt` and `envOrFloat` below existing `envOr`:

```go
func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func envOrFloat(key string, fallback, min, max float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= min && f <= max {
			return f
		}
	}
	return fallback
}
```

**Step 4: Run tests to verify nothing breaks**

Run: `go test ./cmd/sidecar/... -race -count=1`
Expected: All existing tests PASS

**Step 5: Commit**

```bash
git add cmd/sidecar/config.go
git commit -m "feat(sidecar): add circuit breaker config vars (CB_WINDOW, CB_THRESHOLD, CB_PROBE_INTERVAL, CB_MIN_REQUESTS)"
```

---

### Task 2: Circuit Breaker — Sliding Window + State Machine (Tests First)

**Files:**
- Create: `cmd/sidecar/circuitbreaker_test.go`
- Create: `cmd/sidecar/circuitbreaker.go`

**Step 1: Write failing tests**

Create `cmd/sidecar/circuitbreaker_test.go`:

```go
package main

import (
	"testing"
	"time"
)

func TestCircuitBreaker_StartsInClosedState(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	if cb.State() != StateClosed {
		t.Errorf("initial state = %v, want Closed", cb.State())
	}
}

func TestCircuitBreaker_AllowsRequestWhenClosed(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	if !cb.Allow() {
		t.Error("Allow() = false when closed, want true")
	}
}

func TestCircuitBreaker_TripsOpenOnHighFailureRate(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)

	// Record 5 failures out of 5 requests (100% failure rate).
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	if cb.State() != StateOpen {
		t.Errorf("state = %v after 5 failures, want Open", cb.State())
	}
}

func TestCircuitBreaker_DoesNotTripBelowMinRequests(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)

	// 3 failures out of 3 (100% rate, but < minRequests of 5).
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.State() != StateClosed {
		t.Errorf("state = %v after 3 failures (below min), want Closed", cb.State())
	}
}

func TestCircuitBreaker_DoesNotTripBelowThreshold(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)

	// 2 failures + 4 successes = 33% failure rate (below 50% threshold).
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != StateClosed {
		t.Errorf("state = %v at 33%% failure rate, want Closed", cb.State())
	}
}

func TestCircuitBreaker_BlocksRequestsWhenOpen(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	if cb.Allow() {
		t.Error("Allow() = true when open, want false")
	}
}

func TestCircuitBreaker_WindowExpiration(t *testing.T) {
	// Use a very short window so entries expire quickly.
	cb := newCircuitBreaker(50*time.Millisecond, 0.5, 5*time.Second, 2)

	cb.RecordFailure()
	cb.RecordFailure()

	// Should be open now.
	if cb.State() != StateOpen {
		t.Fatalf("state = %v, want Open", cb.State())
	}

	// Wait for window to expire.
	time.Sleep(100 * time.Millisecond)

	// After expiration, old events purged. State should reset to Closed.
	// Recording a success should confirm we're back to Closed.
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("state = %v after window expiration + success, want Closed", cb.State())
	}
}

func TestCircuitBreaker_TransitionToProbing(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	// Simulate successful probe.
	cb.ProbeSucceeded()
	if cb.State() != StateProbing {
		t.Errorf("state = %v after ProbeSucceeded, want Probing", cb.State())
	}
}

func TestCircuitBreaker_ProbingAllowsOneRequest(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	cb.ProbeSucceeded()

	// First Allow in Probing should return true.
	if !cb.Allow() {
		t.Error("Allow() = false in Probing state, want true")
	}
}

func TestCircuitBreaker_ProbingSuccessCloses(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	cb.ProbeSucceeded()
	cb.RecordSuccess() // The probing request succeeded.

	if cb.State() != StateClosed {
		t.Errorf("state = %v after probing success, want Closed", cb.State())
	}
}

func TestCircuitBreaker_ProbingFailureReopens(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	cb.ProbeSucceeded()
	cb.RecordFailure() // The probing request failed.

	if cb.State() != StateOpen {
		t.Errorf("state = %v after probing failure, want Open", cb.State())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/sidecar/... -run TestCircuitBreaker -v`
Expected: FAIL — `newCircuitBreaker` undefined

**Step 3: Write the circuit breaker implementation**

Create `cmd/sidecar/circuitbreaker.go`:

```go
package main

import (
	"sync"
	"time"
)

// CircuitState represents the current state of the circuit breaker.
type CircuitState int

const (
	StateClosed  CircuitState = 0 // normal — requests pass through
	StateOpen    CircuitState = 1 // failing — requests blocked
	StateProbing CircuitState = 2 // testing recovery
)

// String returns the human-readable name of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateProbing:
		return "probing"
	default:
		return "unknown"
	}
}

// event records a single success or failure with a timestamp.
type event struct {
	at     time.Time
	failed bool
}

// circuitBreaker implements a sliding-window circuit breaker.
//
// States:
//   - Closed: requests pass through; failures tracked in window
//   - Open: requests blocked; background probe checks broker health
//   - Probing: probe succeeded; next real request determines recovery
//
// Thread-safe: all methods are guarded by mu.
type circuitBreaker struct {
	mu sync.Mutex

	state         CircuitState
	window        time.Duration
	threshold     float64
	probeInterval time.Duration
	minRequests   int
	events        []event

	// nowFunc is injectable for testing. Defaults to time.Now.
	nowFunc func() time.Time
}

// newCircuitBreaker creates a circuit breaker with the given parameters.
func newCircuitBreaker(window time.Duration, threshold float64, probeInterval time.Duration, minRequests int) *circuitBreaker {
	return &circuitBreaker{
		state:         StateClosed,
		window:        window,
		threshold:     threshold,
		probeInterval: probeInterval,
		minRequests:   minRequests,
		nowFunc:       time.Now,
	}
}

// State returns the current circuit state.
func (cb *circuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.purgeExpired()
	cb.evaluateState()
	return cb.state
}

// Allow returns true if a request should be sent to the broker.
// Closed and Probing allow requests; Open blocks them.
func (cb *circuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.purgeExpired()
	cb.evaluateState()
	return cb.state != StateOpen
}

// RecordSuccess records a successful broker request.
func (cb *circuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.events = append(cb.events, event{at: cb.nowFunc(), failed: false})

	if cb.state == StateProbing {
		// Probing request succeeded — close the circuit.
		cb.state = StateClosed
		cb.events = nil // reset window
	}
}

// RecordFailure records a failed broker request.
func (cb *circuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.events = append(cb.events, event{at: cb.nowFunc(), failed: true})

	if cb.state == StateProbing {
		// Probing request failed — reopen the circuit.
		cb.state = StateOpen
		return
	}

	cb.purgeExpired()
	cb.evaluateState()
}

// ProbeSucceeded is called by the background health probe when the
// broker responds successfully. Transitions Open → Probing.
func (cb *circuitBreaker) ProbeSucceeded() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen {
		cb.state = StateProbing
	}
}

// purgeExpired removes events older than the sliding window.
// Must be called with mu held.
func (cb *circuitBreaker) purgeExpired() {
	cutoff := cb.nowFunc().Add(-cb.window)
	i := 0
	for i < len(cb.events) && cb.events[i].at.Before(cutoff) {
		i++
	}
	if i > 0 {
		cb.events = cb.events[i:]
	}
}

// evaluateState checks the failure rate and transitions Closed → Open
// if the threshold is exceeded and minimum requests are met.
// Must be called with mu held.
func (cb *circuitBreaker) evaluateState() {
	if cb.state != StateClosed {
		return // only evaluate when closed
	}

	total := len(cb.events)
	if total < cb.minRequests {
		return
	}

	failures := 0
	for _, e := range cb.events {
		if e.failed {
			failures++
		}
	}

	rate := float64(failures) / float64(total)
	if rate > cb.threshold {
		cb.state = StateOpen
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/sidecar/... -run TestCircuitBreaker -race -v`
Expected: All 11 tests PASS

**Step 5: Commit**

```bash
git add cmd/sidecar/circuitbreaker.go cmd/sidecar/circuitbreaker_test.go
git commit -m "feat(sidecar): add circuit breaker with sliding-window failure tracking"
```

---

### Task 3: Add Circuit Breaker Prometheus Metrics

**Files:**
- Modify: `cmd/sidecar/metrics.go`
- Modify: `cmd/sidecar/metrics_test.go`

**Step 1: Add 3 new metrics to metrics.go**

Add after `SidecarRequestDuration` (after line 50):

```go
// SidecarCircuitState reports the current circuit breaker state (0=closed, 1=open, 2=probing).
var SidecarCircuitState = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "agentauth_sidecar_circuit_state",
	Help: "Circuit breaker state: 0=closed, 1=open, 2=probing",
})

// SidecarCircuitTripsTotal counts how many times the circuit has tripped open.
var SidecarCircuitTripsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "agentauth_sidecar_circuit_trips_total",
	Help: "Total times circuit breaker has tripped open",
})

// SidecarCachedTokensServedTotal counts tokens served from cache during open circuit.
var SidecarCachedTokensServedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "agentauth_sidecar_cached_tokens_served_total",
	Help: "Total tokens served from cache during open circuit",
})
```

**Step 2: Update metrics_test.go**

Add after existing test cases:

```go
func TestCircuitBreakerMetrics_NotNil(t *testing.T) {
	if SidecarCircuitState == nil {
		t.Error("SidecarCircuitState is nil")
	}
	if SidecarCircuitTripsTotal == nil {
		t.Error("SidecarCircuitTripsTotal is nil")
	}
	if SidecarCachedTokensServedTotal == nil {
		t.Error("SidecarCachedTokensServedTotal is nil")
	}
}
```

**Step 3: Run tests**

Run: `go test ./cmd/sidecar/... -race -count=1`
Expected: All PASS

**Step 4: Commit**

```bash
git add cmd/sidecar/metrics.go cmd/sidecar/metrics_test.go
git commit -m "feat(sidecar): add circuit breaker Prometheus metrics (state, trips, cached tokens)"
```

---

### Task 4: Add Token Cache to Agent Registry

The agent registry already stores agent entries but doesn't cache tokens. We need to store the last successful token response per agent so the circuit breaker can serve it during outages.

**Files:**
- Modify: `cmd/sidecar/registry.go`
- Modify: `cmd/sidecar/registry_test.go`

**Step 1: Add cached token fields to agentEntry**

In `registry.go`, add to the `agentEntry` struct (after `registeredAt time.Time`, line 14):

```go
lastToken    string    // last successfully issued token
lastScope    []string  // scope of last token
lastTokenTTL int       // TTL in seconds
lastTokenAt  time.Time // when the token was issued
```

**Step 2: Add cache methods to agentRegistry**

Add after the `count()` method:

```go
// cacheToken stores a successful token response for the given agent key.
func (r *agentRegistry) cacheToken(key, token string, scope []string, ttl int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.agents[key]; ok {
		e.lastToken = token
		e.lastScope = scope
		e.lastTokenTTL = ttl
		e.lastTokenAt = time.Now()
	}
}

// cachedToken returns a cached token for the given agent key if:
// 1. The agent exists in the registry
// 2. A token was previously cached
// 3. The token has not expired (within original TTL)
// 4. The requested scope is a subset of the cached scope
// Returns the token and true, or empty string and false.
func (r *agentRegistry) cachedToken(key string, requestedScope []string) (string, int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.agents[key]
	if !ok || e.lastToken == "" {
		return "", 0, false
	}

	// Check TTL expiration.
	elapsed := time.Since(e.lastTokenAt)
	remaining := e.lastTokenTTL - int(elapsed.Seconds())
	if remaining <= 0 {
		return "", 0, false
	}

	// Check scope: requested must be subset of cached.
	if !scopeIsSubset(requestedScope, e.lastScope) {
		return "", 0, false
	}

	return e.lastToken, remaining, true
}
```

**Step 3: Write tests for the cache methods**

Add to `registry_test.go`:

```go
func TestAgentRegistry_CacheToken_HappyPath(t *testing.T) {
	reg := newAgentRegistry()
	reg.store("agent-a:task-1", &agentEntry{spiffeID: "spiffe://test/a"})

	reg.cacheToken("agent-a:task-1", "jwt-token-abc", []string{"read:data:*"}, 300)

	token, remaining, ok := reg.cachedToken("agent-a:task-1", []string{"read:data:*"})
	if !ok {
		t.Fatal("cachedToken returned false, want true")
	}
	if token != "jwt-token-abc" {
		t.Errorf("token = %q, want jwt-token-abc", token)
	}
	if remaining <= 0 || remaining > 300 {
		t.Errorf("remaining = %d, want 1-300", remaining)
	}
}

func TestAgentRegistry_CacheToken_Expired(t *testing.T) {
	reg := newAgentRegistry()
	reg.store("agent-a:task-1", &agentEntry{spiffeID: "spiffe://test/a"})

	// Cache with TTL of 0 (already expired).
	reg.cacheToken("agent-a:task-1", "expired-jwt", []string{"read:data:*"}, 0)

	_, _, ok := reg.cachedToken("agent-a:task-1", []string{"read:data:*"})
	if ok {
		t.Error("cachedToken returned true for expired token, want false")
	}
}

func TestAgentRegistry_CacheToken_ScopeMismatch(t *testing.T) {
	reg := newAgentRegistry()
	reg.store("agent-a:task-1", &agentEntry{spiffeID: "spiffe://test/a"})

	reg.cacheToken("agent-a:task-1", "jwt-token", []string{"read:data:*"}, 300)

	// Request write scope — not covered by cached read scope.
	_, _, ok := reg.cachedToken("agent-a:task-1", []string{"write:data:*"})
	if ok {
		t.Error("cachedToken returned true for mismatched scope, want false")
	}
}

func TestAgentRegistry_CacheToken_MissingAgent(t *testing.T) {
	reg := newAgentRegistry()

	_, _, ok := reg.cachedToken("nonexistent:task-1", []string{"read:data:*"})
	if ok {
		t.Error("cachedToken returned true for missing agent, want false")
	}
}
```

**Step 4: Run tests**

Run: `go test ./cmd/sidecar/... -run TestAgentRegistry_CacheToken -race -v`
Expected: All 4 PASS

**Step 5: Commit**

```bash
git add cmd/sidecar/registry.go cmd/sidecar/registry_test.go
git commit -m "feat(sidecar): add token cache to agent registry for failsafe fallback"
```

---

### Task 5: Wire Circuit Breaker into Token Handler

This is the core integration. The token handler calls the circuit breaker before sending requests to the broker. On open circuit, it attempts to serve cached tokens.

**Files:**
- Modify: `cmd/sidecar/handler.go`
- Modify: `cmd/sidecar/handler_test.go`

**Step 1: Add circuit breaker field to tokenHandler**

In `handler.go`, add to the `tokenHandler` struct (after `adminSecret string`, line 38):

```go
cb *circuitBreaker
```

**Step 2: Update newTokenHandler signature**

Change `newTokenHandler` to accept the circuit breaker:

```go
func newTokenHandler(bc *brokerClient, state *sidecarState, ceiling []string, reg *agentRegistry, adminSecret string, cb *circuitBreaker) *tokenHandler {
	return &tokenHandler{
		broker:       bc,
		state:        state,
		scopeCeiling: ceiling,
		registry:     reg,
		adminSecret:  adminSecret,
		cb:           cb,
	}
}
```

**Step 3: Modify ServeHTTP to use circuit breaker**

Replace the token exchange section (lines 101-116 of `handler.go`) with:

```go
	// Check circuit breaker before calling broker.
	if h.cb != nil && !h.cb.Allow() {
		// Circuit is open — try to serve cached token.
		if token, remaining, ok := h.registry.cachedToken(agentKey, req.Scope); ok {
			SidecarCachedTokensServedTotal.Inc()
			obs.Warn("SIDECAR", "TOKEN", "serving cached token (circuit open)", "agent="+agentKey)
			w.Header().Set("X-AgentAuth-Cached", "true")
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token": token,
				"expires_in":   remaining,
				"scope":        req.Scope,
				"agent_id":     agentID,
			})
			return
		}
		writeError(w, http.StatusServiceUnavailable, "broker unavailable and no cached token")
		return
	}

	// Delegate to broker token exchange using the agent's SPIFFE ID.
	exResp, err := h.broker.tokenExchange(h.state.getToken(), agentID, req.Scope, ttl)
	if err != nil {
		RecordExchange("failure")
		if h.cb != nil {
			h.cb.RecordFailure()
		}
		writeError(w, http.StatusBadGateway, "broker token exchange failed: "+err.Error())
		return
	}

	RecordExchange("success")
	if h.cb != nil {
		h.cb.RecordSuccess()
	}

	// Cache the token for failsafe fallback.
	h.registry.cacheToken(agentKey, exResp.AccessToken, req.Scope, exResp.ExpiresIn)

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": exResp.AccessToken,
		"expires_in":   exResp.ExpiresIn,
		"scope":        req.Scope,
		"agent_id":     agentID,
	})
```

**Step 4: Update all existing test calls**

In `handler_test.go`, update all `newTokenHandler` calls to pass `nil` as the last argument (circuit breaker):

- `newTokenHandler(bc, state, ceiling, reg, "test-secret")` → `newTokenHandler(bc, state, ceiling, reg, "test-secret", nil)`

There are 7 occurrences across:
- `TestTokenHandler_HappyPath`
- `TestTokenHandler_ScopeExceedsCeiling`
- `TestTokenHandler_MissingFields`
- `TestTokenHandler_MethodNotAllowed`
- `TestTokenHandler_BrokerError`
- `TestTokenHandler_LazyRegistration_FirstRequest`
- `TestTokenHandler_LazyRegistration_SecondRequestSkipsRegistration`
- `TestTokenHandler_LazyRegistration_BrokerFailure502`

**Step 5: Update main.go**

In `main.go`, update the `newTokenHandler` call (line 52) to pass `nil` for now (circuit breaker wired in Task 7):

```go
mux.Handle("/v1/token", newTokenHandler(bc, state, cfg.ScopeCeiling, registry, cfg.AdminSecret, nil))
```

**Step 6: Add circuit breaker integration tests**

Add to `handler_test.go`:

```go
func TestTokenHandler_CircuitOpen_ServesCachedToken(t *testing.T) {
	// No mock broker needed — circuit is open, should serve from cache.
	bc := newBrokerClient("http://127.0.0.1:1") // unreachable
	state := &sidecarState{}
	state.setToken("sidecar-token", 900)
	ceiling := []string{"read:data:*"}
	reg := newAgentRegistry()

	// Pre-populate registry with agent and cached token.
	reg.store("data-reader:task-1", &agentEntry{spiffeID: "spiffe://test/agent/data-reader/task-1/inst"})
	reg.cacheToken("data-reader:task-1", "cached-jwt-abc", []string{"read:data:*"}, 300)

	// Create circuit breaker and force it open.
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	h := newTokenHandler(bc, state, ceiling, reg, "test-secret", cb)

	body := `{"agent_name":"data-reader","task_id":"task-1","scope":["read:data:*"],"ttl":300}`
	req := httptest.NewRequest("POST", "/v1/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}

	if rr.Header().Get("X-AgentAuth-Cached") != "true" {
		t.Error("missing X-AgentAuth-Cached header")
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["access_token"] != "cached-jwt-abc" {
		t.Errorf("access_token = %v, want cached-jwt-abc", resp["access_token"])
	}
}

func TestTokenHandler_CircuitOpen_NoCachedToken_Returns503(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1")
	state := &sidecarState{}
	state.setToken("sidecar-token", 900)
	ceiling := []string{"read:data:*"}
	reg := newAgentRegistry()

	// Agent exists but no cached token.
	reg.store("data-reader:task-1", &agentEntry{spiffeID: "spiffe://test/agent/data-reader/task-1/inst"})

	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	h := newTokenHandler(bc, state, ceiling, reg, "test-secret", cb)

	body := `{"agent_name":"data-reader","task_id":"task-1","scope":["read:data:*"],"ttl":300}`
	req := httptest.NewRequest("POST", "/v1/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", rr.Code, rr.Body.String())
	}
}
```

**Step 7: Run all tests**

Run: `go test ./cmd/sidecar/... -race -count=1`
Expected: All PASS

**Step 8: Commit**

```bash
git add cmd/sidecar/handler.go cmd/sidecar/handler_test.go cmd/sidecar/main.go
git commit -m "feat(sidecar): wire circuit breaker into token handler with cached fallback"
```

---

### Task 6: Background Health Probe

When the circuit is open, a background goroutine pings the broker's health endpoint to detect recovery.

**Files:**
- Create: `cmd/sidecar/probe.go`
- Create: `cmd/sidecar/probe_test.go`

**Step 1: Write failing test**

Create `cmd/sidecar/probe_test.go`:

```go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbe_TransitionsToProbing(t *testing.T) {
	// Mock broker health endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	cb := newCircuitBreaker(30*time.Second, 0.5, 50*time.Millisecond, 5)

	// Trip the circuit.
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startProbe(ctx, cb, bc)

	// Wait for probe to run and detect healthy broker.
	time.Sleep(200 * time.Millisecond)

	if cb.State() != StateProbing {
		t.Errorf("state = %v, want Probing after healthy probe", cb.State())
	}
}

func TestProbe_StaysOpenWhenBrokerDown(t *testing.T) {
	// Mock broker that always fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	cb := newCircuitBreaker(30*time.Second, 0.5, 50*time.Millisecond, 5)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startProbe(ctx, cb, bc)

	time.Sleep(200 * time.Millisecond)

	if cb.State() != StateOpen {
		t.Errorf("state = %v, want Open when broker is down", cb.State())
	}
}

func TestProbe_StopsOnCancel(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1")
	cb := newCircuitBreaker(30*time.Second, 0.5, 50*time.Millisecond, 5)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Should return without blocking.
	done := make(chan struct{})
	go func() {
		startProbe(ctx, cb, bc)
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(1 * time.Second):
		t.Error("startProbe did not exit after context cancel")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/sidecar/... -run TestProbe -v`
Expected: FAIL — `startProbe` undefined

**Step 3: Implement the probe**

Create `cmd/sidecar/probe.go`:

```go
package main

import (
	"context"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
)

// startProbe runs a blocking loop that pings the broker health endpoint
// while the circuit breaker is open. When the broker responds successfully,
// it transitions the circuit to Probing state. Stops when ctx is cancelled.
func startProbe(ctx context.Context, cb *circuitBreaker, bc *brokerClient) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(cb.probeInterval):
		}

		// Only probe when circuit is open.
		if cb.State() != StateOpen {
			continue
		}

		err := bc.healthCheck()
		if err != nil {
			obs.Trace("SIDECAR", "PROBE", "broker still unreachable", err.Error())
			continue
		}

		obs.Ok("SIDECAR", "PROBE", "broker reachable, transitioning to probing")
		cb.ProbeSucceeded()
		SidecarCircuitState.Set(float64(StateProbing))
	}
}
```

**Step 4: Run tests**

Run: `go test ./cmd/sidecar/... -run TestProbe -race -v`
Expected: All 3 PASS

**Step 5: Commit**

```bash
git add cmd/sidecar/probe.go cmd/sidecar/probe_test.go
git commit -m "feat(sidecar): add background health probe for circuit breaker recovery"
```

---

### Task 7: Wire Everything in main.go

Connect the circuit breaker, probe, and metrics updates in `main.go`.

**Files:**
- Modify: `cmd/sidecar/main.go`

**Step 1: Create circuit breaker after bootstrap**

Replace the section from `state, err := bootstrap(...)` through route setup (lines 35-57) with:

```go
	state, err := bootstrap(bc, cfg)
	if err != nil {
		obs.Fail("SIDECAR", "MAIN", "bootstrap failed", err.Error())
		os.Exit(1)
	}

	// Start background renewal goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go startRenewal(ctx, state, bc.tokenRenew, cfg.RenewalBuffer)
	obs.Ok("SIDECAR", "MAIN", "renewal goroutine started", fmt.Sprintf("buffer=%.0f%%", cfg.RenewalBuffer*100))

	// Create circuit breaker.
	cb := newCircuitBreaker(
		time.Duration(cfg.CBWindow)*time.Second,
		cfg.CBThreshold,
		time.Duration(cfg.CBProbeInterval)*time.Second,
		cfg.CBMinRequests,
	)
	go startProbe(ctx, cb, bc)
	obs.Ok("SIDECAR", "MAIN", "circuit breaker active",
		fmt.Sprintf("window=%ds", cfg.CBWindow),
		fmt.Sprintf("threshold=%.0f%%", cfg.CBThreshold*100),
		fmt.Sprintf("probe_interval=%ds", cfg.CBProbeInterval),
	)

	// Create agent registry (ephemeral, in-memory).
	registry := newAgentRegistry()

	// Set up routes.
	mux := http.NewServeMux()
	mux.Handle("/v1/token", newTokenHandler(bc, state, cfg.ScopeCeiling, registry, cfg.AdminSecret, cb))
	mux.Handle("/v1/token/renew", newRenewHandler(bc))
	mux.Handle("/v1/health", newHealthHandler(state, cfg.ScopeCeiling, registry))
	mux.Handle("/v1/challenge", newChallengeProxyHandler(bc))
	mux.Handle("/v1/register", newRegisterHandler(bc, registry, cfg.AdminSecret, cfg.ScopeCeiling))
	mux.Handle("/v1/metrics", promhttp.Handler())
```

**Step 2: Add `time` import**

Add `"time"` to the import block if not already present.

**Step 3: Run all tests**

Run: `go test ./cmd/sidecar/... -race -count=1`
Expected: All PASS

**Step 4: Commit**

```bash
git add cmd/sidecar/main.go
git commit -m "feat(sidecar): wire circuit breaker + probe in main.go"
```

---

### Task 8: Bootstrap Retry with Backoff

Replace the fail-fast bootstrap with a retry loop. Start the HTTP server before bootstrap so health probes get a response.

**Files:**
- Modify: `cmd/sidecar/main.go`
- Modify: `cmd/sidecar/handler.go`
- Add tests: `cmd/sidecar/handler_test.go`

**Step 1: Add "bootstrapping" status to health handler**

In `handler.go`, modify the health handler's `ServeHTTP` to handle nil state (pre-bootstrap):

The existing code at line 264 already handles `h.state != nil`, so health returns `degraded` / 503 when state is nil. Add a `bootstrapping` status:

Replace lines 264-272 with:

```go
	if h.state == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":  "bootstrapping",
			"healthy": false,
		})
		return
	}

	healthy := h.state.isHealthy()
	connected := h.state.getToken() != ""

	status := "ok"
	httpStatus := http.StatusOK
	if !healthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}
```

**Step 2: Restructure main.go for bootstrap retry**

Replace the entire `main()` function with:

```go
func main() {
	cfg := loadConfig()
	obs.Configure(cfg.LogLevel)

	// Validate required config.
	if cfg.AdminSecret == "" {
		obs.Fail("SIDECAR", "MAIN", "AA_ADMIN_SECRET must be set")
		os.Exit(1)
	}
	if len(cfg.ScopeCeiling) == 0 {
		obs.Fail("SIDECAR", "MAIN", "AA_SIDECAR_SCOPE_CEILING must be set")
		os.Exit(1)
	}

	bc := newBrokerClient(cfg.BrokerURL)

	obs.Ok("SIDECAR", "MAIN", "starting", "broker="+cfg.BrokerURL, "scope_ceiling="+strings.Join(cfg.ScopeCeiling, ","))

	// Create agent registry (ephemeral, in-memory).
	registry := newAgentRegistry()

	// Create circuit breaker.
	cb := newCircuitBreaker(
		time.Duration(cfg.CBWindow)*time.Second,
		cfg.CBThreshold,
		time.Duration(cfg.CBProbeInterval)*time.Second,
		cfg.CBMinRequests,
	)

	// Shared state pointer — nil until bootstrap succeeds.
	var state *sidecarState

	// Set up routes. Health works pre-bootstrap; token routes require state.
	mux := http.NewServeMux()
	healthH := newHealthHandler(nil, cfg.ScopeCeiling, registry)
	mux.Handle("/v1/health", healthH)
	mux.Handle("/v1/metrics", promhttp.Handler())

	// Start HTTP server immediately so health probes get a response.
	addr := ":" + cfg.Port
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			obs.Fail("SIDECAR", "MAIN", "listen failed", err.Error())
			os.Exit(1)
		}
	}()
	obs.Ok("SIDECAR", "MAIN", "http server started (pre-bootstrap)", "addr="+addr)

	// Bootstrap with retry.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		obs.Ok("SIDECAR", "MAIN", "shutting down")
		cancel()
	}()

	backoff := 1 * time.Second
	maxBootBackoff := 60 * time.Second
	attempt := 0
	for {
		var err error
		state, err = bootstrap(bc, cfg)
		if err == nil {
			break
		}
		attempt++
		obs.Warn("SIDECAR", "BOOTSTRAP", "failed, retrying",
			fmt.Sprintf("attempt=%d", attempt),
			fmt.Sprintf("retry_in=%s", backoff),
			err.Error(),
		)
		RecordBootstrap("failure")

		select {
		case <-ctx.Done():
			obs.Fail("SIDECAR", "MAIN", "shutdown during bootstrap")
			os.Exit(1)
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBootBackoff {
			backoff = maxBootBackoff
		}
	}

	// Bootstrap succeeded — wire remaining routes.
	healthH.state = state
	mux.Handle("/v1/token", newTokenHandler(bc, state, cfg.ScopeCeiling, registry, cfg.AdminSecret, cb))
	mux.Handle("/v1/token/renew", newRenewHandler(bc))
	mux.Handle("/v1/challenge", newChallengeProxyHandler(bc))
	mux.Handle("/v1/register", newRegisterHandler(bc, registry, cfg.AdminSecret, cfg.ScopeCeiling))

	go startRenewal(ctx, state, bc.tokenRenew, cfg.RenewalBuffer)
	obs.Ok("SIDECAR", "MAIN", "renewal goroutine started", fmt.Sprintf("buffer=%.0f%%", cfg.RenewalBuffer*100))

	go startProbe(ctx, cb, bc)
	obs.Ok("SIDECAR", "MAIN", "circuit breaker active",
		fmt.Sprintf("window=%ds", cfg.CBWindow),
		fmt.Sprintf("threshold=%.0f%%", cfg.CBThreshold*100),
		fmt.Sprintf("probe_interval=%ds", cfg.CBProbeInterval),
	)

	obs.Ok("SIDECAR", "MAIN", "ready", "addr="+addr, "sidecar_id="+state.sidecarID)

	// Block until shutdown.
	<-ctx.Done()
}
```

**Step 3: Add `"time"` to imports in main.go**

Ensure the import block includes `"time"`.

**Step 4: Add test for bootstrapping health status**

Add to `handler_test.go`:

```go
func TestHealthHandler_Bootstrapping(t *testing.T) {
	// State is nil — sidecar is still bootstrapping.
	h := newHealthHandler(nil, []string{"read:data:*"}, nil)

	req := httptest.NewRequest("GET", "/v1/health", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "bootstrapping" {
		t.Errorf("status = %v, want bootstrapping", resp["status"])
	}
}
```

**Step 5: Run all tests**

Run: `go test ./cmd/sidecar/... -race -count=1`
Expected: All PASS

**Step 6: Commit**

```bash
git add cmd/sidecar/main.go cmd/sidecar/handler.go cmd/sidecar/handler_test.go
git commit -m "feat(sidecar): bootstrap retry with backoff + pre-bootstrap HTTP server"
```

---

### Task 9: Update Documentation

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `docs/DEVELOPER_GUIDE.md`
- Modify: `docs/USER_GUIDE.md`

**Step 1: CHANGELOG.md**

Add under `[Unreleased]` → `### Added`:

```markdown
- **Sidecar Phase 3 — Failsafe Mode**: Circuit breaker with sliding-window failure tracking (Closed → Open → Probing states)
- **Sidecar Phase 3**: Cached token fallback — serves previously-issued tokens during broker outage (`X-AgentAuth-Cached: true` header)
- **Sidecar Phase 3**: Background health probe for automatic circuit breaker recovery
- **Sidecar Phase 3**: Bootstrap retry with exponential backoff — sidecar no longer exits on broker unavailability at startup
- **Sidecar Phase 3**: HTTP server starts pre-bootstrap — health endpoint responds during startup
- **Sidecar Phase 3**: 3 new Prometheus metrics: `circuit_state`, `circuit_trips_total`, `cached_tokens_served_total`
- **Sidecar Phase 3**: 4 new config vars: `AA_SIDECAR_CB_WINDOW`, `AA_SIDECAR_CB_THRESHOLD`, `AA_SIDECAR_CB_PROBE_INTERVAL`, `AA_SIDECAR_CB_MIN_REQUESTS`
```

**Step 2: DEVELOPER_GUIDE.md**

Add new subsection under Sidecar Architecture → after Sidecar Observability:

```markdown
### Sidecar Failsafe (Phase 3)

The sidecar includes a circuit breaker that protects agents from broker outages.

**ADR-001 (Dev vs. Production):** The cached token fallback and bootstrap retry
are dev conveniences for single-broker setups. In production, broker HA via
multiple instances behind a load balancer is the primary resilience strategy.
The circuit breaker remains useful as a secondary fast-fail mechanism.

**Circuit Breaker States:**

| State | Behavior |
|-------|----------|
| Closed | Normal — requests pass through, failures tracked in sliding window |
| Open | Broker down — serve cached tokens, background probe runs |
| Probing | Probe succeeded — next real request tests recovery |

**Config:**

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_SIDECAR_CB_WINDOW` | `30` | Sliding window seconds |
| `AA_SIDECAR_CB_THRESHOLD` | `0.5` | Failure rate to trip |
| `AA_SIDECAR_CB_PROBE_INTERVAL` | `5` | Probe interval seconds |
| `AA_SIDECAR_CB_MIN_REQUESTS` | `5` | Min requests before tripping |

**Cached Token Rules:**
- Same agent + same or subset scope
- Within original TTL
- Response includes `X-AgentAuth-Cached: true` header

**New Prometheus Metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `agentauth_sidecar_circuit_state` | Gauge | 0=closed, 1=open, 2=probing |
| `agentauth_sidecar_circuit_trips_total` | Counter | Times circuit tripped |
| `agentauth_sidecar_cached_tokens_served_total` | Counter | Cache hits during outage |
```

**Step 3: USER_GUIDE.md**

Add new section after Sidecar Metrics:

```markdown
### Sidecar Failsafe (Circuit Breaker)

The sidecar includes a circuit breaker that detects broker outages and serves
cached tokens during brief interruptions.

**How it works:**
1. The sidecar tracks broker request success/failure over a 30-second window
2. If more than 50% of requests fail (minimum 5 requests), the circuit "trips open"
3. While open, the sidecar serves previously-issued tokens from its cache
4. A background probe pings the broker every 5 seconds to detect recovery
5. When the broker recovers, the circuit closes and normal operation resumes

**Cached tokens** include the `X-AgentAuth-Cached: true` response header so
your application can distinguish fresh vs. cached tokens.

**Configuration:**

```bash
export AA_SIDECAR_CB_WINDOW=30          # sliding window (seconds)
export AA_SIDECAR_CB_THRESHOLD=0.5      # failure rate to trip (0.0-1.0)
export AA_SIDECAR_CB_PROBE_INTERVAL=5   # probe interval (seconds)
export AA_SIDECAR_CB_MIN_REQUESTS=5     # min requests before tripping
```

**Note:** This failsafe is designed for single-broker dev environments. In
production, run multiple broker instances behind a load balancer for true HA.
```

**Step 4: Commit**

```bash
git add CHANGELOG.md docs/DEVELOPER_GUIDE.md docs/USER_GUIDE.md
git commit -m "docs(sidecar): add Phase 3 failsafe — circuit breaker, cached tokens, bootstrap retry"
```

---

### Task 10: Docker E2E Verification

**Files:** None modified — verification only.

**Step 1: Run unit tests with race detection**

Run: `go test ./cmd/sidecar/... -race -count=1`
Expected: All PASS

**Step 2: Run full test suite**

Run: `go test ./... -race -count=1`
Expected: All PASS

**Step 3: Run task gate**

Run: `./scripts/gates.sh task`
Expected: All PASS

**Step 4: Run sidecar Docker E2E**

Run: `./scripts/live_test_sidecar.sh`
Expected: 9/9 PASS — circuit breaker is transparent when broker is healthy

**Step 5: Run broker Docker E2E**

Run: `./scripts/live_test_docker.sh`
Expected: All PASS

**Step 6: Verify zero fmt.Print in sidecar**

Run: `grep -rn 'fmt\.Print' cmd/sidecar/ --include='*.go' | grep -v _test.go | grep -v metrics_test.go`
Expected: No output (clean)
