package identity

import "testing"

func TestNewSpiffeId(t *testing.T) {
	got := NewSpiffeId("agentauth.local", "orch-123", "task-789", "inst-abc")
	want := "spiffe://agentauth.local/agent/orch-123/task-789/inst-abc"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestValidateSpiffeId(t *testing.T) {
	tests := []struct {
		name string
		id   string
		ok   bool
	}{
		{"valid standard", "spiffe://agentauth.local/agent/orch-123/task-789/inst-abc", true},
		{"valid with underscores", "spiffe://agentauth.local/agent/orch_123/task_789/inst_abc", true},
		{"valid with dots", "spiffe://prod.agentauth.local/agent/orch.a/task.b/inst.c", true},
		{"valid with mixed symbols", "spiffe://a-b.local/agent/orch_1-2/task.3/inst-4_5", true},
		{"empty", "", false},
		{"whitespace only", "   ", false},
		{"missing scheme", "agentauth.local/agent/orch/task/inst", false},
		{"wrong scheme", "https://agentauth.local/agent/orch/task/inst", false},
		{"missing trust domain", "spiffe:///agent/orch/task/inst", false},
		{"missing agent segment", "spiffe://agentauth.local/service/orch/task/inst", false},
		{"missing orch id", "spiffe://agentauth.local/agent//task/inst", false},
		{"missing task id", "spiffe://agentauth.local/agent/orch//inst", false},
		{"missing instance id", "spiffe://agentauth.local/agent/orch/task/", false},
		{"too many segments", "spiffe://agentauth.local/agent/orch/task/inst/extra", false},
		{"too few segments", "spiffe://agentauth.local/agent/orch/task", false},
		{"space in segment", "spiffe://agentauth.local/agent/orch id/task/inst", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSpiffeId(tc.id)
			if tc.ok && err != nil {
				t.Fatalf("expected valid id, got error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected invalid id, got nil error")
			}
		})
	}
}

func TestParseSpiffeId(t *testing.T) {
	tests := []struct {
		name string
		id   string
		ok   bool
	}{
		{"valid parse", "spiffe://agentauth.local/agent/orch-123/task-789/inst-abc", true},
		{"invalid parse", "spiffe://agentauth.local/agent/orch-123/task-789", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSpiffeId(tc.id)
			if tc.ok {
				if err != nil {
					t.Fatalf("expected nil error, got: %v", err)
				}
				if got.TrustDomain != "agentauth.local" {
					t.Fatalf("unexpected trust domain: %s", got.TrustDomain)
				}
				if got.OrchId != "orch-123" {
					t.Fatalf("unexpected orch id: %s", got.OrchId)
				}
				if got.TaskId != "task-789" {
					t.Fatalf("unexpected task id: %s", got.TaskId)
				}
				if got.InstanceId != "inst-abc" {
					t.Fatalf("unexpected instance id: %s", got.InstanceId)
				}
				if got.Raw != tc.id {
					t.Fatalf("unexpected raw value: %s", got.Raw)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

