package main

import (
	"context"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
)

// startProbe runs a blocking loop that pings the broker health endpoint
// while the circuit breaker is open. When the broker responds successfully,
// it transitions the circuit to Probing state. Stops when ctx is cancelled.
func startProbe(ctx context.Context, cb *circuitBreaker, bc *brokerClient) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(cb.probeInterval):
		}

		// Only probe when circuit is open.
		if cb.State() != StateOpen {
			continue
		}

		err := bc.healthCheck()
		if err != nil {
			obs.Trace("SIDECAR", "PROBE", "broker still unreachable", err.Error())
			continue
		}

		obs.Ok("SIDECAR", "PROBE", "broker reachable, transitioning to probing")
		cb.ProbeSucceeded()
		SidecarCircuitState.Set(float64(StateProbing))
	}
}
