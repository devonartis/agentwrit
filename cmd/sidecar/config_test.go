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
	if cfg.LogLevel != "standard" {
		t.Errorf("LogLevel = %q, want standard", cfg.LogLevel)
	}
}

func TestLoadConfig_CustomEnv(t *testing.T) {
	os.Setenv("AA_BROKER_URL", "http://broker:9090")
	os.Setenv("AA_SIDECAR_PORT", "9091")
	os.Setenv("AA_ADMIN_SECRET", "custom-secret")
	os.Setenv("AA_SIDECAR_SCOPE_CEILING", " read:data:* , write:orders:* , , ")
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
		t.Fatalf("ScopeCeiling has %d entries, want 2: %v", len(cfg.ScopeCeiling), cfg.ScopeCeiling)
	}
	if cfg.ScopeCeiling[0] != "read:data:*" {
		t.Errorf("ScopeCeiling[0] = %q, want read:data:*", cfg.ScopeCeiling[0])
	}
	if cfg.ScopeCeiling[1] != "write:orders:*" {
		t.Errorf("ScopeCeiling[1] = %q, want write:orders:*", cfg.ScopeCeiling[1])
	}
}

func TestLoadConfig_RenewalBuffer(t *testing.T) {
	// Clean up env vars after each sub-test.
	defer os.Unsetenv("AA_SIDECAR_RENEWAL_BUFFER")

	tests := []struct {
		name    string
		envVal  string
		setEnv  bool
		want    float64
	}{
		{"default when unset", "", false, 0.8},
		{"valid 0.5", "0.5", true, 0.5},
		{"valid 0.7", "0.7", true, 0.7},
		{"valid 0.95", "0.95", true, 0.95},
		{"below minimum clamps to default", "0.3", true, 0.8},
		{"above maximum clamps to default", "0.99", true, 0.8},
		{"invalid string clamps to default", "abc", true, 0.8},
		{"empty string uses default", "", true, 0.8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("AA_SIDECAR_RENEWAL_BUFFER")
			if tt.setEnv {
				os.Setenv("AA_SIDECAR_RENEWAL_BUFFER", tt.envVal)
			}
			cfg := loadConfig()
			if cfg.RenewalBuffer != tt.want {
				t.Errorf("RenewalBuffer = %v, want %v", cfg.RenewalBuffer, tt.want)
			}
		})
	}
}

func TestLoadConfig_TLSFields(t *testing.T) {
	os.Setenv("AA_SIDECAR_CA_CERT", "/path/to/ca.pem")
	os.Setenv("AA_SIDECAR_TLS_CERT", "/path/to/client.pem")
	os.Setenv("AA_SIDECAR_TLS_KEY", "/path/to/client-key.pem")
	defer func() {
		os.Unsetenv("AA_SIDECAR_CA_CERT")
		os.Unsetenv("AA_SIDECAR_TLS_CERT")
		os.Unsetenv("AA_SIDECAR_TLS_KEY")
	}()

	cfg := loadConfig()

	if cfg.CACert != "/path/to/ca.pem" {
		t.Fatalf("CACert: expected /path/to/ca.pem, got %q", cfg.CACert)
	}
	if cfg.TLSCert != "/path/to/client.pem" {
		t.Fatalf("TLSCert: expected /path/to/client.pem, got %q", cfg.TLSCert)
	}
	if cfg.TLSKey != "/path/to/client-key.pem" {
		t.Fatalf("TLSKey: expected /path/to/client-key.pem, got %q", cfg.TLSKey)
	}
}

func TestLoadConfig_TLSFieldsDefault(t *testing.T) {
	os.Unsetenv("AA_SIDECAR_CA_CERT")
	os.Unsetenv("AA_SIDECAR_TLS_CERT")
	os.Unsetenv("AA_SIDECAR_TLS_KEY")

	cfg := loadConfig()

	if cfg.CACert != "" {
		t.Fatalf("CACert should be empty by default, got %q", cfg.CACert)
	}
	if cfg.TLSCert != "" {
		t.Fatalf("TLSCert should be empty by default, got %q", cfg.TLSCert)
	}
	if cfg.TLSKey != "" {
		t.Fatalf("TLSKey should be empty by default, got %q", cfg.TLSKey)
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
