package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBootstrap_HappyPath(t *testing.T) {
	// Track the order of endpoint calls to verify the 4-step sequence.
	var callOrder []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/health":
			callOrder = append(callOrder, "health")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})

		case r.Method == "POST" && r.URL.Path == "/v1/admin/auth":
			callOrder = append(callOrder, "admin_auth")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "admin-jwt-token",
				"expires_in":   300,
			})

		case r.Method == "POST" && r.URL.Path == "/v1/admin/sidecar-activations":
			callOrder = append(callOrder, "create_activation")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"activation_token": "act-token-xyz",
			})

		case r.Method == "POST" && r.URL.Path == "/v1/sidecar/activate":
			callOrder = append(callOrder, "activate")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "sidecar-bearer-token",
				"expires_in":   900,
				"sidecar_id":   "sc-test-001",
			})

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL, "", "", "")
	cfg := sidecarConfig{
		AdminSecret:  "test-secret",
		ScopeCeiling: []string{"read:data:*", "write:orders:*"},
	}

	state, err := bootstrap(bc, cfg)
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	// Verify all 4 steps called in correct order.
	if len(callOrder) != 4 {
		t.Fatalf("expected 4 broker calls, got %d: %v", len(callOrder), callOrder)
	}
	expectedOrder := []string{"health", "admin_auth", "create_activation", "activate"}
	for i, want := range expectedOrder {
		if callOrder[i] != want {
			t.Errorf("call[%d] = %q, want %q", i, callOrder[i], want)
		}
	}

	// Verify returned state fields via thread-safe accessors.
	if got := state.getToken(); got != "sidecar-bearer-token" {
		t.Errorf("getToken() = %q, want sidecar-bearer-token", got)
	}
	if state.sidecarID != "sc-test-001" {
		t.Errorf("sidecarID = %q, want sc-test-001", state.sidecarID)
	}
	if got := state.getExpiresIn(); got != 900 {
		t.Errorf("getExpiresIn() = %d, want 900", got)
	}
}

func TestBootstrap_AdminAuthFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/health":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})

		case r.Method == "POST" && r.URL.Path == "/v1/admin/auth":
			// Return 401 Unauthorized to simulate auth failure.
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type":   "unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "invalid credentials",
			})

		default:
			t.Errorf("unexpected request after auth failure: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL, "", "", "")
	cfg := sidecarConfig{
		AdminSecret:  "wrong-secret",
		ScopeCeiling: []string{"read:data:*"},
	}

	state, err := bootstrap(bc, cfg)
	if err == nil {
		t.Fatal("expected bootstrap to return error when admin auth fails, got nil")
	}
	if state != nil {
		t.Errorf("expected nil state on error, got %+v", state)
	}
	if !strings.Contains(err.Error(), "admin auth") {
		t.Errorf("error should mention admin auth, got: %v", err)
	}
}

func TestBootstrap_BrokerUnreachable(t *testing.T) {
	// Use a short health timeout so the test finishes quickly.
	origTimeout := defaultHealthTimeout
	defaultHealthTimeout = 1 * time.Second
	defer func() { defaultHealthTimeout = origTimeout }()

	// Point at a port that is not listening (port 1).
	bc := newBrokerClient("http://127.0.0.1:1", "", "", "")
	// Override the HTTP client timeout so individual requests fail fast.
	bc.http.Timeout = 500 * time.Millisecond

	cfg := sidecarConfig{
		AdminSecret:  "test-secret",
		ScopeCeiling: []string{"read:data:*"},
	}

	start := time.Now()
	state, err := bootstrap(bc, cfg)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected bootstrap to return error when broker is unreachable, got nil")
	}
	if state != nil {
		t.Errorf("expected nil state on error, got %+v", state)
	}
	if !strings.Contains(err.Error(), "broker") {
		t.Errorf("error should mention broker, got: %v", err)
	}
	// With a 1s health timeout the whole bootstrap should finish quickly.
	if elapsed > 5*time.Second {
		t.Errorf("bootstrap took %v, expected to fail within 5s", elapsed)
	}
}
