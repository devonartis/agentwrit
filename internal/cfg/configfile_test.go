package cfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigFile_ValidDevConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	content := "# AgentAuth Configuration\nMODE=development\nADMIN_SECRET=my-test-secret\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AA_CONFIG_PATH", cfgPath)

	mode, secret, path := loadConfigFile()
	if mode != "development" {
		t.Errorf("expected mode=development, got %q", mode)
	}
	if secret != "my-test-secret" {
		t.Errorf("expected secret=my-test-secret, got %q", secret)
	}
	if path != cfgPath {
		t.Errorf("expected path=%q, got %q", cfgPath, path)
	}
}

func TestLoadConfigFile_ValidProdConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	hash := "$2a$12$somebcrypthashvaluehere1234567890abcdefghij"
	content := "MODE=production\nADMIN_SECRET=" + hash + "\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AA_CONFIG_PATH", cfgPath)

	mode, secret, path := loadConfigFile()
	if mode != "production" {
		t.Errorf("expected mode=production, got %q", mode)
	}
	if secret != hash {
		t.Errorf("expected bcrypt hash, got %q", secret)
	}
	if path != cfgPath {
		t.Errorf("expected path=%q, got %q", cfgPath, path)
	}
}

func TestLoadConfigFile_CommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	content := "# Comment\n\n  \nMODE=development\n# Another comment\nADMIN_SECRET=test123\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AA_CONFIG_PATH", cfgPath)

	mode, secret, _ := loadConfigFile()
	if mode != "development" {
		t.Errorf("expected mode=development, got %q", mode)
	}
	if secret != "test123" {
		t.Errorf("expected secret=test123, got %q", secret)
	}
}

func TestLoadConfigFile_NoFileFound(t *testing.T) {
	t.Setenv("AA_CONFIG_PATH", "/nonexistent/path/config")

	mode, secret, path := loadConfigFile()
	if mode != "" {
		t.Errorf("expected empty mode, got %q", mode)
	}
	if secret != "" {
		t.Errorf("expected empty secret, got %q", secret)
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
}

func TestWriteConfigFile_DevMode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "subdir", "config")

	err := WriteConfigFile(cfgPath, "development", "my-plaintext-secret")
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %o", info.Mode().Perm())
	}

	dirInfo, err := os.Stat(filepath.Dir(cfgPath))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("expected dir 0700 permissions, got %o", dirInfo.Mode().Perm())
	}

	mode, secret, _ := loadConfigFileAt(cfgPath)
	if mode != "development" {
		t.Errorf("expected mode=development, got %q", mode)
	}
	if secret != "my-plaintext-secret" {
		t.Errorf("expected secret=my-plaintext-secret, got %q", secret)
	}
}

func TestWriteConfigFile_ProdMode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	bcryptHash := "$2a$12$examplebcrypthashvalue"
	err := WriteConfigFile(cfgPath, "production", bcryptHash)
	if err != nil {
		t.Fatal(err)
	}

	mode, secret, _ := loadConfigFileAt(cfgPath)
	if mode != "production" {
		t.Errorf("expected mode=production, got %q", mode)
	}
	if secret != bcryptHash {
		t.Errorf("expected hash=%q, got %q", bcryptHash, secret)
	}
}

// --- Task 2: Load integration tests ---

func TestLoad_EnvVarOverridesConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	content := "MODE=development\nADMIN_SECRET=from-config-file\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AA_CONFIG_PATH", cfgPath)
	t.Setenv("AA_ADMIN_SECRET", "from-env-var")

	c := Load()
	if c.AdminSecret != "from-env-var" {
		t.Errorf("expected env var to win, got AdminSecret=%q", c.AdminSecret)
	}
}

func TestLoad_ConfigFileUsedWhenNoEnvVar(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	content := "MODE=production\nADMIN_SECRET=$2a$12$examplebcrypthashvalue1234567890abc\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AA_CONFIG_PATH", cfgPath)
	t.Setenv("AA_ADMIN_SECRET", "")

	c := Load()
	if c.AdminSecret != "$2a$12$examplebcrypthashvalue1234567890abc" {
		t.Errorf("expected config file secret, got AdminSecret=%q", c.AdminSecret)
	}
	if c.Mode != "production" {
		t.Errorf("expected mode=production, got %q", c.Mode)
	}
}

func TestLoad_AdminSecretHashFromPlaintext(t *testing.T) {
	t.Setenv("AA_ADMIN_SECRET", "my-plaintext-secret")
	t.Setenv("AA_CONFIG_PATH", "/nonexistent")

	c := Load()
	if c.AdminSecretHash == "" {
		t.Fatal("expected non-empty AdminSecretHash")
	}
	if !strings.HasPrefix(c.AdminSecretHash, "$2a$") && !strings.HasPrefix(c.AdminSecretHash, "$2b$") {
		t.Errorf("expected bcrypt hash prefix, got %q", c.AdminSecretHash)
	}
}

func TestLoad_AdminSecretHashPassthroughBcrypt(t *testing.T) {
	hash := "$2a$12$K4GByoBlaHblah.somethingabcdefghijklmnopqrst"
	t.Setenv("AA_ADMIN_SECRET", hash)
	t.Setenv("AA_CONFIG_PATH", "/nonexistent")

	c := Load()
	if c.AdminSecretHash != hash {
		t.Errorf("expected passthrough of bcrypt hash, got %q", c.AdminSecretHash)
	}
}

func TestLoad_ModeDefaultsDevelopment(t *testing.T) {
	t.Setenv("AA_ADMIN_SECRET", "test")
	t.Setenv("AA_CONFIG_PATH", "/nonexistent")

	c := Load()
	if c.Mode != "development" {
		t.Errorf("expected default mode=development, got %q", c.Mode)
	}
}
