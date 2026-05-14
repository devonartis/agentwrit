// Package main provides an Ed25519 keypair/signature helper for the
// sec-l2b integration script. Called by tests/sec-l2b/integration.sh
// during the challenge-response registration step. Stdlib only —
// matches the project rule that all crypto is Go stdlib.
//
// Usage:
//   ./edsign <hex-encoded-nonce>
//
// Output (JSON on stdout):
//   {"public_key":"<base64>","signature":"<base64>"}
//
// Exit codes: 0 on success, 2 on argument error, 3 on crypto/encoding error.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: edsign <hex-nonce>")
		os.Exit(2)
	}
	nonce, err := hex.DecodeString(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad hex nonce: %v\n", err)
		os.Exit(3)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keygen failed: %v\n", err)
		os.Exit(3)
	}
	sig := ed25519.Sign(priv, nonce)
	out := struct {
		PublicKey string `json:"public_key"`
		Signature string `json:"signature"`
	}{
		PublicKey: base64.StdEncoding.EncodeToString(pub),
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "encode failed: %v\n", err)
		os.Exit(3)
	}
}
