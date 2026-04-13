// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/devonartis/agentwrit/internal/audit"
	"github.com/devonartis/agentwrit/internal/authz"
	"github.com/devonartis/agentwrit/internal/obs"
	"github.com/devonartis/agentwrit/internal/problemdetails"
	"github.com/devonartis/agentwrit/internal/revoke"
)

// RevokeHdl handles POST /v1/revoke — the admin's kill switch. Supports four
// revocation levels: "token" (one JTI), "agent" (all tokens for a subject),
// "task" (all tokens for a task_id), "chain" (all tokens in a delegation
// chain). This is how operators contain a compromised agent or cancel a
// runaway task. Requires admin:revoke:* scope.
type RevokeHdl struct {
	revSvc   *revoke.RevSvc
	auditLog *audit.AuditLog
}

// NewRevokeHdl creates a new revocation handler. The auditLog parameter
// may be nil to disable audit recording.
func NewRevokeHdl(revSvc *revoke.RevSvc, auditLog *audit.AuditLog) *RevokeHdl {
	return &RevokeHdl{revSvc: revSvc, auditLog: auditLog}
}

type revokeReq struct {
	Level  string `json:"level"`
	Target string `json:"target"`
}

type revokeResp struct {
	Revoked bool   `json:"revoked"`
	Level   string `json:"level"`
	Target  string `json:"target"`
	Count   int    `json:"count"`
}

func (h *RevokeHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims := authz.ClaimsFromContext(r.Context())
	if claims == nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "missing authentication", r.URL.Path)
		return
	}

	var req revokeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}

	if req.Level == "" || req.Target == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "level and target are required", r.URL.Path)
		return
	}

	count, err := h.revSvc.Revoke(req.Level, req.Target)
	if err != nil {
		switch {
		case errors.Is(err, revoke.ErrInvalidLevel):
			problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid revocation level: "+req.Level, r.URL.Path)
		case errors.Is(err, revoke.ErrMissingTarget):
			problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "missing target", r.URL.Path)
		default:
			problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "revocation failed", r.URL.Path)
		}
		return
	}

	// Audit
	if h.auditLog != nil {
		h.auditLog.Record(audit.EventTokenRevoked, req.Target, "", "",
			"revoked at level="+req.Level+" target="+req.Target,
			audit.WithOutcome("success"))
	}
	obs.TokensRevokedTotal.WithLabelValues(req.Level).Inc()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(revokeResp{
		Revoked: true,
		Level:   req.Level,
		Target:  req.Target,
		Count:   count,
	}); err != nil {
		obs.Warn("REVOKE", "hdl", "failed to encode response", "err="+err.Error())
	}
}
