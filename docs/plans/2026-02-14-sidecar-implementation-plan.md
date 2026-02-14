# Sidecar Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go sidecar binary that auto-bootstraps with the broker and exposes a simple `POST /v1/token` API so 3rd party developers never touch admin auth, launch tokens, or Ed25519 crypto.

**Architecture:** The sidecar is a separate Go binary in `cmd/sidecar/` that talks to the broker via HTTP. On startup it auto-activates (admin auth → sidecar activation → bearer token). It serves developer requests on `:8081` by proxying to the broker's `/v1/token/exchange` endpoint. Docker compose runs broker + sidecar together.

**Tech Stack:** Go 1.24, net/http, existing broker HTTP API, Docker multi-stage build

**Design doc:** `docs/plans/2026-02-14-sidecar-developer-experience-design.md`

---

## Prerequisites

- Branch: create `feature/sidecar-phase1` from `develop`
- All existing broker tests must pass: `go test ./... -short`
- Broker endpoints used by sidecar (all already exist and tested):
  - `POST /v1/admin/auth` — admin authentication
  - `POST /v1/admin/sidecar-activations` — create activation token
  - `POST /v1/sidecar/activate` — exchange activation for bearer
  - `POST /v1/token/exchange` — exchange bearer for scoped agent token
  - `POST /v1/token/renew` — renew a token
  - `GET /v1/health` — health check

---

## Task 1: Sidecar Configuration

**Files:**
- Create: `cmd/sidecar/config.go`
- Test: `cmd/sidecar/config_test.go`

**Step 1: Write the failing test**

```go
// cmd/sidecar/config_test.go
package main

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	os.Setenv("AA_ADMIN_SECRET", "test-secret")
	os.Setenv("AA_SIDECAR_SCOPE_CEILING", "read:data:*")
	defer os.Unsetenv("AA_ADMIN_SECRET")
	defer os.Unsetenv("AA_SIDECAR_SCOPE_CEILING")

	cfg := loadConfig()

	if cfg.BrokerURL != "http://localhost:8080" {
		t.Errorf("BrokerURL = %q, want http://localhost:8080", cfg.BrokerURL)
	}
	if cfg.Port != "8081" {
		t.Errorf("Port = %q, want 8081", cfg.Port)
	}
	if cfg.AdminSecret != "test-secret" {
		t.Errorf("AdminSecret = %q, want test-secret", cfg.AdminSecret)
	}
	if len(cfg.ScopeCeiling) != 1 || cfg.ScopeCeiling[0] != "read:data:*" {
		t.Errorf("ScopeCeiling = %v, want [read:data:*]", cfg.ScopeCeiling)
	}
}

func TestLoadConfig_CustomEnv(t *testing.T) {
	os.Setenv("AA_BROKER_URL", "http://broker:9090")
	os.Setenv("AA_SIDECAR_PORT", "9091")
	os.Setenv("AA_ADMIN_SECRET", "custom-secret")
	os.Setenv("AA_SIDECAR_SCOPE_CEILING", "read:data:*,write:orders:*")
	defer func() {
		os.Unsetenv("AA_BROKER_URL")
		os.Unsetenv("AA_SIDECAR_PORT")
		os.Unsetenv("AA_ADMIN_SECRET")
		os.Unsetenv("AA_SIDECAR_SCOPE_CEILING")
	}()

	cfg := loadConfig()

	if cfg.BrokerURL != "http://broker:9090" {
		t.Errorf("BrokerURL = %q, want http://broker:9090", cfg.BrokerURL)
	}
	if cfg.Port != "9091" {
		t.Errorf("Port = %q, want 9091", cfg.Port)
	}
	if len(cfg.ScopeCeiling) != 2 {
		t.Errorf("ScopeCeiling has %d entries, want 2", len(cfg.ScopeCeiling))
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	os.Unsetenv("AA_ADMIN_SECRET")
	os.Unsetenv("AA_SIDECAR_SCOPE_CEILING")

	cfg := loadConfig()

	if cfg.AdminSecret != "" {
		t.Errorf("AdminSecret should be empty when unset")
	}
	if len(cfg.ScopeCeiling) != 0 {
		t.Errorf("ScopeCeiling should be empty when unset")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/sidecar/ -run TestLoadConfig -v`
Expected: FAIL — `loadConfig` not defined

**Step 3: Write minimal implementation**

