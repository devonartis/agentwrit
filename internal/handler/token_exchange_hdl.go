package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

const sidecarScopePrefix = "sidecar:scope:"

// maxExchangeTTL caps the TTL a sidecar may request for an exchanged token.
const maxExchangeTTL = 900

var (
	// ErrExchangeMissingField indicates a required field is missing.
	ErrExchangeMissingField = errors.New("missing required field")
	// ErrExchangeScopeViolation indicates requested scope exceeds sidecar scope ceiling.
	ErrExchangeScopeViolation = errors.New("requested scope exceeds sidecar scope ceiling")
	// ErrExchangeSidecarScopeMissing indicates no sidecar scope ceiling exists in caller token.
	ErrExchangeSidecarScopeMissing = errors.New("sidecar scope ceiling missing")
	// ErrExchangeInvalidTTL indicates a TTL outside acceptable bounds.
	ErrExchangeInvalidTTL = errors.New("ttl must be between 1 and 900 seconds")
	// ErrExchangeInvalidScopeFormat indicates a scope entry is not in action:resource:identifier format.
	ErrExchangeInvalidScopeFormat = errors.New("scope must be in action:resource:identifier format")
)

// TokenExchangeReq is the JSON request body for POST /v1/token/exchange.
type TokenExchangeReq struct {
	AgentID   string   `json:"agent_id"`
	Scope     []string `json:"scope"`
	TTL       int      `json:"ttl"`
	SidecarID string   `json:"sidecar_id,omitempty"` // ignored; broker-derived sid is authoritative
}

// TokenExchangeResp is the response for successful sidecar token exchange.
type TokenExchangeResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	AgentID     string `json:"agent_id"`
	SidecarID   string `json:"sidecar_id"`
}

// TokenExchangeHdl handles POST /v1/token/exchange for sidecar-mediated issuance.
type TokenExchangeHdl struct {
	tknSvc   *token.TknSvc
	store    *store.SqlStore
	auditLog *audit.AuditLog
}

// NewTokenExchangeHdl creates a new token exchange handler.
func NewTokenExchangeHdl(tknSvc *token.TknSvc, st *store.SqlStore, al *audit.AuditLog) *TokenExchangeHdl {
	return &TokenExchangeHdl{tknSvc: tknSvc, store: st, auditLog: al}
}

