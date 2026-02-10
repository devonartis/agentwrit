package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestChallengeHdlServeHTTP(t *testing.T) {
	h := NewChallengeHdl(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/challenge", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("want content-type application/json, got %q", got)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected json body: %v", err)
	}

	nonce := body["nonce"]
	if len(nonce) != 64 {
		t.Fatalf("expected nonce length 64, got %d", len(nonce))
	}
	if ok, _ := regexp.MatchString("^[a-f0-9]{64}$", nonce); !ok {
		t.Fatalf("nonce must be 64 hex chars, got %q", nonce)
	}

	expiresAtRaw := body["expires_at"]
	expiresAt, err := time.Parse(time.RFC3339, expiresAtRaw)
	if err != nil {
		t.Fatalf("expires_at must be RFC3339: %v", err)
	}
	if expiresAt.Before(time.Now().UTC()) {
		t.Fatalf("expires_at must be in the future, got %s", expiresAtRaw)
	}
}

func TestChallengeHdlMethodNotAllowed(t *testing.T) {
	h := NewChallengeHdl(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/challenge", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want status 405, got %d", rec.Code)
	}
}
