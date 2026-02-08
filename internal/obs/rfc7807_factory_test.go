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
}
