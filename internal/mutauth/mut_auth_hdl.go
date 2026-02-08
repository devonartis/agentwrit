package mutauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

// Handshake errors.
var (
	// ErrHandshakeInvalidToken indicates a token presented during handshake failed verification.
	ErrHandshakeInvalidToken = errors.New("handshake: invalid token")
	// ErrHandshakeUnknownAgent indicates the agent is not registered in the store.
	ErrHandshakeUnknownAgent = errors.New("handshake: unknown agent")
	// ErrHandshakeNonceMismatch indicates the signed nonce does not match the expected nonce.
	ErrHandshakeNonceMismatch = errors.New("handshake: nonce verification failed")
)

// HandshakeReq is the initiator's opening message in the mutual authentication protocol.
type HandshakeReq struct {
	InitiatorToken string
	InitiatorID    string
	TargetAgentID  string
	Nonce          string
}

// HandshakeResp is the responder's reply, containing its own token and a signed copy of the initiator's nonce.
type HandshakeResp struct {
	ResponderToken string
	ResponderID    string
	SignedNonce    []byte
	Nonce          string // responder's counter-nonce for the initiator to verify
}

// MutAuthHdl orchestrates the 3-step mutual authentication handshake between agents.
type MutAuthHdl struct {
	tknSvc *token.TknSvc
	store  *store.SqlStore
}

// NewMutAuthHdl creates a MutAuthHdl with the required token service and agent store.
func NewMutAuthHdl(tknSvc *token.TknSvc, st *store.SqlStore) *MutAuthHdl {
	return &MutAuthHdl{
		tknSvc: tknSvc,
		store:  st,
	}
}

// InitiateHandshake begins the protocol: verifies the initiator's token, confirms the
// target agent exists, and produces a HandshakeReq containing a fresh nonce.
func (h *MutAuthHdl) InitiateHandshake(initiatorToken, targetAgentID string) (*HandshakeReq, error) {
	claims, err := h.tknSvc.Verify(initiatorToken)
	if err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Initiate", "initiator token invalid", "error="+err.Error())
		return nil, ErrHandshakeInvalidToken
	}

	if _, err := h.store.GetAgent(claims.Sub); err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Initiate", "initiator not registered", "agent_id="+claims.Sub)
		return nil, ErrHandshakeUnknownAgent
	}

	if _, err := h.store.GetAgent(targetAgentID); err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Initiate", "target not registered", "agent_id="+targetAgentID)
		return nil, ErrHandshakeUnknownAgent
	}

	nonce, err := randomNonce()
	if err != nil {
		return nil, err
	}

	obs.Ok("MUTAUTH", "MutAuthHdl.Initiate", "handshake initiated",
		"initiator="+claims.Sub, "target="+targetAgentID)

	return &HandshakeReq{
		InitiatorToken: initiatorToken,
		InitiatorID:    claims.Sub,
		TargetAgentID:  targetAgentID,
		Nonce:          nonce,
	}, nil
}

// RespondToHandshake is step 2: the target agent verifies the initiator's token and identity,
// signs the initiator's nonce with its own private key, and includes a counter-nonce.
func (h *MutAuthHdl) RespondToHandshake(req *HandshakeReq, responderToken string, responderKey ed25519.PrivateKey) (*HandshakeResp, error) {
	if _, err := h.tknSvc.Verify(req.InitiatorToken); err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "initiator token invalid", "error="+err.Error())
		return nil, ErrHandshakeInvalidToken
	}

	if _, err := h.store.GetAgent(req.InitiatorID); err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "initiator not registered", "agent_id="+req.InitiatorID)
		return nil, ErrHandshakeUnknownAgent
	}

	respClaims, err := h.tknSvc.Verify(responderToken)
	if err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "responder token invalid", "error="+err.Error())
		return nil, ErrHandshakeInvalidToken
	}

	if _, err := h.store.GetAgent(respClaims.Sub); err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "responder not registered", "agent_id="+respClaims.Sub)
		return nil, ErrHandshakeUnknownAgent
	}

	signed := ed25519.Sign(responderKey, []byte(req.Nonce))

	counterNonce, err := randomNonce()
	if err != nil {
		return nil, err
	}

	obs.Ok("MUTAUTH", "MutAuthHdl.Respond", "handshake responded",
		"initiator="+req.InitiatorID, "responder="+respClaims.Sub)

	return &HandshakeResp{
		ResponderToken: responderToken,
		ResponderID:    respClaims.Sub,
		SignedNonce:    signed,
		Nonce:          counterNonce,
	}, nil
}

// CompleteHandshake is step 3: the initiator verifies the responder's token, looks up the
// responder's registered public key, and verifies the nonce signature to confirm identity.
func (h *MutAuthHdl) CompleteHandshake(resp *HandshakeResp, originalNonce string) (bool, error) {
	if _, err := h.tknSvc.Verify(resp.ResponderToken); err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Complete", "responder token invalid", "error="+err.Error())
		return false, ErrHandshakeInvalidToken
	}

	rec, err := h.store.GetAgent(resp.ResponderID)
	if err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Complete", "responder not registered", "agent_id="+resp.ResponderID)
		return false, ErrHandshakeUnknownAgent
	}

	if !ed25519.Verify(ed25519.PublicKey(rec.PublicKey), []byte(originalNonce), resp.SignedNonce) {
		obs.Fail("MUTAUTH", "MutAuthHdl.Complete", "nonce signature mismatch", "responder="+resp.ResponderID)
		return false, ErrHandshakeNonceMismatch
	}

	obs.Ok("MUTAUTH", "MutAuthHdl.Complete", "handshake completed",
		"responder="+resp.ResponderID)

	return true, nil
}

func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
