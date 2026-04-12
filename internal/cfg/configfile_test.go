package cfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
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
	hash := "$2a$12$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
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
	t.Setenv("HOME", t.TempDir()) // isolate from real ~/.broker/config

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

	err := WriteConfigFile(cfgPath, "development", "my-plaintext-secret", false)
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

	bcryptHash := "$2a$12$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
	err := WriteConfigFile(cfgPath, "production", bcryptHash, false)
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

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.AdminSecret != "" {
		t.Errorf("expected AdminSecret wiped after hashing, got %q", c.AdminSecret)
	}
	if c.AdminSecretHash == "" {
		t.Fatal("expected non-empty AdminSecretHash from env var")
	}
	// Verify the hash was derived from the env var value, not the config file value.
	if err := bcrypt.CompareHashAndPassword([]byte(c.AdminSecretHash), []byte("from-env-var")); err != nil {
		t.Errorf("AdminSecretHash does not match env var value: %v", err)
	}
}

func TestLoad_ConfigFileUsedWhenNoEnvVar(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	content := "MODE=production\nADMIN_SECRET=$2a$12$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AA_CONFIG_PATH", cfgPath)
	t.Setenv("AA_ADMIN_SECRET", "")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.AdminSecret != "" {
		t.Errorf("expected AdminSecret wiped after hashing, got %q", c.AdminSecret)
	}
	if c.AdminSecretHash != "$2a$12$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234" {
		t.Errorf("expected config file bcrypt hash in AdminSecretHash, got %q", c.AdminSecretHash)
	}
	if c.Mode != "production" {
		t.Errorf("expected mode=production, got %q", c.Mode)
	}
}

func TestLoad_AdminSecretHashFromPlaintext(t *testing.T) {
	t.Setenv("AA_ADMIN_SECRET", "my-plaintext-secret")
	t.Setenv("AA_CONFIG_PATH", "/nonexistent")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.AdminSecretHash == "" {
		t.Fatal("expected non-empty AdminSecretHash")
	}
	if !strings.HasPrefix(c.AdminSecretHash, "$2a$") && !strings.HasPrefix(c.AdminSecretHash, "$2b$") {
		t.Errorf("expected bcrypt hash prefix, got %q", c.AdminSecretHash)
	}
}

func TestLoad_AdminSecretHashPassthroughBcrypt(t *testing.T) {
	hash := "$2a$12$K4GByoBlahblah.somethingABCDEFGHIJKLMNOPQRSTUVWXYZ012"
	t.Setenv("AA_ADMIN_SECRET", hash)
	t.Setenv("AA_CONFIG_PATH", "/nonexistent")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.AdminSecretHash != hash {
		t.Errorf("expected passthrough of bcrypt hash, got %q", c.AdminSecretHash)
	}
}

func TestLoad_ModeDefaultsDevelopment(t *testing.T) {
	t.Setenv("AA_ADMIN_SECRET", "test")
	t.Setenv("AA_CONFIG_PATH", "/nonexistent")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != "development" {
		t.Errorf("expected default mode=development, got %q", c.Mode)
	}
}

func TestConfigLocations_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-config")
	if err := os.WriteFile(target, []byte("MODE=development\nADMIN_SECRET=test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "symlink-config")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AA_CONFIG_PATH", symlink)
	t.Setenv("HOME", t.TempDir()) // isolate from real ~/.broker/config

	_, _, path := loadConfigFile()
	if path != "" {
		t.Errorf("expected symlink config path to be rejected, got %q", path)
	}
}

func TestLoadConfigFileAt_RejectsInsecurePermissions(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	content := "MODE=development\nADMIN_SECRET=test-secret\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	mode, secret, path := loadConfigFileAt(cfgPath)
	if path != "" {
		t.Errorf("expected empty path for insecure file, got %q", path)
	}
	if mode != "" || secret != "" {
		t.Error("expected empty mode/secret for rejected insecure file")
	}
}

func TestLoad_RejectsKnownWeakAdminSecret(t *testing.T) {
	weak := []string{"change-me-in-production", ""}
	for _, secret := range weak {
		t.Run("rejects_"+secret, func(t *testing.T) {
			t.Setenv("AA_ADMIN_SECRET", secret)
			// Ensure no config file provides a secret (block all fallback paths).
			t.Setenv("AA_CONFIG_PATH", "/nonexistent")
			t.Setenv("HOME", t.TempDir())

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for weak secret %q, got nil", secret)
			}
			if !strings.Contains(err.Error(), "known-weak") {
				t.Errorf("expected 'known-weak' in error, got: %v", err)
			}
		})
	}
}

func TestIsBcryptHash_ValidHashes(t *testing.T) {
	valid := []string{
		"$2a$12$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234",
		"$2b$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234",
		"$2y$04$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234",
	}
	for _, h := range valid {
		if !isBcryptHash(h) {
			t.Errorf("expected isBcryptHash(%q) = true", h)
		}
	}
}

func TestIsBcryptHash_InvalidHashes(t *testing.T) {
	invalid := []string{
		"$2a$",                               // prefix only
		"$2a$12$short",                       // too short
		"plaintext-secret",                   // no prefix
		"",                                   // empty
		"$2a$12$" + strings.Repeat("a", 100), // too long
	}
	for _, h := range invalid {
		if isBcryptHash(h) {
			t.Errorf("expected isBcryptHash(%q) = false", h)
		}
	}
}
