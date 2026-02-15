# Sidecar Live Test Script Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Automated Docker-based live test that exercises all 5 sidecar endpoints against a real broker+sidecar stack.

**Architecture:** Bash script using curl from host → sidecar (`:8081`) → broker (`:8080`) via Docker Compose networking. BYOK signing uses inline Python for Ed25519.

---

## What This Tests

| Step | Endpoint | Validates |
|------|----------|-----------|
| 1 | `GET /v1/health` | Sidecar bootstrapped, broker connected, healthy=true |
| 2 | `POST /v1/token` (new agent) | Lazy registration end-to-end |
| 3 | `POST /v1/token` (same agent) | Registry cache hit (same agent_id) |
| 4 | `POST /v1/token` (bad scope) | Scope ceiling enforcement (403) |
| 5 | `POST /v1/token/renew` | Token renewal via sidecar proxy |
| 6 | `GET /v1/challenge` | Challenge proxy returns hex nonce |
| 7 | `POST /v1/register` (BYOK) | BYOK registration with Ed25519 signature |
| 8 | `POST /v1/token` (BYOK agent) | Token exchange for BYOK-registered agent |
| 9 | Broker `/v1/token/validate` | Broker confirms sidecar-issued token is valid |

## Script Location

`scripts/live_test_sidecar.sh`

## Dependencies

- Docker + Docker Compose
- curl
- Python 3 (for Ed25519 signing in BYOK step)
- jq (optional, fall back to sed/grep if unavailable)

## Lifecycle

1. Pick free ports for broker + sidecar
2. `docker compose up -d --build`
3. Poll sidecar `/v1/health` until ready
4. Run 9 test steps sequentially
5. Print PASS/FAIL summary
6. `docker compose down` on exit (trap)
