package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
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
	b, err := os.ReadFile(path)
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

