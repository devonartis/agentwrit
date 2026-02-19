# Sidecar Phase 2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add background auto-renewal and per-agent registration (lazy + BYOK) to the sidecar binary.

**Architecture:** The sidecar gains three new components: (1) a renewal goroutine that atomically swaps the bearer token via RWMutex, (2) an in-memory agent registry keyed by `agent_name:task_id`, and (3) a BYOK registration handler. The existing `POST /v1/token` handler is modified to lazy-register agents on first request. All agent state is ephemeral.

**Tech Stack:** Go 1.24, `crypto/ed25519`, `sync.RWMutex`, `encoding/hex`, `encoding/base64`, `net/http/httptest`

**Design Doc:** `docs/plans/2026-02-14-sidecar-phase2-design.md`

---

## Context for Implementers

### Existing Files (read before starting)

| File | LOC | Purpose |
|------|-----|---------|
| `cmd/sidecar/main.go` | 46 | Wiring: config → broker client → bootstrap → routes → serve |
| `cmd/sidecar/config.go` | 42 | `sidecarConfig` struct, `loadConfig()`, `envOr()` |
| `cmd/sidecar/bootstrap.go` | 78 | 4-step auto-activation, `sidecarState{sidecarToken, sidecarID, expiresIn}` |
| `cmd/sidecar/handler.go` | 258 | `tokenHandler`, `renewHandler`, `healthHandler`, scope matching helpers |
| `cmd/sidecar/broker_client.go` | 240 | `brokerClient` with `doJSON()` helper, 6 methods |
| `cmd/sidecar/integration_test.go` | 396 | Full end-to-end with in-process broker |

### Broker API Shapes (needed for new broker_client methods)

**GET /v1/challenge** → `{"nonce": "hex-string", "expires_in": 30}`

**POST /v1/register** request:
```json
{
  "launch_token": "64-char-hex",
  "nonce": "hex-from-challenge",
  "public_key": "base64-ed25519-pubkey-32bytes",
  "signature": "base64-ed25519-sig-of-nonce-bytes",
  "orch_id": "string",
  "task_id": "string",
  "requested_scope": ["action:resource:id"]
}
```
Response: `{"agent_id": "spiffe://...", "access_token": "jwt", "expires_in": 300}`

**POST /v1/admin/launch-tokens** request (admin Bearer required):
```json
{
  "agent_name": "string",
  "allowed_scope": ["action:resource:id"],
  "max_ttl": 600,
  "ttl": 600
}
```
Response (201): `{"launch_token": "64-char-hex", "expires_at": "RFC3339", "policy": {...}}`

### Key Constraint

The broker's `POST /v1/register` requires: a launch token (from admin API), a nonce (from GET /v1/challenge), the hex-decoded nonce signed with Ed25519, the base64-encoded public key, `orch_id`, `task_id`, and `requested_scope`. The nonce must be hex-decoded to bytes before signing — signing the hex string directly will fail.

---

## Task 1: Thread-Safe sidecarState + Config Update

**Files:**
- Modify: `cmd/sidecar/bootstrap.go:13-18` (sidecarState struct)
- Modify: `cmd/sidecar/config.go:8-14` (sidecarConfig struct)
- Modify: `cmd/sidecar/config.go:16-35` (loadConfig function)
- Create: `cmd/sidecar/state_test.go`
- Modify: `cmd/sidecar/config_test.go`

### What This Task Does

The sidecarState struct is currently a plain struct with no synchronization. The renewal goroutine (Task 2) will write to it while handlers read from it. We need `sync.RWMutex` to make it thread-safe. Also add the `RenewalBuffer` config field.

### Step 1: Write failing tests for thread-safe state

Create `cmd/sidecar/state_test.go`:

```go
package main

import (
	"sync"
	"testing"
)

func TestSidecarState_GetToken_ReturnsCurrentToken(t *testing.T) {
	s := &sidecarState{}
	s.setToken("tok-1", 900)
	if got := s.getToken(); got != "tok-1" {
		t.Errorf("getToken() = %q, want %q", got, "tok-1")
	}
}

func TestSidecarState_SetToken_UpdatesAtomically(t *testing.T) {
	s := &sidecarState{}
	s.setToken("tok-1", 900)

	s.setToken("tok-2", 600)
	if got := s.getToken(); got != "tok-2" {
		t.Errorf("getToken() after set = %q, want %q", got, "tok-2")
	}
	if got := s.getExpiresIn(); got != 600 {
		t.Errorf("getExpiresIn() = %d, want 600", got)
	}
}

func TestSidecarState_ConcurrentAccess(t *testing.T) {
	s := &sidecarState{}
	s.setToken("initial", 900)

	var wg sync.WaitGroup
	// 10 writers + 100 readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.setToken("updated", 300)
		}()
	}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.getToken()
			_ = s.getExpiresIn()
			_ = s.isHealthy()
		}()
	}
	wg.Wait()
	// No race detector panic = pass.
}

func TestSidecarState_Healthy_DefaultTrue(t *testing.T) {
	s := &sidecarState{}
	s.setToken("tok", 900)
	if !s.isHealthy() {
		t.Error("new state should be healthy")
	}
	s.setHealthy(false)
	if s.isHealthy() {
		t.Error("after setHealthy(false), should be unhealthy")
	}
}
```

### Step 2: Run tests to verify they fail

Run: `go test ./cmd/sidecar/ -short -run TestSidecarState -count=1 -v`
Expected: FAIL — `getToken`, `setToken`, `getExpiresIn`, `isHealthy`, `setHealthy` undefined.

### Step 3: Implement thread-safe sidecarState

Modify `cmd/sidecar/bootstrap.go` — replace the `sidecarState` struct:

```go
// sidecarState holds the result of a successful bootstrap sequence.
// All fields are protected by mu for concurrent access from the
// renewal goroutine (writer) and HTTP handlers (readers).
type sidecarState struct {
	mu           sync.RWMutex
	sidecarToken string
	sidecarID    string
	expiresIn    int
	healthy      bool
}

func (s *sidecarState) getToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sidecarToken
}

func (s *sidecarState) getExpiresIn() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.expiresIn
}

func (s *sidecarState) setToken(token string, expiresIn int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sidecarToken = token
	s.expiresIn = expiresIn
	s.healthy = true
}

func (s *sidecarState) isHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.healthy
}

func (s *sidecarState) setHealthy(h bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.healthy = h
}
```

Add `"sync"` to the import block in `bootstrap.go`.

Update the `bootstrap()` function return (line 73-78) to use `setToken`:

```go
	st := &sidecarState{
		sidecarID: resp.sidecarID,
	}
	st.setToken(resp.accessToken, resp.expiresIn)
	return st, nil
```

### Step 4: Update all callers to use accessor methods

In `cmd/sidecar/handler.go`:

- Line 82: `h.state.sidecarToken` → `h.state.getToken()`
- Line 164: `h.state != nil && h.state.sidecarToken != ""` → `h.state != nil && h.state.getToken() != ""`

### Step 5: Add RenewalBuffer to config

In `cmd/sidecar/config.go`, add to `sidecarConfig`:
```go
	RenewalBuffer float64
```

