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
