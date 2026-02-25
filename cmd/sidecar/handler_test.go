package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

	bc := newBrokerClient(srv.URL, "", "", "")
	state := &sidecarState{}
	state.setToken("sidecar-bearer-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*", "write:data:*"})
	reg := newAgentRegistry()
	reg.store("data-reader:task-789", &agentEntry{spiffeID: "spiffe://test/agent/data-reader/task-789/inst"})

	h := newTokenHandler(bc, state, ceiling, reg, "test-secret", nil)

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
	bc := newBrokerClient("http://127.0.0.1:1", "", "", "") // unused
	state := &sidecarState{}
	state.setToken("sidecar-bearer-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*"}) // only read:data allowed
	reg := newAgentRegistry()

	h := newTokenHandler(bc, state, ceiling, reg, "test-secret", nil)

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
	bc := newBrokerClient("http://127.0.0.1:1", "", "", "") // unused
	state := &sidecarState{}
	state.setToken("sidecar-bearer-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*"})
	reg := newAgentRegistry()

	h := newTokenHandler(bc, state, ceiling, reg, "test-secret", nil)

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
	bc := newBrokerClient("http://127.0.0.1:1", "", "", "") // unused
	state := &sidecarState{}
	state.setToken("sidecar-bearer-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*"})
	reg := newAgentRegistry()

	h := newTokenHandler(bc, state, ceiling, reg, "test-secret", nil)

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

	bc := newBrokerClient(srv.URL, "", "", "")
	state := &sidecarState{}
	state.setToken("sidecar-bearer-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*"})
	reg := newAgentRegistry()
	reg.store("data-reader:task-789", &agentEntry{spiffeID: "spiffe://test/agent/data-reader/task-789/inst"})

	h := newTokenHandler(bc, state, ceiling, reg, "test-secret", nil)

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
// TestTokenHandler — Lazy Registration
// ---------------------------------------------------------------------------

func TestTokenHandler_LazyRegistration_FirstRequest(t *testing.T) {
	// Mock broker that handles all 5 lazy registration endpoints:
	// 1. POST /v1/admin/auth
	// 2. POST /v1/admin/launch-tokens
	// 3. GET  /v1/challenge
	// 4. POST /v1/register
	// 5. POST /v1/token/exchange
	const mockSpiffeID = "spiffe://test.local/agent/data-reader/task-001/inst-abc"
	const mockNonce = "aabbccdd" // valid hex string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/admin/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "admin-jwt-token",
			})

		case r.Method == "POST" && r.URL.Path == "/v1/admin/launch-tokens":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"launch_token": "lt-mock-token",
			})

		case r.Method == "GET" && r.URL.Path == "/v1/challenge":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"nonce": mockNonce,
			})

		case r.Method == "POST" && r.URL.Path == "/v1/register":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"agent_id": mockSpiffeID,
			})

		case r.Method == "POST" && r.URL.Path == "/v1/token/exchange":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "lazy-agent-jwt",
				"expires_in":   300,
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL, "", "", "")
	state := &sidecarState{}
	state.setToken("sidecar-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*"})
	reg := newAgentRegistry()

	h := newTokenHandler(bc, state, ceiling, reg, "test-secret", nil)

	body := `{"agent_name":"data-reader","task_id":"task-001","scope":["read:data:*"],"ttl":300}`
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

	if resp["access_token"] != "lazy-agent-jwt" {
		t.Errorf("access_token = %v, want lazy-agent-jwt", resp["access_token"])
	}
	if resp["agent_id"] != mockSpiffeID {
		t.Errorf("agent_id = %v, want %s", resp["agent_id"], mockSpiffeID)
	}

	// Verify agent was cached in registry.
	entry, ok := reg.lookup("data-reader:task-001")
	if !ok {
		t.Fatal("agent not found in registry after lazy registration")
	}
	if entry.spiffeID != mockSpiffeID {
		t.Errorf("cached spiffeID = %q, want %q", entry.spiffeID, mockSpiffeID)
	}
	if entry.pubKey == nil {
		t.Error("cached pubKey is nil, want non-nil")
	}
	if entry.privKey == nil {
		t.Error("cached privKey is nil, want non-nil")
	}
}

func TestTokenHandler_LazyRegistration_SecondRequestSkipsRegistration(t *testing.T) {
	// Mock broker only handles token exchange. Any call to admin/auth,
	// challenge, or register should cause the test to fail.
	exchangeCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/token/exchange":
			exchangeCount++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "cached-agent-jwt",
				"expires_in":   300,
			})

		case r.URL.Path == "/v1/admin/auth",
			r.URL.Path == "/v1/admin/launch-tokens",
			r.URL.Path == "/v1/challenge",
			r.URL.Path == "/v1/register":
			t.Errorf("unexpected registration endpoint called: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL, "", "", "")
	state := &sidecarState{}
	state.setToken("sidecar-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*"})
	reg := newAgentRegistry()
	// Pre-populate registry — agent is already registered.
	reg.store("data-reader:task-001", &agentEntry{
		spiffeID: "spiffe://test.local/agent/data-reader/task-001/inst-abc",
	})

	h := newTokenHandler(bc, state, ceiling, reg, "test-secret", nil)

	body := `{"agent_name":"data-reader","task_id":"task-001","scope":["read:data:*"],"ttl":300}`
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

	if resp["access_token"] != "cached-agent-jwt" {
		t.Errorf("access_token = %v, want cached-agent-jwt", resp["access_token"])
	}

	if exchangeCount != 1 {
		t.Errorf("exchange call count = %d, want 1", exchangeCount)
	}
}

