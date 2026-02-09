package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divineartis/agentauth/internal/audit"
)

func seedAuditLog(t *testing.T, n int) *audit.AuditLog {
	t.Helper()
	al := audit.NewAuditLog()
	for i := 0; i < n; i++ {
		agentId := "agent-a"
		if i%2 == 0 {
			agentId = "agent-b"
		}
		_ = al.LogEvent(&audit.AuditEvt{
			EventType:       audit.EvtAccessGranted,
			AgentInstanceId: agentId,
			TaskId:          "task-1",
			OrchId:          "orch-1",
			Resource:        "res",
			Action:          "read",
			Outcome:         "granted",
		})
	}
	return al
}

func TestAuditHdlNoFilters(t *testing.T) {
	al := seedAuditLog(t, 5)
	hdl := NewAuditHdl(al)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit/events", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var resp auditResp
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 5 {
		t.Fatalf("expected total=5, got %d", resp.Total)
	}
	if len(resp.Events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(resp.Events))
	}
}

func TestAuditHdlFilterByAgent(t *testing.T) {
	al := seedAuditLog(t, 6)
	hdl := NewAuditHdl(al)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit/events?agent_id=agent-b", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var resp auditResp
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Total != 3 {
		t.Fatalf("expected total=3 for agent-b, got %d", resp.Total)
	}
}

func TestAuditHdlPagination(t *testing.T) {
	al := seedAuditLog(t, 15)
	hdl := NewAuditHdl(al)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit/events?limit=5&offset=0", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	var resp auditResp
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Total != 15 {
		t.Fatalf("expected total=15, got %d", resp.Total)
	}
	if len(resp.Events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(resp.Events))
	}
	if resp.NextOffset != 5 {
		t.Fatalf("expected next_offset=5, got %d", resp.NextOffset)
	}
}

func TestAuditHdlMethodNotAllowed(t *testing.T) {
	hdl := NewAuditHdl(audit.NewAuditLog())
	req := httptest.NewRequest(http.MethodPost, "/v1/audit/events", nil)
	rec := httptest.NewRecorder()
	hdl.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rec.Code)
	}
}

func TestAuditHdlBadQueryParams(t *testing.T) {
	al := seedAuditLog(t, 3)
	hdl := NewAuditHdl(al)

	tests := []struct {
		name  string
		query string
	}{
		{"negative_offset", "?offset=-1"},
		{"negative_limit", "?limit=-5"},
		{"both_negative", "?offset=-1&limit=-2"},
		{"malformed_from", "?from=not-a-date"},
		{"malformed_to", "?to=2026-13-99"},
		{"partial_timestamp_from", "?from=2026-02-07"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/audit/events"+tt.query, nil)
			rec := httptest.NewRecorder()
			hdl.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400 for %s, got %d", tt.name, rec.Code)
			}
			ct := rec.Header().Get("Content-Type")
			if ct != "application/problem+json" {
				t.Fatalf("want application/problem+json, got %s", ct)
			}
		})
	}
}
