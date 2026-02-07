package identity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSigningKeyPair(t *testing.T) {
	pub, priv, err := GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("unexpected public key size: %d", len(pub))
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Fatalf("unexpected private key size: %d", len(priv))
	}
}

func TestSaveLoadSigningKeyRoundTrip(t *testing.T) {
	_, priv, err := GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "signing.key")
	if err := SaveSigningKey(priv, path); err != nil {
		t.Fatalf("save key: %v", err)
	}
	got, err := LoadSigningKey(path)
	if err != nil {
		t.Fatalf("load key: %v", err)
	}
	if string(got) != string(priv) {
		t.Fatalf("loaded key does not match original")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected file mode: %o", info.Mode().Perm())
	}
}

func TestParseAgentPubKey(t *testing.T) {
	pub, _, err := GenerateSigningKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}

	jwk := map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(pub),
	}
	raw, _ := json.Marshal(jwk)
	parsed, err := ParseAgentPubKey(raw)
	if err != nil {
		t.Fatalf("parse valid jwk: %v", err)
	}
	if string(parsed) != string(pub) {
		t.Fatalf("parsed pub key mismatch")
	}
}

func TestParseAgentPubKeyInvalid(t *testing.T) {
	tests := []struct {
		name string
		jwk  string
	}{
		{"bad json", `{"kty"`},
		{"wrong kty", `{"kty":"RSA","crv":"Ed25519","x":"abc"}`},
		{"wrong crv", `{"kty":"OKP","crv":"X25519","x":"abc"}`},
		{"bad x", `{"kty":"OKP","crv":"Ed25519","x":"@@@"}`},
		{"short x", `{"kty":"OKP","crv":"Ed25519","x":"AQID"}`},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseAgentPubKey([]byte(tc.jwk))
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

