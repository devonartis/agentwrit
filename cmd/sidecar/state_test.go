package main

import (
	"sync"
	"testing"
	"time"
)

func TestSidecarState_GetToken_ReturnsCurrentToken(t *testing.T) {
	st := &sidecarState{}
	st.setToken("tok-abc", 300)

	if got := st.getToken(); got != "tok-abc" {
		t.Errorf("getToken() = %q, want tok-abc", got)
	}
	if got := st.getExpiresIn(); got != 300 {
		t.Errorf("getExpiresIn() = %d, want 300", got)
	}
}

func TestSidecarState_SetToken_UpdatesAtomically(t *testing.T) {
	st := &sidecarState{}

	// Initially empty.
	if got := st.getToken(); got != "" {
		t.Errorf("initial getToken() = %q, want empty", got)
	}

	// Set token and verify both fields update together.
	st.setToken("first-token", 600)
	if got := st.getToken(); got != "first-token" {
		t.Errorf("getToken() = %q, want first-token", got)
	}
	if got := st.getExpiresIn(); got != 600 {
		t.Errorf("getExpiresIn() = %d, want 600", got)
	}

	// Overwrite with new values.
	st.setToken("second-token", 900)
	if got := st.getToken(); got != "second-token" {
		t.Errorf("getToken() = %q, want second-token", got)
	}
	if got := st.getExpiresIn(); got != 900 {
		t.Errorf("getExpiresIn() = %d, want 900", got)
	}

	// setToken should mark healthy = true.
	if !st.isHealthy() {
		t.Error("isHealthy() = false after setToken, want true")
	}
}

func TestSidecarState_ConcurrentAccess(t *testing.T) {
	st := &sidecarState{}
	st.setToken("initial", 100)

	var wg sync.WaitGroup

	// 10 writers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			st.setToken("writer-token", n*100)
			st.setHealthy(n%2 == 0)
		}(i)
	}

	// 100 readers.
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = st.getToken()
			_ = st.getExpiresIn()
			_ = st.isHealthy()
		}()
	}

	wg.Wait()

	// After all goroutines complete, the state should hold a valid token
	// (any writer-token is acceptable; the race detector validates safety).
	if got := st.getToken(); got == "" {
		t.Error("getToken() is empty after concurrent writes, expected non-empty")
	}
}

func TestSidecarState_Healthy_DefaultTrue(t *testing.T) {
	st := &sidecarState{}

	// Before setToken, healthy is zero-value false.
	if st.isHealthy() {
		t.Error("isHealthy() = true on zero-value state, want false")
	}

	// setToken should set healthy to true.
	st.setToken("tok", 300)
	if !st.isHealthy() {
		t.Error("isHealthy() = false after setToken, want true")
	}

	// setHealthy(false) should override.
	st.setHealthy(false)
	if st.isHealthy() {
		t.Error("isHealthy() = true after setHealthy(false), want false")
	}

	// setHealthy(true) should restore.
	st.setHealthy(true)
	if !st.isHealthy() {
		t.Error("isHealthy() = false after setHealthy(true), want true")
	}
}

func TestSidecarState_LastRenewal(t *testing.T) {
	st := &sidecarState{startTime: time.Now()}
	st.setToken("tok1", 300)

	lr := st.getLastRenewal()
	if lr.IsZero() {
		t.Error("lastRenewal should be set after setToken")
	}
	if time.Since(lr) > 1*time.Second {
		t.Error("lastRenewal should be recent")
	}
}

func TestSidecarState_StartTime(t *testing.T) {
	now := time.Now()
	st := &sidecarState{startTime: now}

	if got := st.getStartTime(); !got.Equal(now) {
		t.Errorf("getStartTime() = %v, want %v", got, now)
	}
}
