package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/store"
)

var (
	// ErrRegisterBadLaunchToken indicates the provided launch token is invalid or expired.
	ErrRegisterBadLaunchToken = errors.New("invalid launch token")
	// ErrRegisterBadNonce indicates the provided nonce is invalid or already consumed.
	ErrRegisterBadNonce = errors.New("invalid nonce")
	// ErrRegisterBadSignature indicates the agent's Ed25519 signature verification failed.
	ErrRegisterBadSignature = errors.New("invalid signature")
)

// RegisterReq holds the parameters required for agent registration.
type RegisterReq struct {
	LaunchToken    string
	Nonce          string
	AgentPubKey    json.RawMessage
	Signature      string
	OrchId         string
	TaskId         string
	RequestedScope []string
}

// RegisterResp contains the result of a successful agent registration.
type RegisterResp struct {
	AgentInstanceID string
	OrchId          string
	TaskId          string
	Scope           []string
}

// IdSvc is the identity service that handles agent registration and SPIFFE ID assignment.
type IdSvc struct {
	sqlStore    *store.SqlStore
	signingKey  ed25519.PrivateKey
	trustDomain string
}

// NewIdSvc creates a new identity service with the given store, signing key, and trust domain.
func NewIdSvc(sqlStore *store.SqlStore, signingKey ed25519.PrivateKey, trustDomain string) *IdSvc {
	return &IdSvc{
		sqlStore:    sqlStore,
		signingKey:  signingKey,
		trustDomain: trustDomain,
	}
}

// Register validates the agent's challenge-response proof and creates a new agent identity.
func (s *IdSvc) Register(req RegisterReq) (*RegisterResp, error) {
	if _, err := ValidateLaunchToken(s.sqlStore, req.LaunchToken); err != nil {
		return nil, ErrRegisterBadLaunchToken
	}
	if err := s.sqlStore.ConsumeNonce(req.Nonce); err != nil {
		return nil, ErrRegisterBadNonce
	}

	pub, err := ParseAgentPubKey(req.AgentPubKey)
	if err != nil {
		return nil, ErrRegisterBadSignature
	}
	sig, err := decodeSignature(req.Signature)
	if err != nil {
		return nil, ErrRegisterBadSignature
	}
	if !ed25519.Verify(pub, []byte(req.Nonce), sig) {
		return nil, ErrRegisterBadSignature
	}

	instID, err := randomInstanceID()
	if err != nil {
		obs.Fail("IDENTITY", "IdSvc.Register", "instance id generation failed", "error="+err.Error())
		return nil, err
	}
	agentID := NewSpiffeId(s.trustDomain, req.OrchId, req.TaskId, instID)

	if err := s.sqlStore.SaveAgent(store.AgentRecord{
		AgentID:    agentID,
		OrchId:     req.OrchId,
		TaskId:     req.TaskId,
		Scope:      append([]string{}, req.RequestedScope...),
		CreatedAt:  time.Now().UTC(),
		PublicKey:  pub,
		LastNonce:  req.Nonce,
		LastSigRaw: sig,
	}); err != nil {
		obs.Fail("IDENTITY", "IdSvc.Register", "agent persist failed", "error="+err.Error())
		return nil, err
	}

	obs.Ok("IDENTITY", "IdSvc.Register", "agent registered", "agent_id="+agentID)
	return &RegisterResp{
		AgentInstanceID: agentID,
		OrchId:          req.OrchId,
		TaskId:          req.TaskId,
		Scope:           append([]string{}, req.RequestedScope...),
	}, nil
}

func decodeSignature(sig string) ([]byte, error) {
	// Register clients can submit either hex or base64url signature payloads.
	if b, err := hex.DecodeString(strings.TrimSpace(sig)); err == nil {
		return b, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(sig))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func randomInstanceID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("random instance id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

