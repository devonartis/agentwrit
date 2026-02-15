package main

import (
	"os"
	"strings"
)

type sidecarConfig struct {
	BrokerURL    string
	Port         string
	AdminSecret  string
	ScopeCeiling []string
	LogLevel     string
}

func loadConfig() sidecarConfig {
	cfg := sidecarConfig{
		BrokerURL:   envOr("AA_BROKER_URL", "http://localhost:8080"),
		Port:        envOr("AA_SIDECAR_PORT", "8081"),
		AdminSecret: os.Getenv("AA_ADMIN_SECRET"),
		LogLevel:    envOr("AA_SIDECAR_LOG_LEVEL", "standard"),
	}

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
