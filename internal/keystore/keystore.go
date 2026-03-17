// Package keystore loads or generates an Ed25519 signing key pair,
// persisting it to disk in PEM-encoded PKCS8 format.
package keystore

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

const pemBlockType = "PRIVATE KEY"

// LoadOrGenerate loads an Ed25519 key pair from the PEM file at path.
// If the file does not exist, it generates a new key pair, creates
// parent directories with 0700, and writes the private key with 0600
// permissions using O_CREATE|O_WRONLY|O_EXCL to prevent overwriting.
//
// Returns an error if the file exists but is corrupt or unreadable.
func LoadOrGenerate(path string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return parseKey(data)
	}
	if !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("keystore: read %s: %w", path, err)
	}

	// Generate new key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("keystore: generate key: %w", err)
	}

	if err := writeKey(path, priv); err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}

func parseKey(data []byte) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, nil, fmt.Errorf("keystore: no PEM block found")
	}
	if block.Type != pemBlockType {
		return nil, nil, fmt.Errorf("keystore: unexpected PEM block type %q, want %q", block.Type, pemBlockType)
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("keystore: parse PKCS8: %w", err)
	}

	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("keystore: key is %T, want ed25519.PrivateKey", key)
	}
	return priv.Public().(ed25519.PublicKey), priv, nil
}

func writeKey(path string, priv ed25519.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("keystore: marshal PKCS8: %w", err)
	}

	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  pemBlockType,
		Bytes: der,
	})

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("keystore: create directory: %w", err)
	}

	// O_EXCL: fail if file already exists (race protection)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("keystore: create file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(pemData); err != nil {
		return fmt.Errorf("keystore: write key: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("keystore: sync key: %w", err)
	}
	return nil
}
