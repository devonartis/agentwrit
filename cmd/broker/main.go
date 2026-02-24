// Command broker starts the AgentAuth broker HTTP server.
//
// It wires all internal services together, registers routes on an
// [http.ServeMux], and listens on the port configured by AA_PORT (default
// 8080). A fresh Ed25519 signing key pair is generated on every startup.
//
// Transport security is controlled by AA_TLS_MODE (default "none"):
//
//   - "none"  — plain HTTP (development and internal deployments)
//   - "tls"   — one-way TLS; requires AA_TLS_CERT and AA_TLS_KEY
//   - "mtls"  — mutual TLS; requires AA_TLS_CERT, AA_TLS_KEY, and AA_TLS_CLIENT_CA
//
// Route table (see also docs/api.md):
//
//	GET  /v1/challenge           – obtain a cryptographic nonce (public)
//	POST /v1/register            – agent registration (launch-token auth)
//	POST /v1/token/validate      – verify a token (public)
//	POST /v1/token/renew         – renew a token (Bearer auth)
//	POST /v1/token/exchange      – sidecar token exchange (Bearer auth + sidecar scope)
//	POST /v1/delegate            – scope-attenuated delegation (Bearer auth)
//	POST /v1/revoke              – revoke tokens (admin scope)
//	GET  /v1/audit/events        – query audit trail (admin scope)
//	GET  /v1/admin/sidecars       – list registered sidecars (admin scope)
//	POST /v1/admin/auth          – admin authentication (public)
//	POST /v1/admin/launch-tokens – create launch token (admin scope)
//	POST /v1/admin/sidecar-activations – create sidecar activation token (admin scope)
//	POST /v1/sidecar/activate    – exchange sidecar activation token (public, single-use)
//	GET  /v1/health              – health check (public)
//	GET  /v1/metrics             – Prometheus metrics (public)
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"

	"github.com/divineartis/agentauth/internal/admin"
	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/deleg"
	"github.com/divineartis/agentauth/internal/handler"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

const version = "2.0.0"

