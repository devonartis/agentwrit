package identity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/store"
)

func TestIdSvcRegisterSuccess(t *testing.T) {
	sqlStore := store.NewSqlStore()
	_, brokerPriv, err := GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("generate broker key: %v", err)
	}
	idSvc := NewIdSvc(sqlStore, brokerPriv, "agentauth.local")

	launch, err := CreateLaunchToken(sqlStore, "orch-1", "task-1", []string{"read:Customers:1"}, 30*time.Second)
	if err != nil {
		t.Fatalf("create launch token: %v", err)
	}

	agentPub, agentPriv, err := GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("generate agent key: %v", err)
	}
	nonce := "abc123nonce"
	sqlStore.PutNonce(nonce, time.Now().UTC().Add(30*time.Second))
	sig := ed25519.Sign(agentPriv, []byte(nonce))
	jwkRaw := mustJWK(t, agentPub)

	resp, err := idSvc.Register(RegisterReq{
		LaunchToken:    launch,
		Nonce:          nonce,
		AgentPubKey:    jwkRaw,
		Signature:      base64.RawURLEncoding.EncodeToString(sig),
		OrchId:         "orch-1",
		TaskId:         "task-1",
		RequestedScope: []string{"read:Customers:1"},
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if err := ValidateSpiffeId(resp.AgentInstanceID); err != nil {
		t.Fatalf("invalid spiffe id: %v", err)
	}
}

func TestIdSvcRegisterBadLaunchToken(t *testing.T) {
	sqlStore := store.NewSqlStore()
	_, brokerPriv, _ := GenerateSigningKeyPair()
	idSvc := NewIdSvc(sqlStore, brokerPriv, "agentauth.local")

	agentPub, agentPriv, _ := GenerateSigningKeyPair()
	nonce := "abc123nonce"
	sqlStore.PutNonce(nonce, time.Now().UTC().Add(30*time.Second))
	sig := ed25519.Sign(agentPriv, []byte(nonce))

	_, err := idSvc.Register(RegisterReq{
		LaunchToken:    "bad",
		Nonce:          nonce,
		AgentPubKey:    mustJWK(t, agentPub),
		Signature:      base64.RawURLEncoding.EncodeToString(sig),
		OrchId:         "orch-1",
		TaskId:         "task-1",
		RequestedScope: []string{"read:Customers:1"},
	})
	if err != ErrRegisterBadLaunchToken {
		t.Fatalf("expected ErrRegisterBadLaunchToken, got %v", err)
	}
}

func TestIdSvcRegisterBadSignature(t *testing.T) {
	sqlStore := store.NewSqlStore()
	_, brokerPriv, _ := GenerateSigningKeyPair()
	idSvc := NewIdSvc(sqlStore, brokerPriv, "agentauth.local")
	launch, _ := CreateLaunchToken(sqlStore, "orch-1", "task-1", []string{"read:Customers:1"}, 30*time.Second)
	agentPub, _, _ := GenerateSigningKeyPair()

	nonce := "abc123nonce"
	sqlStore.PutNonce(nonce, time.Now().UTC().Add(30*time.Second))
	_, err := idSvc.Register(RegisterReq{
		LaunchToken:    launch,
		Nonce:          nonce,
		AgentPubKey:    mustJWK(t, agentPub),
		Signature:      "deadbeef",
		OrchId:         "orch-1",
		TaskId:         "task-1",
		RequestedScope: []string{"read:Customers:1"},
	})
	if err != ErrRegisterBadSignature {
		t.Fatalf("expected ErrRegisterBadSignature, got %v", err)
	}
}

func mustJWK(t *testing.T, pub []byte) json.RawMessage {
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

