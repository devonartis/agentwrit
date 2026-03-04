#!/usr/bin/env bash
# Phase 0 — Legacy Cleanup live test environment
#
# Source this once before running test stories:
#   source ./tests/phase-0/env.sh

export BROKER_URL=http://127.0.0.1:8080
export AACTL=./bin/aactl
export AACTL_BROKER_URL=$BROKER_URL

# AA_ADMIN_SECRET matches the default in docker-compose.yml.
export AACTL_ADMIN_SECRET=change-me-in-production