```go
// cmd/sidecar/config.go
package main

import (
	"os"
	"strings"
)

type sidecarConfig struct {
	BrokerURL    string
	Port         string
	AdminSecret  string
	ScopeCeiling []string
	LogLevel     string
}

func loadConfig() sidecarConfig {
	cfg := sidecarConfig{
		BrokerURL: envOr("AA_BROKER_URL", "http://localhost:8080"),
		Port:      envOr("AA_SIDECAR_PORT", "8081"),
		AdminSecret: os.Getenv("AA_ADMIN_SECRET"),
		LogLevel:    envOr("AA_SIDECAR_LOG_LEVEL", "standard"),
	}

	raw := os.Getenv("AA_SIDECAR_SCOPE_CEILING")
	if raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				cfg.ScopeCeiling = append(cfg.ScopeCeiling, s)
			}
		}
	}

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/sidecar/ -run TestLoadConfig -v`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add cmd/sidecar/config.go cmd/sidecar/config_test.go
git commit -m "feat(sidecar): add configuration loader with env vars"
```

---

## Task 2: Broker Client (HTTP calls to broker)

**Files:**
- Create: `cmd/sidecar/broker_client.go`
- Test: `cmd/sidecar/broker_client_test.go`

**Step 1: Write the failing test**

```go
// cmd/sidecar/broker_client_test.go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrokerClient_AdminAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/admin/auth" {
			t.Errorf("path = %q, want /v1/admin/auth", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["client_secret"] != "test-secret" {
			t.Errorf("client_secret = %q, want test-secret", body["client_secret"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "admin-jwt-123",
			"expires_in":   300,
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	token, err := bc.adminAuth("test-secret")
	if err != nil {
		t.Fatalf("adminAuth: %v", err)
	}
	if token != "admin-jwt-123" {
		t.Errorf("token = %q, want admin-jwt-123", token)
	}
}

func TestBrokerClient_CreateSidecarActivation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/admin/sidecar-activations" {
			t.Errorf("path = %q", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer admin-jwt" {
			t.Errorf("auth = %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{
			"activation_token": "act-token-456",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	token, err := bc.createSidecarActivation("admin-jwt", "read:data:*", 120)
	if err != nil {
		t.Fatalf("createSidecarActivation: %v", err)
	}
	if token != "act-token-456" {
		t.Errorf("token = %q, want act-token-456", token)
	}
}

func TestBrokerClient_ActivateSidecar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sidecar/activate" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "sidecar-bearer-789",
			"expires_in":   900,
			"sidecar_id":   "sc-001",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	resp, err := bc.activateSidecar("act-token-456")
	if err != nil {
		t.Fatalf("activateSidecar: %v", err)
	}
	if resp.accessToken != "sidecar-bearer-789" {
		t.Errorf("accessToken = %q", resp.accessToken)
	}
	if resp.sidecarID != "sc-001" {
		t.Errorf("sidecarID = %q", resp.sidecarID)
	}
}

func TestBrokerClient_TokenExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/token/exchange" {
			t.Errorf("path = %q", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sidecar-bearer" {
			t.Errorf("auth = %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "agent-jwt-abc",
			"expires_in":   300,
			"sidecar_id":   "sc-001",
		})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	resp, err := bc.tokenExchange("sidecar-bearer", "agent-x", []string{"read:data:*"}, 300)
	if err != nil {
		t.Fatalf("tokenExchange: %v", err)
	}
	if resp.AccessToken != "agent-jwt-abc" {
		t.Errorf("AccessToken = %q", resp.AccessToken)
	}
}

func TestBrokerClient_HealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	if err := bc.healthCheck(); err != nil {
		t.Fatalf("healthCheck: %v", err)
	}
}

func TestBrokerClient_HealthCheck_Failure(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1") // port 1 won't connect
	if err := bc.healthCheck(); err == nil {
		t.Fatal("expected error for unreachable broker")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/sidecar/ -run TestBrokerClient -v`
Expected: FAIL — `newBrokerClient` not defined

**Step 3: Write minimal implementation**

```go
// cmd/sidecar/broker_client.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type brokerClient struct {
	baseURL string
	http    *http.Client
}

type activateResp struct {
	accessToken string
	expiresIn   int
	sidecarID   string
}

type exchangeResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	SidecarID   string `json:"sidecar_id"`
}

func newBrokerClient(baseURL string) *brokerClient {
	return &brokerClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *brokerClient) healthCheck() error {
	resp, err := c.http.Get(c.baseURL + "/v1/health")
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("health check: status %d", resp.StatusCode)
	}
	return nil
}

