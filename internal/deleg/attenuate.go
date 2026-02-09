package deleg

import (
	"errors"
	"fmt"

	"github.com/divineartis/agentauth/internal/token"
)

// Attenuation errors.
var (
	// ErrScopeEscalation indicates a requested scope exceeds the parent scope.
	ErrScopeEscalation = errors.New("scope escalation")
	// ErrRequestedScopeEmpty indicates no scopes were requested for delegation.
	ErrRequestedScopeEmpty = errors.New("requested scope is empty")
)

// Attenuate validates that every requested scope is a subset of the parent scope.
// Returns the validated requested scopes on success.
// Returns ErrScopeEscalation with detail if any requested scope exceeds the parent.
func Attenuate(parentScope, requestedScope []string) ([]string, error) {
	if len(requestedScope) == 0 {
		return nil, ErrRequestedScopeEmpty
	}
	for _, req := range requestedScope {
		if !scopeCoveredBy(req, parentScope) {
			return nil, fmt.Errorf("%w: scope %s exceeds parent scope", ErrScopeEscalation, req)
		}
	}
	// Return a defensive copy.
	out := make([]string, len(requestedScope))
	copy(out, requestedScope)
	return out, nil
}

// scopeCoveredBy returns true if at least one scope in parent satisfies req.
func scopeCoveredBy(req string, parent []string) bool {
	for _, p := range parent {
		if token.ScopeMatch(req, p) {
			return true
		}
	}
	return false
}
