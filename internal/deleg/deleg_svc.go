// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

// Package deleg provides scope-attenuated token delegation with chain
// verification and depth limiting.
//
// Delegation allows an authenticated agent to issue a narrower-scoped token
// to another registered agent. The delegated token carries a delegation
// chain that records the full provenance (who delegated what scope, and
// when). Delegation depth is capped at [maxDelegDepth] (5) to prevent
// unbounded chains.
//
// The delegated scope must be a strict subset of the delegator's scope
// (attenuation only — scopes can never expand).
package deleg

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/devonartis/agentwrit/internal/audit"
	"github.com/devonartis/agentwrit/internal/authz"
	"github.com/devonartis/agentwrit/internal/obs"
	"github.com/devonartis/agentwrit/internal/store"
	"github.com/devonartis/agentwrit/internal/token"
)

// maxDelegDepth is the maximum allowed delegation chain length. Attempts
// to delegate beyond this depth return [ErrDepthExceeded].
const maxDelegDepth = 5

// Sentinel errors returned by [DelegSvc.Delegate].
var (
	ErrScopeViolation   = errors.New("delegated scope exceeds delegator scope")
	ErrDepthExceeded    = errors.New("delegation depth limit exceeded")
	ErrDelegateNotFound = errors.New("delegate agent not found")
	ErrMissingField     = errors.New("missing required field")
)

// DelegReq is the JSON request body for POST /v1/delegate. DelegateTo
// must be the SPIFFE ID of an already-registered agent.
type DelegReq struct {
	DelegateTo string   `json:"delegate_to"`
	Scope      []string `json:"scope"`
	TTL        int      `json:"ttl"`
}

// DelegResp is the response returned on successful delegation. It contains
// the newly issued token, its TTL, and the complete delegation chain
// including the new entry.
type DelegResp struct {
	AccessToken     string              `json:"access_token"`
	ExpiresIn       int                 `json:"expires_in"`
	DelegationChain []token.DelegRecord `json:"delegation_chain"`
}

// DelegSvc is the delegation service. It verifies scope attenuation,
// enforces depth limits, appends to the delegation chain, and issues
// a new token for the delegate agent. Each delegation record is signed
// with the broker's Ed25519 key, and the complete chain is hashed
// (SHA-256) into the delegated token's chain_hash claim.
type DelegSvc struct {
	tknSvc     *token.TknSvc
	store      *store.SqlStore
	auditLog   *audit.AuditLog
	signingKey ed25519.PrivateKey
}

// NewDelegSvc creates a new delegation service. The signingKey is the
// broker's Ed25519 private key used to sign delegation records. The
// auditLog parameter may be nil to disable audit recording.
func NewDelegSvc(tknSvc *token.TknSvc, st *store.SqlStore, auditLog *audit.AuditLog, signingKey ed25519.PrivateKey) *DelegSvc {
	return &DelegSvc{
		tknSvc:     tknSvc,
		store:      st,
		auditLog:   auditLog,
		signingKey: signingKey,
	}
}

