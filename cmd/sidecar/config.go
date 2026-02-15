package main

import (
	"os"
	"strconv"
	"strings"
)

type sidecarConfig struct {
	BrokerURL     string
	Port          string
	AdminSecret   string
	ScopeCeiling  []string
	LogLevel      string
	RenewalBuffer float64
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
