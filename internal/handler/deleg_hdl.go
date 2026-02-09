package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/deleg"
	"github.com/divineartis/agentauth/internal/problemdetails"
)

// DelegHdl handles POST /v1/delegate. It extracts the caller's claims
// from context (set by [authz.ValMw]), decodes the delegation request,
// and delegates to [deleg.DelegSvc.Delegate].
type DelegHdl struct {
	delegSvc *deleg.DelegSvc
}

// NewDelegHdl creates a new delegation handler.
func NewDelegHdl(delegSvc *deleg.DelegSvc) *DelegHdl {
	return &DelegHdl{delegSvc: delegSvc}
}

func (h *DelegHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims := authz.ClaimsFromContext(r.Context())
	if claims == nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "missing authentication", r.URL.Path)
		return
	}

	var req deleg.DelegReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}

	resp, err := h.delegSvc.Delegate(claims, req)
	if err != nil {
		switch {
		case errors.Is(err, deleg.ErrScopeViolation):
			problemdetails.WriteProblem(r.Context(), w, http.StatusForbidden, "scope_violation", err.Error(), r.URL.Path)
		case errors.Is(err, deleg.ErrDepthExceeded):
			problemdetails.WriteProblem(r.Context(), w, http.StatusForbidden, "scope_violation", "delegation depth limit exceeded", r.URL.Path)
		case errors.Is(err, deleg.ErrDelegateNotFound):
			problemdetails.WriteProblem(r.Context(), w, http.StatusNotFound, "not_found", "delegate agent not found", r.URL.Path)
		default:
			problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "delegation failed", r.URL.Path)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
