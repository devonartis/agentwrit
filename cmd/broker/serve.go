package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	"github.com/divineartis/agentauth/internal/cfg"
)

// serve starts the HTTP listener using the TLS mode specified in c.
// Mode "none" uses plain HTTP. Mode "tls" uses one-way TLS. Mode "mtls"
// requires and verifies client certificates against the configured CA.
// The call blocks until the server exits.
func serve(c cfg.Cfg, addr string, handler http.Handler) error {
	switch c.TLSMode {
	case "tls":
		return http.ListenAndServeTLS(addr, c.TLSCert, c.TLSKey, handler)
	case "mtls":
		pool, err := loadCA(c.TLSClientCA)
		if err != nil {
			return fmt.Errorf("loading client CA: %w", err)
		}
		tlsCfg := &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  pool,
		}
		srv := &http.Server{Addr: addr, Handler: handler, TLSConfig: tlsCfg}
		return srv.ListenAndServeTLS(c.TLSCert, c.TLSKey)
	default: // "none"
		return http.ListenAndServe(addr, handler)
	}
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
