#!/usr/bin/env bash
set -euo pipefail

echo "Checking for Dockerfile..."
if [ ! -f Dockerfile ]; then
    echo "FAIL: Dockerfile not found"
    exit 1
fi

echo "Checking for multi-stage build..."
if ! grep -q "AS builder" Dockerfile; then
    echo "FAIL: Dockerfile does not appear to be multi-stage (missing 'AS builder')"
    exit 1
fi

echo "Checking for lightweight base image in final stage..."
if ! grep -qE "FROM (alpine|scratch|distroless)" Dockerfile; then
    echo "FAIL: Final stage should use a lightweight base image (alpine, scratch, or distroless)"
    exit 1
fi

echo "PASS: Dockerfile exists and meets basic requirements"
