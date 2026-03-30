// Package handler provides HTTP handlers for all AgentAuth broker endpoints.
//
// Each handler type wraps a domain service (e.g. IdSvc, TknSvc, DelegSvc)
// and translates between HTTP request/response semantics and service method
// calls. Handlers are thin: they parse the request, call the service, and
// format the response. Business logic lives in the service packages.
//
// Error responses use RFC 7807 "application/problem+json" via the
// problemdetails package. All handlers share the same middleware pipeline
// defined in cmd/broker/main.go: RequestID → Logging → MaxBytesBody →
// (optional) ValMw → (optional) RequireScope → Handler.
//
// Handler types and their endpoints:
//
//   - ChallengeHdl:      GET  /v1/challenge
//   - RegHdl:            POST /v1/register
//   - ValHdl:            POST /v1/token/validate
//   - RenewHdl:          POST /v1/token/renew
//   - ReleaseHdl:        POST /v1/token/release
//   - DelegHdl:          POST /v1/delegate
//   - RevokeHdl:         POST /v1/revoke
//   - AuditHdl:          GET  /v1/audit/events
//   - HealthHdl:         GET  /v1/health
//   - MetricsHdl:        GET  /v1/metrics
//   - HandshakeHdl:      Mutual auth (Go API only, not HTTP-exposed)
package handler
