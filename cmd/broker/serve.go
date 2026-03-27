package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/obs"
)

// buildServer creates an http.Server with hardened timeouts.
func buildServer(c cfg.Cfg, addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}
}

// serve starts the HTTP server and blocks until SIGINT or SIGTERM is received.
// On signal, it gracefully shuts down: stops accepting new connections, waits
// up to 10 seconds for in-flight requests, calls onShutdown (for cleanup like
// closing the database), then returns.
func serve(c cfg.Cfg, addr string, handler http.Handler, onShutdown func()) error {
	srv := buildServer(c, addr, handler)

	switch c.TLSMode {
	case "tls":
		srv.TLSConfig = &tls.Config{}
	case "mtls":
		pool, err := loadCA(c.TLSClientCA)
		if err != nil {
			return fmt.Errorf("loading client CA: %w", err)
		}
		srv.TLSConfig = &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  pool,
		}
	}

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		var err error
		switch c.TLSMode {
		case "tls":
			err = srv.ListenAndServeTLS(c.TLSCert, c.TLSKey)
		case "mtls":
			err = srv.ListenAndServeTLS(c.TLSCert, c.TLSKey)
		default:
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for signal or server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		obs.Ok("BROKER", "shutdown", "signal received", "signal="+sig.String())
		fmt.Printf("\nShutting down gracefully (signal: %s)...\n", sig)
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	// Graceful shutdown with 10-second deadline
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		obs.Warn("BROKER", "shutdown", "graceful shutdown exceeded deadline", "error="+err.Error())
	}

	if onShutdown != nil {
		onShutdown()
	}

	obs.Ok("BROKER", "shutdown", "clean exit")
	return nil
}

// loadCA reads a PEM-encoded CA certificate file and returns a cert pool
// containing it. Returns an error if the file cannot be read or contains
// no valid certificates.
func loadCA(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no valid certificates found in %s", path)
	}
	return pool, nil
}