In `loadConfig()`, after the LogLevel line:
```go
	renewalRaw := envOr("AA_SIDECAR_RENEWAL_BUFFER", "0.8")
	renewalBuf := 0.8
	if v, err := strconv.ParseFloat(renewalRaw, 64); err == nil && v >= 0.5 && v <= 0.95 {
		renewalBuf = v
	}
	cfg.RenewalBuffer = renewalBuf
```

Add `"strconv"` to imports.

Add a config test in `cmd/sidecar/config_test.go`:

```go
func TestLoadConfig_RenewalBuffer(t *testing.T) {
	t.Setenv("AA_SIDECAR_RENEWAL_BUFFER", "0.7")
	cfg := loadConfig()
	if cfg.RenewalBuffer != 0.7 {
		t.Errorf("RenewalBuffer = %f, want 0.7", cfg.RenewalBuffer)
	}
}
```

### Step 6: Run all tests

Run: `go test ./cmd/sidecar/ -short -count=1 -race`
Expected: ALL PASS (including existing tests — the accessor change must not break them).

### Step 7: Commit

```bash
git add cmd/sidecar/bootstrap.go cmd/sidecar/state_test.go cmd/sidecar/config.go cmd/sidecar/config_test.go cmd/sidecar/handler.go
git commit -m "feat(sidecar): thread-safe sidecarState with RWMutex and renewal buffer config"
```

---

## Task 2: Background Renewal Goroutine

**Files:**
- Create: `cmd/sidecar/renewal.go`
- Create: `cmd/sidecar/renewal_test.go`

### What This Task Does

A goroutine that periodically renews the sidecar's bearer token at `renewalBuffer * expiresIn` seconds. Uses exponential backoff on failure (1s→2s→4s→...→30s). Sets `state.healthy = false` if the token actually expires.

### Step 1: Write failing tests

Create `cmd/sidecar/renewal_test.go`:

```go
package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// mockRenewer implements the renewFunc signature for testing.
type mockRenewer struct {
	callCount atomic.Int32
	failUntil int32
	returnTTL int
}

func (m *mockRenewer) renew(token string) (string, int, error) {
	n := m.callCount.Add(1)
	if n <= m.failUntil {
		return "", 0, fmt.Errorf("mock failure #%d", n)
	}
	return fmt.Sprintf("renewed-token-%d", n), m.returnTTL, nil
}

func TestStartRenewal_RenewsBeforeExpiry(t *testing.T) {
	state := &sidecarState{}
	state.setToken("initial-token", 1) // 1-second TTL

	m := &mockRenewer{returnTTL: 1}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go startRenewal(ctx, state, m.renew, 0.5) // renew at 50% = 500ms

	time.Sleep(1500 * time.Millisecond)
	cancel()

	if m.callCount.Load() < 1 {
		t.Error("expected at least 1 renewal call")
	}
	tok := state.getToken()
	if tok == "initial-token" {
		t.Error("token was not renewed")
	}
}

func TestStartRenewal_BackoffOnFailure(t *testing.T) {
	state := &sidecarState{}
	state.setToken("initial", 1) // 1-second TTL

	m := &mockRenewer{failUntil: 2, returnTTL: 1}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go startRenewal(ctx, state, m.renew, 0.1) // renew quickly

	time.Sleep(4 * time.Second)
	cancel()

	// Should have retried and eventually succeeded.
	if m.callCount.Load() < 3 {
		t.Errorf("expected at least 3 calls (2 fail + 1 success), got %d", m.callCount.Load())
	}
	tok := state.getToken()
	if tok == "initial" {
		t.Error("token should have been renewed after recovery")
	}
}

func TestStartRenewal_SetsUnhealthyOnExpiry(t *testing.T) {
	state := &sidecarState{}
	state.setToken("initial", 1) // 1-second TTL, will expire fast

	// Always fails — token will expire.
	alwaysFail := func(token string) (string, int, error) {
		return "", 0, fmt.Errorf("always fails")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go startRenewal(ctx, state, alwaysFail, 0.5)

	time.Sleep(2500 * time.Millisecond)
	cancel()

	if state.isHealthy() {
		t.Error("state should be unhealthy after token expiry")
	}
}

func TestStartRenewal_StopsOnContextCancel(t *testing.T) {
	state := &sidecarState{}
	state.setToken("initial", 100) // long TTL

	m := &mockRenewer{returnTTL: 100}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		startRenewal(ctx, state, m.renew, 0.8)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Goroutine exited, good.
	case <-time.After(2 * time.Second):
		t.Error("startRenewal did not exit after context cancel")
	}
}
```

### Step 2: Run tests to verify they fail

Run: `go test ./cmd/sidecar/ -short -run TestStartRenewal -count=1 -v`
Expected: FAIL — `startRenewal` undefined.

### Step 3: Implement renewal goroutine

Create `cmd/sidecar/renewal.go`:

```go
package main

import (
	"context"
	"fmt"
	"time"
)

// renewFunc is the function signature for renewing a token.
// Matches brokerClient.tokenRenew.
type renewFunc func(token string) (newToken string, expiresIn int, err error)

const maxBackoff = 30 * time.Second

// startRenewal runs a blocking loop that renews the sidecar's bearer token
// before it expires. It blocks until ctx is cancelled.
//
// renewalBuffer is the fraction of TTL at which to renew (e.g. 0.8 = renew
// at 80% of TTL). On failure, it retries with exponential backoff capped at
// maxBackoff. If the token expires (all retries failed), state is marked
// unhealthy. Auto-recovers when renewal eventually succeeds.
func startRenewal(ctx context.Context, state *sidecarState, renew renewFunc, renewalBuffer float64) {
	ttl := state.getExpiresIn()
	sleepDur := time.Duration(float64(ttl)*renewalBuffer) * time.Second
	if sleepDur < 1*time.Second {
		sleepDur = 1 * time.Second
	}

	// Track when the current token expires for health status.
	tokenDeadline := time.Now().Add(time.Duration(ttl) * time.Second)

	backoff := 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(sleepDur):
		}

		currentToken := state.getToken()
		newToken, newTTL, err := renew(currentToken)
		if err != nil {
			fmt.Printf("[sidecar] renewal failed: %v (retry in %v)\n", err, backoff)

			// Check if token has actually expired.
			if time.Now().After(tokenDeadline) {
				state.setHealthy(false)
				fmt.Println("[sidecar] token expired, marking unhealthy")
			}

			sleepDur = backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Success — update state and reset timers.
		state.setToken(newToken, newTTL)
		tokenDeadline = time.Now().Add(time.Duration(newTTL) * time.Second)
		sleepDur = time.Duration(float64(newTTL)*renewalBuffer) * time.Second
		if sleepDur < 1*time.Second {
			sleepDur = 1 * time.Second
		}
		backoff = 1 * time.Second

		fmt.Printf("[sidecar] token renewed, next in %v\n", sleepDur)
	}
}
```

### Step 4: Run tests

Run: `go test ./cmd/sidecar/ -short -run TestStartRenewal -count=1 -v -race`
Expected: ALL PASS.

### Step 5: Commit

