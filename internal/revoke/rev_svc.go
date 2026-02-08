package revoke

import (
	"sync"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
)

// RevChecker is the interface for revocation lookups.
// In-memory implementation now; Redis can be plugged in later.
type RevChecker interface {
	IsTokenRevoked(jti string) bool
	IsAgentRevoked(agentID string) bool
	IsTaskRevoked(taskID string) bool
	IsChainRevoked(chainHash string) bool
	// IsRevoked checks all 4 levels given full claims context.
	// Returns (revoked, level) where level indicates which check matched.
	IsRevoked(jti, agentID, taskID, chainHash string) (bool, string)
}

// RevRecord stores metadata about a single revocation.
type RevRecord struct {
	TargetID  string
	Level     string // "token" | "agent" | "task" | "delegation_chain"
	Reason    string
	RevokedAt time.Time
}

// RevSvc manages in-memory revocation state.
type RevSvc struct {
	mu         sync.RWMutex
	tokens     map[string]RevRecord // jti → record
	agents     map[string]RevRecord // agentID → record
	tasks      map[string]RevRecord // taskID → record
	chains     map[string]RevRecord // chainHash → record
	checkCount uint64
	hitCount   uint64
}

// NewRevSvc creates a new revocation service with empty revocation sets.
func NewRevSvc() *RevSvc {
	svc := &RevSvc{
		tokens: make(map[string]RevRecord),
		agents: make(map[string]RevRecord),
		tasks:  make(map[string]RevRecord),
		chains: make(map[string]RevRecord),
	}
	obs.SetRevocationCacheHitRatio(0)
	return svc
}

// RevokeToken revokes a specific token by its JTI.
func (r *RevSvc) RevokeToken(jti, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[jti] = RevRecord{
		TargetID:  jti,
		Level:     "token",
		Reason:    reason,
		RevokedAt: time.Now().UTC(),
	}
	obs.Ok("REVOKE", "RevSvc.RevokeToken", "token revoked", "jti="+jti, "reason="+reason)
	return nil
}

// RevokeAgent revokes all tokens for a given agent ID.
func (r *RevSvc) RevokeAgent(agentID, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agentID] = RevRecord{
		TargetID:  agentID,
		Level:     "agent",
		Reason:    reason,
		RevokedAt: time.Now().UTC(),
	}
	obs.Ok("REVOKE", "RevSvc.RevokeAgent", "agent revoked", "agent_id="+agentID, "reason="+reason)
	return nil
}

// RevokeTask revokes all tokens for a given task ID.
func (r *RevSvc) RevokeTask(taskID, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[taskID] = RevRecord{
		TargetID:  taskID,
		Level:     "task",
		Reason:    reason,
		RevokedAt: time.Now().UTC(),
	}
	obs.Ok("REVOKE", "RevSvc.RevokeTask", "task revoked", "task_id="+taskID, "reason="+reason)
	return nil
}

// RevokeDelegChain revokes all tokens with a given delegation chain hash.
func (r *RevSvc) RevokeDelegChain(chainHash, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chains[chainHash] = RevRecord{
		TargetID:  chainHash,
		Level:     "delegation_chain",
		Reason:    reason,
		RevokedAt: time.Now().UTC(),
	}
	obs.Ok("REVOKE", "RevSvc.RevokeDelegChain", "delegation chain revoked", "chain_hash="+chainHash, "reason="+reason)
	return nil
}

// IsTokenRevoked returns true if the given JTI has been revoked.
func (r *RevSvc) IsTokenRevoked(jti string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tokens[jti]
	return ok
}

// IsAgentRevoked returns true if the given agent ID has been revoked.
func (r *RevSvc) IsAgentRevoked(agentID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.agents[agentID]
	return ok
}

// IsTaskRevoked returns true if the given task ID has been revoked.
func (r *RevSvc) IsTaskRevoked(taskID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tasks[taskID]
	return ok
}

// IsChainRevoked returns true if the given delegation chain hash has been revoked.
func (r *RevSvc) IsChainRevoked(chainHash string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.chains[chainHash]
	return ok
}

// IsRevoked checks all 4 revocation levels given full claims context.
// Returns (revoked bool, level string) where level indicates which check matched.
func (r *RevSvc) IsRevoked(jti, agentID, taskID, chainHash string) (bool, string) {
	r.mu.Lock()
	revoked := false
	level := ""
	if jti != "" {
		if _, ok := r.tokens[jti]; ok {
			revoked = true
			level = "token"
		}
	}
	if !revoked && agentID != "" {
		if _, ok := r.agents[agentID]; ok {
			revoked = true
			level = "agent"
		}
	}
	if !revoked && taskID != "" {
		if _, ok := r.tasks[taskID]; ok {
			revoked = true
			level = "task"
		}
	}
	if !revoked && chainHash != "" {
		if _, ok := r.chains[chainHash]; ok {
			revoked = true
			level = "delegation_chain"
		}
	}
	r.checkCount++
	if revoked {
		r.hitCount++
	}
	ratio := 0.0
	if r.checkCount > 0 {
		ratio = float64(r.hitCount) / float64(r.checkCount)
	}
	r.mu.Unlock()
	obs.SetRevocationCacheHitRatio(ratio)
	return revoked, level
}
