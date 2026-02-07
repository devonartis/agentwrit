package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/divineartis/agentauth/internal/token"
)

type ValHdl struct {
	tknSvc *token.TknSvc
}

func NewValHdl(tknSvc *token.TknSvc) *ValHdl { return &ValHdl{tknSvc: tknSvc} }

type valReq struct {
	Token         string `json:"token"`
	RequiredScope string `json:"required_scope"`
}

func (h *ValHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req valReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "urn:agentauth:error:bad-request", "Malformed JSON body")
		return
	}
	claims, err := h.tknSvc.Verify(req.Token)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "urn:agentauth:error:invalid-token", "Token validation failed")
		return
	}
	if req.RequiredScope != "" {
		matched := false
		for _, s := range claims.Scope {
			if token.ScopeMatch(req.RequiredScope, s) {
				matched = true
				break
			}
		}
		if !matched {
			writeProblem(w, http.StatusForbidden, "urn:agentauth:error:scope-mismatch", "Required scope not granted")
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"valid":            true,
		"agent_id":         claims.Sub,
		"scope":            claims.Scope,
		"expires_in":       int(claims.Exp - time.Now().UTC().Unix()),
		"delegation_depth": len(claims.DelegChain),
	})
}

