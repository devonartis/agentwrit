package main

import "testing"

func TestCeilingCache_GetSet(t *testing.T) {
	cc := newCeilingCache([]string{"read:tickets:*"})
	got := cc.get()
	if len(got) != 1 || got[0] != "read:tickets:*" {
		t.Errorf("expected [read:tickets:*], got %v", got)
	}
	cc.set([]string{"read:tickets:*", "write:tickets:metadata"})
	got = cc.get()
	if len(got) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(got))
	}
}

func TestCeilingCache_ConcurrentAccess(t *testing.T) {
	cc := newCeilingCache([]string{"read:data:*"})
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			cc.set([]string{"read:data:*", "write:data:*"})
		}
		close(done)
	}()
	for i := 0; i < 1000; i++ {
		got := cc.get()
		if len(got) == 0 {
			t.Error("ceiling should never be empty")
		}
	}
	<-done
}
