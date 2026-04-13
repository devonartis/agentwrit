// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0

// Package authz provides scope-based authorization and Bearer token
// validation middleware for the AgentAuth broker.
//
// # Scope Model
//
// Scopes follow the format "action:resource:identifier" (e.g.
// "read:data:project-42"). The identifier segment may be the wildcard "*"
// which matches any identifier for that action/resource pair.
//
// Scope attenuation is enforced at registration, delegation, and every
// authenticated request: a set of requested scopes is valid only when every
// element is covered by the allowed scopes. See [ScopeIsSubset].
//
// # Middleware
//
// [ValMw] is an HTTP middleware that extracts a Bearer token from the
// Authorization header, verifies it with [TokenVerifier], checks revocation
// via [RevocationChecker], and stores the decoded [token.TknClaims] in the
// request context. Downstream handlers retrieve claims with
// [ClaimsFromContext].
//
// For endpoints that require a specific scope, wrap the handler with
// [WithRequiredScope].
package authz

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidScope is returned by [ParseScope] when the input does not
// contain exactly three non-empty colon-separated segments.
var ErrInvalidScope = errors.New("invalid scope format: expected action:resource:identifier")

// ParseScope splits a scope string into its three components. The input
// must have the form "action:resource:identifier" where none of the parts
// are empty. On failure it returns [ErrInvalidScope].
func ParseScope(s string) (action, resource, identifier string, err error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("%w: %q", ErrInvalidScope, s)
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("%w: %q", ErrInvalidScope, s)
	}
	return parts[0], parts[1], parts[2], nil
}

// scopeCovers checks if allowed scope B covers requested scope A.
// A is covered by B if: A.action == B.action AND A.resource == B.resource AND
// (A.identifier == B.identifier OR B.identifier == "*").
func scopeCovers(requested, allowed string) bool {
	rAct, rRes, rId, err := ParseScope(requested)
	if err != nil {
		return false
	}
	aAct, aRes, aId, err := ParseScope(allowed)
	if err != nil {
		return false
	}

	if rAct != aAct || rRes != aRes {
		return false
	}
	return rId == aId || aId == "*"
}

// ScopeIsSubset returns true if every scope in requested is covered by at
// least one scope in allowed. A scope is covered when its action and
// resource match and either the identifiers are equal or the allowed
// identifier is the wildcard "*". This enforces the attenuation rule:
// scopes can only narrow, never expand.
func ScopeIsSubset(requested, allowed []string) bool {
	for _, req := range requested {
		covered := false
		for _, allow := range allowed {
			if scopeCovers(req, allow) {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}