```bash
git add cmd/sidecar/renewal.go cmd/sidecar/renewal_test.go
git commit -m "feat(sidecar): background token renewal goroutine with exponential backoff"
```

---

## Task 3: In-Memory Agent Registry

**Files:**
- Create: `cmd/sidecar/registry.go`
- Create: `cmd/sidecar/registry_test.go`

### What This Task Does

An in-memory store for registered agents, keyed by `agent_name:task_id`. Each entry holds the SPIFFE ID and optionally the Ed25519 keypair (nil for BYOK agents). Includes per-agent locking to prevent duplicate registrations on concurrent first requests.

### Step 1: Write failing tests

Create `cmd/sidecar/registry_test.go`:

```go
package main

import (
	"crypto/ed25519"
	"sync"
	"testing"
)

func TestAgentRegistry_StoreAndLookup(t *testing.T) {
	reg := newAgentRegistry()

	entry := &agentEntry{
		spiffeID: "spiffe://test/agent/orch/task/inst",
		pubKey:   make(ed25519.PublicKey, 32),
		privKey:  make(ed25519.PrivateKey, 64),
	}
	reg.store("reader:t-1", entry)

	got, ok := reg.lookup("reader:t-1")
	if !ok {
		t.Fatal("lookup returned not-found for stored entry")
	}
	if got.spiffeID != entry.spiffeID {
		t.Errorf("spiffeID = %q, want %q", got.spiffeID, entry.spiffeID)
	}
}

func TestAgentRegistry_LookupMissing(t *testing.T) {
	reg := newAgentRegistry()
	_, ok := reg.lookup("nonexistent")
	if ok {
		t.Error("lookup should return false for missing key")
	}
}

func TestAgentRegistry_BYOK_NilPrivateKey(t *testing.T) {
	reg := newAgentRegistry()

	entry := &agentEntry{
		spiffeID: "spiffe://test/agent/orch/task/inst",
		pubKey:   make(ed25519.PublicKey, 32),
		privKey:  nil, // BYOK — sidecar doesn't hold private key
	}
	reg.store("secure-bot:t-2", entry)

	got, ok := reg.lookup("secure-bot:t-2")
	if !ok {
		t.Fatal("BYOK entry not found")
	}
	if got.privKey != nil {
		t.Error("BYOK entry should have nil privKey")
	}
}

func TestAgentRegistry_ConcurrentAccess(t *testing.T) {
	reg := newAgentRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		key := "agent-" + string(rune('A'+i%26))
		go func() {
			defer wg.Done()
			reg.store(key, &agentEntry{spiffeID: "spiffe://test/" + key})
		}()
		go func() {
			defer wg.Done()
			reg.lookup(key)
		}()
	}
	wg.Wait()
	// No race detector panic = pass.
}

func TestAgentRegistry_GetOrLock_SerializesRegistration(t *testing.T) {
	reg := newAgentRegistry()

	// First call — no entry, should return nil + unlock func.
	entry, unlock := reg.getOrLock("reader:t-1")
	if entry != nil {
		t.Fatal("expected nil entry for first getOrLock")
	}

	// Store and release lock.
	reg.store("reader:t-1", &agentEntry{spiffeID: "spiffe://registered"})
	unlock()

	// Second call — entry exists, unlock is nil.
	entry2, unlock2 := reg.getOrLock("reader:t-1")
	if entry2 == nil {
		t.Fatal("expected non-nil entry for second getOrLock")
	}
	if entry2.spiffeID != "spiffe://registered" {
		t.Errorf("spiffeID = %q, want spiffe://registered", entry2.spiffeID)
	}
	if unlock2 != nil {
		t.Error("unlock should be nil when entry found")
	}
}
```

### Step 2: Run tests to verify they fail

Run: `go test ./cmd/sidecar/ -short -run TestAgentRegistry -count=1 -v`
Expected: FAIL — `newAgentRegistry`, `agentEntry`, etc. undefined.

### Step 3: Implement agent registry

Create `cmd/sidecar/registry.go`:

```go
package main

import (
	"crypto/ed25519"
	"sync"
	"time"
)

// agentEntry holds registration state for one agent.
type agentEntry struct {
	spiffeID     string
	pubKey       ed25519.PublicKey
	privKey      ed25519.PrivateKey // nil for BYOK agents
	registeredAt time.Time
}

// agentRegistry is an in-memory, ephemeral store of registered agents.
// It is safe for concurrent use.
type agentRegistry struct {
	mu      sync.RWMutex
	agents  map[string]*agentEntry
	locks   map[string]*sync.Mutex // per-agent registration locks
	locksMu sync.Mutex
}

// newAgentRegistry creates an empty agent registry.
func newAgentRegistry() *agentRegistry {
	return &agentRegistry{
		agents: make(map[string]*agentEntry),
		locks:  make(map[string]*sync.Mutex),
	}
}

// lookup returns the agent entry for the given key, or nil if not found.
func (r *agentRegistry) lookup(key string) (*agentEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.agents[key]
	return e, ok
}

// store adds or replaces an agent entry.
func (r *agentRegistry) store(key string, entry *agentEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[key] = entry
}

// getOrLock checks if an agent is already registered. If found, returns
// the entry and a nil unlock function. If NOT found, acquires a per-agent
// lock to serialize registration and returns nil entry + an unlock function.
// The caller MUST call unlock() after registration completes.
//
// This prevents duplicate challenge-response flows when concurrent
// POST /v1/token requests arrive for the same unregistered agent.
func (r *agentRegistry) getOrLock(key string) (*agentEntry, func()) {
	// Fast path: already registered.
	if entry, ok := r.lookup(key); ok {
		return entry, nil
	}

	// Get or create per-agent lock.
	r.locksMu.Lock()
	agentLock, exists := r.locks[key]
	if !exists {
		agentLock = &sync.Mutex{}
		r.locks[key] = agentLock
	}
	r.locksMu.Unlock()

	// Acquire per-agent lock.
	agentLock.Lock()

	// Double-check: another goroutine may have registered while we waited.
	if entry, ok := r.lookup(key); ok {
		agentLock.Unlock()
		return entry, nil
	}

	// Not registered — caller must register, then call unlock.
	return nil, func() { agentLock.Unlock() }
}
```

### Step 4: Run tests

Run: `go test ./cmd/sidecar/ -short -run TestAgentRegistry -count=1 -v -race`
Expected: ALL PASS.

### Step 5: Commit

```bash
git add cmd/sidecar/registry.go cmd/sidecar/registry_test.go
git commit -m "feat(sidecar): in-memory ephemeral agent registry with per-agent locking"
```

---

## Task 4: Broker Client — Challenge + Register Methods

**Files:**
- Modify: `cmd/sidecar/broker_client.go` (add 3 new methods)
- Modify: `cmd/sidecar/broker_client_test.go` (add 3 new tests)

### What This Task Does

Add `getChallenge()`, `createLaunchToken()`, and `registerAgent()` methods to `brokerClient`. These are needed by both lazy registration (Task 5) and BYOK (Task 6).

### Step 1: Write failing tests

Add to `cmd/sidecar/broker_client_test.go`:

