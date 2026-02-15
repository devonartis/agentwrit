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
