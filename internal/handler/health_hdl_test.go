package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/store"
)

func TestHealthHdlHealthy(t *testing.T) {
	hdl := NewHealthHdlWithStart(store.NewSqlStore(), nil, "0.1.0", time.Now().Add(-15*time.Second))

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", got)
	}

	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if got["status"] != "healthy" {
		t.Fatalf("expected healthy status, got %v", got["status"])
	}
	if got["version"] != "0.1.0" {
		t.Fatalf("expected version 0.1.0, got %v", got["version"])
	}
	uptime, ok := got["uptime_seconds"].(float64)
	if !ok {
		t.Fatalf("expected numeric uptime_seconds, got %T", got["uptime_seconds"])
	}
	if uptime < 10 {
		t.Fatalf("expected uptime_seconds >= 10, got %v", uptime)
	}
}

func TestHealthHdlUnhealthyWhenSQLiteMissing(t *testing.T) {
	hdl := NewHealthHdlWithStart(nil, nil, "0.1.0", time.Now())

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var got struct {
		Status     string            `json:"status"`
		Components map[string]string `json:"components"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if got.Status != "unhealthy" {
		t.Fatalf("expected unhealthy status, got %q", got.Status)
	}
	if got.Components["sqlite"] != "unhealthy" {
		t.Fatalf("expected sqlite=unhealthy, got %q", got.Components["sqlite"])
	}
}

func TestHealthHdlDegradedWhenRedisDown(t *testing.T) {
	hdl := NewHealthHdlWithStart(
		store.NewSqlStore(),
		&store.RedisStore{Addr: "127.0.0.1:1"},
		"0.1.0",
		time.Now(),
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var got struct {
		Status     string            `json:"status"`
		Components map[string]string `json:"components"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if got.Status != "degraded" {
		t.Fatalf("expected degraded status, got %q", got.Status)
	}
	if got.Components["redis"] != "unhealthy" {
		t.Fatalf("expected redis=unhealthy, got %q", got.Components["redis"])
	}
}

func TestHealthHdlMethodNotAllowed(t *testing.T) {
	hdl := NewHealthHdl(store.NewSqlStore(), nil, "0.1.0")

	req := httptest.NewRequest(http.MethodPost, "/v1/health", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHealthHdlDefaultVersionAndStart(t *testing.T) {
	hdl := NewHealthHdlWithStart(store.NewSqlStore(), nil, "", time.Time{})

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if got["version"] != "0.1.0" {
		t.Fatalf("expected default version 0.1.0, got %v", got["version"])
	}
	uptime, ok := got["uptime_seconds"].(float64)
	if !ok {
		t.Fatalf("expected numeric uptime_seconds, got %T", got["uptime_seconds"])
	}
	if uptime < 0 {
		t.Fatalf("expected non-negative uptime_seconds, got %v", uptime)
	}
}
