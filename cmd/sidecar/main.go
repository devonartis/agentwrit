package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

	state, err := bootstrap(bc, cfg)
	if err != nil {
		obs.Fail("SIDECAR", "MAIN", "bootstrap failed", err.Error())
		os.Exit(1)
	}

	// Start background renewal goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go startRenewal(ctx, state, bc.tokenRenew, cfg.RenewalBuffer)
	obs.Ok("SIDECAR", "MAIN", "renewal goroutine started", fmt.Sprintf("buffer=%.0f%%", cfg.RenewalBuffer*100))

	// Create agent registry (ephemeral, in-memory).
	registry := newAgentRegistry()

	// Set up routes.
	mux := http.NewServeMux()
	mux.Handle("/v1/token", newTokenHandler(bc, state, cfg.ScopeCeiling, registry, cfg.AdminSecret))
	mux.Handle("/v1/token/renew", newRenewHandler(bc))
	mux.Handle("/v1/health", newHealthHandler(state, cfg.ScopeCeiling, registry))
	mux.Handle("/v1/challenge", newChallengeProxyHandler(bc))
	mux.Handle("/v1/register", newRegisterHandler(bc, registry, cfg.AdminSecret, cfg.ScopeCeiling))
	mux.Handle("/v1/metrics", promhttp.Handler())

	addr := ":" + cfg.Port
	obs.Ok("SIDECAR", "MAIN", "ready", "addr="+addr, "sidecar_id="+state.sidecarID)

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		obs.Ok("SIDECAR", "MAIN", "shutting down")
		cancel()
	}()

	if err := http.ListenAndServe(addr, mux); err != nil {
		obs.Fail("SIDECAR", "MAIN", "listen failed", err.Error())
		os.Exit(1)
	}
}
