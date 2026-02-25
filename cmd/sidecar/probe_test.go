package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbe_TransitionsToProbing(t *testing.T) {
	// Mock broker health endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL, "", "", "")
	cb := newCircuitBreaker(30*time.Second, 0.5, 50*time.Millisecond, 5)

	// Trip the circuit.
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startProbe(ctx, cb, bc)

	// Wait for probe to run and detect healthy broker.
	time.Sleep(200 * time.Millisecond)

	if cb.State() != StateProbing {
		t.Errorf("state = %v, want Probing after healthy probe", cb.State())
	}
}

func TestProbe_StaysOpenWhenBrokerDown(t *testing.T) {
	// Mock broker that always fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	bc := newBrokerClient(srv.URL, "", "", "")
	cb := newCircuitBreaker(30*time.Second, 0.5, 50*time.Millisecond, 5)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startProbe(ctx, cb, bc)

	time.Sleep(200 * time.Millisecond)

	if cb.State() != StateOpen {
		t.Errorf("state = %v, want Open when broker is down", cb.State())
	}
}

func TestProbe_StopsOnCancel(t *testing.T) {
	bc := newBrokerClient("http://127.0.0.1:1", "", "", "")
	cb := newCircuitBreaker(30*time.Second, 0.5, 50*time.Millisecond, 5)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Should return without blocking.
	done := make(chan struct{})
	go func() {
		startProbe(ctx, cb, bc)
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(1 * time.Second):
		t.Error("startProbe did not exit after context cancel")
	}
}
