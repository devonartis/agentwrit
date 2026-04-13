// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0

// Package handler provides the HTTP layer for the broker. Handlers are thin —
// they parse requests, call a domain service, and format responses. All
// business logic lives in the service packages (token, admin, app, identity,
// deleg, revoke). Errors go out as RFC 9457 problem+json via problemdetails.
//
// The middleware pipeline (wired in cmd/broker/main.go) runs on every request:
// RequestID → Logging → MaxBytesBody → SecurityHeaders → Handler.
// Auth-protected routes add: ValMw (Bearer verification) → RequireScope.
//
// Endpoints by audience:
//
//	Public (no auth):
//	- ChallengeHdl:      GET  /v1/challenge          — nonce for agent registration
//	- RegHdl:            POST /v1/register            — agent gets first credential
//	- ValHdl:            POST /v1/token/validate      — apps verify agent tokens
//	- HealthHdl:         GET  /v1/health              — liveness + readiness
//	- MetricsHdl:        GET  /v1/metrics             — Prometheus scrape
//
//	Agent (Bearer auth):
//	- RenewHdl:          POST /v1/token/renew         — extend session
//	- ReleaseHdl:        POST /v1/token/release       — self-revoke when done
//	- DelegHdl:          POST /v1/delegate            — create sub-token for another agent
//
//	Admin (Bearer + admin:* scope):
//	- RevokeHdl:         POST /v1/revoke              — kill switch (4 levels)
//	- AuditHdl:          GET  /v1/audit/events        — query tamper-evident trail
package handler
