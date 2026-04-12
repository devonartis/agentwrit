package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devonartis/agentwrit/internal/problemdetails"
)

func TestRequestIDMiddleware(t *testing.T) {
	// Red Phase: This test will fail because the middleware is not yet implemented.

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := problemdetails.GetRequestID(r.Context())
		if id == "" {
			t.Error("expected request ID in context, got empty string")
		}
		w.WriteHeader(http.StatusOK)
	})

	mw := problemdetails.RequestIDMiddleware(innerHandler)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", rec.Code)
	}

	id := rec.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("expected X-Request-ID header in response, got empty")
	}
}
