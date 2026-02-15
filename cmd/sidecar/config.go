package main

import (
	"os"
	"strconv"
	"strings"
)

type sidecarConfig struct {
	BrokerURL       string
	Port            string
	AdminSecret     string
	ScopeCeiling    []string
	LogLevel        string
	RenewalBuffer   float64
	CBWindow        int     // sliding window duration in seconds
	CBThreshold     float64 // failure rate 0.0-1.0 to trip circuit
	CBProbeInterval int     // seconds between health probes when open
	CBMinRequests   int     // min requests in window before tripping
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
