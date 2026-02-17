package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestChallengeHandler — GET /v1/challenge (proxy)
// ---------------------------------------------------------------------------

func TestChallengeHandler_ProxiesBrokerChallenge(t *testing.T) {
	// Mock broker returns a nonce on GET /v1/challenge.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/challenge" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"nonce":      "deadbeef01234567",
				"expires_in": 30,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	h := newChallengeProxyHandler(bc)

	req := httptest.NewRequest("GET", "/v1/challenge", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["nonce"] != "deadbeef01234567" {
		t.Errorf("nonce = %v, want deadbeef01234567", resp["nonce"])
	}
	if resp["expires_in"] != float64(30) {
		t.Errorf("expires_in = %v, want 30", resp["expires_in"])
	}
}

func TestChallengeHandler_MethodNotAllowed(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1") // unused — should reject before calling broker
	h := newChallengeProxyHandler(bc)

	req := httptest.NewRequest("POST", "/v1/challenge", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405; body = %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// TestRegisterHandler — POST /v1/register (BYOK)
// ---------------------------------------------------------------------------

func TestRegisterHandler_BYOK_HappyPath(t *testing.T) {
	const mockSpiffeID = "spiffe://test.local/agent/my-agent/task-001/inst-xyz"

	// Mock broker handles: admin/auth → launch-tokens → register.
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

		case r.Method == "POST" && r.URL.Path == "/v1/register":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"agent_id": mockSpiffeID,
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL)
	reg := newAgentRegistry()
	ceiling := newCeilingCache([]string{"read:data:*", "write:data:*"})

	h := newRegisterHandler(bc, reg, "test-admin-secret", ceiling)

	// Build a valid base64-encoded 32-byte public key.
	pubKeyB64 := base64.StdEncoding.EncodeToString(make([]byte, ed25519.PublicKeySize))

	body := map[string]any{
		"agent_name": "my-agent",
		"task_id":    "task-001",
		"public_key": pubKeyB64,
		"signature":  base64.StdEncoding.EncodeToString([]byte("fake-sig-bytes")),
		"nonce":      "aabbccdd",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/register", strings.NewReader(string(bodyBytes)))
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

	if resp["agent_id"] != mockSpiffeID {
		t.Errorf("agent_id = %v, want %s", resp["agent_id"], mockSpiffeID)
	}

	// Verify cached in registry with nil privKey (BYOK marker).
	entry, ok := reg.lookup("my-agent:task-001")
	if !ok {
		t.Fatal("agent not found in registry after BYOK registration")
	}
	if entry.spiffeID != mockSpiffeID {
		t.Errorf("cached spiffeID = %q, want %q", entry.spiffeID, mockSpiffeID)
	}
	if entry.pubKey == nil {
		t.Error("cached pubKey is nil, want non-nil for BYOK")
	}
	if entry.privKey != nil {
		t.Error("cached privKey should be nil for BYOK agent")
	}
}

func TestRegisterHandler_MissingFields_400(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1") // unused
	reg := newAgentRegistry()
	ceiling := newCeilingCache([]string{"read:data:*"})

	h := newRegisterHandler(bc, reg, "test-secret", ceiling)

	// Missing public_key, signature, and nonce — only agent_name provided.
	body := `{"agent_name":"my-agent"}`
	req := httptest.NewRequest("POST", "/v1/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp["error"] == nil || resp["error"] == "" {
		t.Error("expected non-empty error field in response")
	}
}

func TestRegisterHandler_MethodNotAllowed(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1") // unused
	reg := newAgentRegistry()
	ceiling := newCeilingCache([]string{"read:data:*"})

	h := newRegisterHandler(bc, reg, "test-secret", ceiling)

	req := httptest.NewRequest("GET", "/v1/register", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405; body = %s", rr.Code, rr.Body.String())
	}
}
