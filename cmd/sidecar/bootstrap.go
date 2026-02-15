package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
)

// defaultHealthTimeout is the maximum time bootstrap will wait for the broker
// health endpoint to respond. Tests override this to avoid long waits.
var defaultHealthTimeout = 30 * time.Second

// sidecarState holds the result of a successful bootstrap sequence.
// Fields guarded by mu are updated by the renewal goroutine (writer) and
// read by HTTP handlers (readers), so all access goes through accessors.
type sidecarState struct {
	mu           sync.RWMutex
	sidecarToken string
	sidecarID    string
	expiresIn    int
	healthy      bool
	lastRenewal  time.Time
	startTime    time.Time
}

// getToken returns the current sidecar bearer token (read-locked).
func (s *sidecarState) getToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sidecarToken
}

// getExpiresIn returns the current token TTL in seconds (read-locked).
func (s *sidecarState) getExpiresIn() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.expiresIn
}

// setToken atomically updates the bearer token, TTL, and marks the sidecar
// as healthy (write-locked).
func (s *sidecarState) setToken(token string, expiresIn int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sidecarToken = token
	s.expiresIn = expiresIn
	s.healthy = true
	s.lastRenewal = time.Now()
}

// isHealthy reports whether the sidecar is in a healthy state (read-locked).
func (s *sidecarState) isHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.healthy
}

// setHealthy sets the sidecar health flag (write-locked).
func (s *sidecarState) setHealthy(h bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.healthy = h
}

// getLastRenewal returns when the token was last renewed (read-locked).
func (s *sidecarState) getLastRenewal() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastRenewal
}

// getStartTime returns the sidecar start time (read-locked).
func (s *sidecarState) getStartTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startTime
}

// waitForBroker retries the broker health check until it succeeds or the
// timeout elapses. Returns nil on success, or an error if the broker did
// not become ready in time.
func waitForBroker(bc *brokerClient, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := bc.healthCheck(); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("broker not ready within %v", timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// bootstrap executes the 4-step auto-activation sequence:
//  1. Wait for broker health (retry loop with timeout)
//  2. Authenticate as admin
//  3. Create sidecar activation token
//  4. Activate sidecar (single-use exchange)
//
// On success it returns a sidecarState containing the bearer token, sidecar
// ID, and TTL. Any step failure aborts the sequence and returns an error.
func bootstrap(bc *brokerClient, cfg sidecarConfig) (st *sidecarState, err error) {
	defer func() {
		if err != nil {
			RecordBootstrap("failure")
		}
	}()

	// Step 1: Wait for broker to become healthy.
	if err := waitForBroker(bc, defaultHealthTimeout); err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}
	obs.Ok("SIDECAR", "BOOTSTRAP", "broker ready")

	// Step 2: Authenticate as admin.
	adminToken, err := bc.adminAuth(cfg.AdminSecret)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: admin auth: %w", err)
	}
	obs.Ok("SIDECAR", "BOOTSTRAP", "admin authenticated")

	// Step 3: Create sidecar activation token.
	activationToken, err := bc.createSidecarActivation(adminToken, cfg.ScopeCeiling, 600)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: create activation: %w", err)
	}
	obs.Ok("SIDECAR", "BOOTSTRAP", "activation token created")

	// Step 4: Activate sidecar (single-use exchange).
	resp, err := bc.activateSidecar(activationToken)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: activate sidecar: %w", err)
	}
	obs.Ok("SIDECAR", "BOOTSTRAP", "sidecar activated", "sidecar_id="+resp.sidecarID)
	RecordBootstrap("success")

	st = &sidecarState{sidecarID: resp.sidecarID, startTime: time.Now()}
	st.setToken(resp.accessToken, resp.expiresIn)
	return st, nil
}
