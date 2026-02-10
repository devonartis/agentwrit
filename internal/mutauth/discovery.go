package mutauth

import (
	"errors"
	"sync"

	"github.com/divineartis/agentauth/internal/obs"
)

// Discovery errors.
var (
	// ErrAgentNotBound indicates no endpoint binding exists for the given agent ID.
	ErrAgentNotBound = errors.New("discovery: agent not bound")
	// ErrBindingMismatch indicates the presented ID does not match the discovery-bound identity.
	ErrBindingMismatch = errors.New("discovery: binding mismatch")
)

// DiscoveryRegistry maps agent SPIFFE IDs to network endpoints and verifies
// that agents communicating via the handshake protocol present identities
// consistent with the directory.
type DiscoveryRegistry struct {
	mu       sync.RWMutex
	bindings map[string]string // agentID → endpoint
}

// NewDiscoveryRegistry creates an empty discovery registry.
func NewDiscoveryRegistry() *DiscoveryRegistry {
	return &DiscoveryRegistry{
		bindings: make(map[string]string),
	}
}

// Bind associates an agent's SPIFFE ID with a reachable endpoint.
func (d *DiscoveryRegistry) Bind(agentID, endpoint string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bindings[agentID] = endpoint
	obs.Ok("MUTAUTH", "Discovery.Bind", "agent bound", "agent_id="+agentID, "endpoint="+endpoint)
	return nil
}

// Resolve looks up the endpoint for a given agent ID.
func (d *DiscoveryRegistry) Resolve(agentID string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ep, ok := d.bindings[agentID]
	if !ok {
		return "", ErrAgentNotBound
	}
	return ep, nil
}

// Unbind removes an agent's endpoint binding.
func (d *DiscoveryRegistry) Unbind(agentID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.bindings, agentID)
	obs.Ok("MUTAUTH", "Discovery.Unbind", "agent unbound", "agent_id="+agentID)
}

// VerifyBinding checks that a presented agent ID matches the identity bound
// in the discovery registry. This is an identity-consistency check only; it
// does not by itself prove transport endpoint authenticity.
func (d *DiscoveryRegistry) VerifyBinding(agentID, presentedID string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.bindings[agentID]
	if !ok {
		return false, ErrAgentNotBound
	}
	if agentID != presentedID {
		obs.Warn("MUTAUTH", "Discovery.VerifyBinding", "binding mismatch detected",
			"expected="+agentID, "presented="+presentedID)
		return false, ErrBindingMismatch
	}
	return true, nil
}
