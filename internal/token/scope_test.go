package token

import "testing"

func TestParseScope(t *testing.T) {
	tests := []struct {
		name string
		in   string
		ok   bool
	}{
		{"valid", "read:Customers:12345", true},
		{"valid wildcard", "read:Customers:*", true},
		{"empty", "", false},
		{"missing part", "read:Customers", false},
		{"extra part", "read:Customers:123:extra", false},
		{"empty action", ":Customers:123", false},
		{"empty resource", "read::123", false},
		{"empty id", "read:Customers:", false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseScope(tc.in)
			if tc.ok && err != nil {
				t.Fatalf("expected valid scope, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected parse failure")
			}
		})
	}
}

func TestScopeMatch(t *testing.T) {
	tests := []struct {
		name      string
		required  string
		available string
		want      bool
	}{
		{"exact match", "read:Customers:12345", "read:Customers:12345", true},
		{"wildcard parent", "read:Customers:12345", "read:Customers:*", true},
		{"action mismatch", "write:Customers:12345", "read:Customers:*", false},
		{"resource mismatch", "read:Orders:12345", "read:Customers:*", false},
		{"id mismatch", "read:Customers:12345", "read:Customers:67890", false},
		{"invalid required", "bad-scope", "read:Customers:*", false},
		{"invalid available", "read:Customers:1", "bad-scope", false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ScopeMatch(tc.required, tc.available)
			if got != tc.want {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestScopeIsSubset(t *testing.T) {
	tests := []struct {
		name   string
		child  []string
		parent []string
		want   bool
	}{
		{
			name:   "single exact in parent",
			child:  []string{"read:Customers:12345"},
			parent: []string{"read:Customers:12345", "write:Tickets:12345"},
			want:   true,
		},
		{
			name:   "single wildcard parent",
			child:  []string{"read:Customers:12345"},
			parent: []string{"read:Customers:*"},
			want:   true,
		},
		{
			name:   "multiple child covered",
			child:  []string{"read:Customers:1", "write:Tickets:1"},
			parent: []string{"read:Customers:*", "write:Tickets:*"},
			want:   true,
		},
		{
			name:   "child not covered",
			child:  []string{"read:Orders:1"},
			parent: []string{"read:Customers:*"},
			want:   false,
		},
		{
			name:   "empty child is subset",
			child:  []string{},
			parent: []string{"read:Customers:*"},
			want:   true,
		},
		{
			name:   "non-empty child empty parent",
			child:  []string{"read:Customers:1"},
			parent: []string{},
			want:   false,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ScopeIsSubset(tc.child, tc.parent)
			if got != tc.want {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