func TestTokenHandler_LazyRegistration_BrokerFailure502(t *testing.T) {
	// Mock broker fails on admin auth with 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "internal server error",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL, "", "", "")
	state := &sidecarState{}
	state.setToken("sidecar-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*"})
	reg := newAgentRegistry() // empty — will trigger lazy registration

	h := newTokenHandler(bc, state, ceiling, reg, "test-secret", nil)

	body := `{"agent_name":"data-reader","task_id":"task-001","scope":["read:data:*"],"ttl":300}`
	req := httptest.NewRequest("POST", "/v1/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	detail, _ := resp["detail"].(string)
	if detail == "" {
		t.Error("expected non-empty detail field in error response")
	}
}

// ---------------------------------------------------------------------------
// TestTokenHandler — Circuit Breaker integration
// ---------------------------------------------------------------------------

func TestTokenHandler_CircuitOpen_ServesCachedToken(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1", "", "", "") // unreachable
	state := &sidecarState{}
	state.setToken("sidecar-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*"})
	reg := newAgentRegistry()

	reg.store("data-reader:task-1", &agentEntry{spiffeID: "spiffe://test/agent/data-reader/task-1/inst"})
	reg.cacheToken("data-reader:task-1", "cached-jwt-abc", []string{"read:data:*"}, 300)

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
	bc := newBrokerClient("http://127.0.0.1:1", "", "", "")
	state := &sidecarState{}
	state.setToken("sidecar-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*"})
	reg := newAgentRegistry()

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

	bc := newBrokerClient(srv.URL, "", "", "")
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
	bc := newBrokerClient("http://127.0.0.1:1", "", "", "") // unused
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
	bc := newBrokerClient("http://127.0.0.1:1", "", "", "") // unused
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

	bc := newBrokerClient(srv.URL, "", "", "")
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
	state := &sidecarState{}
	state.setToken("sidecar-bearer-token", 900)
	state.sidecarID = "sc-test-001"
	ceiling := newCeilingCache([]string{"read:data:*", "write:data:*"})

	h := newHealthHandler(state, ceiling, nil)

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
	if resp["healthy"] != true {
		t.Errorf("healthy = %v, want true", resp["healthy"])
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
	state := &sidecarState{}
	state.setToken("sidecar-bearer-token", 900)
	state.sidecarID = "sc-test-001"
	ceiling := newCeilingCache([]string{"read:data:*"})

	h := newHealthHandler(state, ceiling, nil)

	req := httptest.NewRequest("POST", "/v1/health", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405; body = %s", rr.Code, rr.Body.String())
	}
}

func TestHealthHandler_Degraded(t *testing.T) {
	state := &sidecarState{}
	// Don't set token — healthy defaults to false.
	ceiling := newCeilingCache([]string{"read:data:*"})

	h := newHealthHandler(state, ceiling, nil)

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

	if resp["status"] != "degraded" {
		t.Errorf("status = %v, want degraded", resp["status"])
	}
	if resp["healthy"] != false {
		t.Errorf("healthy = %v, want false", resp["healthy"])
	}
	if resp["broker_connected"] != false {
		t.Errorf("broker_connected = %v, want false", resp["broker_connected"])
	}
}

// ---------------------------------------------------------------------------
// TestHealthHandler — Bootstrapping
// ---------------------------------------------------------------------------

func TestHealthHandler_Bootstrapping(t *testing.T) {
	// State is nil — sidecar is still bootstrapping.
	h := newHealthHandler(nil, newCeilingCache([]string{"read:data:*"}), nil)

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

// ---------------------------------------------------------------------------
// TestHealthHandler — Dynamic ceiling reflected in response
// ---------------------------------------------------------------------------

func TestHealthHandler_IncludesSidecarID(t *testing.T) {
	state := &sidecarState{}
	state.sidecarID = "sc-test-123"
	state.setHealthy(true)
	state.setToken("some-token", 900)
	ceiling := newCeilingCache([]string{"read:customer:*"})
	registry := newAgentRegistry()

	h := newHealthHandler(state, ceiling, registry)
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	sid, ok := resp["sidecar_id"]
	if !ok {
		t.Fatal("expected sidecar_id in health response")
	}
	if sid != "sc-test-123" {
		t.Fatalf("expected sc-test-123, got %v", sid)
	}
}

func TestHealthHandler_ReflectsDynamicCeiling(t *testing.T) {
	state := &sidecarState{}
	state.setToken("sidecar-bearer-token", 900)
	ceiling := newCeilingCache([]string{"read:data:*"})

	h := newHealthHandler(state, ceiling, nil)

	// Initial request — ceiling is ["read:data:*"].
	rr1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/v1/health", nil)
	h.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr1.Code)
	}
	var resp1 map[string]any
	_ = json.Unmarshal(rr1.Body.Bytes(), &resp1)
	sc1, _ := resp1["scope_ceiling"].([]any)
	if len(sc1) != 1 || sc1[0] != "read:data:*" {
		t.Fatalf("initial scope_ceiling = %v, want [read:data:*]", sc1)
	}

	// Update ceiling dynamically.
	ceiling.set([]string{"read:data:*", "write:data:*"})

	// Second request — should reflect updated ceiling.
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/v1/health", nil)
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr2.Code)
	}
	var resp2 map[string]any
	_ = json.Unmarshal(rr2.Body.Bytes(), &resp2)
	sc2, _ := resp2["scope_ceiling"].([]any)
	if len(sc2) != 2 || sc2[0] != "read:data:*" || sc2[1] != "write:data:*" {
		t.Errorf("updated scope_ceiling = %v, want [read:data:* write:data:*]", sc2)
	}
}
