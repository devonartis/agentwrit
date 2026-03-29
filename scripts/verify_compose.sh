#!/usr/bin/env bash
set -euo pipefail

echo "Checking for docker-compose.yml..."
if [ ! -f docker-compose.yml ]; then
    echo "FAIL: docker-compose.yml not found"
    exit 1
fi

echo "Checking for broker service..."
if ! grep -q "broker:" docker-compose.yml; then
    echo "FAIL: docker-compose.yml missing 'broker' service"
    exit 1
fi

echo "Checking for AA_ADMIN_SECRET environment variable..."
if ! grep -q "AA_ADMIN_SECRET" docker-compose.yml; then
    echo "FAIL: broker service missing 'AA_ADMIN_SECRET' environment variable"
    exit 1
fi

echo "PASS: docker-compose.yml exists and contains required services and variables"
