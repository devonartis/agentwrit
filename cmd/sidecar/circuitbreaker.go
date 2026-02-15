package main

import (
	"sync"
	"time"
)

type CircuitState int

const (
	StateClosed  CircuitState = 0
	StateOpen    CircuitState = 1
	StateProbing CircuitState = 2
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateProbing:
		return "probing"
	default:
		return "unknown"
	}
}

type event struct {
	at     time.Time
	failed bool
}

type circuitBreaker struct {
	mu sync.Mutex

	state         CircuitState
	window        time.Duration
	threshold     float64
	probeInterval time.Duration
	minRequests   int
	events        []event

	nowFunc func() time.Time
}

func newCircuitBreaker(window time.Duration, threshold float64, probeInterval time.Duration, minRequests int) *circuitBreaker {
	return &circuitBreaker{
		state:         StateClosed,
		window:        window,
		threshold:     threshold,
		probeInterval: probeInterval,
		minRequests:   minRequests,
		nowFunc:       time.Now,
	}
}

func (cb *circuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.purgeExpired()
	cb.evaluateState()
	return cb.state
}

func (cb *circuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.purgeExpired()
	cb.evaluateState()
	return cb.state != StateOpen
}

func (cb *circuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.events = append(cb.events, event{at: cb.nowFunc(), failed: false})
	if cb.state == StateProbing {
		cb.state = StateClosed
		cb.events = nil
		return
	}
	cb.purgeExpired()
	cb.evaluateState()
}

func (cb *circuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.events = append(cb.events, event{at: cb.nowFunc(), failed: true})
	if cb.state == StateProbing {
		cb.state = StateOpen
		return
	}
	cb.purgeExpired()
	cb.evaluateState()
}

func (cb *circuitBreaker) ProbeSucceeded() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == StateOpen {
		cb.state = StateProbing
	}
}

func (cb *circuitBreaker) purgeExpired() {
	cutoff := cb.nowFunc().Add(-cb.window)
	i := 0
	for i < len(cb.events) && cb.events[i].at.Before(cutoff) {
		i++
	}
	if i > 0 {
		cb.events = cb.events[i:]
	}
}

func (cb *circuitBreaker) evaluateState() {
	if cb.state == StateProbing {
		return
	}
	total := len(cb.events)
	if total < cb.minRequests {
		if cb.state == StateOpen {
			cb.state = StateClosed
		}
		return
	}
	failures := 0
	for _, e := range cb.events {
		if e.failed {
			failures++
		}
	}
	rate := float64(failures) / float64(total)
	if rate > cb.threshold {
		cb.state = StateOpen
	} else if cb.state == StateOpen {
		cb.state = StateClosed
	}
}
