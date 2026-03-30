# App Launch Token Route Separation — Acceptance Test Evidence

**Date:** 2026-03-30
**Branch:** `fix/app-launch-tokens-endpoint`
**Cherry-pick:** `393d376` from agentauth-internal
**Mode:** VPS (compiled binary on host)

## Summary: 4/4 PASS

| Story | Description | Persona | Verdict |
|-------|------------|---------|---------|
| ALT-S1 | App creates launch token on app route | Developer | **PASS** |
| ALT-S2 | App blocked from admin launch token route | Security Reviewer | **PASS** |
| ALT-S3 | Admin creates launch token on admin route | Operator | **PASS** |
| ALT-S4 | Admin blocked from app launch token route | Security Reviewer | **PASS** |

## What Was Proven

The two launch token endpoints are fully separated:

- `POST /v1/admin/launch-tokens` — accepts only `admin:launch-tokens:*` scope (operator path)
- `POST /v1/app/launch-tokens` — accepts only `app:launch-tokens:*` scope (app path)

Cross-calling is blocked in both directions (403). Neither scope type can access the other's route.