func (h *TokenExchangeHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims := authz.ClaimsFromContext(r.Context())
	if claims == nil {
		obs.Warn("EXCHANGE", "hdl", "missing authentication claims")
		h.recordDenial("", "", "", "unauthenticated exchange attempt")
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusUnauthorized, "unauthorized",
			"missing authentication", r.URL.Path, "missing_credentials",
			"include a valid Bearer token in the Authorization header")
		return
	}

	// Validate Content-Type: POST with JSON body must declare application/json
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		obs.Warn("EXCHANGE", "hdl", "invalid content-type", "content_type="+ct)
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusBadRequest, "invalid_request",
			"Content-Type must be application/json", r.URL.Path, "invalid_content_type",
			"set Content-Type: application/json in the request header")
		return
	}

	var req TokenExchangeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		obs.Warn("EXCHANGE", "hdl", "malformed JSON body", "err="+err.Error())
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusBadRequest, "invalid_request",
			"malformed JSON body", r.URL.Path, "malformed_body",
			"request body must be valid JSON")
		return
	}
	if req.AgentID == "" || len(req.Scope) == 0 {
		obs.Warn("EXCHANGE", "hdl", "missing required field",
			"agent_id_present="+fmt.Sprintf("%t", req.AgentID != ""),
			"scope_len="+fmt.Sprintf("%d", len(req.Scope)))
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusBadRequest, "invalid_request",
			ErrExchangeMissingField.Error(), r.URL.Path, "missing_field",
			"agent_id and scope are required fields")
		return
	}

	// Validate TTL bounds (0 means use default, negative or >max is rejected)
	if req.TTL < 0 || req.TTL > maxExchangeTTL {
		obs.Warn("EXCHANGE", "hdl", "invalid TTL", "ttl="+fmt.Sprintf("%d", req.TTL))
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusBadRequest, "invalid_request",
			ErrExchangeInvalidTTL.Error(), r.URL.Path, "invalid_ttl",
			fmt.Sprintf("max TTL is %d seconds", maxExchangeTTL))
		return
	}

	// Clamp TTL=0 to exchange cap so cfg.DefaultTTL cannot exceed maxExchangeTTL
	effectiveTTL := req.TTL
	if effectiveTTL == 0 {
		effectiveTTL = maxExchangeTTL
	}

	// Validate scope format: each entry must be action:resource:identifier
	for _, s := range req.Scope {
		if _, _, _, err := authz.ParseScope(s); err != nil {
			obs.Warn("EXCHANGE", "hdl", "invalid scope format", "scope="+s)
			problemdetails.WriteProblemExtended(r.Context(), w, http.StatusBadRequest, "invalid_request",
				ErrExchangeInvalidScopeFormat.Error(), r.URL.Path, "invalid_scope_format",
				fmt.Sprintf("each scope must have 3 non-empty colon-separated parts; got %q", s))
			return
		}
	}

	// Extract sidecar scope ceiling from caller token
	allowedScopes := sidecarAllowedScopes(claims.Scope)
	if len(allowedScopes) == 0 {
		obs.Warn("EXCHANGE", "hdl", "sidecar scope ceiling missing", "sub="+claims.Sub)
		h.recordDenial(claims.Sub, "", "", "sidecar scope ceiling missing")
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusForbidden, "insufficient_scope",
			ErrExchangeSidecarScopeMissing.Error(), r.URL.Path, "sidecar_scope_missing",
			"caller token must include sidecar:scope:* entries")
		return
	}

	// Scope attenuation: requested must be subset of ceiling
	if !authz.ScopeIsSubset(req.Scope, allowedScopes) {
		obs.Warn("EXCHANGE", "hdl", "scope escalation denied",
			"requested="+strings.Join(req.Scope, ","),
			"ceiling="+strings.Join(allowedScopes, ","))
		h.recordDenial(claims.Sub, "", "",
			fmt.Sprintf("scope escalation denied: requested=%v ceiling=%v", req.Scope, allowedScopes))
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusForbidden, "scope_violation",
			ErrExchangeScopeViolation.Error(), r.URL.Path, "scope_escalation_denied",
			"requested scope must be a subset of the sidecar scope ceiling")
		return
	}

	// Look up agent in store
	agent, err := h.store.GetAgent(req.AgentID)
	if err != nil {
		obs.Warn("EXCHANGE", "hdl", "agent not found", "agent_id="+req.AgentID)
		h.recordDenial(claims.Sub, "", "",
			fmt.Sprintf("agent not found: %s", req.AgentID))
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusNotFound, "not_found",
			"agent not found", r.URL.Path, "agent_not_found",
			"the agent_id must refer to a registered agent")
		return
	}

	// Derive sidecar_id from broker claims (anti-spoof: ignore client-supplied value)
	sidecarID := claims.Sid
	if sidecarID == "" {
		sidecarID = claims.Sub
	}
	if sidecarID == "" {
		obs.Fail("EXCHANGE", "hdl", "sidecar identity derivation failed")
		h.recordDenial(claims.Sub, "", "",
			fmt.Sprintf("sidecar identity derivation failed for sub=%s agent_id=%s", claims.Sub, req.AgentID))
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusInternalServerError, "internal_error",
			"could not derive sidecar identity from caller token", r.URL.Path, "sidecar_derivation_failed",
			"could not derive sidecar identity from caller token")
		return
	}

	issResp, err := h.tknSvc.Issue(token.IssueReq{
		Sub:       req.AgentID,
		Aud:       claims.Aud,
		Scope:     req.Scope,
		TaskId:    agent.TaskID,
		OrchId:    agent.OrchID,
		Sid:       sidecarID,
		SidecarID: sidecarID,
		TTL:       effectiveTTL,
	})
	if err != nil {
		obs.Fail("EXCHANGE", "hdl", "token issuance failed", "err="+err.Error())
		h.recordDenial(req.AgentID, agent.TaskID, agent.OrchID,
			fmt.Sprintf("token issuance failed for sidecar_id=%s scope=%v: %v", sidecarID, req.Scope, err))
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusInternalServerError, "internal_error",
			"token exchange failed", r.URL.Path, "token_issuance_failed",
			"the broker could not issue the requested token; retry or contact the administrator")
		return
	}

	// Record audit success
	h.recordSuccess(req.AgentID, agent.TaskID, agent.OrchID,
		fmt.Sprintf("sidecar_id=%s scope=%v ttl=%d", sidecarID, req.Scope, issResp.ExpiresIn))
	obs.Ok("EXCHANGE", "hdl", "token exchange success",
		"agent_id="+req.AgentID, "sidecar_id="+sidecarID,
		"ttl="+fmt.Sprintf("%d", issResp.ExpiresIn))

	resp := TokenExchangeResp{
		AccessToken: issResp.AccessToken,
		ExpiresIn:   issResp.ExpiresIn,
		TokenType:   "Bearer",
		AgentID:     req.AgentID,
		SidecarID:   sidecarID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		obs.Warn("EXCHANGE", "hdl", "failed to encode response", "err="+err.Error())
	}
}

func (h *TokenExchangeHdl) recordSuccess(agentID, taskID, orchID, detail string) {
	if h.auditLog != nil {
		h.auditLog.Record(audit.EventSidecarExchangeSuccess, agentID, taskID, orchID, detail,
			audit.WithOutcome("success"))
	}
}

func (h *TokenExchangeHdl) recordDenial(agentID, taskID, orchID, detail string) {
	if h.auditLog != nil {
		h.auditLog.Record(audit.EventSidecarExchangeDenied, agentID, taskID, orchID, detail,
			audit.WithOutcome("denied"))
	}
}

func sidecarAllowedScopes(scopes []string) []string {
	out := make([]string, 0)
	for _, scope := range scopes {
		if strings.HasPrefix(scope, sidecarScopePrefix) {
			allowed := strings.TrimPrefix(scope, sidecarScopePrefix)
			if allowed != "" {
				out = append(out, allowed)
			}
		}
	}
	return out
}
