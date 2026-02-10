package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/store"
)

// ChallengeRecord stores issued nonce metadata.
type ChallengeRecord struct {
	Nonce     string
	ExpiresAt time.Time
}

// ChallengeHdl serves GET /v1/challenge and tracks nonce TTL.
type ChallengeHdl struct {
	mu     sync.Mutex
	nonces map[string]ChallengeRecord
	ttl    time.Duration
	store  *store.SqlStore
}

// NewChallengeHdl creates a challenge handler with 60s nonce TTL.
func NewChallengeHdl(sqlStore *store.SqlStore) *ChallengeHdl {
	return &ChallengeHdl{
		nonces: make(map[string]ChallengeRecord),
		ttl:    60 * time.Second,
		store:  sqlStore,
	}
}

// ServeHTTP issues a nonce challenge response.
func (h *ChallengeHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		obs.Fail("IDENTITY", "ChallengeHdl.ServeHTTP", "nonce generation failed", "error="+err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	nonce := hex.EncodeToString(raw)
	expiresAt := time.Now().UTC().Add(h.ttl)

	h.mu.Lock()
	h.nonces[nonce] = ChallengeRecord{Nonce: nonce, ExpiresAt: expiresAt}
	h.mu.Unlock()
	if h.store != nil {
		h.store.PutNonce(nonce, expiresAt)
	}

	resp := map[string]string{
		"nonce":      nonce,
		"expires_at": expiresAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
	obs.Ok("IDENTITY", "ChallengeHdl.ServeHTTP", "challenge issued", "nonce="+nonce)
}
