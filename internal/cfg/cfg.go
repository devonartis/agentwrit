// Package cfg loads broker configuration from AA_* environment variables.
//
// All configuration keys are prefixed with AA_ to avoid collisions.
// Unset or empty variables fall back to sensible defaults suitable for
// local development. In production the operator should set at minimum
// AA_ADMIN_SECRET to a strong random value.
//
// Supported variables:
//
//	AA_PORT          – HTTP listen port             (default "8080")
//	AA_LOG_LEVEL     – quiet|standard|verbose|trace (default "verbose")
//	AA_TRUST_DOMAIN  – SPIFFE trust domain          (default "agentauth.local")
//	AA_DEFAULT_TTL   – token TTL in seconds         (default 300)
//	AA_ADMIN_SECRET  – shared secret for admin auth (required in production)
//	AA_SEED_TOKENS   – print seed tokens on startup (default "false", dev only)
//	AA_DB_PATH       – SQLite database file path    (default "./agentauth.db")
//	AA_TLS_MODE      – none|tls|mtls               (default "none")
//	AA_TLS_CERT      – path to TLS certificate PEM file
//	AA_TLS_KEY       – path to TLS private key PEM file
//	AA_TLS_CLIENT_CA – path to client CA certificate PEM file (mtls only)
package cfg

import (
	"os"
	"strconv"
)

// Cfg holds the complete broker configuration derived from environment
// variables. Use [Load] to create an instance with defaults applied.
type Cfg struct {
	Port        string // AA_PORT (default "8080")
	LogLevel    string // AA_LOG_LEVEL (default "verbose")
	TrustDomain string // AA_TRUST_DOMAIN (default "agentauth.local")
	DefaultTTL  int    // AA_DEFAULT_TTL (default 300 seconds)
	AdminSecret string // AA_ADMIN_SECRET (required for admin auth)
	SeedTokens  bool   // AA_SEED_TOKENS (dev only, default false)
	DBPath      string // AA_DB_PATH (default "./agentauth.db")
	TLSMode     string // AA_TLS_MODE: none|tls|mtls (default "none")
	TLSCert     string // AA_TLS_CERT: path to TLS certificate PEM file
	TLSKey      string // AA_TLS_KEY: path to TLS private key PEM file
	TLSClientCA string // AA_TLS_CLIENT_CA: path to client CA PEM file (mtls only)
}

// Load reads AA_* environment variables and returns a Cfg with defaults
// applied for any missing values. It never returns an error; invalid
// numeric values silently fall back to their defaults.
func Load() Cfg {
	c := Cfg{
		Port:        envOr("AA_PORT", "8080"),
		LogLevel:    envOr("AA_LOG_LEVEL", "verbose"),
		TrustDomain: envOr("AA_TRUST_DOMAIN", "agentauth.local"),
		DefaultTTL:  envIntOr("AA_DEFAULT_TTL", 300),
		AdminSecret: os.Getenv("AA_ADMIN_SECRET"),
		SeedTokens:  envOr("AA_SEED_TOKENS", "false") == "true",
		DBPath:      envOr("AA_DB_PATH", "./agentauth.db"),
		TLSMode:     envOr("AA_TLS_MODE", "none"),
		TLSCert:     os.Getenv("AA_TLS_CERT"),
		TLSKey:      os.Getenv("AA_TLS_KEY"),
		TLSClientCA: os.Getenv("AA_TLS_CLIENT_CA"),
	}
	return c
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
