package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/token"
)

// RegHdl handles POST /v1/register requests for agent registration.
type RegHdl struct {
	idSvc  *identity.IdSvc
	tknSvc *token.TknSvc
	cfg    cfg.Cfg
}

// NewRegHdl creates a registration handler with identity, token, and config dependencies.
func NewRegHdl(idSvc *identity.IdSvc, tknSvc *token.TknSvc, c cfg.Cfg) *RegHdl {
	return &RegHdl{idSvc: idSvc, tknSvc: tknSvc, cfg: c}
}

type registerBody struct {
	LaunchToken    string          `json:"launch_token"`
	Nonce          string          `json:"nonce"`
	AgentPublicKey json.RawMessage `json:"agent_public_key"`
	Signature      string          `json:"signature"`
	Orchestration  string          `json:"orchestration_id"`
	TaskID         string          `json:"task_id"`
	RequestedScope []string        `json:"requested_scope"`
}

type registerResp struct {
	AgentInstanceID string `json:"agent_instance_id"`
	AccessToken     string `json:"access_token"`
	ExpiresIn       int    `json:"expires_in"`
	RefreshAfter    int    `json:"refresh_after"`
}

// ServeHTTP processes agent registration requests, verifying identity and issuing an initial token.
func (h *RegHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body registerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		obs.WriteProblem(w, http.StatusBadRequest, "urn:agentauth:error:bad-request", "Malformed JSON body")
		return
	}

	idResp, err := h.idSvc.Register(identity.RegisterReq{
		LaunchToken:    body.LaunchToken,
		Nonce:          body.Nonce,
		AgentPubKey:    body.AgentPublicKey,
		Signature:      body.Signature,
		OrchId:         body.Orchestration,
		TaskId:         body.TaskID,
		RequestedScope: body.RequestedScope,
	})
	if err != nil {
		switch {
		case errors.Is(err, identity.ErrRegisterBadLaunchToken):
			obs.WriteProblem(w, http.StatusUnauthorized, "urn:agentauth:error:bad-launch-token", "Launch token is invalid or expired")
		case errors.Is(err, identity.ErrRegisterBadSignature), errors.Is(err, identity.ErrRegisterBadNonce):
			obs.WriteProblem(w, http.StatusForbidden, "urn:agentauth:error:register-forbidden", "Agent proof verification failed")
		default:
			obs.WriteProblem(w, http.StatusInternalServerError, "urn:agentauth:error:internal", "Registration failed")
		}
		obs.Fail("IDENTITY", "RegHdl.ServeHTTP", "register failed", "error="+err.Error())
		return
	}

	tknResp, err := h.tknSvc.Issue(token.IssueReq{
		AgentID:   idResp.AgentInstanceID,
		OrchID:    idResp.OrchId,
		TaskID:    idResp.TaskId,
		Scope:     idResp.Scope,
		TTLSecond: h.cfg.DefaultTTL,
	})
	if err != nil {
		obs.WriteProblem(w, http.StatusInternalServerError, "urn:agentauth:error:token-issue-failed", "Token issuance failed")
		obs.Fail("TOKEN", "RegHdl.ServeHTTP", "initial token issue failed", "error="+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(registerResp{
		AgentInstanceID: idResp.AgentInstanceID,
		AccessToken:     tknResp.AccessToken,
		ExpiresIn:       tknResp.ExpiresIn,
		RefreshAfter:    tknResp.RefreshAfter,
	})
	obs.Ok("IDENTITY", "RegHdl.ServeHTTP", "register success", "agent_id="+idResp.AgentInstanceID)
}
