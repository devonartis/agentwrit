// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

package store

import (
	"os"
	"testing"
)

func TestSaveRevocation(t *testing.T) {
	s := NewSqlStore()
	tmp := t.TempDir() + "/test.db"
	defer os.Remove(tmp)
	if err := s.InitDB(tmp); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.SaveRevocation("token", "jti-123"); err != nil {
		t.Fatalf("SaveRevocation: %v", err)
	}

	// Idempotent: second save should not error
	if err := s.SaveRevocation("token", "jti-123"); err != nil {
		t.Fatalf("SaveRevocation idempotent: %v", err)
	}
}

func TestLoadAllRevocations(t *testing.T) {
	s := NewSqlStore()
	tmp := t.TempDir() + "/test.db"
	defer os.Remove(tmp)
	if err := s.InitDB(tmp); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Save entries at multiple levels
	for _, tc := range []struct{ level, target string }{
		{"token", "jti-1"},
		{"agent", "spiffe://test/agent/a"},
		{"task", "task-42"},
		{"chain", "spiffe://test/agent/root"},
	} {
		if err := s.SaveRevocation(tc.level, tc.target); err != nil {
			t.Fatalf("SaveRevocation(%s, %s): %v", tc.level, tc.target, err)
		}
	}

	revs, err := s.LoadAllRevocations()
	if err != nil {
		t.Fatalf("LoadAllRevocations: %v", err)
	}
	if len(revs) != 4 {
		t.Fatalf("expected 4 revocations, got %d", len(revs))
	}
}
