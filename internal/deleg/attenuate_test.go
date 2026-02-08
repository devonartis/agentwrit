package deleg

import (
	"errors"
	"testing"
)

func TestAttenuate(t *testing.T) {
	tests := []struct {
		name      string
		parent    []string
		requested []string
		wantErr   error
		wantLen   int
	}{
		// --- Pass cases ---
		{
			name:      "exact match single scope",
			parent:    []string{"read:Customers:12345"},
			requested: []string{"read:Customers:12345"},
			wantLen:   1,
		},
		{
			name:      "exact match multiple scopes",
			parent:    []string{"read:Customers:12345", "write:Orders:789"},
			requested: []string{"read:Customers:12345", "write:Orders:789"},
			wantLen:   2,
		},
		{
			name:      "wildcard narrowing to specific",
			parent:    []string{"read:Customers:*"},
			requested: []string{"read:Customers:12345"},
			wantLen:   1,
		},
		{
			name:      "wildcard parent with multiple specific requests",
			parent:    []string{"read:Customers:*"},
			requested: []string{"read:Customers:12345", "read:Customers:67890"},
			wantLen:   2,
		},
		{
			name:      "subset of parent scopes",
			parent:    []string{"read:Customers:*", "write:Orders:*", "read:Tickets:*"},
			requested: []string{"read:Customers:12345"},
			wantLen:   1,
		},
		{
			name:      "wildcard to wildcard same scope",
			parent:    []string{"read:Customers:*"},
			requested: []string{"read:Customers:*"},
			wantLen:   1,
		},
		{
			name:      "mixed wildcard and specific parent",
			parent:    []string{"read:Customers:*", "write:Orders:789"},
			requested: []string{"read:Customers:42", "write:Orders:789"},
			wantLen:   2,
		},
		{
			name:      "single scope from many parents",
			parent:    []string{"read:A:*", "read:B:*", "read:C:*"},
			requested: []string{"read:B:1"},
			wantLen:   1,
		},
		// --- Fail cases ---
		{
			name:      "action mismatch write vs read",
			parent:    []string{"read:Customers:*"},
			requested: []string{"write:Customers:12345"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "resource mismatch",
			parent:    []string{"read:Customers:*"},
			requested: []string{"read:Orders:12345"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "expansion attempt specific to wildcard",
			parent:    []string{"read:Customers:12345"},
			requested: []string{"read:Customers:*"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "expansion attempt different identifier",
			parent:    []string{"read:Customers:12345"},
			requested: []string{"read:Customers:67890"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "empty requested scope",
			parent:    []string{"read:Customers:*"},
			requested: []string{},
			wantErr:   ErrRequestedScopeEmpty,
		},
		{
			name:      "nil requested scope",
			parent:    []string{"read:Customers:*"},
			requested: nil,
			wantErr:   ErrRequestedScopeEmpty,
		},
		{
			name:      "empty parent scope",
			parent:    []string{},
			requested: []string{"read:Customers:12345"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "nil parent scope",
			parent:    nil,
			requested: []string{"read:Customers:12345"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "partial match one passes one fails",
			parent:    []string{"read:Customers:*"},
			requested: []string{"read:Customers:12345", "write:Orders:789"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "completely unrelated scopes",
			parent:    []string{"read:Customers:*"},
			requested: []string{"delete:Servers:prod-01"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "action case sensitive",
			parent:    []string{"read:Customers:*"},
			requested: []string{"Read:Customers:12345"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "resource case sensitive",
			parent:    []string{"read:Customers:*"},
			requested: []string{"read:customers:12345"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "all requested exceed parent",
			parent:    []string{"read:Customers:12345"},
			requested: []string{"write:Orders:1", "delete:Tickets:2"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "wildcard resource not supported",
			parent:    []string{"read:Customers:*"},
			requested: []string{"read:*:12345"},
			wantErr:   ErrScopeEscalation,
		},
		{
			name:      "three-level deep narrowing pass",
			parent:    []string{"read:Customers:*", "write:Tickets:*"},
			requested: []string{"write:Tickets:42"},
			wantLen:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Attenuate(tt.parent, tt.requested)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("expected %d scopes, got %d", tt.wantLen, len(got))
			}
			// Verify returned scopes match requested.
			for i, s := range got {
				if s != tt.requested[i] {
					t.Errorf("scope[%d] = %s, want %s", i, s, tt.requested[i])
				}
			}
		})
	}
}

// TestAttenuateReturnsCopy verifies the returned slice is a defensive copy.
func TestAttenuateReturnsCopy(t *testing.T) {
	parent := []string{"read:Customers:*"}
	requested := []string{"read:Customers:12345"}
	got, err := Attenuate(parent, requested)
	if err != nil {
		t.Fatal(err)
	}
	got[0] = "tampered"
	if requested[0] == "tampered" {
		t.Fatal("Attenuate must return a copy, not a reference to the input")
	}
}