```go
func TestBrokerClient_GetChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/challenge" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"nonce":      "abc123",
			"expires_in": 30,
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	nonce, err := bc.getChallenge()
	if err != nil {
		t.Fatalf("getChallenge() error: %v", err)
	}
	if nonce != "abc123" {
		t.Errorf("nonce = %q, want %q", nonce, "abc123")
	}
}

func TestBrokerClient_CreateLaunchToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/admin/launch-tokens" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer admin-jwt" {
			t.Error("missing admin bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"launch_token": "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	lt, err := bc.createLaunchToken("admin-jwt", "test-agent", []string{"read:data:*"}, 600)
	if err != nil {
		t.Fatalf("createLaunchToken() error: %v", err)
	}
	if lt == "" {
		t.Error("expected non-empty launch token")
	}
}

func TestBrokerClient_RegisterAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/register" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["launch_token"] == nil || body["public_key"] == nil {
			t.Error("missing required fields")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"agent_id":     "spiffe://test/agent/orch/task/inst",
			"access_token": "agent-jwt",
			"expires_in":   300,
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	agentID, err := bc.registerAgent("launch-token", "nonce-hex", "pubkey-b64", "sig-b64", "orch-1", "task-1", []string{"read:data:*"})
	if err != nil {
		t.Fatalf("registerAgent() error: %v", err)
	}
	if agentID != "spiffe://test/agent/orch/task/inst" {
		t.Errorf("agentID = %q, want spiffe://...", agentID)
	}
}
```

### Step 2: Run tests to verify they fail

Run: `go test ./cmd/sidecar/ -short -run "TestBrokerClient_(GetChallenge|CreateLaunchToken|RegisterAgent)" -count=1 -v`
Expected: FAIL — methods undefined.

### Step 3: Implement broker client methods

Add to `cmd/sidecar/broker_client.go`:

```go
// getChallenge fetches a nonce from the broker. Calls GET /v1/challenge.
func (c *brokerClient) getChallenge() (string, error) {
	resp, err := c.doJSON("GET", "/v1/challenge", nil, "")
	if err != nil {
		return "", fmt.Errorf("get challenge: %w", err)
	}

	nonce, ok := resp["nonce"].(string)
	if !ok || nonce == "" {
		return "", fmt.Errorf("get challenge: missing nonce in response")
	}
	return nonce, nil
}

// createLaunchToken creates a launch token via the admin API.
// Calls POST /v1/admin/launch-tokens with admin Bearer auth.
func (c *brokerClient) createLaunchToken(adminToken, agentName string, scope []string, ttl int) (string, error) {
	body, err := json.Marshal(map[string]any{
		"agent_name":    agentName,
		"allowed_scope": scope,
		"max_ttl":       ttl,
		"ttl":           ttl,
	})
	if err != nil {
		return "", fmt.Errorf("marshal launch token request: %w", err)
	}

	resp, err := c.doJSON("POST", "/v1/admin/launch-tokens", body, adminToken)
	if err != nil {
		return "", fmt.Errorf("create launch token: %w", err)
	}

	lt, ok := resp["launch_token"].(string)
	if !ok || lt == "" {
		return "", fmt.Errorf("create launch token: missing launch_token in response")
	}
	return lt, nil
}

// registerAgent registers an agent with the broker via challenge-response.
// Calls POST /v1/register.
func (c *brokerClient) registerAgent(launchToken, nonce, pubKeyB64, sigB64, orchID, taskID string, scope []string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"launch_token":   launchToken,
		"nonce":          nonce,
		"public_key":     pubKeyB64,
		"signature":      sigB64,
		"orch_id":        orchID,
		"task_id":        taskID,
		"requested_scope": scope,
	})
	if err != nil {
		return "", fmt.Errorf("marshal register request: %w", err)
	}

	resp, err := c.doJSON("POST", "/v1/register", body, "")
	if err != nil {
		return "", fmt.Errorf("register agent: %w", err)
	}

	agentID, ok := resp["agent_id"].(string)
	if !ok || agentID == "" {
		return "", fmt.Errorf("register agent: missing agent_id in response")
	}
	return agentID, nil
}
```

### Step 4: Run tests

Run: `go test ./cmd/sidecar/ -short -run "TestBrokerClient" -count=1 -v`
Expected: ALL PASS (new + existing).

### Step 5: Commit

```bash
git add cmd/sidecar/broker_client.go cmd/sidecar/broker_client_test.go
git commit -m "feat(sidecar): add challenge, launch-token, and register broker client methods"
```

---

## Task 5: Lazy Registration in tokenHandler

**Files:**
- Modify: `cmd/sidecar/handler.go:25-93` (tokenHandler)
- Modify: `cmd/sidecar/handler_test.go` (add lazy registration tests)

### What This Task Does

Modify `tokenHandler` to check the agent registry before token exchange. If the agent isn't registered, run the full challenge-response flow transparently (generate keypair, get nonce, sign, register at broker), cache the result, then proceed with token exchange using the SPIFFE agent_id.

### Step 1: Write failing tests for lazy registration

Add to `cmd/sidecar/handler_test.go`:

```go
func TestTokenHandler_LazyRegistration_FirstRequest(t *testing.T) {
	// Mock broker that handles: admin auth, launch token, challenge, register, token exchange.
	callLog := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callLog = append(callLog, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v1/admin/auth":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "admin-jwt"})
		case "/v1/admin/launch-tokens":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"launch_token": "lt-64charhex000000000000000000000000000000000000000000000000000"})
		case "/v1/challenge":
			json.NewEncoder(w).Encode(map[string]any{"nonce": "aabbccdd", "expires_in": 30})
		case "/v1/register":
			json.NewEncoder(w).Encode(map[string]any{
				"agent_id":     "spiffe://test/agent/orch/task/inst",
				"access_token": "agent-registration-jwt",
				"expires_in":   300,
			})
		case "/v1/token/exchange":
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "exchanged-jwt",
				"expires_in":   300,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	state := &sidecarState{}
	state.setToken("sidecar-jwt", 900)
	reg := newAgentRegistry()

	th := newTokenHandler(bc, state, []string{"read:data:*"}, reg, "test-secret")

	body, _ := json.Marshal(map[string]any{
		"agent_name": "reader",
		"task_id":    "t-1",
		"scope":      []string{"read:data:*"},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify the agent was cached.
	entry, ok := reg.lookup("reader:t-1")
	if !ok {
		t.Fatal("agent should be cached in registry after lazy registration")
	}
	if entry.spiffeID != "spiffe://test/agent/orch/task/inst" {
		t.Errorf("cached spiffeID = %q", entry.spiffeID)
	}
}

func TestTokenHandler_LazyRegistration_SecondRequestSkipsRegistration(t *testing.T) {
	exchangeCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/token/exchange" {
			exchangeCalls++
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "exchanged-jwt",
				"expires_in":   300,
			})
			return
		}
		// Any other call means lazy registration happened (bad for second request).
		t.Errorf("unexpected broker call on second request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	state := &sidecarState{}
	state.setToken("sidecar-jwt", 900)
	reg := newAgentRegistry()

	// Pre-populate registry (simulate first request already completed).
	reg.store("reader:t-1", &agentEntry{spiffeID: "spiffe://cached/agent"})

	th := newTokenHandler(bc, state, []string{"read:data:*"}, reg, "test-secret")

	body, _ := json.Marshal(map[string]any{
		"agent_name": "reader",
		"task_id":    "t-1",
		"scope":      []string{"read:data:*"},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if exchangeCalls != 1 {
		t.Errorf("expected exactly 1 exchange call, got %d", exchangeCalls)
	}
}

func TestTokenHandler_LazyRegistration_BrokerFailure502(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/admin/auth" {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"error": "broker down"})
			return
		}
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	state := &sidecarState{}
	state.setToken("sidecar-jwt", 900)
	reg := newAgentRegistry()

	th := newTokenHandler(bc, state, []string{"read:data:*"}, reg, "test-secret")

	body, _ := json.Marshal(map[string]any{
		"agent_name": "reader",
		"task_id":    "t-1",
		"scope":      []string{"read:data:*"},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d: %s", rr.Code, rr.Body.String())
	}
}
```

