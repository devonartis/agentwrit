package handler

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

func TestRegHdlSuccess(t *testing.T) {
	sqlStore := store.NewSqlStore()
	_, brokerPriv, _ := identity.GenerateSigningKeyPair()
	idSvc := identity.NewIdSvc(sqlStore, brokerPriv, "agentauth.local")
	regHdl := NewRegHdl(idSvc, token.NewTknSvc(brokerPriv, brokerPriv.Public().(ed25519.PublicKey), cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300}), cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300})

	launch, _ := identity.CreateLaunchToken(sqlStore, "orch-1", "task-1", []string{"read:Customers:1"}, 30*time.Second)
	agentPub, agentPriv, _ := identity.GenerateSigningKeyPair()
	nonce := "nonce-1"
	sqlStore.PutNonce(nonce, time.Now().UTC().Add(30*time.Second))
	signature := ed25519.Sign(agentPriv, []byte(nonce))
	pubJWK := mustAgentJWK(t, agentPub)

	body := map[string]any{
		"launch_token":     launch,
		"nonce":            nonce,
		"agent_public_key": json.RawMessage(pubJWK),
		"signature":        base64.RawURLEncoding.EncodeToString(signature),
		"orchestration_id": "orch-1",
		"task_id":          "task-1",
		"requested_scope":  []string{"read:Customers:1"},
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	regHdl.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRegHdlMalformedBody(t *testing.T) {
	sqlStore := store.NewSqlStore()
	_, brokerPriv, _ := identity.GenerateSigningKeyPair()
	regHdl := NewRegHdl(identity.NewIdSvc(sqlStore, brokerPriv, "agentauth.local"), token.NewTknSvc(brokerPriv, brokerPriv.Public().(ed25519.PublicKey), cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300}), cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300})
	req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader([]byte("{bad")))
	rec := httptest.NewRecorder()
	regHdl.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestRegHdlBadLaunchToken(t *testing.T) {
	sqlStore := store.NewSqlStore()
	_, brokerPriv, _ := identity.GenerateSigningKeyPair()
	regHdl := NewRegHdl(identity.NewIdSvc(sqlStore, brokerPriv, "agentauth.local"), token.NewTknSvc(brokerPriv, brokerPriv.Public().(ed25519.PublicKey), cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300}), cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300})

	agentPub, agentPriv, _ := identity.GenerateSigningKeyPair()
	nonce := "nonce-2"
	sqlStore.PutNonce(nonce, time.Now().UTC().Add(30*time.Second))
	signature := ed25519.Sign(agentPriv, []byte(nonce))

	body := map[string]any{
		"launch_token":     "invalid",
		"nonce":            nonce,
		"agent_public_key": json.RawMessage(mustAgentJWK(t, agentPub)),
		"signature":        base64.RawURLEncoding.EncodeToString(signature),
		"orchestration_id": "orch-1",
		"task_id":          "task-1",
		"requested_scope":  []string{"read:Customers:1"},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	regHdl.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func mustAgentJWK(t *testing.T, pub []byte) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(pub),
	})
	if err != nil {
		t.Fatalf("marshal jwk: %v", err)
	}
	return raw
}
