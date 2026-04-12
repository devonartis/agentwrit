package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devonartis/agentwrit/internal/problemdetails"
)

func TestLoggingMiddleware(t *testing.T) {
	// Red Phase: This test will fail because the middleware is not yet implemented.

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created")) //nolint:errcheck // test handler
	})

	// Wrap with RequestIDMiddleware so we have an ID to log.
	mw := problemdetails.RequestIDMiddleware(LoggingMiddleware(innerHandler))
	req := httptest.NewRequest("POST", "/v1/test", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status Created, got %d", rec.Code)
	}
}