func (c *brokerClient) adminAuth(secret string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"client_id":     "sidecar",
		"client_secret": secret,
	})
	resp, err := c.doJSON("POST", "/v1/admin/auth", body, "")
	if err != nil {
		return "", fmt.Errorf("admin auth: %w", err)
	}
	token, ok := resp["access_token"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("admin auth: no access_token in response")
	}
	return token, nil
}

func (c *brokerClient) createSidecarActivation(adminToken, scopePrefix string, ttl int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"allowed_scope_prefix": scopePrefix,
		"ttl":                  ttl,
	})
	resp, err := c.doJSON("POST", "/v1/admin/sidecar-activations", body, adminToken)
	if err != nil {
		return "", fmt.Errorf("create sidecar activation: %w", err)
	}
	token, ok := resp["activation_token"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("create sidecar activation: no activation_token in response")
	}
	return token, nil
}

func (c *brokerClient) activateSidecar(activationToken string) (*activateResp, error) {
	body, _ := json.Marshal(map[string]string{
		"sidecar_activation_token": activationToken,
	})
	resp, err := c.doJSON("POST", "/v1/sidecar/activate", body, "")
	if err != nil {
		return nil, fmt.Errorf("activate sidecar: %w", err)
	}
	token, _ := resp["access_token"].(string)
	sid, _ := resp["sidecar_id"].(string)
	expiresIn, _ := resp["expires_in"].(float64)
	if token == "" {
		return nil, fmt.Errorf("activate sidecar: no access_token in response")
	}
	return &activateResp{
		accessToken: token,
		expiresIn:   int(expiresIn),
		sidecarID:   sid,
	}, nil
}

func (c *brokerClient) tokenExchange(sidecarToken, agentID string, scope []string, ttl int) (*exchangeResp, error) {
	body, _ := json.Marshal(map[string]any{
		"agent_id": agentID,
		"scope":    scope,
		"ttl":      ttl,
	})
	resp, err := c.doJSON("POST", "/v1/token/exchange", body, sidecarToken)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	var er exchangeResp
	b, _ := json.Marshal(resp)
	json.Unmarshal(b, &er)
	if er.AccessToken == "" {
		return nil, fmt.Errorf("token exchange: no access_token in response")
	}
	return &er, nil
}

func (c *brokerClient) tokenRenew(token string) (string, int, error) {
	resp, err := c.doJSON("POST", "/v1/token/renew", nil, token)
	if err != nil {
		return "", 0, fmt.Errorf("token renew: %w", err)
	}
	newToken, _ := resp["access_token"].(string)
	expiresIn, _ := resp["expires_in"].(float64)
	if newToken == "" {
		return "", 0, fmt.Errorf("token renew: no access_token in response")
	}
	return newToken, int(expiresIn), nil
}

func (c *brokerClient) doJSON(method, path string, body []byte, bearerToken string) (map[string]any, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncBody(respBody, 200))
	}

	var result map[string]any
	if len(respBody) > 0 {
		json.Unmarshal(respBody, &result)
	}
	return result, nil
}

func truncBody(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/sidecar/ -run TestBrokerClient -v`
Expected: PASS (6 tests)

**Step 5: Commit**

```bash
git add cmd/sidecar/broker_client.go cmd/sidecar/broker_client_test.go
git commit -m "feat(sidecar): add broker HTTP client for all bootstrap and exchange calls"
```

---

## Task 3: Auto-Bootstrap Sequence

**Files:**
- Create: `cmd/sidecar/bootstrap.go`
- Test: `cmd/sidecar/bootstrap_test.go`

**Step 1: Write the failing test**

```go
// cmd/sidecar/bootstrap_test.go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestBootstrap_HappyPath(t *testing.T) {
	var step atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/health":
			step.Add(1)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/admin/auth":
			step.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"access_token": "admin-jwt", "expires_in": 300})
		case "/v1/admin/sidecar-activations":
			step.Add(1)
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]string{"activation_token": "act-token"})
		case "/v1/sidecar/activate":
			step.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"access_token": "sidecar-bearer", "expires_in": 900, "sidecar_id": "sc-1"})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	cfg := sidecarConfig{
		AdminSecret:  "test-secret",
		ScopeCeiling: []string{"read:data:*"},
	}

	state, err := bootstrap(bc, cfg)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if state.sidecarToken != "sidecar-bearer" {
		t.Errorf("sidecarToken = %q", state.sidecarToken)
	}
	if state.sidecarID != "sc-1" {
		t.Errorf("sidecarID = %q", state.sidecarID)
	}
	if step.Load() != 4 {
		t.Errorf("expected 4 steps, got %d", step.Load())
	}
}

