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
// during middleware validation (see [authz.ValMw]). When a [RevocationStore]
// is provided, entries are written through to SQLite so they survive broker
// restarts. On startup, [RevSvc.LoadFromEntries] rebuilds in-memory state
// from the persisted records.
package revoke

import (
	"errors"
	"sync"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/token"
)

// RevocationStore is the optional persistence backend for revocations.
type RevocationStore interface {
	SaveRevocation(level, target string) error
}

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
	store  RevocationStore
}

// NewRevSvc returns an empty revocation service ready for use. If store is
// non-nil, every Revoke call writes through to the persistence backend.
func NewRevSvc(store RevocationStore) *RevSvc {
	if store == nil {
		panic("rev_svc: RevocationStore must not be nil")
	}
	return &RevSvc{
		tokens: make(map[string]bool),
		agents: make(map[string]bool),
		tasks:  make(map[string]bool),
		chains: make(map[string]bool),
		store:  store,
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
	case "agent":
		s.agents[target] = true
	case "task":
		s.tasks[target] = true
	case "chain":
		s.chains[target] = true
	default:
		return 0, ErrInvalidLevel
	}

	if s.store != nil {
		if err := s.store.SaveRevocation(level, target); err != nil {
			obs.Warn("REVOKE", "svc", "persistence failed", "level="+level, "target="+target, "err="+err.Error())
		}
	}
	return 1, nil
}

// RevokeByJTI revokes a single token by its JTI. This implements
// [token.Revoker] so RevSvc can be wired into TknSvc for revocation
// checks inside Verify() and Renew().
func (s *RevSvc) RevokeByJTI(jti string) error {
	_, err := s.Revoke("token", jti)
	return err
}

// LoadFromEntries populates the in-memory revocation maps from a slice
// of level/target pairs. Called at broker startup after loading from SQLite.
func (s *RevSvc) LoadFromEntries(entries []struct{ Level, Target string }) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entries {
		switch e.Level {
		case "token":
			s.tokens[e.Target] = true
		case "agent":
			s.agents[e.Target] = true
		case "task":
			s.tasks[e.Target] = true
		case "chain":
			s.chains[e.Target] = true
		}
	}
}
