package obs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProblem(rec, http.StatusForbidden, "urn:agentauth:error:test", "test failure")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("expected application/problem+json content type, got %q", got)
	}

	var p RFC7807
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if p.Type != "urn:agentauth:error:test" || p.Title != "test failure" || p.Status != http.StatusForbidden {
		t.Fatalf("unexpected problem payload: %+v", p)
	}
	if p.Detail != "test failure" {
		t.Fatalf("expected detail to default to title, got %q", p.Detail)
	}
	if p.Instance != "" {
		t.Fatalf("expected empty instance, got %q", p.Instance)
	}
}

func TestWriteProblemForRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/revoke", nil)
	WriteProblemForRequest(
		rec,
		req,
		http.StatusUnauthorized,
		"urn:agentauth:error:unauthorized",
		"missing bearer token",
		"admin token required",
	)

	var p RFC7807
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if p.Detail != "admin token required" {
		t.Fatalf("expected detail, got %q", p.Detail)
	}
	if p.Instance != "/v1/revoke" {
		t.Fatalf("expected instance /v1/revoke, got %q", p.Instance)
	}
}
