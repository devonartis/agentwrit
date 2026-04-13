// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"
)

func TestLoadCA_MissingFile(t *testing.T) {
	_, err := loadCA("/nonexistent/ca.pem")
	if err == nil {
		t.Fatal("expected error for missing CA file, got nil")
	}
}

func TestLoadCA_InvalidPEM(t *testing.T) {
	f := t.TempDir() + "/ca.pem"
	if err := os.WriteFile(f, []byte("this is not valid PEM"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := loadCA(f)
	if err == nil {
		t.Fatal("expected error for invalid PEM, got nil")
	}
}

func TestLoadCA_ValidPEM(t *testing.T) {
	// Generate a real self-signed CA cert for the test.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	f := t.TempDir() + "/ca.pem"
	if err := os.WriteFile(f, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	pool, err := loadCA(f)
	if err != nil {
		t.Fatalf("unexpected error for valid PEM: %v", err)
	}
	if pool == nil {
		t.Fatal("expected non-nil cert pool")
	}
}
