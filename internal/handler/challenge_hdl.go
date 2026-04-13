// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

package handler

import (
	"encoding/json"
	"net/http"

	"github.com/devonartis/agentwrit/internal/obs"
	"github.com/devonartis/agentwrit/internal/store"
)

// ChallengeHdl handles GET /v1/challenge — step 1 of the agent registration
// flow. The agent fetches a nonce, signs it with its Ed25519 private key, and
// sends the signature back in POST /v1/register. This proves the agent holds
// the key without ever transmitting it. The nonce expires in 30 seconds to
// prevent replay.
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
