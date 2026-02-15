package main

import (
	"fmt"
	"strings"
	"time"
)

// defaultHealthTimeout is the maximum time bootstrap will wait for the broker
// health endpoint to respond. Tests override this to avoid long waits.
var defaultHealthTimeout = 30 * time.Second

// sidecarState holds the result of a successful bootstrap sequence.
type sidecarState struct {
	sidecarToken string
	sidecarID    string
	expiresIn    int
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
func bootstrap(bc *brokerClient, cfg sidecarConfig) (*sidecarState, error) {
	// Step 1: Wait for broker to become healthy.
	if err := waitForBroker(bc, defaultHealthTimeout); err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}
	fmt.Println("[sidecar] broker is ready")

	// Step 2: Authenticate as admin.
	adminToken, err := bc.adminAuth(cfg.AdminSecret)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: admin auth: %w", err)
	}
	fmt.Println("[sidecar] admin authenticated")

	// Step 3: Create sidecar activation token.
	scopePrefix := strings.Join(cfg.ScopeCeiling, ",")
	activationToken, err := bc.createSidecarActivation(adminToken, scopePrefix, 600)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: create activation: %w", err)
	}
	fmt.Println("[sidecar] activation token created")

	// Step 4: Activate sidecar (single-use exchange).
	resp, err := bc.activateSidecar(activationToken)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: activate sidecar: %w", err)
	}
	fmt.Println("[sidecar] sidecar activated")

	return &sidecarState{
		sidecarToken: resp.accessToken,
		sidecarID:    resp.sidecarID,
		expiresIn:    resp.expiresIn,
	}, nil
}
