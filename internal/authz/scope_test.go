// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

package authz

import (
	"testing"
)

func TestParseScope(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		action     string
		resource   string
		identifier string
		wantErr    bool
	}{
		{"valid specific", "read:Customers:12345", "read", "Customers", "12345", false},
		{"valid wildcard", "read:Customers:*", "read", "Customers", "*", false},
		{"valid admin", "admin:launch-tokens:*", "admin", "launch-tokens", "*", false},
		{"missing parts", "read:Customers", "", "", "", true},
		{"single part", "read", "", "", "", true},
		{"empty string", "", "", "", "", true},
		{"empty action", ":Customers:*", "", "", "", true},
		{"empty resource", "read::*", "", "", "", true},
		{"empty identifier", "read:Customers:", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, resource, identifier, err := ParseScope(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if action != tt.action || resource != tt.resource || identifier != tt.identifier {
				t.Errorf("ParseScope(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tt.input, action, resource, identifier, tt.action, tt.resource, tt.identifier)
			}
		})
	}
}

func TestScopeIsSubset(t *testing.T) {
	tests := []struct {
		name      string
		requested []string
		allowed   []string
		want      bool
	}{
		{
			name:      "exact match",
			requested: []string{"read:Customers:*"},
			allowed:   []string{"read:Customers:*"},
			want:      true,
		},
		{
			name:      "wildcard covers specific",
			requested: []string{"read:Customers:12345"},
			allowed:   []string{"read:Customers:*"},
			want:      true,
		},
		{
			name:      "specific does not cover wildcard",
			requested: []string{"read:Customers:*"},
			allowed:   []string{"read:Customers:12345"},
			want:      false,
		},
		{
			name:      "different action",
			requested: []string{"write:Customers:*"},
			allowed:   []string{"read:Customers:*"},
			want:      false,
		},
		{
			name:      "different resource",
			requested: []string{"read:Orders:*"},
			allowed:   []string{"read:Customers:*"},
			want:      false,
		},
		{
			name:      "multiple requested all covered",
			requested: []string{"read:Customers:12345", "read:Customers:67890"},
			allowed:   []string{"read:Customers:*"},
			want:      true,
		},
		{
			name:      "multiple requested one not covered",
			requested: []string{"read:Customers:12345", "write:Customers:*"},
			allowed:   []string{"read:Customers:*"},
			want:      false,
		},
		{
			name:      "multiple allowed scopes",
			requested: []string{"read:Customers:12345", "write:Orders:*"},
			allowed:   []string{"read:Customers:*", "write:Orders:*"},
			want:      true,
		},
		{
			name:      "empty requested is subset",
			requested: []string{},
			allowed:   []string{"read:Customers:*"},
			want:      true,
		},
		{
			name:      "admin scope exact",
			requested: []string{"admin:launch-tokens:*"},
			allowed:   []string{"admin:launch-tokens:*", "admin:revoke:*", "admin:audit:*"},
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScopeIsSubset(tt.requested, tt.allowed)
			if got != tt.want {
				t.Errorf("ScopeIsSubset(%v, %v) = %v, want %v",
					tt.requested, tt.allowed, got, tt.want)
			}
		})
	}
}
