package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrokerClient_AdminAuth(t *testing.T) {
	var gotMethod, gotPath, gotContentType string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "admin-jwt-token",
			"expires_in":   300,
			"token_type":   "Bearer",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	token, err := bc.adminAuth("test-secret")
	if err != nil {
		t.Fatalf("adminAuth returned error: %v", err)
	}

	// Verify correct HTTP method
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}

	// Verify correct path
	if gotPath != "/v1/admin/auth" {
		t.Errorf("path = %q, want /v1/admin/auth", gotPath)
	}

	// Verify Content-Type header
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}

	// Verify request body fields
	if gotBody["client_id"] != "sidecar" {
		t.Errorf("client_id = %v, want sidecar", gotBody["client_id"])
	}
	if gotBody["client_secret"] != "test-secret" {
		t.Errorf("client_secret = %v, want test-secret", gotBody["client_secret"])
	}

	// Verify response parsing
	if token != "admin-jwt-token" {
		t.Errorf("token = %q, want admin-jwt-token", token)
	}
}

func TestBrokerClient_CreateSidecarActivation(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"activation_token": "act-token-123",
			"expires_at":       "2026-01-01T00:00:00Z",
			"scope":            "sidecar:activate:read:data:*",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	actToken, err := bc.createSidecarActivation("admin-jwt", "read:data:*", 600)
	if err != nil {
		t.Fatalf("createSidecarActivation returned error: %v", err)
	}

	// Verify correct HTTP method
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}

	// Verify correct path
	if gotPath != "/v1/admin/sidecar-activations" {
		t.Errorf("path = %q, want /v1/admin/sidecar-activations", gotPath)
	}

	// Verify Authorization header
	if gotAuth != "Bearer admin-jwt" {
		t.Errorf("Authorization = %q, want Bearer admin-jwt", gotAuth)
	}

	// Verify request body fields
	if gotBody["allowed_scope_prefix"] != "read:data:*" {
		t.Errorf("allowed_scope_prefix = %v, want read:data:*", gotBody["allowed_scope_prefix"])
	}
	if gotBody["ttl"] != float64(600) {
		t.Errorf("ttl = %v, want 600", gotBody["ttl"])
	}

	// Verify response parsing
	if actToken != "act-token-123" {
		t.Errorf("activation_token = %q, want act-token-123", actToken)
	}
}

func TestBrokerClient_ActivateSidecar(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "sidecar-bearer-token",
			"expires_in":   900,
			"token_type":   "Bearer",
			"sidecar_id":   "sc-abc-123",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	resp, err := bc.activateSidecar("act-token-123")
	if err != nil {
		t.Fatalf("activateSidecar returned error: %v", err)
	}

	// Verify correct HTTP method
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}

	// Verify correct path
	if gotPath != "/v1/sidecar/activate" {
		t.Errorf("path = %q, want /v1/sidecar/activate", gotPath)
	}

	// Verify request body fields
	if gotBody["sidecar_activation_token"] != "act-token-123" {
		t.Errorf("sidecar_activation_token = %v, want act-token-123", gotBody["sidecar_activation_token"])
	}

	// Verify response parsing
	if resp.accessToken != "sidecar-bearer-token" {
		t.Errorf("accessToken = %q, want sidecar-bearer-token", resp.accessToken)
	}
	if resp.expiresIn != 900 {
		t.Errorf("expiresIn = %d, want 900", resp.expiresIn)
	}
	if resp.sidecarID != "sc-abc-123" {
		t.Errorf("sidecarID = %q, want sc-abc-123", resp.sidecarID)
	}
}

