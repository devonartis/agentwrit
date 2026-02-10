// Package revoke provides four-level token revocation for the AgentAuth
// broker.
//
// Revocation operates at four granularity levels:
//
//   - token: revoke a single token by its JTI.
//   - agent: revoke all tokens belonging to a SPIFFE agent ID.
//   - task:  revoke all tokens associated with a task_id.
//   - chain: revoke all tokens in a delegation chain by the root delegator's agent ID.
//
// Revocation entries are stored in memory and checked by [RevSvc.IsRevoked]
// during middleware validation (see [authz.ValMw]).
package revoke

import (
	"errors"
	"sync"

	"github.com/divineartis/agentauth/internal/token"
)

// Sentinel errors returned by [RevSvc.Revoke].
var (
	ErrInvalidLevel  = errors.New("invalid revocation level")
	ErrMissingTarget = errors.New("missing revocation target")
)

// RevSvc maintains in-memory revocation lists at four levels (token, agent,
// task, chain). All methods are safe for concurrent use.
type RevSvc struct {
	mu     sync.RWMutex
	tokens map[string]bool // JTI → revoked
	agents map[string]bool // agent SPIFFE ID → revoked
	tasks  map[string]bool // task_id → revoked
	chains map[string]bool // root delegator agent ID → revoked
}

// NewRevSvc returns an empty revocation service ready for use.
func NewRevSvc() *RevSvc {
	return &RevSvc{
		tokens: make(map[string]bool),
		agents: make(map[string]bool),
		tasks:  make(map[string]bool),
		chains: make(map[string]bool),
	}
}

// IsRevoked checks whether the given token claims match any active
// revocation entry. It checks all four levels in order: token (JTI),
// agent (subject), task (task_id), and chain (root delegator agent ID).
// It returns true if any match is found.
func (s *RevSvc) IsRevoked(claims *token.TknClaims) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check token-level (JTI)
	if s.tokens[claims.Jti] {
		return true
	}

	// Check agent-level (subject = SPIFFE ID)
	if s.agents[claims.Sub] {
		return true
	}

	// Check task-level
	if claims.TaskId != "" && s.tasks[claims.TaskId] {
		return true
	}

	// Check chain-level: if the token was delegated, the first entry in the
	// chain is the root delegator. Revoking at chain level targets the root
	// delegator's agent ID (SPIFFE ID), which invalidates every token in
	// that delegation lineage.
	if len(claims.DelegChain) > 0 {
		if s.chains[claims.DelegChain[0].Agent] {
			return true
		}
	}

	return false
}

// Revoke adds a revocation entry at the specified level for the given
// target. Valid levels are "token", "agent", "task", and "chain". The
// target is the JTI, SPIFFE ID, task_id, or root delegator agent ID respectively.
// It returns the count of entries affected (always 1 on success) and an
// error if the level is invalid or the target is empty.
func (s *RevSvc) Revoke(level, target string) (int, error) {
	if target == "" {
		return 0, ErrMissingTarget
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch level {
	case "token":
		s.tokens[target] = true
		return 1, nil
	case "agent":
		s.agents[target] = true
		return 1, nil
	case "task":
		s.tasks[target] = true
		return 1, nil
	case "chain":
		s.chains[target] = true
		return 1, nil
	default:
		return 0, ErrInvalidLevel
	}
}
