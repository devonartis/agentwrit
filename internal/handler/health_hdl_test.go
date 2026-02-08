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
}

func TestHealthHdlUnhealthyWhenSQLiteMissing(t *testing.T) {
	hdl := NewHealthHdlWithStart(nil, nil, "0.1.0", time.Now())

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
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
