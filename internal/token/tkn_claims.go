// Package token implements EdDSA (Ed25519) JWT token issuance, verification,
// and renewal for the AgentAuth broker.
//
// Tokens follow the compact JWT serialization (header.payload.signature) with
// algorithm "EdDSA". Claims include standard fields (iss, sub, exp, nbf, iat,
// jti) plus AgentAuth extensions (scope, task_id, orch_id, delegation_chain).
//
// The issuer is always "agentauth". Subjects are SPIFFE-format agent IDs
// (see package identity). Scopes use the "action:resource:identifier" format
// (see package authz).
package token

import (
	"errors"
	"time"
)

// Sentinel errors returned by [TknClaims.Validate].
var (
	ErrTokenExpired     = errors.New("token expired")
	ErrTokenNotYetValid = errors.New("token not yet valid")
	ErrInvalidIssuer    = errors.New("invalid issuer")
	ErrMissingJTI       = errors.New("missing jti")
	ErrMissingSubject   = errors.New("missing subject")
)

// TknClaims represents the JWT payload for an AgentAuth token. Standard
// registered claims (iss, sub, aud, exp, nbf, iat, jti) are complemented
// by private claims for scope enforcement, task tracking, and delegation.
type TknClaims struct {
	Iss        string        `json:"iss"`
	Sub        string        `json:"sub"`
	Aud        []string      `json:"aud,omitempty"`
	Exp        int64         `json:"exp"`
	Nbf        int64         `json:"nbf"`
	Iat        int64         `json:"iat"`
	Jti        string        `json:"jti"`
	Scope      []string      `json:"scope"`
	TaskId     string        `json:"task_id,omitempty"`
	OrchId     string        `json:"orch_id,omitempty"`
	DelegChain []DelegRecord `json:"delegation_chain,omitempty"`
	ChainHash  string        `json:"chain_hash,omitempty"`
}

// DelegRecord represents a single link in a delegation chain. Each entry
// records the delegating agent, the scope it held at the time of
// delegation, and a timestamp. The chain is appended to the token claims
// of the delegated token so the full provenance is visible to verifiers.
type DelegRecord struct {
	Agent       string    `json:"agent"`
	Scope       []string  `json:"scope"`
	DelegatedAt time.Time `json:"delegated_at"`
	Signature   string    `json:"signature,omitempty"`
}

// Validate checks structural integrity and temporal validity of the claims.
// It returns an error if the issuer is not "agentauth", the subject or JTI
// is empty, the token has expired, or the token is not yet valid (nbf).
func (c *TknClaims) Validate() error {
	if c.Iss != "agentauth" {
		return ErrInvalidIssuer
	}
	if c.Sub == "" {
		return ErrMissingSubject
	}
	if c.Jti == "" {
		return ErrMissingJTI
	}
	now := time.Now().Unix()
	if c.Exp != 0 && now > c.Exp {
		return ErrTokenExpired
	}
	if c.Nbf != 0 && now < c.Nbf {
		return ErrTokenNotYetValid
	}
	return nil
}
