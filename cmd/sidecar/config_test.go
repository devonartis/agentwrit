package main

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	os.Setenv("AA_ADMIN_SECRET", "test-secret")
	os.Setenv("AA_SIDECAR_SCOPE_CEILING", "read:data:*")
	defer os.Unsetenv("AA_ADMIN_SECRET")
	defer os.Unsetenv("AA_SIDECAR_SCOPE_CEILING")

	cfg := loadConfig()

	if cfg.BrokerURL != "http://localhost:8080" {
		t.Errorf("BrokerURL = %q, want http://localhost:8080", cfg.BrokerURL)
	}
	if cfg.Port != "8081" {
		t.Errorf("Port = %q, want 8081", cfg.Port)
	}
	if cfg.AdminSecret != "test-secret" {
		t.Errorf("AdminSecret = %q, want test-secret", cfg.AdminSecret)
	}
	if len(cfg.ScopeCeiling) != 1 || cfg.ScopeCeiling[0] != "read:data:*" {
		t.Errorf("ScopeCeiling = %v, want [read:data:*]", cfg.ScopeCeiling)
	}
}

func TestLoadConfig_CustomEnv(t *testing.T) {
	os.Setenv("AA_BROKER_URL", "http://broker:9090")
	os.Setenv("AA_SIDECAR_PORT", "9091")
	os.Setenv("AA_ADMIN_SECRET", "custom-secret")
	os.Setenv("AA_SIDECAR_SCOPE_CEILING", "read:data:*,write:orders:*")
	defer func() {
		os.Unsetenv("AA_BROKER_URL")
		os.Unsetenv("AA_SIDECAR_PORT")
		os.Unsetenv("AA_ADMIN_SECRET")
		os.Unsetenv("AA_SIDECAR_SCOPE_CEILING")
	}()

	cfg := loadConfig()

	if cfg.BrokerURL != "http://broker:9090" {
		t.Errorf("BrokerURL = %q, want http://broker:9090", cfg.BrokerURL)
	}
	if cfg.Port != "9091" {
		t.Errorf("Port = %q, want 9091", cfg.Port)
	}
	if len(cfg.ScopeCeiling) != 2 {
		t.Errorf("ScopeCeiling has %d entries, want 2", len(cfg.ScopeCeiling))
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	os.Unsetenv("AA_ADMIN_SECRET")
	os.Unsetenv("AA_SIDECAR_SCOPE_CEILING")

	cfg := loadConfig()

	if cfg.AdminSecret != "" {
		t.Errorf("AdminSecret should be empty when unset")
	}
	if len(cfg.ScopeCeiling) != 0 {
		t.Errorf("ScopeCeiling should be empty when unset")
	}
}
