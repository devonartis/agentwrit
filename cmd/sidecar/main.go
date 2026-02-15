package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := loadConfig()

	// Validate required config.
	if cfg.AdminSecret == "" {
		fmt.Fprintln(os.Stderr, "FATAL: AA_ADMIN_SECRET must be set")
		os.Exit(1)
	}
	if len(cfg.ScopeCeiling) == 0 {
		fmt.Fprintln(os.Stderr, "FATAL: AA_SIDECAR_SCOPE_CEILING must be set")
		os.Exit(1)
	}

	bc := newBrokerClient(cfg.BrokerURL)

	fmt.Printf("[sidecar] starting, broker=%s, scope_ceiling=%v\n", cfg.BrokerURL, cfg.ScopeCeiling)

	state, err := bootstrap(bc, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	// Start background renewal goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go startRenewal(ctx, state, bc.tokenRenew, cfg.RenewalBuffer)
	fmt.Printf("[sidecar] renewal goroutine started (buffer=%.0f%%)\n", cfg.RenewalBuffer*100)

	// Create agent registry (ephemeral, in-memory).
	registry := newAgentRegistry()

	// Set up routes.
	mux := http.NewServeMux()
	mux.Handle("/v1/token", newTokenHandler(bc, state, cfg.ScopeCeiling, registry, cfg.AdminSecret))
	mux.Handle("/v1/token/renew", newRenewHandler(bc))
	mux.Handle("/v1/health", newHealthHandler(state, cfg.ScopeCeiling))
	mux.Handle("/v1/challenge", newChallengeProxyHandler(bc))
	mux.Handle("/v1/register", newRegisterHandler(bc, registry, cfg.AdminSecret, cfg.ScopeCeiling))

	addr := ":" + cfg.Port
	fmt.Printf("[sidecar] ready on %s, sidecar_id=%s\n", addr, state.sidecarID)

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("[sidecar] shutting down...")
		cancel()
	}()

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}
