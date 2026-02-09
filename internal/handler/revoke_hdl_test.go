package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divineartis/agentauth/internal/revoke"
)

func TestRevokeTokenSuccess(t *testing.T) {
	svc := revoke.NewRevSvc()
	hdl := NewRevokeHdl(svc, nil)
	body, _ := json.Marshal(map[string]string{
		"level":     "token",
		"target_id": "jti-123",
		"reason":    "compromised",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/revoke", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["revoked"] != true {
		t.Fatal("expected revoked=true")
	}
	if resp["level"] != "token" {
		t.Fatalf("expected level=token, got %v", resp["level"])
	}
	if resp["target_id"] != "jti-123" {
		t.Fatalf("expected target_id=jti-123, got %v", resp["target_id"])
	}
	if !svc.IsTokenRevoked("jti-123") {
		t.Fatal("expected token to be revoked in service")
	}
}

func TestRevokeInvalidLevel(t *testing.T) {
	svc := revoke.NewRevSvc()
	hdl := NewRevokeHdl(svc, nil)
	body, _ := json.Marshal(map[string]string{
		"level":     "invalid",
		"target_id": "abc",
		"reason":    "test",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/revoke", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["type"] != "urn:agentauth:error:invalid-revocation-level" {
		t.Fatalf("unexpected error type: %v", resp["type"])
	}
}

func TestRevokeMissingFields(t *testing.T) {
	svc := revoke.NewRevSvc()
	hdl := NewRevokeHdl(svc, nil)
	// Empty body.
	req := httptest.NewRequest(http.MethodPost, "/v1/revoke", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for empty body, got %d", rec.Code)
	}
}

func TestRevokeAllLevels(t *testing.T) {
	svc := revoke.NewRevSvc()
	hdl := NewRevokeHdl(svc, nil)
	levels := []string{"token", "agent", "task", "delegation_chain"}
	for _, level := range levels {
		body, _ := json.Marshal(map[string]string{
			"level":     level,
			"target_id": "target-" + level,
			"reason":    "test-" + level,
		})
		req := httptest.NewRequest(http.MethodPost, "/v1/revoke", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		hdl.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("level %s: want 200, got %d", level, rec.Code)
		}
	}
	if !svc.IsTokenRevoked("target-token") {
		t.Fatal("expected token revoked")
	}
	if !svc.IsAgentRevoked("target-agent") {
		t.Fatal("expected agent revoked")
	}
	if !svc.IsTaskRevoked("target-task") {
		t.Fatal("expected task revoked")
	}
	if !svc.IsChainRevoked("target-delegation_chain") {
		t.Fatal("expected chain revoked")
	}
}
