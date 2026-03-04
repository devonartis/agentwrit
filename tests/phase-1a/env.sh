#!/usr/bin/env bash
# Phase 1a test environment — source this once before running live test stories.
# Usage: source ./tests/phase-1a-env.sh
#
# This sets the two variables aactl needs to connect and authenticate.
# AA_ADMIN_SECRET matches the default in docker-compose.yml.
# Change AA_ADMIN_SECRET if you started the stack with a custom secret.

export AACTL_BROKER_URL=http://127.0.0.1:8080
export AACTL_ADMIN_SECRET=change-me-in-production

echo "Phase 1a env loaded. Broker: $AACTL_BROKER_URL"
