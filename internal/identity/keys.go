package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type jwkOkp struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
}

// GenerateSigningKeyPair creates a broker Ed25519 keypair.
func GenerateSigningKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}

// LoadSigningKey loads an Ed25519 private key from disk.
func LoadSigningKey(path string) (ed25519.PrivateKey, error) {
	clean := filepath.Clean(path)
	if clean == "" || clean == "." {
		return nil, fmt.Errorf("invalid key path")
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("signing key path must not be a symlink")
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("signing key path must point to a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("signing key file must not be readable by group/other")
	}
	// #nosec G304 -- path is validated for type/symlink/permissions before opening.
	f, err := os.Open(clean)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, ed25519.PrivateKeySize+1))
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid key length: %d", len(b))
	}
	return ed25519.PrivateKey(b), nil
}

// SaveSigningKey persists an Ed25519 private key with secure permissions.
func SaveSigningKey(key ed25519.PrivateKey, path string) error {
	if len(key) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid key length: %d", len(key))
	}
	return os.WriteFile(path, key, 0o600)
}

// ParseAgentPubKey parses a JWK-encoded Ed25519 public key.
func ParseAgentPubKey(jwk json.RawMessage) (ed25519.PublicKey, error) {
	var k jwkOkp
	if err := json.Unmarshal(jwk, &k); err != nil {
		return nil, err
	}
	if k.Kty != "OKP" {
		return nil, fmt.Errorf("invalid kty: %s", k.Kty)
	}
	if k.Crv != "Ed25519" {
		return nil, fmt.Errorf("invalid crv: %s", k.Crv)
	}
	raw, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid x key length: %d", len(raw))
	}
	return ed25519.PublicKey(raw), nil
}
