// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/devonartis/agentwrit/internal/audit"
	"github.com/devonartis/agentwrit/internal/authz"
	"github.com/devonartis/agentwrit/internal/obs"
	"github.com/devonartis/agentwrit/internal/problemdetails"
)

// AuditHdl handles GET /v1/audit/events. It accepts optional query
// parameters for filtering (agent_id, task_id, event_type, since, until)
// and pagination (limit, offset). Requires the "admin:audit:*" scope.
type AuditHdl struct {
	auditLog *audit.AuditLog
}

// NewAuditHdl creates a new audit event query handler.
func NewAuditHdl(auditLog *audit.AuditLog) *AuditHdl {
	return &AuditHdl{auditLog: auditLog}
}

type auditResp struct {
	Events []audit.AuditEvent `json:"events"`
	Total  int                `json:"total"`
	Offset int                `json:"offset"`
	Limit  int                `json:"limit"`
}

func (h *AuditHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims := authz.ClaimsFromContext(r.Context())
	if claims == nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "missing authentication", r.URL.Path)
		return
	}

	q := r.URL.Query()
	filters := audit.QueryFilters{
		AgentID:   q.Get("agent_id"),
		TaskID:    q.Get("task_id"),
		EventType: q.Get("event_type"),
		Outcome:   q.Get("outcome"),
	}

	if since := q.Get("since"); since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err == nil {
			filters.Since = &t
		}
	}
	if until := q.Get("until"); until != "" {
		t, err := time.Parse(time.RFC3339, until)
		if err == nil {
			filters.Until = &t
		}
	}
	if limitStr := q.Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			filters.Limit = n
		}
	}
	if offsetStr := q.Get("offset"); offsetStr != "" {
		if n, err := strconv.Atoi(offsetStr); err == nil {
			filters.Offset = n
		}
	}

	events, total := h.auditLog.Query(filters)
	if events == nil {
		events = []audit.AuditEvent{}
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(auditResp{
		Events: events,
		Total:  total,
		Offset: filters.Offset,
		Limit:  limit,
	}); err != nil {
		obs.Warn("AUDIT", "hdl", "failed to encode response", "err="+err.Error())
	}
}
