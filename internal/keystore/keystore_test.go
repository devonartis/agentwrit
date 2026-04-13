// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0

package keystore

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrGenerate_CreatesNewKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.key")

	pub, priv, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("public key size = %d, want %d", len(pub), ed25519.PublicKeySize)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Fatalf("private key size = %d, want %d", len(priv), ed25519.PrivateKeySize)
	}

	// File must exist
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("key file not created: %v", err)
	}
}

func TestLoadOrGenerate_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.key")

	_, _, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Fatalf("file permissions = %o, want 0600", perm)
	}
}

func TestLoadOrGenerate_LoadsExistingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.key")

	// Generate first
	pub1, priv1, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Load same file — must return identical keys
	pub2, priv2, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !pub1.Equal(pub2) {
		t.Fatal("public keys differ on reload")
	}
	if !priv1.Equal(priv2) {
		t.Fatal("private keys differ on reload")
	}
}

func TestLoadOrGenerate_CorruptFileErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.key")

	// Write garbage
	if err := os.WriteFile(path, []byte("not a pem file"), 0600); err != nil {
		t.Fatal(err)
	}

	_, _, err := LoadOrGenerate(path)
	if err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
}

func TestLoadOrGenerate_InvalidPEMBlockType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.key")

	// Write a valid PEM but wrong block type
	bad := "-----BEGIN RSA PRIVATE KEY-----\nMC4CAQ==\n-----END RSA PRIVATE KEY-----\n"
	if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
		t.Fatal(err)
	}

	_, _, err := LoadOrGenerate(path)
	if err == nil {
		t.Fatal("expected error for wrong PEM block type, got nil")
	}
}

func TestLoadOrGenerate_CreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "dir")
	path := filepath.Join(nested, "test.key")

	_, _, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("parent dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("parent path is not a directory")
	}
}

func TestLoadOrGenerate_SignAndVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.key")

	pub, priv, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := []byte("agent-auth-test-message")
	sig := ed25519.Sign(priv, msg)
	if !ed25519.Verify(pub, msg, sig) {
		t.Fatal("signature verification failed for generated key")
	}

	// Reload and verify again
	pub2, _, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !ed25519.Verify(pub2, msg, sig) {
		t.Fatal("signature verification failed after reload")
	}
}
