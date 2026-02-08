package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/divineartis/agentauth/internal/obs"
)

func TestMetricsHdlServeHTTP(t *testing.T) {
	obs.RecordValidation(true)
	hdl := NewMetricsHdl()

	req := httptest.NewRequest(http.MethodGet, "/v1/metrics", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "aa_validation_decision_total") {
		t.Fatalf("expected metrics payload to include validation decision metric")
	}
}

func TestMetricsHdlMethodNotAllowed(t *testing.T) {
	hdl := NewMetricsHdl()

	req := httptest.NewRequest(http.MethodPost, "/v1/metrics", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}
