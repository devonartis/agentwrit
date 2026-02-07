package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/revoke"
)

// RevokeHdl handles POST /v1/revoke requests.
type RevokeHdl struct {
	revSvc *revoke.RevSvc
}

// NewRevokeHdl creates a revocation handler.
func NewRevokeHdl(revSvc *revoke.RevSvc) *RevokeHdl {
	return &RevokeHdl{revSvc: revSvc}
}

type revokeReq struct {
	Level    string `json:"level"`
	TargetID string `json:"target_id"`
	Reason   string `json:"reason"`
}

type revokeResp struct {
	Revoked   bool   `json:"revoked"`
	Level     string `json:"level"`
	TargetID  string `json:"target_id"`
	RevokedAt string `json:"revoked_at"`
}

// ServeHTTP processes revocation requests at the specified level.
func (h *RevokeHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req revokeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "urn:agentauth:error:bad-request", "Malformed JSON body")
		return
	}

	if req.TargetID == "" {
		writeProblem(w, http.StatusBadRequest, "urn:agentauth:error:bad-request", "target_id is required")
		return
	}

	var err error
	switch req.Level {
	case "token":
		err = h.revSvc.RevokeToken(req.TargetID, req.Reason)
	case "agent":
		err = h.revSvc.RevokeAgent(req.TargetID, req.Reason)
	case "task":
		err = h.revSvc.RevokeTask(req.TargetID, req.Reason)
	case "delegation_chain":
		err = h.revSvc.RevokeDelegChain(req.TargetID, req.Reason)
	default:
		writeProblem(w, http.StatusBadRequest, "urn:agentauth:error:invalid-revocation-level",
			"level must be one of: token, agent, task, delegation_chain")
		obs.Fail("REVOKE", "RevokeHdl.ServeHTTP", "invalid revocation level", "level="+req.Level)
		return
	}

	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "urn:agentauth:error:internal", "Revocation failed")
		obs.Fail("REVOKE", "RevokeHdl.ServeHTTP", "revocation failed", "error="+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(revokeResp{
		Revoked:   true,
		Level:     req.Level,
		TargetID:  req.TargetID,
		RevokedAt: time.Now().UTC().Format(time.RFC3339),
	})
	obs.Ok("REVOKE", "RevokeHdl.ServeHTTP", "revocation success", "level="+req.Level, "target_id="+req.TargetID)
}
