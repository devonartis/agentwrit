// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

package handler

import (
	"encoding/json"
	"net/http"

	"github.com/devonartis/agentwrit/internal/obs"
	"github.com/devonartis/agentwrit/internal/problemdetails"
	"github.com/devonartis/agentwrit/internal/revoke"
	"github.com/devonartis/agentwrit/internal/token"
)

// ValHdl handles POST /v1/token/validate — the token introspection endpoint.
// This is how apps (and any resource server) check whether an agent's token
// is still valid before granting access. No auth required on this endpoint
// because the token itself is the proof — the response just says valid/invalid
// with decoded claims. Revoked tokens show as invalid with no detail about why,
// preventing information leakage.
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
	Valid  bool             `json:"valid"`
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
		obs.Warn("VALIDATE", "hdl", "token verification failed", "err="+err.Error(),
			"request_id="+problemdetails.GetRequestID(r.Context()))
		if encErr := json.NewEncoder(w).Encode(validateRespInvalid{
			Valid: false,
			Error: "token is invalid or expired",
		}); encErr != nil {
			obs.Warn("VALIDATE", "hdl", "failed to encode response", "err="+encErr.Error())
		}
		return
	}

	// Check revocation status after signature verification
	if h.revSvc != nil && h.revSvc.IsRevoked(claims) {
		obs.Warn("VALIDATE", "hdl", "token revoked", "sub="+claims.Sub,
			"request_id="+problemdetails.GetRequestID(r.Context()))
		if encErr := json.NewEncoder(w).Encode(validateRespInvalid{
			Valid: false,
			Error: "token is invalid or expired",
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
