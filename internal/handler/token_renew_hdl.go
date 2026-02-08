package handler

import (
	"encoding/json"
	"net/http"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/token"
)

// RenewHdl handles POST /v1/renew requests for token renewal.
type RenewHdl struct {
	tknSvc *token.TknSvc
}

// NewRenewHdl creates a renewal handler with the given token service.
func NewRenewHdl(tknSvc *token.TknSvc) *RenewHdl { return &RenewHdl{tknSvc: tknSvc} }

type renewReq struct {
	Token string `json:"token"`
}

// ServeHTTP processes token renewal requests and returns a refreshed token.
func (h *RenewHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req renewReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		obs.WriteProblem(w, http.StatusBadRequest, "urn:agentauth:error:bad-request", "Malformed JSON body")
		return
	}
	renewed, err := h.tknSvc.Renew(req.Token)
	if err != nil {
		obs.WriteProblem(w, http.StatusUnauthorized, "urn:agentauth:error:invalid-token", "Token renewal failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(renewed)
}