// Delegate creates a scope-attenuated delegation token for the agent
// specified in req.DelegateTo. It performs the following checks:
//
//  1. Required fields (delegate_to, scope) are present.
//  2. Delegation depth has not exceeded [maxDelegDepth].
//  3. Requested scope is a subset of the delegator's scope.
//  4. Delegate agent exists in the store.
//
// On success it appends a new [token.DelegRecord] to the chain, issues
// a JWT for the delegate, and records an audit event.
func (s *DelegSvc) Delegate(delegatorClaims *token.TknClaims, req DelegReq) (*DelegResp, error) {
	if req.DelegateTo == "" {
		return nil, fmt.Errorf("%w: delegate_to", ErrMissingField)
	}
	if len(req.Scope) == 0 {
		return nil, fmt.Errorf("%w: scope", ErrMissingField)
	}

	// Check delegation depth
	currentDepth := len(delegatorClaims.DelegChain)
	if currentDepth >= maxDelegDepth {
		return nil, ErrDepthExceeded
	}

	// Check scope attenuation — delegated scope MUST be subset of delegator's scope
	if !authz.ScopeIsSubset(req.Scope, delegatorClaims.Scope) {
		if s.auditLog != nil {
			s.auditLog.Record(audit.EventDelegationAttenuationViolation,
				delegatorClaims.Sub, delegatorClaims.TaskId, delegatorClaims.OrchId,
				fmt.Sprintf("delegation_attenuation_violation | delegator=%s | target=%s | requested=%v | allowed=%v",
					delegatorClaims.Sub, req.DelegateTo, req.Scope, delegatorClaims.Scope),
				audit.WithOutcome("denied"), audit.WithDelegDepth(currentDepth))
		}
		return nil, ErrScopeViolation
	}

	// Verify delegate agent exists
	_, err := s.store.GetAgent(req.DelegateTo)
	if err != nil {
		return nil, ErrDelegateNotFound
	}

	// Build delegation chain
	chain := make([]token.DelegRecord, len(delegatorClaims.DelegChain))
	copy(chain, delegatorClaims.DelegChain)

	newRecord := token.DelegRecord{
		Agent:       delegatorClaims.Sub,
		Scope:       delegatorClaims.Scope,
		DelegatedAt: time.Now().UTC(),
	}

	// Sign the delegation record: Agent + Scope + DelegatedAt
	newRecord.Signature = s.signRecord(newRecord)

	chain = append(chain, newRecord)

	// Compute chain_hash: SHA-256 of JSON-serialized chain
	chainHash, err := computeChainHash(chain)
	if err != nil {
		return nil, fmt.Errorf("compute chain hash: %w", err)
	}

	ttl := req.TTL
	if ttl <= 0 {
		ttl = 60
	}

	// Issue delegated token
	issResp, err := s.tknSvc.Issue(token.IssueReq{
		Sub:        req.DelegateTo,
		Aud:        delegatorClaims.Aud,
		Scope:      req.Scope,
		TaskId:     delegatorClaims.TaskId,
		OrchId:     delegatorClaims.OrchId,
		TTL:        ttl,
		DelegChain: chain,
		ChainHash:  chainHash,
	})
	if err != nil {
		return nil, fmt.Errorf("issue delegated token: %w", err)
	}

	// Audit
	if s.auditLog != nil {
		s.auditLog.Record(audit.EventDelegationCreated, req.DelegateTo, delegatorClaims.TaskId, delegatorClaims.OrchId,
			fmt.Sprintf("delegation from %s to %s scope=%v depth=%d", delegatorClaims.Sub, req.DelegateTo, req.Scope, currentDepth+1),
			audit.WithOutcome("success"), audit.WithDelegDepth(currentDepth+1), audit.WithDelegChainHash(chainHash))
	}

	obs.Ok("DELEG", "DelegSvc", "delegation created",
		"from="+delegatorClaims.Sub, "to="+req.DelegateTo, fmt.Sprintf("scope=%v", req.Scope))

	return &DelegResp{
		AccessToken:     issResp.AccessToken,
		ExpiresIn:       issResp.ExpiresIn,
		DelegationChain: chain,
	}, nil
}

// signRecord signs the delegation record content (Agent + Scope +
// DelegatedAt) with the broker's Ed25519 private key and returns
// the hex-encoded signature.
func (s *DelegSvc) signRecord(rec token.DelegRecord) string {
	// Canonical content: agent|scope_csv|timestamp_rfc3339
	content := rec.Agent + "|" + scopeCSV(rec.Scope) + "|" + rec.DelegatedAt.Format(time.RFC3339Nano)
	sig := ed25519.Sign(s.signingKey, []byte(content))
	return hex.EncodeToString(sig)
}

// scopeCSV joins scope strings with commas for canonical signing input.
func scopeCSV(scopes []string) string {
	result := ""
	for i, s := range scopes {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}

// computeChainHash returns the hex-encoded SHA-256 hash of the
// JSON-serialized delegation chain.
func computeChainHash(chain []token.DelegRecord) (string, error) {
	data, err := json.Marshal(chain)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}
