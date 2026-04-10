package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders_BaseHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := SecurityHeaders("none")(inner)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-store",
		"X-Frame-Options":        "DENY",
	}
	for header, expected := range want {
		if got := rr.Header().Get(header); got != expected {
			t.Errorf("header %s: want %q, got %q", header, expected, got)
		}
	}
	if rr.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should not be set in non-TLS mode")
	}
}

func TestSecurityHeaders_HSTSWhenTLS(t *testing.T) {
	for _, tlsMode := range []string{"tls", "mtls"} {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		h := SecurityHeaders(tlsMode)(inner)

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		want := "max-age=63072000; includeSubDomains"
		if got := rr.Header().Get("Strict-Transport-Security"); got != want {
			t.Errorf("tlsMode=%s: HSTS want %q, got %q", tlsMode, want, got)
		}
	}
}

func TestSecurityHeaders_HandlerCanOverrideCacheControl(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(http.StatusOK)
	})
	h := SecurityHeaders("none")(inner)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Errorf("handler override: want %q, got %q", "public, max-age=3600", got)
	}
}
