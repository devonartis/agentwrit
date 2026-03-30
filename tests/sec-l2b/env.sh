#!/usr/bin/env bash
# SEC-L2b acceptance test environment
# Source this once before running stories:  source tests/sec-l2b/env.sh

export AA_ADMIN_SECRET="live-test-secret-32bytes-long-ok"
export AA_DB_PATH="/tmp/aa-sec-l2b/agentauth.db"
export AA_SIGNING_KEY_PATH="/tmp/aa-sec-l2b/signing.key"
export AA_BIND_ADDRESS="127.0.0.1"
export BROKER_URL="http://127.0.0.1:8080"
export EVIDENCE_DIR="tests/sec-l2b/evidence"
