package main

import (
	"os"
	"strconv"
	"strings"
)

// sidecarConfig holds all runtime configuration for the sidecar process,
// loaded from AA_* environment variables at startup. See loadConfig for
// defaults and validation rules.
type sidecarConfig struct {
	// BrokerURL is the base URL of the AgentAuth broker (e.g. "http://broker:8080").
	BrokerURL string
	// Port is the HTTP listen port for the sidecar (default "8081").
	Port string
	// AdminSecret is the shared secret used for broker admin authentication.
	// Must match the broker's AA_ADMIN_SECRET. Required.
	AdminSecret string
	// ScopeCeiling is the maximum set of scopes this sidecar can issue to agents.
	// Parsed from the comma-separated AA_SIDECAR_SCOPE_CEILING env var. Required.
	ScopeCeiling []string
	// LogLevel controls sidecar log verbosity: "quiet", "standard", "verbose", "trace".
	LogLevel string
	// RenewalBuffer is the fraction of TTL at which the sidecar renews its own
	// broker bearer token. Valid range: 0.5–0.95 (default 0.8).
	RenewalBuffer float64
	// CBWindow is the circuit breaker sliding window duration in seconds (default 30).
	CBWindow int
	// CBThreshold is the failure rate (0.0–1.0) within the window that trips
	// the circuit breaker (default 0.5).
	CBThreshold float64
	// CBProbeInterval is the seconds between broker health probes when the
	// circuit breaker is open (default 5).
	CBProbeInterval int
	// CBMinRequests is the minimum number of requests in the sliding window
	// before the circuit breaker can trip (default 5).
	CBMinRequests int
	// CACert is the path to the CA certificate PEM file for verifying the
	// broker's TLS certificate. When set, the sidecar connects over HTTPS.
	CACert string
	// TLSCert is the path to the sidecar's client certificate PEM file
	// for mTLS. Only used when CACert is also set.
	TLSCert string
	// TLSKey is the path to the sidecar's client private key PEM file
	// for mTLS. Only used when CACert is also set.
	TLSKey string
}

func loadConfig() sidecarConfig {
	cfg := sidecarConfig{
		BrokerURL:   envOr("AA_BROKER_URL", "http://localhost:8080"),
		Port:        envOr("AA_SIDECAR_PORT", "8081"),
		AdminSecret: os.Getenv("AA_ADMIN_SECRET"),
		LogLevel:    envOr("AA_SIDECAR_LOG_LEVEL", "standard"),
	}

	renewalRaw := envOr("AA_SIDECAR_RENEWAL_BUFFER", "0.8")
	renewalBuf := 0.8
	if v, err := strconv.ParseFloat(renewalRaw, 64); err == nil && v >= 0.5 && v <= 0.95 {
		renewalBuf = v
	}
	cfg.RenewalBuffer = renewalBuf

	cfg.CBWindow = envOrInt("AA_SIDECAR_CB_WINDOW", 30)
	cfg.CBThreshold = envOrFloat("AA_SIDECAR_CB_THRESHOLD", 0.5, 0.0, 1.0)
	cfg.CBProbeInterval = envOrInt("AA_SIDECAR_CB_PROBE_INTERVAL", 5)
	cfg.CBMinRequests = envOrInt("AA_SIDECAR_CB_MIN_REQUESTS", 5)

	raw := os.Getenv("AA_SIDECAR_SCOPE_CEILING")
	if raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				cfg.ScopeCeiling = append(cfg.ScopeCeiling, s)
			}
		}
	}

	cfg.CACert = os.Getenv("AA_SIDECAR_CA_CERT")
	cfg.TLSCert = os.Getenv("AA_SIDECAR_TLS_CERT")
	cfg.TLSKey = os.Getenv("AA_SIDECAR_TLS_KEY")

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func envOrFloat(key string, fallback, min, max float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= min && f <= max {
			return f
		}
	}
	return fallback
}