### Step 2: Run tests to verify they fail

Run: `go test ./cmd/sidecar/ -short -run "TestTokenHandler_Lazy" -count=1 -v`
Expected: FAIL — `newTokenHandler` signature changed (needs registry + secret params).

### Step 3: Implement lazy registration

Modify `cmd/sidecar/handler.go`. Update `tokenHandler` struct and constructor:

```go
type tokenHandler struct {
	broker       *brokerClient
	state        *sidecarState
	scopeCeiling []string
	registry     *agentRegistry
	adminSecret  string
}

func newTokenHandler(bc *brokerClient, state *sidecarState, ceiling []string, reg *agentRegistry, adminSecret string) *tokenHandler {
	return &tokenHandler{
		broker:       bc,
		state:        state,
		scopeCeiling: ceiling,
		registry:     reg,
		adminSecret:  adminSecret,
	}
}
```

Add the lazy registration method and update ServeHTTP. Replace lines 75-86 (the agentID construction + token exchange) with:

```go
	// Resolve agent identity: check registry or lazy-register.
	agentKey := req.AgentName
	if req.TaskID != "" {
		agentKey = req.AgentName + ":" + req.TaskID
	}

	agentID, err := h.resolveAgent(agentKey, req.AgentName, req.TaskID, req.Scope)
	if err != nil {
		writeError(w, http.StatusBadGateway, "agent registration failed: "+err.Error())
		return
	}

	// Delegate to broker token exchange using the agent's SPIFFE ID.
	exResp, err := h.broker.tokenExchange(h.state.getToken(), agentID, req.Scope, ttl)
	if err != nil {
		writeError(w, http.StatusBadGateway, "broker token exchange failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": exResp.AccessToken,
		"expires_in":   exResp.ExpiresIn,
		"scope":        req.Scope,
		"agent_id":     agentID,
	})
```

Add the `resolveAgent` method and the `lazyRegister` helper:

```go
// resolveAgent returns the SPIFFE agent_id for the given key. If the agent
// is not in the registry, it performs lazy registration via challenge-response.
func (h *tokenHandler) resolveAgent(key, agentName, taskID string, scope []string) (string, error) {
	entry, unlock := h.registry.getOrLock(key)
	if entry != nil {
		return entry.spiffeID, nil
	}
	defer unlock()

	// Lazy registration: full challenge-response flow.
	spiffeID, pubKey, privKey, err := h.lazyRegister(agentName, taskID, scope)
	if err != nil {
		return "", err
	}

	h.registry.store(key, &agentEntry{
		spiffeID:     spiffeID,
		pubKey:       pubKey,
		privKey:      privKey,
		registeredAt: time.Now(),
	})

	return spiffeID, nil
}

// lazyRegister performs the full broker registration flow:
// 1. Admin auth → 2. Create launch token → 3. Get challenge → 4. Sign → 5. Register
func (h *tokenHandler) lazyRegister(agentName, taskID string, scope []string) (string, ed25519.PublicKey, ed25519.PrivateKey, error) {
	// Step 1: Admin auth.
	adminToken, err := h.broker.adminAuth(h.adminSecret)
	if err != nil {
		return "", nil, nil, fmt.Errorf("admin auth: %w", err)
	}

	// Step 2: Create launch token for this agent.
	launchToken, err := h.broker.createLaunchToken(adminToken, agentName, scope, 600)
	if err != nil {
		return "", nil, nil, fmt.Errorf("create launch token: %w", err)
	}

	// Step 3: Get challenge nonce.
	nonce, err := h.broker.getChallenge()
	if err != nil {
		return "", nil, nil, fmt.Errorf("get challenge: %w", err)
	}

	// Step 4: Generate Ed25519 keypair and sign the nonce.
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, nil, fmt.Errorf("generate keypair: %w", err)
	}

	nonceBytes, err := hex.DecodeString(nonce)
	if err != nil {
		return "", nil, nil, fmt.Errorf("decode nonce: %w", err)
	}
	sig := ed25519.Sign(privKey, nonceBytes)

	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	// Step 5: Register at broker.
	orchID := agentName
	tid := taskID
	if tid == "" {
		tid = "default"
	}

	agentID, err := h.broker.registerAgent(launchToken, nonce, pubKeyB64, sigB64, orchID, tid, scope)
	if err != nil {
		return "", nil, nil, fmt.Errorf("register: %w", err)
	}

	fmt.Printf("[sidecar] lazy-registered agent %s → %s\n", agentName, agentID)
	return agentID, pubKey, privKey, nil
}
```

Add imports at top of handler.go: `"crypto/ed25519"`, `"crypto/rand"`, `"encoding/base64"`, `"encoding/hex"`, `"fmt"`, `"time"`.

### Step 4: Fix existing tests

Existing `tokenHandler` tests use the old constructor signature `newTokenHandler(bc, state, ceiling)`. Update ALL existing test calls to: `newTokenHandler(bc, state, ceiling, newAgentRegistry(), "test-secret")`.

Also, pre-populate the registry in existing tests so they don't trigger lazy registration:

```go
reg := newAgentRegistry()
reg.store("data-reader:task-789", &agentEntry{spiffeID: "data-reader:task-789"})
// ... use reg in constructor
```

### Step 5: Run all sidecar tests

Run: `go test ./cmd/sidecar/ -short -count=1 -v -race`
Expected: ALL PASS.

### Step 6: Commit

```bash
git add cmd/sidecar/handler.go cmd/sidecar/handler_test.go
git commit -m "feat(sidecar): lazy agent registration on first POST /v1/token request"
```

---

## Task 6: BYOK Register Handler + Challenge Proxy

**Files:**
- Create: `cmd/sidecar/register_handler.go`
- Create: `cmd/sidecar/register_handler_test.go`

### What This Task Does

Two new endpoints: `GET /v1/challenge` (proxies broker challenge for BYOK developers) and `POST /v1/register` (explicit registration with developer-provided public key and signature).

### Step 1: Write failing tests