func TestBootstrap_AdminAuthFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/health":
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/admin/auth":
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	cfg := sidecarConfig{
		AdminSecret:  "wrong",
		ScopeCeiling: []string{"read:data:*"},
	}

	_, err := bootstrap(bc, cfg)
	if err == nil {
		t.Fatal("expected error for failed admin auth")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/sidecar/ -run TestBootstrap -v`
Expected: FAIL — `bootstrap` not defined

**Step 3: Write minimal implementation**

```go
// cmd/sidecar/bootstrap.go
package main

import (
	"fmt"
	"strings"
	"time"
)

type sidecarState struct {
	sidecarToken string
	sidecarID    string
	expiresIn    int
}

// bootstrap executes the 4-step auto-activation sequence:
//  1. Wait for broker health
//  2. Authenticate as admin
//  3. Create sidecar activation token
//  4. Activate sidecar (single-use exchange)
func bootstrap(bc *brokerClient, cfg sidecarConfig) (*sidecarState, error) {
	// Step 1: Wait for broker
	if err := waitForBroker(bc, 30*time.Second); err != nil {
		return nil, fmt.Errorf("broker not ready: %w", err)
	}
	fmt.Println("[sidecar] broker is ready")

	// Step 2: Admin auth
	adminToken, err := bc.adminAuth(cfg.AdminSecret)
	if err != nil {
		return nil, fmt.Errorf("admin auth failed: %w", err)
	}
	fmt.Println("[sidecar] admin authenticated")

	// Step 3: Create activation token
	// Join scope ceiling into comma-separated prefix for activation
	scopePrefix := strings.Join(cfg.ScopeCeiling, ",")
	activationToken, err := bc.createSidecarActivation(adminToken, scopePrefix, 120)
	if err != nil {
		return nil, fmt.Errorf("create activation failed: %w", err)
	}
	fmt.Println("[sidecar] activation token created")

	// Step 4: Activate
	resp, err := bc.activateSidecar(activationToken)
	if err != nil {
		return nil, fmt.Errorf("sidecar activation failed: %w", err)
	}
	fmt.Println("[sidecar] activated, sidecar_id=" + resp.sidecarID)

	return &sidecarState{
		sidecarToken: resp.accessToken,
		sidecarID:    resp.sidecarID,
		expiresIn:    resp.expiresIn,
	}, nil
}

func waitForBroker(bc *brokerClient, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := bc.healthCheck(); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("broker did not become ready within %s", timeout)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/sidecar/ -run TestBootstrap -v`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add cmd/sidecar/bootstrap.go cmd/sidecar/bootstrap_test.go
git commit -m "feat(sidecar): add auto-bootstrap sequence (health, admin auth, activate)"
```

---

## Task 4: Developer-Facing HTTP Handlers

**Files:**
- Create: `cmd/sidecar/handler.go`
- Test: `cmd/sidecar/handler_test.go`

**Step 1: Write the failing test**

```go
// cmd/sidecar/handler_test.go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenHandler_HappyPath(t *testing.T) {
	// Mock broker: token exchange endpoint
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "agent-jwt-xyz",
			"expires_in":   300,
			"sidecar_id":   "sc-1",
		})
	}))
	defer broker.Close()

	bc := newBrokerClient(broker.URL)
	state := &sidecarState{sidecarToken: "sidecar-bearer", sidecarID: "sc-1"}
	ceiling := []string{"read:data:*"}

	h := newTokenHandler(bc, state, ceiling)

	body, _ := json.Marshal(map[string]any{
		"agent_name": "test-agent",
		"task_id":    "task-1",
		"scope":      []string{"read:data:*"},
	})
	req := httptest.NewRequest("POST", "/v1/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["access_token"] != "agent-jwt-xyz" {
		t.Errorf("access_token = %v", resp["access_token"])
	}
}

func TestTokenHandler_ScopeExceedsCeiling(t *testing.T) {
	bc := newBrokerClient("http://unused")
	state := &sidecarState{sidecarToken: "sidecar-bearer", sidecarID: "sc-1"}
	ceiling := []string{"read:data:*"}

	h := newTokenHandler(bc, state, ceiling)

	body, _ := json.Marshal(map[string]any{
		"agent_name": "test-agent",
		"task_id":    "task-1",
		"scope":      []string{"write:data:*"},
	})
	req := httptest.NewRequest("POST", "/v1/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403; body: %s", w.Code, w.Body.String())
	}
}

func TestTokenHandler_MissingFields(t *testing.T) {
	bc := newBrokerClient("http://unused")
	state := &sidecarState{sidecarToken: "sidecar-bearer", sidecarID: "sc-1"}
	ceiling := []string{"read:data:*"}

	h := newTokenHandler(bc, state, ceiling)

	body, _ := json.Marshal(map[string]any{
		"agent_name": "test-agent",
		// missing scope
	})
	req := httptest.NewRequest("POST", "/v1/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestHealthHandler(t *testing.T) {
	state := &sidecarState{sidecarToken: "token", sidecarID: "sc-1"}
	ceiling := []string{"read:data:*"}

	h := newHealthHandler(state, ceiling)

	req := httptest.NewRequest("GET", "/v1/health", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %v", resp["status"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/sidecar/ -run "TestTokenHandler|TestHealthHandler" -v`
Expected: FAIL — `newTokenHandler` not defined

**Step 3: Write minimal implementation**

```go
// cmd/sidecar/handler.go
package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// tokenReq is the developer's request to the sidecar.
type tokenReq struct {
	AgentName string   `json:"agent_name"`
	TaskID    string   `json:"task_id"`
	Scope     []string `json:"scope"`
	TTL       int      `json:"ttl"`
}

// tokenResp is what the developer gets back.
type tokenResp struct {
	AccessToken string   `json:"access_token"`
	ExpiresIn   int      `json:"expires_in"`
	Scope       []string `json:"scope"`
}

type tokenHandler struct {
	broker       *brokerClient
	state        *sidecarState
	scopeCeiling []string
}

func newTokenHandler(bc *brokerClient, state *sidecarState, ceiling []string) *tokenHandler {
	return &tokenHandler{broker: bc, state: state, scopeCeiling: ceiling}
}

func (h *tokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed")
		return
	}

	var req tokenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON body")
		return
	}

	if len(req.Scope) == 0 {
		writeError(w, 400, "scope is required")
		return
	}
	if req.AgentName == "" {
		writeError(w, 400, "agent_name is required")
		return
	}

	// Local scope ceiling check before hitting broker
	if !scopeIsSubset(req.Scope, h.scopeCeiling) {
		writeError(w, 403, "requested scope exceeds sidecar scope ceiling")
		return
	}

	ttl := req.TTL
	if ttl <= 0 {
		ttl = 300
	}

	// Build agent_id from metadata
	agentID := req.AgentName
	if req.TaskID != "" {
		agentID = req.AgentName + "/" + req.TaskID
	}

	resp, err := h.broker.tokenExchange(h.state.sidecarToken, agentID, req.Scope, ttl)
	if err != nil {
		writeError(w, 502, "broker token exchange failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenResp{
		AccessToken: resp.AccessToken,
		ExpiresIn:   resp.ExpiresIn,
		Scope:       req.Scope,
	})
}

type healthHandler struct {
	state        *sidecarState
	scopeCeiling []string
}

func newHealthHandler(state *sidecarState, ceiling []string) *healthHandler {
	return &healthHandler{state: state, scopeCeiling: ceiling}
}

func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	connected := h.state.sidecarToken != ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":           status,
		"broker_connected": connected,
		"scope_ceiling":    h.scopeCeiling,
	})
}

func writeError(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error":  http.StatusText(status),
		"detail": detail,
		"status": status,
	})
}

// scopeIsSubset checks that every requested scope is covered by at least
// one ceiling scope. Supports wildcard (*) on identifier.
func scopeIsSubset(requested, allowed []string) bool {
	for _, r := range requested {
		rParts := strings.SplitN(r, ":", 3)
		if len(rParts) != 3 {
			return false
		}
		covered := false
		for _, a := range allowed {
			aParts := strings.SplitN(a, ":", 3)
			if len(aParts) != 3 {
				continue
			}
			if rParts[0] == aParts[0] && rParts[1] == aParts[1] &&
				(rParts[2] == aParts[2] || aParts[2] == "*") {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/sidecar/ -run "TestTokenHandler|TestHealthHandler" -v`
Expected: PASS (4 tests)

**Step 5: Commit**

```bash
git add cmd/sidecar/handler.go cmd/sidecar/handler_test.go
git commit -m "feat(sidecar): add developer-facing /v1/token and /v1/health handlers"
```

---

## Task 5: Sidecar Main (Server Entrypoint)

**Files:**
- Create: `cmd/sidecar/main.go`

**Step 1: Write the entrypoint** (no test — this is wiring only)

```go
// cmd/sidecar/main.go
package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	cfg := loadConfig()

	// Validate required config
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

	// Set up routes
	mux := http.NewServeMux()
	mux.Handle("POST /v1/token", newTokenHandler(bc, state, cfg.ScopeCeiling))
	mux.Handle("GET /v1/health", newHealthHandler(state, cfg.ScopeCeiling))

	addr := ":" + cfg.Port
	fmt.Printf("[sidecar] ready on %s, sidecar_id=%s\n", addr, state.sidecarID)

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 2: Verify compilation**

Run: `go build ./cmd/sidecar/`
Expected: builds without errors

**Step 3: Commit**

```bash
git add cmd/sidecar/main.go
git commit -m "feat(sidecar): add main entrypoint wiring bootstrap and HTTP server"
```

---

## Task 6: Update Dockerfile (Multi-Stage)

**Files:**
- Modify: `Dockerfile`

**Step 1: Update Dockerfile**

Replace entire contents of `Dockerfile` with:

```dockerfile
# Stage 1: Build both binaries
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o broker ./cmd/broker
RUN CGO_ENABLED=0 GOOS=linux go build -o sidecar ./cmd/sidecar

# Stage 2: Broker image
FROM alpine:3.18 AS broker

RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/broker .
EXPOSE 8080
ENTRYPOINT ["./broker"]

# Stage 3: Sidecar image
FROM alpine:3.18 AS sidecar

RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/sidecar .
EXPOSE 8081
ENTRYPOINT ["./sidecar"]
```

**Step 2: Verify docker build**

Run: `docker build --target broker -t agentauth-broker . && docker build --target sidecar -t agentauth-sidecar .`
Expected: both build successfully

**Step 3: Commit**

```bash
git add Dockerfile
git commit -m "feat(docker): multi-stage build for broker and sidecar targets"
```

---

## Task 7: Update Docker Compose

**Files:**
- Modify: `docker-compose.yml`

**Step 1: Replace docker-compose.yml**

```yaml
services:
  broker:
    build:
      context: .
      target: broker
    ports:
      - "${AA_HOST_PORT:-8080}:8080"
    environment:
      - AA_PORT=8080
      - AA_ADMIN_SECRET=${AA_ADMIN_SECRET:-change-me-in-production}
      - AA_SEED_TOKENS=${AA_SEED_TOKENS:-false}
      - AA_LOG_LEVEL=${AA_LOG_LEVEL:-standard}
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/v1/health"]
      interval: 2s
      timeout: 3s
      retries: 10
    networks:
      - agentauth-net

  sidecar:
    build:
      context: .
      target: sidecar
    ports:
      - "${AA_SIDECAR_HOST_PORT:-8081}:8081"
    environment:
      - AA_BROKER_URL=http://broker:8080
      - AA_ADMIN_SECRET=${AA_ADMIN_SECRET:-change-me-in-production}
      - AA_SIDECAR_SCOPE_CEILING=${AA_SIDECAR_SCOPE_CEILING:-read:data:*,write:data:*}
      - AA_SIDECAR_PORT=8081
    depends_on:
      broker:
        condition: service_healthy
    networks:
      - agentauth-net

networks:
  agentauth-net:
    driver: bridge
```

**Step 2: Test docker compose**

Run: `AA_ADMIN_SECRET=test-secret-32chars-long-enough docker compose up --build -d && sleep 5 && curl -s http://localhost:8081/v1/health | python3 -m json.tool && docker compose down`
Expected: sidecar health returns `{"status": "ok", "broker_connected": true, ...}`

**Step 3: Commit**

```bash
git add docker-compose.yml
git commit -m "feat(compose): wire sidecar service with broker health dependency"
```

---

## Task 8: End-to-End Integration Test

**Files:**
- Create: `cmd/sidecar/integration_test.go`

This test starts a real broker, bootstraps the sidecar, and verifies the developer flow end-to-end.

**Step 1: Write the integration test**

```go
// cmd/sidecar/integration_test.go
//go:build !short

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/admin"
	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/handler"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

func startTestBroker(t *testing.T, secret string) *httptest.Server {
	t.Helper()
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	sqlStore := store.NewSqlStore()
	auditLog := audit.NewAuditLog()
	tknSvc := token.NewTknSvc(privKey, pubKey, &token.Config{DefaultTTL: 300, Issuer: "agentauth"})
	revSvc := revoke.NewRevSvc()
	idSvc := identity.NewIdSvc(sqlStore, tknSvc, "agentauth.local", auditLog)
	adminSvc := admin.NewAdminSvc(secret, tknSvc, sqlStore, auditLog)
	valMw := authz.NewValMw(tknSvc, revSvc, auditLog)

	mux := http.NewServeMux()
	mux.Handle("GET /v1/challenge", handler.NewChallengeHdl(sqlStore))
	mux.Handle("POST /v1/register", problemdetails.MaxBytesBody(handler.NewRegHdl(idSvc)))
	mux.Handle("POST /v1/token/validate", problemdetails.MaxBytesBody(handler.NewValHdl(tknSvc, revSvc)))
	mux.Handle("POST /v1/token/exchange",
		problemdetails.MaxBytesBody(valMw.Wrap(authz.WithRequiredScope("sidecar:manage:*", handler.NewTokenExchangeHdl(tknSvc, sqlStore, auditLog)))))
	mux.Handle("GET /v1/health", handler.NewHealthHdl("test"))
	admin.NewAdminHdl(adminSvc, valMw, auditLog).RegisterRoutes(mux)

	var rootHandler http.Handler = mux
	rootHandler = problemdetails.RequestIDMiddleware(rootHandler)

	return httptest.NewServer(rootHandler)
}

func TestIntegration_DeveloperFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	secret := "integration-test-secret-long-enough!!"
	broker := startTestBroker(t, secret)
	defer broker.Close()

	// First register an agent via the normal flow so the store has it
	bc := newBrokerClient(broker.URL)
	adminToken, err := bc.adminAuth(secret)
	if err != nil {
		t.Fatalf("admin auth: %v", err)
	}

	// Create launch token for the agent
	launchBody, _ := json.Marshal(map[string]any{
		"agent_name":    "test-agent",
		"allowed_scope": []string{"read:data:*"},
		"max_ttl":       600,
		"ttl":           600,
	})
	launchResp, err := bc.doJSON("POST", "/v1/admin/launch-tokens", launchBody, adminToken)
	if err != nil {
		t.Fatalf("create launch token: %v", err)
	}
	launchToken := launchResp["launch_token"].(string)

	// Get challenge nonce
	challengeResp, err := bc.doJSON("GET", "/v1/challenge", nil, "")
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	nonce := challengeResp["nonce"].(string)

	// Register the agent
	agentPub, agentPriv, _ := ed25519.GenerateKey(rand.Reader)
	nonceBytes, _ := hex.DecodeString(nonce)
	sig := ed25519.Sign(agentPriv, nonceBytes)

	regBody, _ := json.Marshal(map[string]any{
		"launch_token":    launchToken,
		"nonce":           nonce,
		"public_key":      base64.StdEncoding.EncodeToString(agentPub),
		"signature":       base64.StdEncoding.EncodeToString(sig),
		"orch_id":         "test-orch",
		"task_id":         "test-task",
		"requested_scope": []string{"read:data:*"},
	})
	regResp, err := bc.doJSON("POST", "/v1/register", regBody, "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	agentID := regResp["agent_id"].(string)

	// Now bootstrap the sidecar
	cfg := sidecarConfig{
		AdminSecret:  secret,
		ScopeCeiling: []string{"read:data:*"},
	}
	state, err := bootstrap(bc, cfg)
	if err != nil {
		t.Fatalf("sidecar bootstrap: %v", err)
	}

	// Set up sidecar handler
	tokenHdl := newTokenHandler(bc, state, cfg.ScopeCeiling)

	// Developer calls POST /v1/token
	devBody, _ := json.Marshal(map[string]any{
		"agent_name": agentID,
		"task_id":    "test-task",
		"scope":      []string{"read:data:*"},
	})
	req := httptest.NewRequest("POST", "/v1/token", bytes.NewReader(devBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	tokenHdl.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("POST /v1/token: status %d, body: %s", w.Code, w.Body.String())
	}

	var devResp tokenResp
	json.NewDecoder(w.Body).Decode(&devResp)
	if devResp.AccessToken == "" {
		t.Fatal("no access_token in response")
	}
	t.Logf("developer got token: %s... (expires_in=%d)", devResp.AccessToken[:20], devResp.ExpiresIn)

	// Validate the token against broker
	valBody, _ := json.Marshal(map[string]string{"token": devResp.AccessToken})
	valResp, err := bc.doJSON("POST", "/v1/token/validate", valBody, "")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if valid, ok := valResp["valid"].(bool); !ok || !valid {
		t.Fatalf("token not valid: %v", valResp)
	}
	t.Log("broker validated token: valid=true")

	// Verify scope escalation is denied
	badBody, _ := json.Marshal(map[string]any{
		"agent_name": agentID,
		"task_id":    "test-task",
		"scope":      []string{"write:data:*"},
	})
	badReq := httptest.NewRequest("POST", "/v1/token", bytes.NewReader(badBody))
	badReq.Header.Set("Content-Type", "application/json")
	badW := httptest.NewRecorder()
	tokenHdl.ServeHTTP(badW, badReq)

	if badW.Code != 403 {
		t.Fatalf("scope escalation: expected 403, got %d, body: %s", badW.Code, badW.Body.String())
	}
	t.Log("scope escalation correctly denied: 403")

	_ = time.Now() // keep time import used
	_ = os.Getenv  // keep os import used
}
```

**Step 2: Run test** (skip -short flag to include integration)

Run: `go test ./cmd/sidecar/ -run TestIntegration -v -count=1`
Expected: PASS — full developer flow works end-to-end

Note: This test may need adjustment based on exact `token.Config` constructor. Check `internal/token/tkn_svc.go` for the actual `NewTknSvc` signature and `Config` struct.

**Step 3: Commit**

```bash
git add cmd/sidecar/integration_test.go
git commit -m "test(sidecar): add end-to-end integration test for developer flow"
```

---

## Task 9: Run All Gates

**Step 1: Run unit tests**

Run: `go test ./... -short -count=1`
Expected: all PASS

**Step 2: Run full test suite**

Run: `go test ./... -count=1`
Expected: all PASS including sidecar integration test

**Step 3: Run live test**

Run: `./scripts/live_test.sh --self-host`
Expected: 12/12 PASS (existing smoketest still works)

**Step 4: Commit any fixes, then run gates**

Run: `./scripts/gates.sh task`
Expected: all gates pass

---

## Task 10: Update Documentation

**Files:**
- Modify: `docs/USER_GUIDE.md` — add "Developer Quick Start" section
- Modify: `docs/DEVELOPER_GUIDE.md` — add sidecar architecture section
- Modify: `docs/API_REFERENCE.md` — add sidecar API section
- Modify: `CHANGELOG.md` — add sidecar entry under [Unreleased]

**Content for USER_GUIDE.md Developer Quick Start:**

```markdown
## Developer Quick Start (Sidecar)

1. Run the stack: `docker compose up`
2. Request a token:
   ```bash
   curl -X POST http://localhost:8081/v1/token \
     -H "Content-Type: application/json" \
     -d '{"agent_name": "my-agent", "task_id": "task-1", "scope": ["read:data:*"]}'
   ```
3. Use the token: `Authorization: Bearer <token>`
```

**Commit:**

```bash
git add docs/ CHANGELOG.md
git commit -m "docs(sidecar): add developer quick start and sidecar API reference"
```

---

## Summary

| Task | Description | Files | Test Count |
|------|-------------|-------|------------|
| 1 | Config loader | config.go, config_test.go | 3 |
| 2 | Broker HTTP client | broker_client.go, broker_client_test.go | 6 |
| 3 | Auto-bootstrap | bootstrap.go, bootstrap_test.go | 2 |
| 4 | Developer handlers | handler.go, handler_test.go | 4 |
| 5 | Main entrypoint | main.go | compile check |
| 6 | Dockerfile multi-stage | Dockerfile | docker build |
| 7 | Docker compose | docker-compose.yml | compose up |
| 8 | Integration test | integration_test.go | 1 (E2E) |
| 9 | Gates | — | all existing |
| 10 | Documentation | docs/* | — |

**Total new files:** 8 Go files + Dockerfile + compose changes
**Total new tests:** 16 unit + 1 integration
**Estimated commits:** 10
