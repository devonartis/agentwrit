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
//	AA_TRUST_DOMAIN  – SPIFFE trust domain          (default "agentwrit.local")
//	AA_ISSUER        – JWT iss claim — broker identity (empty = skip iss check on verify)
//	AA_DEFAULT_TTL   – token TTL in seconds         (default 300)
//	AA_ADMIN_SECRET  – shared secret for admin auth (required in production)
//	AA_SEED_TOKENS   – print seed tokens on startup (default "false", dev only)
//	AA_DB_PATH       – SQLite database file path    (default "./data.db")
//	AA_SIGNING_KEY_PATH   – Ed25519 signing key file path     (default "./signing.key")
//	AA_TLS_MODE      – none|tls|mtls               (default "none")
//	AA_TLS_CERT      – path to TLS certificate PEM file
//	AA_TLS_KEY       – path to TLS private key PEM file
//	AA_TLS_CLIENT_CA – path to client CA certificate PEM file (mtls only)
//	AA_AUDIENCE      – expected token audience claim        (empty = skip — operator opt-in)
//	AA_APP_TOKEN_TTL – app JWT TTL in seconds              (default 1800 / 30 min)
//	AA_ADMIN_TOKEN_TTL – admin JWT TTL in seconds          (default 300 / 5 min)
package cfg

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/devonartis/agentauth/internal/obs"

	"golang.org/x/crypto/bcrypt"
)

// AdminBcryptCost is the bcrypt cost factor used for hashing admin secrets.
// Cost 12 ≈ 250ms per hash on modern hardware — good balance of security and latency.
const AdminBcryptCost = 12

// defaultAdminTokenTTL is the default lifetime of an admin JWT in seconds.
// 300s = 5 minutes — short enough to limit blast radius of a leaked admin
// token, long enough for interactive operator workflows. Operators override
// via AA_ADMIN_TOKEN_TTL. Named so no magic number leaks into Load().
const defaultAdminTokenTTL = 300

// Cfg holds the complete broker configuration derived from environment
// variables. Use [Load] to create an instance with defaults applied.
type Cfg struct {
	Port            string // AA_PORT (default "8080")
	BindAddress     string // AA_BIND_ADDRESS (default "127.0.0.1")
	LogLevel        string // AA_LOG_LEVEL (default "verbose")
	TrustDomain     string // AA_TRUST_DOMAIN (default "agentwrit.local")
	Issuer          string // AA_ISSUER: JWT iss claim — broker identity. Empty = skip issuer check on verify (mirrors Audience contract). Operators set this to a value that uniquely identifies their broker instance.
	DefaultTTL      int    // AA_DEFAULT_TTL (default 300 seconds)
	AppTokenTTL     int    // AA_APP_TOKEN_TTL (default 1800 seconds / 30 min)
	AdminTokenTTL   int    // AA_ADMIN_TOKEN_TTL (default 300 seconds / 5 min)
	AdminSecret     string // AA_ADMIN_SECRET (required for admin auth)
	SeedTokens      bool   // AA_SEED_TOKENS (dev only, default false)
	DBPath          string // AA_DB_PATH (default "./data.db")
	SigningKeyPath  string // AA_SIGNING_KEY_PATH (default "./signing.key")
	TLSMode         string // AA_TLS_MODE: none|tls|mtls (default "none")
	TLSCert         string // AA_TLS_CERT: path to TLS certificate PEM file
	TLSKey          string // AA_TLS_KEY: path to TLS private key PEM file
	TLSClientCA     string // AA_TLS_CLIENT_CA: path to client CA PEM file (mtls only)
	Audience        string // AA_AUDIENCE: expected token audience (empty = skip — operator opt-in)
	Mode            string // MODE: development|production (default "development")
	AdminSecretHash string // bcrypt hash of admin secret (derived at load time)
	ConfigPath      string // resolved config file path (empty if none found)
	MaxTTL          int    // AA_MAX_TTL: max token TTL in seconds (default 86400 = 24h, 0 = no limit)
}

