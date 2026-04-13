// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0

package mutauth

import (
	"errors"
	"testing"
)

func TestDiscoveryBindAndResolve(t *testing.T) {
	dr := NewDiscoveryRegistry()
	agentID := "spiffe://test.local/agent/orch-1/task-1/inst-a"
	endpoint := "https://agent-a.internal:8443"

	if err := dr.Bind(agentID, endpoint); err != nil {
		t.Fatalf("bind: %v", err)
	}
	got, err := dr.Resolve(agentID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != endpoint {
		t.Fatalf("resolve: got %q, want %q", got, endpoint)
	}
}

func TestDiscoveryResolveUnknown(t *testing.T) {
	dr := NewDiscoveryRegistry()
	_, err := dr.Resolve("spiffe://test.local/agent/orch-1/task-1/unknown")
	if !errors.Is(err, ErrAgentNotBound) {
		t.Fatalf("expected ErrAgentNotBound, got %v", err)
	}
}

func TestDiscoveryVerifyBindingMatch(t *testing.T) {
	dr := NewDiscoveryRegistry()
	agentID := "spiffe://test.local/agent/orch-1/task-1/inst-a"
	_ = dr.Bind(agentID, "https://agent-a.internal:8443")

	ok, err := dr.VerifyBinding(agentID, agentID)
	if err != nil {
		t.Fatalf("verify binding: %v", err)
	}
	if !ok {
		t.Fatal("expected binding to match")
	}
}

func TestDiscoveryVerifyBindingMismatch(t *testing.T) {
	dr := NewDiscoveryRegistry()
	agentID := "spiffe://test.local/agent/orch-1/task-1/inst-a"
	_ = dr.Bind(agentID, "https://agent-a.internal:8443")

	impostor := "spiffe://test.local/agent/orch-1/task-1/impostor"
	ok, err := dr.VerifyBinding(agentID, impostor)
	if !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("expected ErrBindingMismatch, got ok=%v err=%v", ok, err)
	}
}

func TestDiscoveryVerifyBindingNotBound(t *testing.T) {
	dr := NewDiscoveryRegistry()
	_, err := dr.VerifyBinding("spiffe://test.local/agent/orch-1/task-1/unknown", "anything")
	if !errors.Is(err, ErrAgentNotBound) {
		t.Fatalf("expected ErrAgentNotBound, got %v", err)
	}
}

func TestDiscoveryUnbind(t *testing.T) {
	dr := NewDiscoveryRegistry()
	agentID := "spiffe://test.local/agent/orch-1/task-1/inst-a"
	_ = dr.Bind(agentID, "https://agent-a.internal:8443")
	dr.Unbind(agentID)

	_, err := dr.Resolve(agentID)
	if !errors.Is(err, ErrAgentNotBound) {
		t.Fatalf("expected ErrAgentNotBound after unbind, got %v", err)
	}
}
