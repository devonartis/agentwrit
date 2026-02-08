package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/divineartis/agentauth/internal/deleg"
	"github.com/divineartis/agentauth/internal/obs"
)

// DelegHdl handles POST /v1/delegate requests for delegation token creation.
type DelegHdl struct {
	delegSvc *deleg.DelegSvc
}

// NewDelegHdl creates a delegation handler backed by the given DelegSvc.
func NewDelegHdl(delegSvc *deleg.DelegSvc) *DelegHdl {
	return &DelegHdl{delegSvc: delegSvc}
}

// ServeHTTP processes delegation requests, enforcing scope attenuation and chain depth limits.
func (h *DelegHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req deleg.DelegReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		obs.WriteProblem(w, http.StatusBadRequest, "urn:agentauth:error:bad-request", "Malformed JSON body")
		return
	}

	resp, err := h.delegSvc.Delegate(req)
	if err != nil {
		h.handleDelegError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
	obs.Ok("DELEG", "DelegHdl.ServeHTTP", "delegation issued",
		"target="+req.TargetAgentId,
	)
}

// handleDelegError maps DelegSvc errors to RFC 7807 responses.
func (h *DelegHdl) handleDelegError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, deleg.ErrDelegatorTokenInvalid):
		obs.WriteProblem(w, http.StatusUnauthorized, "urn:agentauth:error:invalid-token", err.Error())
		obs.Fail("DELEG", "DelegHdl.ServeHTTP", "invalid delegator token", "error="+err.Error())

	case errors.Is(err, deleg.ErrScopeEscalation):
		obs.WriteProblem(w, http.StatusForbidden, "urn:agentauth:error:scope-escalation", err.Error())
		obs.Warn("DELEG", "DelegHdl.ServeHTTP", "scope escalation blocked", "error="+err.Error())

	case errors.Is(err, deleg.ErrDepthExceeded):
		obs.WriteProblem(w, http.StatusForbidden, "urn:agentauth:error:delegation-depth-exceeded", err.Error())
		obs.Warn("DELEG", "DelegHdl.ServeHTTP", "depth exceeded", "error="+err.Error())

	case errors.Is(err, deleg.ErrTTLExceedsRemaining):
		obs.WriteProblem(w, http.StatusBadRequest, "urn:agentauth:error:ttl-exceeded", err.Error())
		obs.Warn("DELEG", "DelegHdl.ServeHTTP", "TTL exceeds remaining", "error="+err.Error())

	case errors.Is(err, deleg.ErrTargetAgentEmpty):
		obs.WriteProblem(w, http.StatusBadRequest, "urn:agentauth:error:bad-request", err.Error())

	case errors.Is(err, deleg.ErrRequestedScopeEmpty):
		obs.WriteProblem(w, http.StatusBadRequest, "urn:agentauth:error:bad-request", err.Error())

	default:
		obs.WriteProblem(w, http.StatusInternalServerError, "urn:agentauth:error:internal", "Delegation failed")
		obs.Fail("DELEG", "DelegHdl.ServeHTTP", "delegation failed", "error="+err.Error())
	}
}
