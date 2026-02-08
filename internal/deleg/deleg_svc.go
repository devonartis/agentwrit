package deleg

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/token"
)

// Delegation errors.
var (
	// ErrDelegatorTokenInvalid indicates the delegator's token failed verification.
	ErrDelegatorTokenInvalid = errors.New("delegator token invalid")
	// ErrDepthExceeded indicates the delegation chain has reached maximum depth.
	ErrDepthExceeded = errors.New("delegation depth exceeded")
	// ErrTTLExceedsRemaining indicates the requested TTL exceeds the delegator's remaining TTL.
	ErrTTLExceedsRemaining = errors.New("max_ttl exceeds delegator remaining TTL")
	// ErrTargetAgentEmpty indicates the target agent ID is missing.
	ErrTargetAgentEmpty = errors.New("target_agent_id is required")
)

// DelegReq holds the parameters for a delegation request.
type DelegReq struct {
	DelegatorToken string   `json:"delegator_token"`
	TargetAgentId  string   `json:"target_agent_id"`
	DelegatedScope []string `json:"delegated_scope"`
	MaxTTL         int      `json:"max_ttl"`
}

// DelegResp holds the result of a successful delegation.
type DelegResp struct {
	DelegationToken string `json:"delegation_token"`
	ChainHash       string `json:"chain_hash"`
	DelegationDepth int    `json:"delegation_depth"`
}

// DelegSvc manages delegation token creation with scope attenuation and chain integrity.
type DelegSvc struct {
	tknSvc     *token.TknSvc
	signingKey ed25519.PrivateKey
	maxDepth   int
}

// NewDelegSvc creates a DelegSvc with the given token service, signing key, and max chain depth.
func NewDelegSvc(tknSvc *token.TknSvc, signingKey ed25519.PrivateKey, maxDepth int) *DelegSvc {
	return &DelegSvc{
		tknSvc:     tknSvc,
		signingKey: signingKey,
		maxDepth:   maxDepth,
	}
}

// Delegate creates a delegation token with attenuated scope for a target agent.
// It verifies the delegator's token, enforces scope attenuation, TTL constraints,
// and maximum chain depth. Returns the new delegation token with chain metadata.
func (s *DelegSvc) Delegate(req DelegReq) (*DelegResp, error) {
	// Verify delegator's token.
	claims, err := s.tknSvc.Verify(req.DelegatorToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDelegatorTokenInvalid, err)
	}

	if req.TargetAgentId == "" {
		return nil, ErrTargetAgentEmpty
	}

	// Enforce max delegation depth.
	currentDepth := len(claims.DelegChain)
	if currentDepth >= s.maxDepth {
		return nil, fmt.Errorf("%w: current=%d max=%d", ErrDepthExceeded, currentDepth, s.maxDepth)
	}

	// Attenuate scope.
	attenuated, err := Attenuate(claims.Scope, req.DelegatedScope)
	if err != nil {
		return nil, err
	}

	// Enforce TTL constraint.
	now := time.Now().UTC()
	remaining := int(claims.Exp - now.Unix())
	if remaining <= 0 {
		return nil, fmt.Errorf("%w: delegator token expired", ErrDelegatorTokenInvalid)
	}
	ttl := req.MaxTTL
	if ttl <= 0 {
		ttl = remaining
	}
	if ttl > remaining {
		return nil, fmt.Errorf("%w: requested=%d remaining=%d", ErrTTLExceedsRemaining, ttl, remaining)
	}

	// Build new delegation record with Ed25519 signature.
	record := token.DelegRecord{
		Agent:       claims.Sub,
		Scope:       attenuated,
		DelegatedAt: now.Format(time.RFC3339),
	}
	recordBytes, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal delegation record: %w", err)
	}
	sig := ed25519.Sign(s.signingKey, recordBytes)
	record.Signature = hex.EncodeToString(sig)

	// Build the full chain.
	chain := make([]token.DelegRecord, len(claims.DelegChain), len(claims.DelegChain)+1)
	copy(chain, claims.DelegChain)
	chain = append(chain, record)

	// Compute chain hash (SHA-256 of the entire serialized chain).
	chainHash, err := computeChainHash(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to compute chain hash: %w", err)
	}

	// Issue new JWT with delegation chain for the target agent.
	resp, err := s.tknSvc.Issue(token.IssueReq{
		AgentID:    req.TargetAgentId,
		OrchID:     claims.OrchId,
		TaskID:     claims.TaskId,
		Scope:      attenuated,
		TTLSecond:  ttl,
		DelegChain: chain,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to issue delegation token: %w", err)
	}

	obs.Ok("DELEG", "DelegSvc.Delegate", "delegation created",
		"delegator="+claims.Sub,
		"target="+req.TargetAgentId,
		fmt.Sprintf("depth=%d", currentDepth+1),
		"chain_hash="+chainHash,
	)

	return &DelegResp{
		DelegationToken: resp.AccessToken,
		ChainHash:       chainHash,
		DelegationDepth: currentDepth + 1,
	}, nil
}

// computeChainHash returns the hex-encoded SHA-256 hash of the JSON-serialized chain.
func computeChainHash(chain []token.DelegRecord) (string, error) {
	data, err := json.Marshal(chain)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}
