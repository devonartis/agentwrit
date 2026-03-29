package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/token"
)

// RenewHdl handles POST /v1/token/renew. It extracts the Bearer token
// from the Authorization header, verifies it, and issues a replacement
// token with fresh timestamps. Must be wrapped with [authz.ValMw].
type RenewHdl struct {
	tknSvc   *token.TknSvc
	auditLog *audit.AuditLog
}

// NewRenewHdl creates a new token renewal handler. The auditLog parameter
// may be nil to disable audit recording.
func NewRenewHdl(tknSvc *token.TknSvc, auditLog *audit.AuditLog) *RenewHdl {
	return &RenewHdl{tknSvc: tknSvc, auditLog: auditLog}
}

type renewResp struct {
	// AccessToken is the newly issued JWT with refreshed timestamps.
	AccessToken string `json:"access_token"`
	// ExpiresIn is the token lifetime in seconds.
	ExpiresIn int `json:"expires_in"`
}

func (h *RenewHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenStr := authz.TokenFromRequest(r)
	if tokenStr == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "missing Bearer token", r.URL.Path)
		return
	}

	// Extract claims from context (set by ValMw) for audit trail
	claims := authz.ClaimsFromContext(r.Context())

	resp, err := h.tknSvc.Renew(tokenStr)
	if err != nil {
		if h.auditLog != nil && claims != nil {
			h.auditLog.Record(audit.EventTokenRenewalFailed, claims.Sub, claims.TaskId, claims.OrchId,
				fmt.Sprintf("token renewal failed for agent=%s: %s", claims.Sub, err.Error()),
			audit.WithOutcome("denied"))
		}
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "token renewal failed: "+err.Error(), r.URL.Path)
		return
	}

	if h.auditLog != nil && claims != nil {
		h.auditLog.Record(audit.EventTokenRenewed, claims.Sub, claims.TaskId, claims.OrchId,
			fmt.Sprintf("token renewed for agent=%s new_jti=%s", claims.Sub, resp.Claims.Jti),
			audit.WithOutcome("success"))
	}

	rr := renewResp{
		AccessToken: resp.AccessToken,
		ExpiresIn:   resp.ExpiresIn,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(rr); err != nil {
		obs.Warn("RENEW", "hdl", "failed to encode response", "err="+err.Error())
	}
}
