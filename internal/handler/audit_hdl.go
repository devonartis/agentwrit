package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/obs"
)

// AuditHdl handles GET /v1/audit/events requests for querying the audit trail.
type AuditHdl struct {
	auditLog *audit.AuditLog
}

// NewAuditHdl creates an audit handler with the given audit log.
func NewAuditHdl(auditLog *audit.AuditLog) *AuditHdl {
	return &AuditHdl{auditLog: auditLog}
}

type auditResp struct {
	Events     []audit.AuditEvt `json:"events"`
	Total      int              `json:"total"`
	NextOffset int              `json:"next_offset"`
}

// ServeHTTP parses query params, calls QueryEvents, and returns paginated JSON.
func (h *AuditHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	if limit < 0 || offset < 0 {
		obs.WriteProblemForRequest(w, r, http.StatusBadRequest, "urn:agentauth:error:bad-request", "limit and offset must be non-negative", "limit and offset must be non-negative")
		return
	}

	from := q.Get("from")
	to := q.Get("to")
	if from != "" {
		if _, err := time.Parse(time.RFC3339, from); err != nil {
			obs.WriteProblemForRequest(w, r, http.StatusBadRequest, "urn:agentauth:error:bad-request", "from must be a valid RFC 3339 timestamp", "from must be a valid RFC 3339 timestamp")
			return
		}
	}
	if to != "" {
		if _, err := time.Parse(time.RFC3339, to); err != nil {
			obs.WriteProblemForRequest(w, r, http.StatusBadRequest, "urn:agentauth:error:bad-request", "to must be a valid RFC 3339 timestamp", "to must be a valid RFC 3339 timestamp")
			return
		}
	}

	filter := audit.AuditFilter{
		AgentId:   q.Get("agent_id"),
		TaskId:    q.Get("task_id"),
		OrchId:    q.Get("orchestration_id"),
		EventType: q.Get("event_type"),
		From:      from,
		To:        to,
		Limit:     limit,
		Offset:    offset,
	}

	events, total, err := h.auditLog.QueryEvents(filter)
	if err != nil {
		obs.WriteProblemForRequest(w, r, http.StatusInternalServerError, "urn:agentauth:error:internal", "Audit query failed", "Audit query failed")
		obs.Fail("AUDIT", "AuditHdl.ServeHTTP", "query failed", "error="+err.Error())
		return
	}

	if events == nil {
		events = []audit.AuditEvt{}
	}

	nextOffset := offset + len(events)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(auditResp{
		Events:     events,
		Total:      total,
		NextOffset: nextOffset,
	})
	obs.Ok("AUDIT", "AuditHdl.ServeHTTP", "query served",
		"total="+strconv.Itoa(total),
		"returned="+strconv.Itoa(len(events)),
	)
}
