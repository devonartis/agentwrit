package cfg

import (
	"os"
	"testing"
)

func TestLoad_DBPathDefault(t *testing.T) {
	os.Unsetenv("AA_DB_PATH")
	c := Load()
	if c.DBPath != "./agentauth.db" {
		t.Fatalf("expected default ./agentauth.db, got %q", c.DBPath)
	}
}

func TestLoad_DBPathCustom(t *testing.T) {
	os.Setenv("AA_DB_PATH", "/tmp/test.db")
	defer os.Unsetenv("AA_DB_PATH")
	c := Load()
	if c.DBPath != "/tmp/test.db" {
		t.Fatalf("expected /tmp/test.db, got %q", c.DBPath)
	}
}

func TestLoad_TLSModeDefault(t *testing.T) {
	os.Unsetenv("AA_TLS_MODE")
	c := Load()
	if c.TLSMode != "none" {
		t.Fatalf("expected default TLSMode 'none', got %q", c.TLSMode)
	}
}

func TestLoad_TLSModeSet(t *testing.T) {
	os.Setenv("AA_TLS_MODE", "mtls")
	defer os.Unsetenv("AA_TLS_MODE")
	c := Load()
	if c.TLSMode != "mtls" {
		t.Fatalf("expected TLSMode 'mtls', got %q", c.TLSMode)
	}
}

func TestLoad_TLSFields(t *testing.T) {
	os.Setenv("AA_TLS_CERT", "/etc/certs/cert.pem")
	os.Setenv("AA_TLS_KEY", "/etc/certs/key.pem")
	os.Setenv("AA_TLS_CLIENT_CA", "/etc/certs/ca.pem")
	defer os.Unsetenv("AA_TLS_CERT")
	defer os.Unsetenv("AA_TLS_KEY")
	defer os.Unsetenv("AA_TLS_CLIENT_CA")
	c := Load()
	if c.TLSCert != "/etc/certs/cert.pem" {
		t.Fatalf("expected TLSCert '/etc/certs/cert.pem', got %q", c.TLSCert)
	}
	if c.TLSKey != "/etc/certs/key.pem" {
		t.Fatalf("expected TLSKey '/etc/certs/key.pem', got %q", c.TLSKey)
	}
	if c.TLSClientCA != "/etc/certs/ca.pem" {
		t.Fatalf("expected TLSClientCA '/etc/certs/ca.pem', got %q", c.TLSClientCA)
	}
}
