package handler

import (
	"encoding/json"
	"net/http"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/store"
)

// ChallengeHdl handles GET /v1/challenge. It generates a cryptographic
// nonce that the agent must sign with its Ed25519 private key during
// registration. The nonce is valid for 30 seconds.
type ChallengeHdl struct {
	store *store.SqlStore
}

// NewChallengeHdl creates a new challenge handler backed by the given store.
func NewChallengeHdl(s *store.SqlStore) *ChallengeHdl {
	return &ChallengeHdl{store: s}
}

type challengeResp struct {
	Nonce     string `json:"nonce"`
	ExpiresIn int    `json:"expires_in"`
}

func (h *ChallengeHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nonce := h.store.CreateNonce()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(challengeResp{
		Nonce:     nonce,
		ExpiresIn: 30,
	}); err != nil {
		obs.Warn("CHALLENGE", "hdl", "failed to encode response", "err="+err.Error())
	}
}
