#!/usr/bin/env bash
set -euo pipefail

# gen_test_certs.sh — generates CA, broker, and sidecar certs for TLS/mTLS testing.
# Outputs to $1 (default: /tmp/agentauth-certs).
# All certs are self-signed, valid for 1 day, suitable only for testing.

CERT_DIR="${1:-/tmp/agentauth-certs}"
mkdir -p "$CERT_DIR"

echo "=== Generating test certs in $CERT_DIR ==="

# CA key + cert
openssl ecparam -genkey -name prime256v1 -noout -out "$CERT_DIR/ca-key.pem" 2>/dev/null
openssl req -new -x509 -sha256 -key "$CERT_DIR/ca-key.pem" \
  -out "$CERT_DIR/ca.pem" -days 1 \
  -subj "/CN=agentauth-test-ca" 2>/dev/null

# Broker server cert (SAN: broker, localhost)
openssl ecparam -genkey -name prime256v1 -noout -out "$CERT_DIR/broker-key.pem" 2>/dev/null
openssl req -new -key "$CERT_DIR/broker-key.pem" \
  -out "$CERT_DIR/broker.csr" \
  -subj "/CN=broker" 2>/dev/null

cat > "$CERT_DIR/broker-ext.cnf" <<EXTEOF
[v3_req]
subjectAltName = DNS:broker,DNS:localhost,IP:127.0.0.1
EXTEOF

openssl x509 -req -sha256 -in "$CERT_DIR/broker.csr" \
  -CA "$CERT_DIR/ca.pem" -CAkey "$CERT_DIR/ca-key.pem" -CAcreateserial \
  -out "$CERT_DIR/broker.pem" -days 1 \
  -extfile "$CERT_DIR/broker-ext.cnf" -extensions v3_req 2>/dev/null

# Sidecar client cert (for mTLS)
openssl ecparam -genkey -name prime256v1 -noout -out "$CERT_DIR/sidecar-key.pem" 2>/dev/null
openssl req -new -key "$CERT_DIR/sidecar-key.pem" \
  -out "$CERT_DIR/sidecar.csr" \
  -subj "/CN=sidecar" 2>/dev/null
openssl x509 -req -sha256 -in "$CERT_DIR/sidecar.csr" \
  -CA "$CERT_DIR/ca.pem" -CAkey "$CERT_DIR/ca-key.pem" -CAcreateserial \
  -out "$CERT_DIR/sidecar.pem" -days 1 2>/dev/null

# Cleanup CSRs
rm -f "$CERT_DIR"/*.csr "$CERT_DIR"/*.cnf "$CERT_DIR"/*.srl

echo "=== Certs generated ==="
ls -la "$CERT_DIR"/*.pem
