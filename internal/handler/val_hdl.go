package handler

import (
	"encoding/json"
	"net/http"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/token"
)

// ValHdl handles POST /v1/token/validate. It accepts a token in the
// request body and returns a JSON response indicating whether the token
// is valid, along with the decoded claims on success. If a [revoke.RevSvc]
// is provided, revoked tokens are reported as invalid.
type ValHdl struct {
	tknSvc *token.TknSvc
	revSvc *revoke.RevSvc
}

// NewValHdl creates a new token validation handler. The revSvc parameter
// may be nil to disable revocation checking.
func NewValHdl(tknSvc *token.TknSvc, revSvc *revoke.RevSvc) *ValHdl {
	return &ValHdl{tknSvc: tknSvc, revSvc: revSvc}
}

type validateReq struct {
	Token string `json:"token"`
}

type validateRespValid struct {
	Valid  bool            `json:"valid"`
	Claims *token.TknClaims `json:"claims"`
}

type validateRespInvalid struct {
	Valid bool   `json:"valid"`
	Error string `json:"error"`
}

func (h *ValHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req validateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}
	if req.Token == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "token field is required", r.URL.Path)
		return
	}

	claims, err := h.tknSvc.Verify(req.Token)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err != nil {
		if encErr := json.NewEncoder(w).Encode(validateRespInvalid{
			Valid: false,
			Error: err.Error(),
		}); encErr != nil {
			obs.Warn("VALIDATE", "hdl", "failed to encode response", "err="+encErr.Error())
		}
		return
	}

	// Check revocation status after signature verification
	if h.revSvc != nil && h.revSvc.IsRevoked(claims) {
		if encErr := json.NewEncoder(w).Encode(validateRespInvalid{
			Valid: false,
			Error: "token has been revoked",
		}); encErr != nil {
			obs.Warn("VALIDATE", "hdl", "failed to encode response", "err="+encErr.Error())
		}
		return
	}

	if err := json.NewEncoder(w).Encode(validateRespValid{
		Valid:  true,
		Claims: claims,
	}); err != nil {
		obs.Warn("VALIDATE", "hdl", "failed to encode response", "err="+err.Error())
	}
}
