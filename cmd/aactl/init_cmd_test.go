package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestInitCmd_DevMode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	secret, err := runInit("dev", cfgPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}
	if len(secret) < 32 {
		t.Errorf("expected secret >= 32 chars, got %d", len(secret))
	}

	content, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "MODE=development") {
		t.Error("expected MODE=development in config")
	}
	if !strings.Contains(string(content), secret) {
		t.Error("expected plaintext secret in dev config")
	}
}

func TestInitCmd_ProdMode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	secret, err := runInit("prod", cfgPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}

	content, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "MODE=production") {
		t.Error("expected MODE=production in config")
	}
	if strings.Contains(string(content), secret) {
		t.Error("plaintext secret should NOT appear in prod config")
	}

	// Extract the hash and verify it matches the secret.
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ADMIN_SECRET=") {
			hash := strings.TrimPrefix(line, "ADMIN_SECRET=")
			if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)); err != nil {
				t.Errorf("bcrypt hash doesn't match secret: %v", err)
			}
		}
	}
}

func TestInitCmd_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	_, err := runInit("dev", cfgPath, false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = runInit("dev", cfgPath, false)
	if err == nil {
		t.Fatal("expected error on overwrite without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestInitCmd_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	secret1, err := runInit("dev", cfgPath, false)
	if err != nil {
		t.Fatal(err)
	}

	secret2, err := runInit("dev", cfgPath, true)
	if err != nil {
		t.Fatal(err)
	}

	if secret1 == secret2 {
		t.Error("expected different secret on re-init")
	}
}

func TestInitCmd_AtomicCreate_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target-file")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "config-symlink")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}

	_, err := runInit("dev", symlink, false)
	if err == nil {
		t.Fatal("expected error when path is a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("expected symlink error, got: %v", err)
	}

	// Verify target file was NOT overwritten.
	content, _ := os.ReadFile(target)
	if string(content) != "original" {
		t.Error("symlink target was overwritten — TOCTOU vulnerability")
	}
}

func TestInitCmd_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "subdir", "config")

	_, err := runInit("prod", cfgPath, false)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600, got %o", info.Mode().Perm())
	}

	dirInfo, err := os.Stat(filepath.Dir(cfgPath))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("expected dir 0700, got %o", dirInfo.Mode().Perm())
	}
}