Create `cmd/sidecar/register_handler_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChallengeHandler_ProxiesBrokerChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/challenge" || r.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"nonce": "deadbeef", "expires_in": 30})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	ch := newChallengeProxyHandler(bc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/challenge", nil)
	ch.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["nonce"] != "deadbeef" {
		t.Errorf("nonce = %v, want deadbeef", resp["nonce"])
	}
}

func TestChallengeHandler_MethodNotAllowed(t *testing.T) {
	bc := newBrokerClient("http://unused")
	ch := newChallengeProxyHandler(bc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/challenge", nil)
	ch.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestRegisterHandler_BYOK_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/admin/auth":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "admin-jwt"})
		case "/v1/admin/launch-tokens":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"launch_token": "lt-64hex000000000000000000000000000000000000000000000000000000000"})
		case "/v1/register":
			json.NewEncoder(w).Encode(map[string]any{
				"agent_id":     "spiffe://test/agent/orch/task/inst",
				"access_token": "agent-jwt",
				"expires_in":   300,
			})
		default:
			t.Errorf("unexpected call: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	reg := newAgentRegistry()
	rh := newRegisterHandler(bc, reg, "admin-secret", []string{"read:data:*"})

	body, _ := json.Marshal(map[string]any{
		"agent_name": "secure-bot",
		"task_id":    "t-2",
		"public_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", // 32 zero bytes b64
		"signature":  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==",
		"nonce":      "aabbccdd",
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rh.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify cached in registry.
	entry, ok := reg.lookup("secure-bot:t-2")
	if !ok {
		t.Fatal("BYOK agent should be cached in registry")
	}
	if entry.privKey != nil {
		t.Error("BYOK agent should have nil privKey")
	}
}

func TestRegisterHandler_MissingFields_400(t *testing.T) {
	bc := newBrokerClient("http://unused")
	reg := newAgentRegistry()
	rh := newRegisterHandler(bc, reg, "secret", []string{"read:data:*"})

	body, _ := json.Marshal(map[string]any{
		"agent_name": "bot",
		// Missing public_key, signature, nonce.
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rh.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRegisterHandler_MethodNotAllowed(t *testing.T) {
	bc := newBrokerClient("http://unused")
	reg := newAgentRegistry()
	rh := newRegisterHandler(bc, reg, "secret", []string{"read:data:*"})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/register", nil)
	rh.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}
```

### Step 2: Run tests to verify they fail

Run: `go test ./cmd/sidecar/ -short -run "Test(Challenge|Register)Handler" -count=1 -v`
Expected: FAIL — types undefined.

### Step 3: Implement handlers

Create `cmd/sidecar/register_handler.go`:

```go
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ---------------------------------------------------------------------------
// challengeProxyHandler — GET /v1/challenge
// ---------------------------------------------------------------------------

type challengeProxyHandler struct {
	broker *brokerClient
}

func newChallengeProxyHandler(bc *brokerClient) *challengeProxyHandler {
	return &challengeProxyHandler{broker: bc}
}

func (h *challengeProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET is allowed")
		return
	}

	nonce, err := h.broker.getChallenge()
	if err != nil {
		writeError(w, http.StatusBadGateway, "broker challenge unavailable: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nonce":      nonce,
		"expires_in": 30,
	})
}

// ---------------------------------------------------------------------------
// registerHandler — POST /v1/register (BYOK)
// ---------------------------------------------------------------------------

type registerReq struct {
	AgentName string `json:"agent_name"`
	TaskID    string `json:"task_id"`
	PublicKey string `json:"public_key"` // base64-encoded Ed25519 public key
	Signature string `json:"signature"`  // base64-encoded Ed25519 signature of nonce
	Nonce     string `json:"nonce"`      // hex nonce from GET /v1/challenge
}

type registerHandler struct {
	broker       *brokerClient
	registry     *agentRegistry
	adminSecret  string
	scopeCeiling []string
}

func newRegisterHandler(bc *brokerClient, reg *agentRegistry, adminSecret string, ceiling []string) *registerHandler {
	return &registerHandler{
		broker:       bc,
		registry:     reg,
		adminSecret:  adminSecret,
		scopeCeiling: ceiling,
	}
}

func (h *registerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST is allowed")
		return
	}

	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	// Validate required fields.
	if req.AgentName == "" {
		writeError(w, http.StatusBadRequest, "agent_name is required")
		return
	}
	if req.PublicKey == "" || req.Signature == "" || req.Nonce == "" {
		writeError(w, http.StatusBadRequest, "public_key, signature, and nonce are required")
		return
	}

	// Validate public key is valid base64 and correct size.
	pubKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		writeError(w, http.StatusBadRequest, "invalid public key: must be 32-byte Ed25519 key, base64-encoded")
		return
	}

	// Build agent key.
	agentKey := req.AgentName
	if req.TaskID != "" {
		agentKey = req.AgentName + ":" + req.TaskID
	}

	// Check if already registered.
	if entry, ok := h.registry.lookup(agentKey); ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"agent_id":  entry.spiffeID,
			"cached":    true,
		})
		return
	}

	// Admin auth to get launch token for this agent.
	adminToken, err := h.broker.adminAuth(h.adminSecret)
	if err != nil {
		writeError(w, http.StatusBadGateway, "admin auth failed: "+err.Error())
		return
	}

	launchToken, err := h.broker.createLaunchToken(adminToken, req.AgentName, h.scopeCeiling, 600)
	if err != nil {
		writeError(w, http.StatusBadGateway, "create launch token failed: "+err.Error())
		return
	}

	// Register at broker — pass through the developer's nonce, pubkey, signature.
	orchID := req.AgentName
	taskID := req.TaskID
	if taskID == "" {
		taskID = "default"
	}

	agentID, err := h.broker.registerAgent(launchToken, req.Nonce, req.PublicKey, req.Signature, orchID, taskID, h.scopeCeiling)
	if err != nil {
		writeError(w, http.StatusBadGateway, "broker registration failed: "+err.Error())
		return
	}

	// Cache in registry — BYOK so privKey is nil.
	h.registry.store(agentKey, &agentEntry{
		spiffeID:     agentID,
		pubKey:       ed25519.PublicKey(pubKeyBytes),
		privKey:      nil,
		registeredAt: time.Now(),
	})

	fmt.Printf("[sidecar] BYOK registered agent %s → %s\n", req.AgentName, agentID)

	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":  agentID,
	})
}
```

### Step 4: Run tests

Run: `go test ./cmd/sidecar/ -short -run "Test(Challenge|Register)Handler" -count=1 -v -race`
Expected: ALL PASS.

### Step 5: Commit

```bash
git add cmd/sidecar/register_handler.go cmd/sidecar/register_handler_test.go
git commit -m "feat(sidecar): BYOK registration handler and challenge proxy endpoint"
```

---

## Task 7: Update healthHandler + Main.go Wiring

**Files:**
- Modify: `cmd/sidecar/handler.go:143-171` (healthHandler)
- Modify: `cmd/sidecar/main.go` (wire registry, renewal, new routes)
- Modify: `cmd/sidecar/handler_test.go` (update health test)

### What This Task Does

Update health endpoint to include renewal status (`healthy` field). Wire all new components in `main.go`: create registry, start renewal goroutine, register new routes.

### Step 1: Update healthHandler

Modify the healthHandler in `handler.go` to use `isHealthy()`:

```go
func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET is allowed")
		return
	}

	healthy := h.state != nil && h.state.isHealthy()
	connected := h.state != nil && h.state.getToken() != ""

	status := "ok"
	httpStatus := http.StatusOK
	if !healthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	writeJSON(w, httpStatus, map[string]any{
		"status":           status,
		"broker_connected": connected,
		"healthy":          healthy,
		"scope_ceiling":    h.scopeCeiling,
	})
}
```

### Step 2: Update main.go

Replace `cmd/sidecar/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := loadConfig()

	// Validate required config.
	if cfg.AdminSecret == "" {
		fmt.Fprintln(os.Stderr, "FATAL: AA_ADMIN_SECRET must be set")
		os.Exit(1)
	}
	if len(cfg.ScopeCeiling) == 0 {
		fmt.Fprintln(os.Stderr, "FATAL: AA_SIDECAR_SCOPE_CEILING must be set")
		os.Exit(1)
	}

	bc := newBrokerClient(cfg.BrokerURL)

	fmt.Printf("[sidecar] starting, broker=%s, scope_ceiling=%v\n", cfg.BrokerURL, cfg.ScopeCeiling)

	state, err := bootstrap(bc, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	// Start background renewal goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go startRenewal(ctx, state, bc.tokenRenew, cfg.RenewalBuffer)
	fmt.Printf("[sidecar] renewal goroutine started (buffer=%.0f%%)\n", cfg.RenewalBuffer*100)

	// Create agent registry (ephemeral, in-memory).
	registry := newAgentRegistry()

	// Set up routes.
	mux := http.NewServeMux()
	mux.Handle("/v1/token", newTokenHandler(bc, state, cfg.ScopeCeiling, registry, cfg.AdminSecret))
	mux.Handle("/v1/token/renew", newRenewHandler(bc))
	mux.Handle("/v1/health", newHealthHandler(state, cfg.ScopeCeiling))
	mux.Handle("/v1/challenge", newChallengeProxyHandler(bc))
	mux.Handle("/v1/register", newRegisterHandler(bc, registry, cfg.AdminSecret, cfg.ScopeCeiling))

	addr := ":" + cfg.Port
	fmt.Printf("[sidecar] ready on %s, sidecar_id=%s\n", addr, state.sidecarID)

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("[sidecar] shutting down...")
		cancel()
	}()

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}
```

### Step 3: Run all tests

Run: `go test ./cmd/sidecar/ -short -count=1 -v -race`
Expected: ALL PASS.

### Step 4: Build binary

Run: `go build ./cmd/sidecar/`
Expected: Compiles with no errors.

### Step 5: Commit

```bash
git add cmd/sidecar/main.go cmd/sidecar/handler.go cmd/sidecar/handler_test.go
git commit -m "feat(sidecar): wire renewal goroutine, registry, and new routes in main"
```

---

## Task 8: Extended Integration Test

**Files:**
- Modify: `cmd/sidecar/integration_test.go`

### What This Task Does

Extend `TestIntegration_DeveloperFlow` to cover Phase 2 features: lazy registration, cached second request, BYOK flow, and auto-renewal.

### Step 1: Update the integration test

The existing test manually registers an agent at the broker before using the sidecar. Phase 2's lazy registration means the sidecar should handle registration itself. Rewrite the test to verify this.

Add a new test function alongside the existing one:

```go
func TestIntegration_Phase2_LazyRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	origTimeout := defaultHealthTimeout
	defaultHealthTimeout = 5 * time.Second
	defer func() { defaultHealthTimeout = origTimeout }()

	const adminSecret = "integration-test-secret"

	// Step 1: Start broker.
	broker := startTestBroker(t, adminSecret)
	defer broker.Close()

	// Step 2: Bootstrap sidecar.
	bc := newBrokerClient(broker.URL)
	sidecarCfg := sidecarConfig{
		AdminSecret:   adminSecret,
		ScopeCeiling:  []string{"read:data:*"},
		RenewalBuffer: 0.8,
	}

	state, err := bootstrap(bc, sidecarCfg)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	registry := newAgentRegistry()
	th := newTokenHandler(bc, state, sidecarCfg.ScopeCeiling, registry, adminSecret)

	// Step 3: First POST /v1/token — should trigger lazy registration.
	body1, _ := json.Marshal(map[string]any{
		"agent_name": "lazy-agent",
		"task_id":    "task-1",
		"scope":      []string{"read:data:*"},
	})

	rr1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d: %s", rr1.Code, rr1.Body.String())
	}

	var resp1 map[string]any
	json.NewDecoder(rr1.Body).Decode(&resp1)

	token1 := resp1["access_token"].(string)
	agentID := resp1["agent_id"].(string)
	t.Logf("lazy registration: agent_id=%s, token_len=%d", agentID, len(token1))

	if agentID == "" {
		t.Fatal("agent_id should be non-empty (SPIFFE ID from broker)")
	}

	// Verify agent is in registry.
	entry, ok := registry.lookup("lazy-agent:task-1")
	if !ok {
		t.Fatal("agent should be cached in registry")
	}
	if entry.spiffeID != agentID {
		t.Errorf("cached spiffeID = %q, want %q", entry.spiffeID, agentID)
	}
	if entry.privKey == nil {
		t.Error("lazy-registered agent should have sidecar-managed keypair")
	}

	// Validate token at broker.
	valResult := brokerValidateToken(t, broker.URL, token1)
	if valid, _ := valResult["valid"].(bool); !valid {
		t.Fatalf("broker says token is invalid: %v", valResult)
	}

	// Step 4: Second request — should use cached agent (no re-registration).
	body2, _ := json.Marshal(map[string]any{
		"agent_name": "lazy-agent",
		"task_id":    "task-1",
		"scope":      []string{"read:data:*"},
	})

	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var resp2 map[string]any
	json.NewDecoder(rr2.Body).Decode(&resp2)
	if resp2["agent_id"] != agentID {
		t.Errorf("second request agent_id = %v, want %v", resp2["agent_id"], agentID)
	}
	t.Log("second request used cached registration (no re-register)")

	// Step 5: Scope escalation still denied.
	bodyEsc, _ := json.Marshal(map[string]any{
		"agent_name": "lazy-agent",
		"task_id":    "task-1",
		"scope":      []string{"write:data:*"},
	})

	rrEsc := httptest.NewRecorder()
	reqEsc := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(bodyEsc))
	reqEsc.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rrEsc, reqEsc)

	if rrEsc.Code != http.StatusForbidden {
		t.Errorf("scope escalation: expected 403, got %d", rrEsc.Code)
	}
}

func TestIntegration_Phase2_BYOKRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	origTimeout := defaultHealthTimeout
	defaultHealthTimeout = 5 * time.Second
	defer func() { defaultHealthTimeout = origTimeout }()

	const adminSecret = "integration-test-secret"

	broker := startTestBroker(t, adminSecret)
	defer broker.Close()

	bc := newBrokerClient(broker.URL)
	sidecarCfg := sidecarConfig{
		AdminSecret:   adminSecret,
		ScopeCeiling:  []string{"read:data:*"},
		RenewalBuffer: 0.8,
	}

	state, err := bootstrap(bc, sidecarCfg)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	registry := newAgentRegistry()

	// Step 1: Get challenge via sidecar proxy.
	ch := newChallengeProxyHandler(bc)
	rrCh := httptest.NewRecorder()
	reqCh := httptest.NewRequest(http.MethodGet, "/v1/challenge", nil)
	ch.ServeHTTP(rrCh, reqCh)

	if rrCh.Code != http.StatusOK {
		t.Fatalf("challenge: expected 200, got %d", rrCh.Code)
	}
	var chResp map[string]any
	json.NewDecoder(rrCh.Body).Decode(&chResp)
	nonce := chResp["nonce"].(string)

	// Step 2: Developer signs the nonce with their own key.
	devPub, devPriv, _ := ed25519.GenerateKey(rand.Reader)
	nonceBytes, _ := hex.DecodeString(nonce)
	sig := ed25519.Sign(devPriv, nonceBytes)

	// Step 3: Register via sidecar BYOK endpoint.
	rh := newRegisterHandler(bc, registry, adminSecret, sidecarCfg.ScopeCeiling)

	regBody, _ := json.Marshal(map[string]any{
		"agent_name": "byok-agent",
		"task_id":    "task-byok",
		"public_key": base64.StdEncoding.EncodeToString(devPub),
		"signature":  base64.StdEncoding.EncodeToString(sig),
		"nonce":      nonce,
	})

	rrReg := httptest.NewRecorder()
	reqReg := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(regBody))
	reqReg.Header.Set("Content-Type", "application/json")
	rh.ServeHTTP(rrReg, reqReg)

	if rrReg.Code != http.StatusOK {
		t.Fatalf("register: expected 200, got %d: %s", rrReg.Code, rrReg.Body.String())
	}

	var regResp map[string]any
	json.NewDecoder(rrReg.Body).Decode(&regResp)
	agentID := regResp["agent_id"].(string)
	t.Logf("BYOK registered: agent_id=%s", agentID)

	// Verify BYOK entry in registry (nil privKey).
	entry, ok := registry.lookup("byok-agent:task-byok")
	if !ok {
		t.Fatal("BYOK agent should be in registry")
	}
	if entry.privKey != nil {
		t.Error("BYOK entry should have nil privKey")
	}

	// Step 4: Token request using cached BYOK registration.
	th := newTokenHandler(bc, state, sidecarCfg.ScopeCeiling, registry, adminSecret)

	tokBody, _ := json.Marshal(map[string]any{
		"agent_name": "byok-agent",
		"task_id":    "task-byok",
		"scope":      []string{"read:data:*"},
	})

	rrTok := httptest.NewRecorder()
	reqTok := httptest.NewRequest(http.MethodPost, "/v1/token", bytes.NewReader(tokBody))
	reqTok.Header.Set("Content-Type", "application/json")
	th.ServeHTTP(rrTok, reqTok)

	if rrTok.Code != http.StatusOK {
		t.Fatalf("token after BYOK: expected 200, got %d: %s", rrTok.Code, rrTok.Body.String())
	}

	var tokResp map[string]any
	json.NewDecoder(rrTok.Body).Decode(&tokResp)
	accessToken := tokResp["access_token"].(string)

	// Validate at broker.
	valResult := brokerValidateToken(t, broker.URL, accessToken)
	if valid, _ := valResult["valid"].(bool); !valid {
		t.Fatalf("broker says BYOK token invalid: %v", valResult)
	}
	t.Log("BYOK token validated at broker")
}
```

