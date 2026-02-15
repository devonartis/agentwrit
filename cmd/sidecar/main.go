package main

import (
	"fmt"
	"net/http"
	"os"
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

	// Set up routes. Handlers already enforce method checks internally,
	// so plain path patterns avoid redundant double-checking.
	reg := newAgentRegistry()
	mux := http.NewServeMux()
	mux.Handle("/v1/token", newTokenHandler(bc, state, cfg.ScopeCeiling, reg, cfg.AdminSecret))
	mux.Handle("/v1/token/renew", newRenewHandler(bc))
	mux.Handle("/v1/health", newHealthHandler(state, cfg.ScopeCeiling))

	addr := ":" + cfg.Port
	fmt.Printf("[sidecar] ready on %s, sidecar_id=%s\n", addr, state.sidecarID)

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}