func main() {
	// Load configuration
	c := cfg.Load()
	obs.Configure(c.LogLevel)

	// P0: Fail fast if admin secret is not configured.
	if c.AdminSecret == "" {
		fmt.Fprintln(os.Stderr, "FATAL: AA_ADMIN_SECRET must be set (non-empty)")
		os.Exit(1)
	}

	// Generate broker signing key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: generate signing key: %v\n", err)
		os.Exit(1)
	}

	// Initialize SQLite
	sqlStore := store.NewSqlStore()
	if err := sqlStore.InitDB(c.DBPath); err != nil {
		obs.Fail("BROKER", "main", "database init failed", "error="+err.Error())
		fmt.Fprintf(os.Stderr, "FATAL: init database: %v\n", err)
		os.Exit(1)
	}
	obs.Ok("BROKER", "main", "database initialized", "path="+c.DBPath)

	// Load existing audit events from SQLite to rebuild hash chain
	existingEvents, err := sqlStore.LoadAllAuditEvents()
	if err != nil {
		obs.Fail("BROKER", "main", "audit event load failed", "error="+err.Error())
		fmt.Fprintf(os.Stderr, "FATAL: load audit events: %v\n", err)
		os.Exit(1)
	}
	obs.Ok("BROKER", "main", "audit events loaded", fmt.Sprintf("count=%d", len(existingEvents)))
	obs.AuditEventsLoaded.Set(float64(len(existingEvents)))

	// Load existing sidecars from SQLite to populate ceiling map
	sidecarCeilings, err := sqlStore.LoadAllSidecars()
	if err != nil {
		obs.Fail("BROKER", "main", "sidecar load failed", "error="+err.Error())
		fmt.Fprintf(os.Stderr, "FATAL: load sidecars: %v\n", err)
		os.Exit(1)
	}
	for id, ceiling := range sidecarCeilings {
		if err := sqlStore.SaveCeiling(id, ceiling); err != nil {
			obs.Fail("BROKER", "main", "failed to restore sidecar ceiling", "id="+id)
		}
	}
	obs.Ok("BROKER", "main", "sidecars loaded", fmt.Sprintf("count=%d", len(sidecarCeilings)))
	obs.SidecarsTotal.WithLabelValues("active").Set(float64(len(sidecarCeilings)))

	// Initialize audit log with persistence
	var auditLog *audit.AuditLog
	if len(existingEvents) > 0 {
		auditLog = audit.NewAuditLogWithEvents(sqlStore, existingEvents)
	} else {
		auditLog = audit.NewAuditLog(sqlStore)
	}
	tknSvc := token.NewTknSvc(privKey, pubKey, c)
	revSvc := revoke.NewRevSvc()
	idSvc := identity.NewIdSvc(sqlStore, tknSvc, c.TrustDomain, auditLog)
	delegSvc := deleg.NewDelegSvc(tknSvc, sqlStore, auditLog, privKey)
	adminSvc := admin.NewAdminSvc(c.AdminSecret, tknSvc, sqlStore, auditLog)

	// Seed tokens for development (AA_SEED_TOKENS=true)
	if c.SeedTokens {
		seedLaunch(adminSvc)
		seedAdmin(tknSvc)
	}

	// Middleware
	valMw := authz.NewValMw(tknSvc, revSvc, auditLog)

	// Handlers
	challengeHdl := handler.NewChallengeHdl(sqlStore)
	regHdl := handler.NewRegHdl(idSvc)
	valHdl := handler.NewValHdl(tknSvc, revSvc)
	renewHdl := handler.NewRenewHdl(tknSvc, auditLog, sqlStore)
	revokeHdl := handler.NewRevokeHdl(revSvc, auditLog)
	delegHdl := handler.NewDelegHdl(delegSvc)
	tokenExchangeHdl := handler.NewTokenExchangeHdl(tknSvc, sqlStore, auditLog)
	auditHdl := handler.NewAuditHdl(auditLog)
	healthHdl := handler.NewHealthHdl(version, auditLog, sqlStore)
	metricsHdl := handler.NewMetricsHdl()
	adminHdl := admin.NewAdminHdl(adminSvc, valMw, auditLog, revSvc)

	// Route table per Tech Spec Section 2
	mux := http.NewServeMux()

	// Public endpoints (no auth)
	mux.Handle("GET /v1/challenge", challengeHdl)
	mux.Handle("GET /v1/health", healthHdl)
	mux.Handle("GET /v1/metrics", metricsHdl)

	// Token validation (no auth required per spec)
	mux.Handle("POST /v1/token/validate", problemdetails.MaxBytesBody(valHdl))

	// Agent registration (launch token auth, not Bearer)
	mux.Handle("POST /v1/register", problemdetails.MaxBytesBody(regHdl))

	// Authenticated endpoints (Bearer token)
	mux.Handle("POST /v1/token/renew", problemdetails.MaxBytesBody(valMw.Wrap(renewHdl)))
	mux.Handle("POST /v1/token/exchange",
		problemdetails.MaxBytesBody(valMw.Wrap(valMw.RequireScope("sidecar:manage:*", tokenExchangeHdl))))
	mux.Handle("POST /v1/delegate", problemdetails.MaxBytesBody(valMw.Wrap(delegHdl)))

	// Admin endpoints (Bearer + admin scope)
	mux.Handle("POST /v1/revoke",
		problemdetails.MaxBytesBody(valMw.Wrap(valMw.RequireScope("admin:revoke:*", revokeHdl))))
	mux.Handle("GET /v1/audit/events",
		valMw.Wrap(valMw.RequireScope("admin:audit:*", auditHdl)))

	// Admin auth and launch token routes (registered by AdminHdl)
	adminHdl.RegisterRoutes(mux)

	// Global Middleware
	var rootHandler http.Handler = mux
	rootHandler = handler.LoggingMiddleware(rootHandler)
	rootHandler = problemdetails.RequestIDMiddleware(rootHandler)

	addr := ":" + c.Port
	obs.Ok("BROKER", "main", "starting broker", "addr="+addr, "version="+version)
	fmt.Printf("AgentAuth broker v%s listening on %s\n", version, addr)

	if err := serve(c, addr, rootHandler); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}

// seedLaunch creates a seed launch token with full wildcard scope and prints
// it to stdout. This is for development/testing only (AA_SEED_TOKENS=true).
func seedLaunch(adminSvc *admin.AdminSvc) {
	resp, err := adminSvc.CreateLaunchToken(admin.CreateLaunchTokenReq{
		AgentName:    "seed-agent",
		AllowedScope: []string{"*:*:*"},
		MaxTTL:       3600,
		TTL:          3600,
	}, "seed")
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: seed launch token: %v\n", err)
		return
	}
	fmt.Printf("SEED_LAUNCH_TOKEN=%s\n", resp.LaunchToken)
}

// seedAdmin issues a seed admin JWT and prints it to stdout. This is for
// development/testing only (AA_SEED_TOKENS=true).
func seedAdmin(tknSvc *token.TknSvc) {
	resp, err := tknSvc.Issue(token.IssueReq{
		Sub:   "admin",
		Scope: []string{"admin:launch-tokens:*", "admin:revoke:*", "admin:audit:*"},
		TTL:   3600,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: seed admin token: %v\n", err)
		return
	}
	fmt.Printf("SEED_ADMIN_TOKEN=%s\n", resp.AccessToken)
}