Also update the existing `TestIntegration_DeveloperFlow` to pass the new `newTokenHandler` signature (add `newAgentRegistry()` and `adminSecret`).

### Step 2: Run integration tests

Run: `go test ./cmd/sidecar/ -run "TestIntegration" -count=1 -v -race`
Expected: ALL PASS (existing + new integration tests).

### Step 3: Run all tests (unit + integration)

Run: `go test ./cmd/sidecar/ -count=1 -v -race`
Expected: ALL PASS.

### Step 4: Commit

```bash
git add cmd/sidecar/integration_test.go
git commit -m "test(sidecar): integration tests for lazy registration and BYOK flow"
```

---

## Task 9: Documentation Updates

**Files:**
- Modify: `docs/USER_GUIDE.md`
- Modify: `docs/DEVELOPER_GUIDE.md`
- Modify: `docs/API_REFERENCE.md`
- Modify: `CHANGELOG.md`

### What This Task Does

Update all four documentation files to reflect Phase 2 changes: new endpoints, lazy registration, BYOK flow, auto-renewal.

### Step 1: Update USER_GUIDE.md

Add to the "Developer Quick Start (Sidecar)" section:

- Note that agents are **automatically registered** on first token request (no setup needed)
- Add BYOK flow for advanced users: `GET /v1/challenge` → sign → `POST /v1/register`
- Note auto-renewal (sidecar handles its own token lifecycle)

### Step 2: Update DEVELOPER_GUIDE.md

Update the "5c. Sidecar Architecture" section:

- Add agent registry component description
- Add renewal goroutine description
- Update architecture diagram to show lazy registration flow
- Add BYOK registration flow

### Step 3: Update API_REFERENCE.md

Add to the "Sidecar API" section:

- `GET /v1/challenge` — Proxy broker challenge (for BYOK)
- `POST /v1/register` — Explicit BYOK registration
- Update `POST /v1/token` to note lazy registration behavior and `agent_id` in response
- Update `GET /v1/health` to document `healthy` and `status: "degraded"` states

### Step 4: Update CHANGELOG.md

Add under `[Unreleased]`:

```markdown
### Added
- Background auto-renewal goroutine for sidecar bearer token (80% TTL default)
- Per-agent registration: lazy on first `POST /v1/token` with sidecar-managed Ed25519 keypairs
- BYOK registration: `GET /v1/challenge` + `POST /v1/register` for developer-provided keys
- In-memory agent registry with per-agent locking for concurrent safety
- Health endpoint now reports `status: "degraded"` when renewal fails
- `AA_SIDECAR_RENEWAL_BUFFER` environment variable (default 0.8)
```

### Step 5: Commit

```bash
git add docs/USER_GUIDE.md docs/DEVELOPER_GUIDE.md docs/API_REFERENCE.md CHANGELOG.md
git commit -m "docs(sidecar): Phase 2 — lazy registration, BYOK, auto-renewal documentation"
```

---

## Task 10: Run All Gates

**Files:** None (verification only)

### Step 1: Unit tests

Run: `go test ./... -short -count=1 -race`
Expected: ALL PASS.

### Step 2: Integration tests

Run: `go test ./cmd/sidecar/ -count=1 -v -race`
Expected: ALL PASS (including new Phase 2 integration tests).

### Step 3: Build

Run: `go build ./...`
Expected: Compiles with no errors.

### Step 4: Lint (if available)

Run: `golangci-lint run ./...`
Expected: No errors.

### Step 5: Task gate

Run: `./scripts/gates.sh task`
Expected: ALL PASS.

### Step 6: Module gate

Run: `./scripts/gates.sh module`
Expected: ALL PASS.

### Step 7: Report results

List all test counts, any warnings, pass/fail status for each gate.

---

**END OF PLAN**
