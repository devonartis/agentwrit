package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiter_AllowBurst(t *testing.T) {
	rl := NewRateLimiter(1, 5)
	for i := 0; i < 5; i++ {
		if !rl.Allow("192.0.2.1") {
			t.Fatalf("request %d should be allowed within burst", i+1)
		}
	}
	if rl.Allow("192.0.2.1") {
		t.Fatal("request after burst should be denied")
	}
}

func TestRateLimiter_DifferentKeysIndependent(t *testing.T) {
	rl := NewRateLimiter(1, 2)
	// Exhaust IP A.
	rl.Allow("192.0.2.1")
	rl.Allow("192.0.2.1")
	if rl.Allow("192.0.2.1") {
		t.Fatal("IP A should be exhausted")
	}
	// IP B is still fresh.
	if !rl.Allow("192.0.2.2") {
		t.Fatal("IP B should still have tokens")
	}
}

func TestRateLimiter_WrapMiddleware_429(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 burst = 1 allowed, then blocked
	handler := rl.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request: allowed.
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", nil)
	req.RemoteAddr = "192.0.2.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec.Code)
	}

	// Second request: rate limited.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("expected problem+json, got %s", ct)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header")
	}
}

func TestRateLimiter_WrapMiddleware_XForwardedFor(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	handler := rl.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request with X-Forwarded-For.
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.1")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Second request from same forwarded IP: should be limited.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for same X-Forwarded-For IP, got %d", rec.Code)
	}

	// Different forwarded IP: should be allowed.
	req2 := httptest.NewRequest(http.MethodPost, "/v1/admin/auth", nil)
	req2.RemoteAddr = "10.0.0.1:12345"
	req2.Header.Set("X-Forwarded-For", "198.51.100.1")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req2)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for different forwarded IP, got %d", rec.Code)
	}
}
