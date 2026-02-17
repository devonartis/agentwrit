package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/divineartis/agentauth/internal/obs"
)

func main() {
	cfg := loadConfig()
	obs.Configure(cfg.LogLevel)

	// Validate required config.
	if cfg.AdminSecret == "" {
		obs.Fail("SIDECAR", "MAIN", "AA_ADMIN_SECRET must be set")
		os.Exit(1)
	}
	if len(cfg.ScopeCeiling) == 0 {
		obs.Fail("SIDECAR", "MAIN", "AA_SIDECAR_SCOPE_CEILING must be set")
		os.Exit(1)
	}

	bc := newBrokerClient(cfg.BrokerURL)

	obs.Ok("SIDECAR", "MAIN", "starting", "broker="+cfg.BrokerURL, "scope_ceiling="+strings.Join(cfg.ScopeCeiling, ","))

	// Create agent registry (ephemeral, in-memory).
	registry := newAgentRegistry()

	// Create circuit breaker.
	cb := newCircuitBreaker(
		time.Duration(cfg.CBWindow)*time.Second,
		cfg.CBThreshold,
		time.Duration(cfg.CBProbeInterval)*time.Second,
		cfg.CBMinRequests,
	)

	// Shared state pointer — nil until bootstrap succeeds.
	var state *sidecarState

	// Set up routes. Health works pre-bootstrap; token routes require state.
	ceiling := newCeilingCache(cfg.ScopeCeiling)
	mux := http.NewServeMux()
	healthH := newHealthHandler(nil, ceiling, registry)
	mux.Handle("/v1/health", healthH)
	mux.Handle("/v1/metrics", promhttp.Handler())

	// Start HTTP server immediately so health probes get a response.
	addr := ":" + cfg.Port
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			obs.Fail("SIDECAR", "MAIN", "listen failed", err.Error())
			os.Exit(1)
		}
	}()
	obs.Ok("SIDECAR", "MAIN", "http server started (pre-bootstrap)", "addr="+addr)

	// Bootstrap with retry.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		obs.Ok("SIDECAR", "MAIN", "shutting down")
		cancel()
	}()

	backoff := 1 * time.Second
	maxBootBackoff := 60 * time.Second
	attempt := 0
	for {
		var err error
		state, err = bootstrap(bc, cfg)
		if err == nil {
			break
		}
		attempt++
		obs.Warn("SIDECAR", "BOOTSTRAP", "failed, retrying",
			fmt.Sprintf("attempt=%d", attempt),
			fmt.Sprintf("retry_in=%s", backoff),
			err.Error(),
		)
		RecordBootstrap("failure")

		select {
		case <-ctx.Done():
			obs.Fail("SIDECAR", "MAIN", "shutdown during bootstrap")
			os.Exit(1)
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBootBackoff {
			backoff = maxBootBackoff
		}
	}

	// Bootstrap succeeded — wire remaining routes.
	healthH.state = state
	mux.Handle("/v1/token", newTokenHandler(bc, state, ceiling, registry, cfg.AdminSecret, cb))
	mux.Handle("/v1/token/renew", newRenewHandler(bc))
	mux.Handle("/v1/challenge", newChallengeProxyHandler(bc))
	mux.Handle("/v1/register", newRegisterHandler(bc, registry, cfg.AdminSecret, ceiling))

	go startRenewal(ctx, state, bc.tokenRenew, cfg.RenewalBuffer, ceiling)
	obs.Ok("SIDECAR", "MAIN", "renewal goroutine started", fmt.Sprintf("buffer=%.0f%%", cfg.RenewalBuffer*100))

	go startProbe(ctx, cb, bc)
	obs.Ok("SIDECAR", "MAIN", "circuit breaker active",
		fmt.Sprintf("window=%ds", cfg.CBWindow),
		fmt.Sprintf("threshold=%.0f%%", cfg.CBThreshold*100),
		fmt.Sprintf("probe_interval=%ds", cfg.CBProbeInterval),
	)

	obs.Ok("SIDECAR", "MAIN", "ready", "addr="+addr, "sidecar_id="+state.sidecarID)

	// Block until shutdown.
	<-ctx.Done()
}
