// Package mutauth provides agent-to-agent mutual authentication using a
// three-step cryptographic handshake protocol.
//
// The protocol guarantees that both communicating agents hold valid
// AgentAuth tokens and registered Ed25519 key pairs. The three steps are:
//
//  1. [MutAuthHdl.InitiateHandshake] — the initiator verifies its own
//     token and the target agent's registration, then produces a
//     random nonce.
//  2. [MutAuthHdl.RespondToHandshake] — the responder verifies the
//     initiator's token and identity, signs the nonce with its private
//     key, and returns a counter-nonce.
//  3. [MutAuthHdl.CompleteHandshake] — the initiator looks up the
//     responder's registered public key and verifies the nonce
//     signature, confirming the responder's identity.
//
// Supplementary types:
//
//   - [DiscoveryRegistry] maps agent SPIFFE IDs to network endpoints
//     and provides identity-consistency checks during handshakes.
//   - [HeartbeatMgr] tracks agent liveness via periodic heartbeats
//     and optionally auto-revokes agents that miss too many windows.
//
// This package exposes a Go API only; it is not registered on any HTTP
// mux in the current broker. HTTP exposure is planned for a future
// release.
package mutauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/devonartis/agentwrit/internal/obs"
	"github.com/devonartis/agentwrit/internal/store"
	"github.com/devonartis/agentwrit/internal/token"
)

// Handshake errors.
var (
	// ErrHandshakeInvalidToken indicates a token presented during handshake failed verification.
	ErrHandshakeInvalidToken = errors.New("handshake: invalid token")
	// ErrHandshakeUnknownAgent indicates the agent is not registered in the store.
	ErrHandshakeUnknownAgent = errors.New("handshake: unknown agent")
	// ErrHandshakeNonceMismatch indicates the signed nonce does not match the expected nonce.
	ErrHandshakeNonceMismatch = errors.New("handshake: nonce verification failed")
	// ErrPeerMismatch indicates the responder is not the intended target of the handshake.
	ErrPeerMismatch = errors.New("handshake: peer mismatch - responder is not the intended target")
	// ErrInitiatorMismatch indicates the initiator's token subject does not match the declared InitiatorID.
	ErrInitiatorMismatch = errors.New("handshake: initiator mismatch - token subject does not match declared ID")
	// ErrResponderMismatch indicates the responder's token subject does not match the declared ResponderID.
	ErrResponderMismatch = errors.New("handshake: responder mismatch - token subject does not match declared ID")
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
	tknSvc       *token.TknSvc
	store        *store.SqlStore
	discoveryReg *DiscoveryRegistry // nil = skip discovery binding checks
}

// NewMutAuthHdl creates a MutAuthHdl with the required token service and agent store.
// The DiscoveryRegistry is optional; when non-nil it adds discovery binding verification.
func NewMutAuthHdl(tknSvc *token.TknSvc, st *store.SqlStore, dr *DiscoveryRegistry) *MutAuthHdl {
	return &MutAuthHdl{
		tknSvc:       tknSvc,
		store:        st,
		discoveryReg: dr,
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
	initClaims, err := h.tknSvc.Verify(req.InitiatorToken)
	if err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "initiator token invalid", "error="+err.Error())
		return nil, ErrHandshakeInvalidToken
	}

	// Verify initiator's declared identity matches their token (prevents initiator spoofing).
	if initClaims.Sub != req.InitiatorID {
		obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "initiator ID mismatch",
			"declared="+req.InitiatorID, "token_sub="+initClaims.Sub)
		return nil, ErrInitiatorMismatch
	}

	if _, err := h.store.GetAgent(initClaims.Sub); err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "initiator not registered", "agent_id="+initClaims.Sub)
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

	// Verify responder is the intended target (prevents peer substitution).
	if respClaims.Sub != req.TargetAgentID {
		obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "peer mismatch",
			"expected="+req.TargetAgentID, "presented="+respClaims.Sub)
		return nil, ErrPeerMismatch
	}

	// Optional: verify target identity is consistent with discovery registry.
	if h.discoveryReg != nil {
		if _, err := h.discoveryReg.VerifyBinding(req.TargetAgentID, respClaims.Sub); err != nil {
			obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "discovery binding failed",
				"target="+req.TargetAgentID, "error="+err.Error())
			return nil, err
		}
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
	claims, err := h.tknSvc.Verify(resp.ResponderToken)
	if err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Complete", "responder token invalid", "error="+err.Error())
		return false, ErrHandshakeInvalidToken
	}
	if claims.Sub != resp.ResponderID {
		obs.Fail("MUTAUTH", "MutAuthHdl.Complete", "responder ID mismatch",
			"declared="+resp.ResponderID, "token_sub="+claims.Sub)
		return false, ErrResponderMismatch
	}

	rec, err := h.store.GetAgent(claims.Sub)
	if err != nil {
		obs.Fail("MUTAUTH", "MutAuthHdl.Complete", "responder not registered", "agent_id="+claims.Sub)
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