func TestBrokerClient_TokenExchange(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotContentType string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "agent-scoped-token",
			"expires_in":   300,
			"token_type":   "Bearer",
			"agent_id":     "agent-1",
			"sidecar_id":   "sc-abc-123",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	resp, err := bc.tokenExchange("sidecar-token", "agent-1", []string{"read:data:*", "write:data:*"}, 300)
	if err != nil {
		t.Fatalf("tokenExchange returned error: %v", err)
	}

	// Verify correct HTTP method
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}

	// Verify correct path
	if gotPath != "/v1/token/exchange" {
		t.Errorf("path = %q, want /v1/token/exchange", gotPath)
	}

	// Verify Authorization header
	if gotAuth != "Bearer sidecar-token" {
		t.Errorf("Authorization = %q, want Bearer sidecar-token", gotAuth)
	}

	// Verify Content-Type header
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}

	// Verify request body fields
	if gotBody["agent_id"] != "agent-1" {
		t.Errorf("agent_id = %v, want agent-1", gotBody["agent_id"])
	}
	scopeRaw, ok := gotBody["scope"].([]any)
	if !ok || len(scopeRaw) != 2 {
		t.Fatalf("scope = %v, want [read:data:* write:data:*]", gotBody["scope"])
	}
	if scopeRaw[0] != "read:data:*" || scopeRaw[1] != "write:data:*" {
		t.Errorf("scope = %v, want [read:data:* write:data:*]", scopeRaw)
	}
	if gotBody["ttl"] != float64(300) {
		t.Errorf("ttl = %v, want 300", gotBody["ttl"])
	}

	// Verify response parsing
	if resp.AccessToken != "agent-scoped-token" {
		t.Errorf("AccessToken = %q, want agent-scoped-token", resp.AccessToken)
	}
	if resp.ExpiresIn != 300 {
		t.Errorf("ExpiresIn = %d, want 300", resp.ExpiresIn)
	}
	if resp.SidecarID != "sc-abc-123" {
		t.Errorf("SidecarID = %q, want sc-abc-123", resp.SidecarID)
	}
}

func TestBrokerClient_HealthCheck(t *testing.T) {
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"version": "2.0.0",
			"uptime":  42,
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	err := bc.healthCheck()
	if err != nil {
		t.Fatalf("healthCheck returned error: %v", err)
	}

	// Verify correct HTTP method
	if gotMethod != "GET" {
		t.Errorf("method = %q, want GET", gotMethod)
	}

	// Verify correct path
	if gotPath != "/v1/health" {
		t.Errorf("path = %q, want /v1/health", gotPath)
	}
}

func TestBrokerClient_HealthCheck_Failure(t *testing.T) {
	// Connect to port 1 which is almost certainly not listening.
	// This tests that the client correctly reports connection failures.
	bc := newBrokerClient("http://127.0.0.1:1")
	err := bc.healthCheck()
	if err == nil {
		t.Fatal("healthCheck should return error for unreachable host")
	}
}

func TestBrokerClient_TokenRenew(t *testing.T) {
	var gotMethod, gotPath, gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "renewed-token-xyz",
			"expires_in":   600,
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	newToken, expiresIn, err := bc.tokenRenew("old-sidecar-token")
	if err != nil {
		t.Fatalf("tokenRenew returned error: %v", err)
	}

	// Verify correct HTTP method
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}

	// Verify correct path
	if gotPath != "/v1/token/renew" {
		t.Errorf("path = %q, want /v1/token/renew", gotPath)
	}

	// Verify Authorization header
	if gotAuth != "Bearer old-sidecar-token" {
		t.Errorf("Authorization = %q, want Bearer old-sidecar-token", gotAuth)
	}

	// Verify response parsing
	if newToken != "renewed-token-xyz" {
		t.Errorf("newToken = %q, want renewed-token-xyz", newToken)
	}
	if expiresIn != 600 {
		t.Errorf("expiresIn = %d, want 600", expiresIn)
	}
}

func TestBrokerClient_GetChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/challenge" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"nonce": "abc123", "expires_in": 30})
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
		_ = json.NewEncoder(w).Encode(map[string]any{
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
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["launch_token"] == nil || body["public_key"] == nil {
			t.Error("missing required fields")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
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

func TestBrokerClient_doJSON_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":   "unauthorized",
			"title":  "Unauthorized",
			"status": 401,
			"detail": "invalid credentials",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	_, err := bc.adminAuth("wrong-secret")
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}

	// Error message should contain the response body details
	errMsg := err.Error()
	if len(errMsg) == 0 {
		t.Error("error message should be non-empty")
	}
}
