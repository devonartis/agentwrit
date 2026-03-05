#!/usr/bin/env bash
# TD-006 test environment — source this once before running live test stories.
# Usage: source ./tests/td-006/env.sh

export BROKER_URL=http://127.0.0.1:8080
export AACTL=./bin/aactl
export AACTL_BROKER_URL=$BROKER_URL
export AACTL_ADMIN_SECRET=change-me-in-production

echo "TD-006 env loaded. Broker: $BROKER_URL"
