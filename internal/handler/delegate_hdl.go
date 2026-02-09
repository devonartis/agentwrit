package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/deleg"
	"github.com/divineartis/agentauth/internal/obs"
)

// DelegHdl handles POST /v1/delegate requests for delegation token creation.
type DelegHdl struct {
	delegSvc *deleg.DelegSvc
	auditLog *audit.AuditLog
}

// NewDelegHdl creates a delegation handler with optional audit logging.
func NewDelegHdl(delegSvc *deleg.DelegSvc, auditLog *audit.AuditLog) *DelegHdl {
	return &DelegHdl{delegSvc: delegSvc, auditLog: auditLog}
}

// ServeHTTP processes delegation requests, enforcing scope attenuation and chain depth limits.
func (h *DelegHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req deleg.DelegReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		obs.WriteProblemForRequest(w, r, http.StatusBadRequest, "urn:agentauth:error:bad-request", "Malformed JSON body", "Malformed JSON body")
		return
	}

	resp, err := h.delegSvc.Delegate(req)
	if err != nil {
		h.handleDelegError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
	obs.Ok("DELEG", "DelegHdl.ServeHTTP", "delegation issued",
		"target="+req.TargetAgentId,
	)

	if h.auditLog != nil {
		_ = h.auditLog.LogEvent(&audit.AuditEvt{
			EventType:      audit.EvtDelegationCreated,
			Resource:       req.TargetAgentId,
			Action:         "delegate",
			Outcome:        "created",
			DelegDepth:     resp.DelegationDepth,
			DelegChainHash: resp.ChainHash,
		})
	}
}

// handleDelegError maps DelegSvc errors to RFC 7807 responses.
func (h *DelegHdl) handleDelegError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, deleg.ErrDelegatorTokenInvalid):
		obs.WriteProblemForRequest(w, r, http.StatusUnauthorized, "urn:agentauth:error:invalid-token", "Delegator token invalid", err.Error())
		obs.Fail("DELEG", "DelegHdl.ServeHTTP", "invalid delegator token", "error="+err.Error())

	case errors.Is(err, deleg.ErrScopeEscalation):
		obs.WriteProblemForRequest(w, r, http.StatusForbidden, "urn:agentauth:error:scope-escalation", "Scope escalation blocked", err.Error())
		obs.Warn("DELEG", "DelegHdl.ServeHTTP", "scope escalation blocked", "error="+err.Error())

	case errors.Is(err, deleg.ErrDepthExceeded):
		obs.WriteProblemForRequest(w, r, http.StatusForbidden, "urn:agentauth:error:delegation-depth-exceeded", "Delegation depth exceeded", err.Error())
		obs.Warn("DELEG", "DelegHdl.ServeHTTP", "depth exceeded", "error="+err.Error())

	case errors.Is(err, deleg.ErrTTLExceedsRemaining):
		obs.WriteProblemForRequest(w, r, http.StatusBadRequest, "urn:agentauth:error:ttl-exceeded", "Delegation TTL exceeds remaining", err.Error())
		obs.Warn("DELEG", "DelegHdl.ServeHTTP", "TTL exceeds remaining", "error="+err.Error())

	case errors.Is(err, deleg.ErrTargetAgentEmpty):
		obs.WriteProblemForRequest(w, r, http.StatusBadRequest, "urn:agentauth:error:bad-request", "Invalid delegation request", err.Error())

	case errors.Is(err, deleg.ErrRequestedScopeEmpty):
		obs.WriteProblemForRequest(w, r, http.StatusBadRequest, "urn:agentauth:error:bad-request", "Invalid delegation request", err.Error())

	default:
		obs.WriteProblemForRequest(w, r, http.StatusInternalServerError, "urn:agentauth:error:internal", "Delegation failed", err.Error())
		obs.Fail("DELEG", "DelegHdl.ServeHTTP", "delegation failed", "error="+err.Error())
	}
}
