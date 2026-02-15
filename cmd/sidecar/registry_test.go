package main

import (
	"crypto/ed25519"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TestAgentRegistry_StoreAndLookup — Store entry, lookup returns it
// ---------------------------------------------------------------------------

func TestAgentRegistry_StoreAndLookup(t *testing.T) {
	reg := newAgentRegistry()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	now := time.Now()
	entry := &agentEntry{
		spiffeID:     "spiffe://agentauth.local/agent/orch-1/task-1/inst-1",
		pubKey:       pub,
		privKey:      priv,
		registeredAt: now,
	}

	reg.store("reader:task-1", entry)

	got, ok := reg.lookup("reader:task-1")
	if !ok {
		t.Fatal("lookup returned false, want true")
	}
	if got.spiffeID != entry.spiffeID {
		t.Errorf("spiffeID = %q, want %q", got.spiffeID, entry.spiffeID)
	}
	if !got.pubKey.Equal(pub) {
		t.Error("pubKey does not match")
	}
	if !got.privKey.Equal(priv) {
		t.Error("privKey does not match")
	}
	if !got.registeredAt.Equal(now) {
		t.Errorf("registeredAt = %v, want %v", got.registeredAt, now)
	}
}

// ---------------------------------------------------------------------------
// TestAgentRegistry_LookupMissing — Lookup non-existent key returns false
// ---------------------------------------------------------------------------

func TestAgentRegistry_LookupMissing(t *testing.T) {
	reg := newAgentRegistry()

	got, ok := reg.lookup("nonexistent:task-99")
	if ok {
		t.Fatal("lookup returned true for missing key, want false")
	}
	if got != nil {
		t.Errorf("entry = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// TestAgentRegistry_BYOK_NilPrivateKey — BYOK entry has nil privKey
// ---------------------------------------------------------------------------

func TestAgentRegistry_BYOK_NilPrivateKey(t *testing.T) {
	reg := newAgentRegistry()

	// BYOK agents supply their own key; the sidecar only stores the public key.
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	entry := &agentEntry{
		spiffeID:     "spiffe://agentauth.local/agent/orch-1/task-2/inst-2",
		pubKey:       pub,
		privKey:      nil, // BYOK: no private key stored
		registeredAt: time.Now(),
	}

	reg.store("byok-agent:task-2", entry)

	got, ok := reg.lookup("byok-agent:task-2")
	if !ok {
		t.Fatal("lookup returned false, want true")
	}
	if got.privKey != nil {
		t.Errorf("privKey = %v, want nil for BYOK agent", got.privKey)
	}
	if !got.pubKey.Equal(pub) {
		t.Error("pubKey does not match")
	}
}

// ---------------------------------------------------------------------------
// TestAgentRegistry_ConcurrentAccess — 50 concurrent writers + readers, race
// detector validates safety
// ---------------------------------------------------------------------------

func TestAgentRegistry_ConcurrentAccess(t *testing.T) {
	reg := newAgentRegistry()

	const numWorkers = 50
	var wg sync.WaitGroup
	wg.Add(numWorkers * 2) // 50 writers + 50 readers

	// 50 concurrent writers — each stores a unique entry.
	for i := 0; i < numWorkers; i++ {
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("agent-%d:task-%d", n, n)
			pub, priv, _ := ed25519.GenerateKey(nil)
			reg.store(key, &agentEntry{
				spiffeID:     fmt.Sprintf("spiffe://test/agent/o/%d/%d", n, n),
				pubKey:       pub,
				privKey:      priv,
				registeredAt: time.Now(),
			})
		}(i)
	}

	// 50 concurrent readers — each reads a key that may or may not exist yet.
	for i := 0; i < numWorkers; i++ {
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("agent-%d:task-%d", n, n)
			// We don't assert the result because the write may not have
			// happened yet; we just exercise the read path under contention.
			reg.lookup(key)
		}(i)
	}

	wg.Wait()

	// After all writes complete, every key should be present.
	for i := 0; i < numWorkers; i++ {
		key := fmt.Sprintf("agent-%d:task-%d", i, i)
		if _, ok := reg.lookup(key); !ok {
			t.Errorf("key %q missing after concurrent writes", key)
		}
	}
}

// ---------------------------------------------------------------------------
// TestAgentRegistry_GetOrLock_SerializesRegistration — First getOrLock returns
// nil + unlock; store + unlock; second getOrLock returns entry + nil unlock
// ---------------------------------------------------------------------------

func TestAgentRegistry_GetOrLock_SerializesRegistration(t *testing.T) {
	reg := newAgentRegistry()

	key := "new-agent:task-42"

	// First call: agent not registered yet — should return nil entry + unlock fn.
	entry, unlock := reg.getOrLock(key)
	if entry != nil {
		t.Fatal("first getOrLock returned non-nil entry, want nil")
	}
	if unlock == nil {
		t.Fatal("first getOrLock returned nil unlock, want non-nil")
	}

	// Simulate registration: store the entry, then unlock.
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	reg.store(key, &agentEntry{
		spiffeID:     "spiffe://agentauth.local/agent/o/task-42/inst-1",
		pubKey:       pub,
		privKey:      priv,
		registeredAt: time.Now(),
	})
	unlock()

	// Second call: agent is now registered — should return entry + nil unlock.
	entry2, unlock2 := reg.getOrLock(key)
	if entry2 == nil {
		t.Fatal("second getOrLock returned nil entry, want non-nil")
	}
	if unlock2 != nil {
		t.Fatal("second getOrLock returned non-nil unlock, want nil")
	}
	if entry2.spiffeID != "spiffe://agentauth.local/agent/o/task-42/inst-1" {
		t.Errorf("spiffeID = %q, want spiffe://agentauth.local/agent/o/task-42/inst-1", entry2.spiffeID)
	}

	// Verify serialization: launch two goroutines that both try getOrLock
	// on a new key. Only one should get the lock; the other should see the
	// entry stored by the first.
	key2 := "concurrent-agent:task-99"
	var wg sync.WaitGroup
	wg.Add(2)

	registrations := make(chan string, 2) // track who registered vs who found

	for i := 0; i < 2; i++ {
		go func(n int) {
			defer wg.Done()
			e, uf := reg.getOrLock(key2)
			if e != nil {
				// Found existing entry — another goroutine registered first.
				registrations <- fmt.Sprintf("goroutine-%d:found", n)
				return
			}
			// We got the lock — register and unlock.
			p, k, _ := ed25519.GenerateKey(nil)
			reg.store(key2, &agentEntry{
				spiffeID:     "spiffe://test/agent/o/task-99/inst-1",
				pubKey:       p,
				privKey:      k,
				registeredAt: time.Now(),
			})
			uf()
			registrations <- fmt.Sprintf("goroutine-%d:registered", n)
		}(i)
	}

	wg.Wait()
	close(registrations)

	var registered, found int
	for r := range registrations {
		if len(r) > 0 {
			if r[len(r)-len("registered"):] == "registered" {
				registered++
			} else {
				found++
			}
		}
	}

	// Exactly one goroutine should register, the other should find it.
	if registered != 1 {
		t.Errorf("expected exactly 1 registration, got %d", registered)
	}
	if found != 1 {
		t.Errorf("expected exactly 1 found, got %d", found)
	}
}
