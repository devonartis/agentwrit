package token

import (
	"errors"
	"time"
)

var (
	ErrClaimsSubjectRequired = errors.New("subject is required")
	ErrClaimsScopeRequired   = errors.New("scope is required")
	ErrClaimsTaskIDRequired  = errors.New("task_id is required")
	ErrClaimsExpired         = errors.New("token is expired")
)

type DelegRecord struct {
	Agent       string   `json:"agent"`
	Scope       []string `json:"scope"`
	DelegatedAt string   `json:"delegated_at"`
	Signature   string   `json:"signature"`
}

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

