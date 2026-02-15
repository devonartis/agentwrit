package main

import (
	"crypto/ed25519"
	"sync"
	"time"
)

// agentEntry holds registration state for one agent.
type agentEntry struct {
	spiffeID     string
	pubKey       ed25519.PublicKey
	privKey      ed25519.PrivateKey // nil for BYOK agents
	registeredAt time.Time
	lastToken    string    // last successfully issued token
	lastScope    []string  // scope of last token
	lastTokenTTL int       // TTL in seconds
	lastTokenAt  time.Time // when the token was issued
}

// agentRegistry is an in-memory, ephemeral store of registered agents.
// Agents are keyed by "agent_name:task_id". Each entry holds the SPIFFE ID
// and optionally the Ed25519 keypair (nil privKey for BYOK agents).
type agentRegistry struct {
	mu      sync.RWMutex
	agents  map[string]*agentEntry
	locks   map[string]*sync.Mutex // per-agent registration locks
	locksMu sync.Mutex
}

func newAgentRegistry() *agentRegistry {
	return &agentRegistry{
		agents: make(map[string]*agentEntry),
		locks:  make(map[string]*sync.Mutex),
	}
}

// lookup returns the agent entry for the given key, or nil+false if not found.
func (r *agentRegistry) lookup(key string) (*agentEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.agents[key]
	return e, ok
}

// store adds or replaces an agent entry.
func (r *agentRegistry) store(key string, entry *agentEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[key] = entry
}

// count returns the number of registered agents.
func (r *agentRegistry) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// cacheToken stores a successful token response for the given agent key.
func (r *agentRegistry) cacheToken(key, token string, scope []string, ttl int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.agents[key]; ok {
		e.lastToken = token
		e.lastScope = scope
		e.lastTokenTTL = ttl
		e.lastTokenAt = time.Now()
	}
}

// cachedToken returns a cached token for the given agent key if:
// 1. The agent exists in the registry
// 2. A token was previously cached
// 3. The token has not expired (within original TTL)
// 4. The requested scope is a subset of the cached scope
// Returns the token and remaining TTL, or empty string and false.
func (r *agentRegistry) cachedToken(key string, requestedScope []string) (string, int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.agents[key]
	if !ok || e.lastToken == "" {
		return "", 0, false
	}

	// Check TTL expiration.
	elapsed := time.Since(e.lastTokenAt)
	remaining := e.lastTokenTTL - int(elapsed.Seconds())
	if remaining <= 0 {
		return "", 0, false
	}

	// Check scope: requested must be subset of cached.
	if !scopeIsSubset(requestedScope, e.lastScope) {
		return "", 0, false
	}

	return e.lastToken, remaining, true
}

// getOrLock checks if an agent is already registered. If found, returns
// the entry and a nil unlock function. If NOT found, acquires a per-agent
// lock to serialize registration and returns nil entry + an unlock function.
// The caller MUST call unlock() after registration completes.
//
// This prevents duplicate challenge-response flows when concurrent
// POST /v1/token requests arrive for the same unregistered agent.
func (r *agentRegistry) getOrLock(key string) (*agentEntry, func()) {
	// Fast path: already registered.
	if entry, ok := r.lookup(key); ok {
		return entry, nil
	}

	// Get or create per-agent lock.
	r.locksMu.Lock()
	agentLock, exists := r.locks[key]
	if !exists {
		agentLock = &sync.Mutex{}
		r.locks[key] = agentLock
	}
	r.locksMu.Unlock()

	// Acquire per-agent lock.
	agentLock.Lock()

	// Double-check: another goroutine may have registered while we waited.
	if entry, ok := r.lookup(key); ok {
		agentLock.Unlock()
		return entry, nil
	}

	// Not registered -- caller must register, then call unlock.
	return nil, func() { agentLock.Unlock() }
}
