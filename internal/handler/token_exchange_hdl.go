package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

const sidecarScopePrefix = "sidecar:scope:"

var (
	// ErrExchangeMissingField indicates a required field is missing.
	ErrExchangeMissingField = errors.New("missing required field")
	// ErrExchangeScopeViolation indicates requested scope exceeds sidecar scope ceiling.
	ErrExchangeScopeViolation = errors.New("requested scope exceeds sidecar scope ceiling")
	// ErrExchangeSidecarScopeMissing indicates no sidecar scope ceiling exists in caller token.
	ErrExchangeSidecarScopeMissing = errors.New("sidecar scope ceiling missing")
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
	tknSvc *token.TknSvc
	store  *store.SqlStore
}

// NewTokenExchangeHdl creates a new token exchange handler.
func NewTokenExchangeHdl(tknSvc *token.TknSvc, st *store.SqlStore) *TokenExchangeHdl {
	return &TokenExchangeHdl{tknSvc: tknSvc, store: st}
}

func (h *TokenExchangeHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims := authz.ClaimsFromContext(r.Context())
	if claims == nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "missing authentication", r.URL.Path)
		return
	}

	var req TokenExchangeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}
	if req.AgentID == "" || len(req.Scope) == 0 {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", ErrExchangeMissingField.Error(), r.URL.Path)
		return
	}

	allowedScopes := sidecarAllowedScopes(claims.Scope)
	if len(allowedScopes) == 0 {
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusForbidden, "insufficient_scope", ErrExchangeSidecarScopeMissing.Error(), r.URL.Path, "sidecar_scope_missing", "")
		return
	}
	if !authz.ScopeIsSubset(req.Scope, allowedScopes) {
		problemdetails.WriteProblemExtended(r.Context(), w, http.StatusForbidden, "scope_violation", ErrExchangeScopeViolation.Error(), r.URL.Path, "scope_escalation_denied", "")
		return
	}

	agent, err := h.store.GetAgent(req.AgentID)
	if err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusNotFound, "not_found", "agent not found", r.URL.Path)
		return
	}

	sidecarID := claims.Sid
	if sidecarID == "" {
		sidecarID = claims.Sub
	}

	issResp, err := h.tknSvc.Issue(token.IssueReq{
		Sub:       req.AgentID,
		Scope:     req.Scope,
		TaskId:    agent.TaskID,
		OrchId:    agent.OrchID,
		Sid:       sidecarID,
		SidecarID: sidecarID,
		TTL:       req.TTL,
	})
	if err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "token exchange failed", r.URL.Path)
		return
	}

	resp := TokenExchangeResp{
		AccessToken: issResp.AccessToken,
		ExpiresIn:   issResp.ExpiresIn,
		TokenType:   "Bearer",
		AgentID:     req.AgentID,
		SidecarID:   sidecarID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
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
