package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestScopeIsSubset — table-driven tests for scope matching logic
// ---------------------------------------------------------------------------

func TestScopeIsSubset(t *testing.T) {
	tests := []struct {
		name      string
		requested []string
		allowed   []string
		want      bool
	}{
		{
			name:      "exact match",
			requested: []string{"read:data:*"},
			allowed:   []string{"read:data:*"},
			want:      true,
		},
		{
			name:      "wildcard covers specific identifier",
			requested: []string{"read:data:report-1"},
			allowed:   []string{"read:data:*"},
			want:      true,
		},
		{
			name:      "different action rejected",
			requested: []string{"write:data:*"},
			allowed:   []string{"read:data:*"},
			want:      false,
		},
		{
			name:      "different resource rejected",
			requested: []string{"read:orders:*"},
			allowed:   []string{"read:data:*"},
			want:      false,
		},
		{
			name:      "empty request is always valid",
			requested: []string{},
			allowed:   []string{"read:data:*"},
			want:      true,
		},
		{
			name:      "multiple requested all covered",
			requested: []string{"read:data:*", "write:orders:*"},
			allowed:   []string{"read:data:*", "write:orders:*"},
			want:      true,
		},
		{
			name:      "one of multiple not covered",
			requested: []string{"read:data:*", "delete:data:*"},
			allowed:   []string{"read:data:*", "write:orders:*"},
			want:      false,
		},
		{
			name:      "specific identifier within wildcard ceiling",
			requested: []string{"read:data:report-1", "read:data:report-2"},
			allowed:   []string{"read:data:*"},
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scopeIsSubset(tt.requested, tt.allowed)
			if got != tt.want {
				t.Errorf("scopeIsSubset(%v, %v) = %v, want %v",
					tt.requested, tt.allowed, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestTokenHandler — POST /v1/token
// ---------------------------------------------------------------------------

func TestTokenHandler_HappyPath(t *testing.T) {
	// Mock broker that returns a token on /v1/token/exchange.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/token/exchange" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "agent-jwt-token",
				"expires_in":   300,
				"sidecar_id":   "sc-test-001",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	state := &sidecarState{
		sidecarToken: "sidecar-bearer-token",
		sidecarID:    "sc-test-001",
		expiresIn:    900,
	}
	ceiling := []string{"read:data:*", "write:data:*"}

	h := newTokenHandler(bc, state, ceiling)

	body := `{"agent_name":"data-reader","task_id":"task-789","scope":["read:data:*"],"ttl":300}`
	req := httptest.NewRequest("POST", "/v1/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["access_token"] != "agent-jwt-token" {
		t.Errorf("access_token = %v, want agent-jwt-token", resp["access_token"])
	}
	if resp["expires_in"] != float64(300) {
		t.Errorf("expires_in = %v, want 300", resp["expires_in"])
	}

	// Response scope should echo back the requested scope.
	scopeRaw, ok := resp["scope"].([]any)
	if !ok || len(scopeRaw) != 1 {
		t.Fatalf("scope = %v, want [read:data:*]", resp["scope"])
	}
	if scopeRaw[0] != "read:data:*" {
		t.Errorf("scope[0] = %v, want read:data:*", scopeRaw[0])
	}
}

func TestTokenHandler_ScopeExceedsCeiling(t *testing.T) {
	// No mock broker needed — scope check should reject before calling broker.
	bc := newBrokerClient("http://127.0.0.1:1") // unused
	state := &sidecarState{
		sidecarToken: "sidecar-bearer-token",
		sidecarID:    "sc-test-001",
		expiresIn:    900,
	}
	ceiling := []string{"read:data:*"} // only read:data allowed

	h := newTokenHandler(bc, state, ceiling)

	body := `{"agent_name":"data-writer","task_id":"task-789","scope":["write:data:*"],"ttl":300}`
	req := httptest.NewRequest("POST", "/v1/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp["error"] == nil || resp["error"] == "" {
		t.Error("expected non-empty error field in response")
	}
}

func TestTokenHandler_MissingFields(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1") // unused
	state := &sidecarState{
		sidecarToken: "sidecar-bearer-token",
		sidecarID:    "sc-test-001",
		expiresIn:    900,
	}
	ceiling := []string{"read:data:*"}

	h := newTokenHandler(bc, state, ceiling)

	// Missing scope field entirely.
	body := `{"agent_name":"data-reader","task_id":"task-789","ttl":300}`
	req := httptest.NewRequest("POST", "/v1/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
}

func TestTokenHandler_MethodNotAllowed(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1") // unused
	state := &sidecarState{
		sidecarToken: "sidecar-bearer-token",
		sidecarID:    "sc-test-001",
		expiresIn:    900,
	}
	ceiling := []string{"read:data:*"}

	h := newTokenHandler(bc, state, ceiling)

	req := httptest.NewRequest("GET", "/v1/token", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405; body = %s", rr.Code, rr.Body.String())
	}
}

func TestTokenHandler_BrokerError(t *testing.T) {
	// Mock broker that returns 500 on token exchange.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "internal error",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	state := &sidecarState{
		sidecarToken: "sidecar-bearer-token",
		sidecarID:    "sc-test-001",
		expiresIn:    900,
	}
	ceiling := []string{"read:data:*"}

	h := newTokenHandler(bc, state, ceiling)

	body := `{"agent_name":"data-reader","task_id":"task-789","scope":["read:data:*"],"ttl":300}`
	req := httptest.NewRequest("POST", "/v1/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body = %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// TestRenewHandler — POST /v1/token/renew
// ---------------------------------------------------------------------------

func TestRenewHandler_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/token/renew" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "renewed-jwt-token",
				"expires_in":   600,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	h := newRenewHandler(bc)

	req := httptest.NewRequest("POST", "/v1/token/renew", nil)
	req.Header.Set("Authorization", "Bearer old-agent-token")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["access_token"] != "renewed-jwt-token" {
		t.Errorf("access_token = %v, want renewed-jwt-token", resp["access_token"])
	}
	if resp["expires_in"] != float64(600) {
		t.Errorf("expires_in = %v, want 600", resp["expires_in"])
	}
}

func TestRenewHandler_NoBearer(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1") // unused
	h := newRenewHandler(bc)

	req := httptest.NewRequest("POST", "/v1/token/renew", nil)
	// No Authorization header.
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", rr.Code, rr.Body.String())
	}
}

func TestRenewHandler_MethodNotAllowed(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1") // unused
	h := newRenewHandler(bc)

	req := httptest.NewRequest("GET", "/v1/token/renew", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405; body = %s", rr.Code, rr.Body.String())
	}
}

func TestRenewHandler_BrokerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "internal"})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	h := newRenewHandler(bc)

	req := httptest.NewRequest("POST", "/v1/token/renew", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body = %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// TestHealthHandler — GET /v1/health
// ---------------------------------------------------------------------------

func TestHealthHandler(t *testing.T) {
	state := &sidecarState{
		sidecarToken: "sidecar-bearer-token",
		sidecarID:    "sc-test-001",
		expiresIn:    900,
	}
	ceiling := []string{"read:data:*", "write:data:*"}

	h := newHealthHandler(state, ceiling)

	req := httptest.NewRequest("GET", "/v1/health", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["broker_connected"] != true {
		t.Errorf("broker_connected = %v, want true", resp["broker_connected"])
	}

	scopeRaw, ok := resp["scope_ceiling"].([]any)
	if !ok || len(scopeRaw) != 2 {
		t.Fatalf("scope_ceiling = %v, want [read:data:* write:data:*]", resp["scope_ceiling"])
	}
	if scopeRaw[0] != "read:data:*" || scopeRaw[1] != "write:data:*" {
		t.Errorf("scope_ceiling = %v, want [read:data:* write:data:*]", scopeRaw)
	}
}

func TestHealthHandler_MethodNotAllowed(t *testing.T) {
	state := &sidecarState{
		sidecarToken: "sidecar-bearer-token",
		sidecarID:    "sc-test-001",
		expiresIn:    900,
	}
	ceiling := []string{"read:data:*"}

	h := newHealthHandler(state, ceiling)

	req := httptest.NewRequest("POST", "/v1/health", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405; body = %s", rr.Code, rr.Body.String())
	}
}
