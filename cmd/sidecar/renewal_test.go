package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// mockRenewer is a configurable mock for renewFunc. It tracks call count
// and returns errors for the first failCount calls, then succeeds.
type mockRenewer struct {
	calls         atomic.Int64
	failCount     int64
	newTTL        int
	scopeCeiling  []string // if set, included in successful responses
}

func (m *mockRenewer) renew(token string) (*renewResp, error) {
	n := m.calls.Add(1)
	if n <= m.failCount {
		return nil, fmt.Errorf("mock renew error #%d", n)
	}
	ttl := m.newTTL
	if ttl == 0 {
		ttl = 2
	}
	return &renewResp{
		AccessToken:  fmt.Sprintf("renewed-token-%d", n),
		ExpiresIn:    ttl,
		ScopeCeiling: m.scopeCeiling,
	}, nil
}

// alwaysFailRenewer always returns an error.
func alwaysFailRenewer(_ string) (*renewResp, error) {
	return nil, fmt.Errorf("permanent failure")
}

func TestStartRenewal_RenewsBeforeExpiry(t *testing.T) {
	// Setup: 1s TTL, 0.5 buffer = should renew after ~500ms.
	st := &sidecarState{}
	st.setToken("initial-token", 1)

	mock := &mockRenewer{newTTL: 2}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		startRenewal(ctx, st, mock.renew, 0.5, nil)
		close(done)
	}()

	// Wait for at least one successful renewal.
	deadline := time.After(2 * time.Second)
	for {
		if mock.calls.Load() >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for renewal call")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Token should have been updated.
	tok := st.getToken()
	if tok == "initial-token" {
		t.Errorf("token was not renewed, still %q", tok)
	}
	if !st.isHealthy() {
		t.Error("state should be healthy after successful renewal")
	}

	cancel()
	<-done
}

func TestStartRenewal_BackoffOnFailure(t *testing.T) {
	// Setup: 1s TTL, 0.5 buffer. Fail first 2 calls, succeed on 3rd.
	st := &sidecarState{}
	st.setToken("initial-token", 1)

	mock := &mockRenewer{failCount: 2, newTTL: 2}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		startRenewal(ctx, st, mock.renew, 0.5, nil)
		close(done)
	}()

	// Wait for at least 3 calls (2 fails + 1 success).
	deadline := time.After(8 * time.Second)
	for {
		if mock.calls.Load() >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for 3 renewal calls, got %d", mock.calls.Load())
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// After recovery, token should be updated and state healthy.
	tok := st.getToken()
	if tok == "initial-token" || tok == "" {
		t.Errorf("token should be updated after recovery, got %q", tok)
	}
	if !st.isHealthy() {
		t.Error("state should be healthy after recovery")
	}

	cancel()
	<-done
}

func TestStartRenewal_SetsUnhealthyOnExpiry(t *testing.T) {
	// Setup: 1s TTL, 0.5 buffer. Renewals always fail.
	// After 1s the token deadline passes and state should become unhealthy.
	st := &sidecarState{}
	st.setToken("doomed-token", 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		startRenewal(ctx, st, alwaysFailRenewer, 0.5, nil)
		close(done)
	}()

	// Wait for state to become unhealthy.
	deadline := time.After(4 * time.Second)
	for {
		if !st.isHealthy() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for unhealthy state")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Verify the token is still the old one (no renewal succeeded).
	if got := st.getToken(); got != "doomed-token" {
		t.Errorf("token should remain %q, got %q", "doomed-token", got)
	}

	cancel()
	<-done
}

func TestStartRenewal_StopsOnContextCancel(t *testing.T) {
	// Setup: long TTL so renewal won't fire naturally.
	st := &sidecarState{}
	st.setToken("long-lived", 3600)

	mock := &mockRenewer{newTTL: 3600}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		startRenewal(ctx, st, mock.renew, 0.8, nil)
		close(done)
	}()

	// Cancel quickly and verify the goroutine exits.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Goroutine exited as expected.
	case <-time.After(2 * time.Second):
		t.Fatal("startRenewal did not exit after context cancellation")
	}

	// No renewals should have happened (TTL is 3600s, buffer 0.8 = ~2880s sleep).
	if mock.calls.Load() != 0 {
		t.Errorf("expected 0 renewal calls, got %d", mock.calls.Load())
	}
}

func TestStartRenewal_UpdatesCeilingFromResponse(t *testing.T) {
	// Setup: 1s TTL, 0.5 buffer. Mock returns a scope_ceiling in the response.
	st := &sidecarState{}
	st.setToken("initial-token", 1)

	ceiling := newCeilingCache([]string{"read:data:*"})

	mock := &mockRenewer{
		newTTL:       2,
		scopeCeiling: []string{"read:data:*", "write:data:*"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		startRenewal(ctx, st, mock.renew, 0.5, ceiling)
		close(done)
	}()

	// Wait for at least one successful renewal.
	deadline := time.After(2 * time.Second)
	for {
		if mock.calls.Load() >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for renewal call")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Ceiling should have been updated.
	got := ceiling.get()
	if len(got) != 2 {
		t.Fatalf("ceiling = %v, want [read:data:* write:data:*]", got)
	}
	if got[0] != "read:data:*" || got[1] != "write:data:*" {
		t.Errorf("ceiling = %v, want [read:data:* write:data:*]", got)
	}

	cancel()
	<-done
}

func TestStartRenewal_NilCeilingDoesNotPanic(t *testing.T) {
	// Verify that passing nil ceiling works when broker returns scope_ceiling.
	st := &sidecarState{}
	st.setToken("initial-token", 1)

	mock := &mockRenewer{
		newTTL:       2,
		scopeCeiling: []string{"read:data:*"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		startRenewal(ctx, st, mock.renew, 0.5, nil) // nil ceiling
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for {
		if mock.calls.Load() >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for renewal call")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Should not panic — just a sanity check.
	cancel()
	<-done
}
