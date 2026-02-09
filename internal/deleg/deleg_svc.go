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
	"errors"
	"fmt"
	"time"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
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
	AccessToken     string               `json:"access_token"`
	ExpiresIn       int                  `json:"expires_in"`
	DelegationChain []token.DelegRecord  `json:"delegation_chain"`
}

// DelegSvc is the delegation service. It verifies scope attenuation,
// enforces depth limits, appends to the delegation chain, and issues
// a new token for the delegate agent.
type DelegSvc struct {
	tknSvc   *token.TknSvc
	store    *store.SqlStore
	auditLog *audit.AuditLog
}

// NewDelegSvc creates a new delegation service. The auditLog parameter may
// be nil to disable audit recording.
func NewDelegSvc(tknSvc *token.TknSvc, st *store.SqlStore, auditLog *audit.AuditLog) *DelegSvc {
	return &DelegSvc{
		tknSvc:   tknSvc,
		store:    st,
		auditLog: auditLog,
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
	chain = append(chain, token.DelegRecord{
		Agent:       delegatorClaims.Sub,
		Scope:       delegatorClaims.Scope,
		DelegatedAt: time.Now().UTC(),
	})

	ttl := req.TTL
	if ttl <= 0 {
		ttl = 60
	}

	// Issue delegated token
	issResp, err := s.tknSvc.Issue(token.IssueReq{
		Sub:        req.DelegateTo,
		Scope:      req.Scope,
		TaskId:     delegatorClaims.TaskId,
		OrchId:     delegatorClaims.OrchId,
		TTL:        ttl,
		DelegChain: chain,
	})
	if err != nil {
		return nil, fmt.Errorf("issue delegated token: %w", err)
	}

	// Audit
	if s.auditLog != nil {
		s.auditLog.Record(audit.EventDelegationCreated, req.DelegateTo, delegatorClaims.TaskId, delegatorClaims.OrchId,
			fmt.Sprintf("delegation from %s to %s scope=%v depth=%d", delegatorClaims.Sub, req.DelegateTo, req.Scope, currentDepth+1))
	}

	obs.Ok("DELEG", "DelegSvc", "delegation created",
		"from="+delegatorClaims.Sub, "to="+req.DelegateTo, fmt.Sprintf("scope=%v", req.Scope))

	return &DelegResp{
		AccessToken:     issResp.AccessToken,
		ExpiresIn:       issResp.ExpiresIn,
		DelegationChain: chain,
	}, nil
}
