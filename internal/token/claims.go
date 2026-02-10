package token

import (
	"errors"
	"time"
)

// Claims validation errors.
var (
	// ErrClaimsSubjectRequired indicates the sub claim is missing.
	ErrClaimsSubjectRequired = errors.New("subject is required")
	// ErrClaimsScopeRequired indicates the scope claim is empty.
	ErrClaimsScopeRequired = errors.New("scope is required")
	// ErrClaimsTaskIDRequired indicates the task_id claim is missing.
	ErrClaimsTaskIDRequired = errors.New("task_id is required")
	// ErrClaimsExpired indicates the token has expired based on its exp claim.
	ErrClaimsExpired = errors.New("token is expired")
)

// DelegRecord represents a single entry in a token's delegation chain.
type DelegRecord struct {
	Agent       string   `json:"agent"`
	Scope       []string `json:"scope"`
	DelegatedAt string   `json:"delegated_at"`
	Signature   string   `json:"signature"`
}

// TknClaims holds the JWT claims for an AgentAuth broker-issued token.
type TknClaims struct {
	Iss        string        `json:"iss"`
	Sub        string        `json:"sub"`
	Aud        []string      `json:"aud,omitempty"`
	Exp        int64         `json:"exp"`
	Nbf        int64         `json:"nbf,omitempty"`
	Iat        int64         `json:"iat,omitempty"`
	Jti        string        `json:"jti,omitempty"`
	Scope      []string      `json:"scope"`
	TaskId     string        `json:"task_id"`
	OrchId     string        `json:"orchestration_id"`
	DelegChain []DelegRecord `json:"delegation_chain"`
}

// Validate enforces required claims for AgentAuth broker-issued task tokens.
func (c TknClaims) Validate(now time.Time) error {
	if c.Sub == "" {
		return ErrClaimsSubjectRequired
	}
	if len(c.Scope) == 0 {
		return ErrClaimsScopeRequired
	}
	if c.TaskId == "" {
		return ErrClaimsTaskIDRequired
	}
	if c.Exp == 0 || now.UTC().Unix() >= c.Exp {
		return ErrClaimsExpired
	}
	return nil
}