// Load reads AA_* environment variables and returns a Cfg with defaults
// applied for any missing values. Returns an error if the admin secret
// cannot be hashed. Invalid numeric values silently fall back to their defaults.
func Load() (Cfg, error) {
	// Read config file defaults first.
	cfgMode, cfgSecret, cfgPath := loadConfigFile()

	c := Cfg{
		Port:           envOr("AA_PORT", "8080"),
		BindAddress:    envOr("AA_BIND_ADDRESS", "127.0.0.1"),
		LogLevel:       envOr("AA_LOG_LEVEL", "verbose"),
		TrustDomain:    envOr("AA_TRUST_DOMAIN", "agentwrit.local"),
		Issuer:         os.Getenv("AA_ISSUER"),
		DefaultTTL:     envIntOr("AA_DEFAULT_TTL", 300),
		AppTokenTTL:    envIntOr("AA_APP_TOKEN_TTL", 1800),
		AdminTokenTTL:  envIntOr("AA_ADMIN_TOKEN_TTL", defaultAdminTokenTTL),
		AdminSecret:    os.Getenv("AA_ADMIN_SECRET"),
		SeedTokens:     envOr("AA_SEED_TOKENS", "false") == "true",
		DBPath:         envOr("AA_DB_PATH", "./data.db"),
		SigningKeyPath: envOr("AA_SIGNING_KEY_PATH", "./signing.key"),
		TLSMode:        envOr("AA_TLS_MODE", "none"),
		TLSCert:        os.Getenv("AA_TLS_CERT"),
		TLSKey:         os.Getenv("AA_TLS_KEY"),
		TLSClientCA:    os.Getenv("AA_TLS_CLIENT_CA"),
		ConfigPath:     cfgPath,
		Mode:           "development",
	}
	// AA_AUDIENCE: empty (unset OR explicitly empty) skips audience validation.
	// Operators opt in by setting a value. No brand-coupled default.
	c.Audience = os.Getenv("AA_AUDIENCE")

	// Config file values are defaults; env vars override.
	if c.AdminSecret == "" && cfgSecret != "" {
		c.AdminSecret = cfgSecret
	}
	if cfgMode != "" {
		c.Mode = cfgMode
	}

	// Reject known-weak admin secrets at startup (H5).
	denylist := []string{"change-me-in-production", ""}
	if slices.Contains(denylist, c.AdminSecret) {
		return Cfg{}, fmt.Errorf("admin secret is a known-weak default; run 'aactl init' to generate a secure config, or set a strong AA_ADMIN_SECRET")
	}

	// Derive bcrypt hash for comparison.
	if c.AdminSecret != "" {
		if isBcryptHash(c.AdminSecret) {
			c.AdminSecretHash = c.AdminSecret
		} else {
			hash, err := bcrypt.GenerateFromPassword([]byte(c.AdminSecret), AdminBcryptCost)
			if err != nil {
				return Cfg{}, fmt.Errorf("cfg: hash admin secret: %w", err)
			}
			c.AdminSecretHash = string(hash)
		}
		// Wipe plaintext from struct field. Note: Go strings are immutable —
		// the original bytes may linger in heap until GC. This is a known
		// limitation of Go's memory model. Using []byte would reduce the
		// window but bcrypt internally copies to string. Accepted risk.
		c.AdminSecret = ""
	}

	// Security hardening (L2a)
	c.MaxTTL = envIntOr("AA_MAX_TTL", 86400)
	if c.MaxTTL > 0 && c.DefaultTTL > c.MaxTTL {
		obs.Warn("CFG", "load", "AA_DEFAULT_TTL exceeds AA_MAX_TTL — tokens will be clamped to MaxTTL",
			fmt.Sprintf("default_ttl=%d max_ttl=%d", c.DefaultTTL, c.MaxTTL))
	}

	return c, nil
}

// isBcryptHash returns true if the value is a well-formed bcrypt hash.
// A valid bcrypt hash is exactly 60 characters: $2a$XX$ + 53 chars.
func isBcryptHash(s string) bool {
	if len(s) != 60 {
		return false
	}
	return strings.HasPrefix(s, "$2a$") || strings.HasPrefix(s, "$2b$") || strings.HasPrefix(s, "$2y$")
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
