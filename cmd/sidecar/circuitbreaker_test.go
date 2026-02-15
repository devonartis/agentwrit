package main

import (
	"testing"
	"time"
)

func TestCircuitBreaker_StartsInClosedState(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	if cb.State() != StateClosed {
		t.Errorf("initial state = %v, want Closed", cb.State())
	}
}

func TestCircuitBreaker_AllowsRequestWhenClosed(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	if !cb.Allow() {
		t.Error("Allow() = false when closed, want true")
	}
}

func TestCircuitBreaker_TripsOpenOnHighFailureRate(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateOpen {
		t.Errorf("state = %v after 5 failures, want Open", cb.State())
	}
}

func TestCircuitBreaker_DoesNotTripBelowMinRequests(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateClosed {
		t.Errorf("state = %v after 3 failures (below min), want Closed", cb.State())
	}
}

func TestCircuitBreaker_DoesNotTripBelowThreshold(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("state = %v at 33%% failure rate, want Closed", cb.State())
	}
}

func TestCircuitBreaker_BlocksRequestsWhenOpen(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	if cb.Allow() {
		t.Error("Allow() = true when open, want false")
	}
}

func TestCircuitBreaker_WindowExpiration(t *testing.T) {
	cb := newCircuitBreaker(50*time.Millisecond, 0.5, 5*time.Second, 2)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("state = %v, want Open", cb.State())
	}
	time.Sleep(100 * time.Millisecond)
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("state = %v after window expiration + success, want Closed", cb.State())
	}
}

func TestCircuitBreaker_TransitionToProbing(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	cb.ProbeSucceeded()
	if cb.State() != StateProbing {
		t.Errorf("state = %v after ProbeSucceeded, want Probing", cb.State())
	}
}

func TestCircuitBreaker_ProbingAllowsOneRequest(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	cb.ProbeSucceeded()
	if !cb.Allow() {
		t.Error("Allow() = false in Probing state, want true")
	}
}

func TestCircuitBreaker_ProbingSuccessCloses(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	cb.ProbeSucceeded()
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("state = %v after probing success, want Closed", cb.State())
	}
}

func TestCircuitBreaker_ProbingFailureReopens(t *testing.T) {
	cb := newCircuitBreaker(30*time.Second, 0.5, 5*time.Second, 5)
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	cb.ProbeSucceeded()
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Errorf("state = %v after probing failure, want Open", cb.State())
	}
}
