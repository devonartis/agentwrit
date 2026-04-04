// Command broker starts the AgentAuth broker HTTP server.
//
// It wires all internal services together, registers routes on an
// [http.ServeMux], and listens on the port configured by AA_PORT (default
// 8080). The Ed25519 signing key is loaded from disk (or generated on
// first startup) via AA_SIGNING_KEY_PATH.
//
// Transport security is controlled by AA_TLS_MODE (default "none"):
//
//   - "none"  — plain HTTP (development and internal deployments)
//   - "tls"   — one-way TLS; requires AA_TLS_CERT and AA_TLS_KEY
//   - "mtls"  — mutual TLS; requires AA_TLS_CERT, AA_TLS_KEY, and AA_TLS_CLIENT_CA
//
// Route table (see also docs/api.md):
//
//	GET    /v1/challenge               – obtain a cryptographic nonce (public)
//	POST   /v1/register                – agent registration (launch-token auth)
//	POST   /v1/token/validate          – verify a token (public)
//	POST   /v1/token/renew             – renew a token (Bearer auth)
//	POST   /v1/token/release           – agent self-revocation (Bearer auth)
//	POST   /v1/delegate                – scope-attenuated delegation (Bearer auth)
//	POST   /v1/revoke                  – revoke tokens (admin scope)
//	GET    /v1/audit/events            – query audit trail (admin scope)
//	POST   /v1/admin/auth              – admin authentication (public)
//	POST   /v1/admin/launch-tokens     – create launch token (admin scope)
//	POST   /v1/admin/apps              – register app (admin scope)
//	GET    /v1/admin/apps              – list apps (admin scope)
//	GET    /v1/admin/apps/{id}         – get app details (admin scope)
//	PUT    /v1/admin/apps/{id}         – update app scopes (admin scope)
//	DELETE /v1/admin/apps/{id}         – deregister app (admin scope)
//	POST   /v1/app/auth                – app authentication (public, rate-limited)
//	GET    /v1/health                  – health check (public)
//	GET    /v1/metrics                 – Prometheus metrics (public)
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/devonartis/agentauth/internal/admin"
	"github.com/devonartis/agentauth/internal/keystore"
	"github.com/devonartis/agentauth/internal/app"
	"github.com/devonartis/agentauth/internal/audit"
	"github.com/devonartis/agentauth/internal/authz"
	"github.com/devonartis/agentauth/internal/cfg"
	"github.com/devonartis/agentauth/internal/deleg"
	"github.com/devonartis/agentauth/internal/handler"
	"github.com/devonartis/agentauth/internal/identity"
	"github.com/devonartis/agentauth/internal/obs"
	"github.com/devonartis/agentauth/internal/problemdetails"
	"github.com/devonartis/agentauth/internal/revoke"
	"github.com/devonartis/agentauth/internal/store"
	"github.com/devonartis/agentauth/internal/token"
)

const version = "2.0.0"

func main() {
	// Load configuration
	c, err := cfg.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
	obs.Configure(c.LogLevel)

	// P1: Fail fast if admin secret is not configured.
	if c.AdminSecretHash == "" {
		fmt.Fprintln(os.Stderr, "FATAL: No admin secret configured. Run 'aactl init' or set the AA_ADMIN_SECRET environment variable.")
		os.Exit(1)
	}

	// Dev mode warning.
	if c.Mode == "development" {
		obs.Warn("BROKER", "main", "Running in development mode -- admin secret stored in plaintext")
	}

	// Load or generate persistent signing key
	pubKey, privKey, err := keystore.LoadOrGenerate(c.SigningKeyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: signing key: %v\n", err)
		os.Exit(1)
	}
	obs.Ok("BROKER", "main", "signing key loaded", "path="+c.SigningKeyPath)

	// Initialize SQLite
	sqlStore := store.NewSqlStore()
	if err := sqlStore.InitDB(c.DBPath); err != nil {
		obs.Fail("BROKER", "main", "database init failed", "error="+err.Error())
		fmt.Fprintf(os.Stderr, "FATAL: init database: %v\n", err)
		os.Exit(1)
	}
	obs.Ok("BROKER", "main", "database initialized", "path="+c.DBPath)
	defer func() {
		if sqlStore.HasDB() {
			sqlStore.Close()
		}
	}()

	// Load existing audit events from SQLite to rebuild hash chain
	existingEvents, err := sqlStore.LoadAllAuditEvents()
	if err != nil {
		obs.Fail("BROKER", "main", "audit event load failed", "error="+err.Error())
		fmt.Fprintf(os.Stderr, "FATAL: load audit events: %v\n", err)
		os.Exit(1)
	}
	obs.Ok("BROKER", "main", "audit events loaded", fmt.Sprintf("count=%d", len(existingEvents)))
	obs.AuditEventsLoaded.Set(float64(len(existingEvents)))

	// Load existing revocations from SQLite
	revEntries, err := sqlStore.LoadAllRevocations()
	if err != nil {
		obs.Fail("BROKER", "main", "revocation load failed", "error="+err.Error())
		fmt.Fprintf(os.Stderr, "FATAL: load revocations: %v\n", err)
		os.Exit(1)
	}

	// Initialize audit log with persistence
	var auditLog *audit.AuditLog
	if len(existingEvents) > 0 {
		auditLog = audit.NewAuditLogWithEvents(sqlStore, existingEvents)
	} else {
		auditLog = audit.NewAuditLog(sqlStore)
	}
	tknSvc := token.NewTknSvc(privKey, pubKey, c)
	revSvc := revoke.NewRevSvc(sqlStore)
	tknSvc.SetRevoker(revSvc)
	if len(revEntries) > 0 {
		typed := make([]struct{ Level, Target string }, len(revEntries))
		for i, e := range revEntries {
			typed[i] = struct{ Level, Target string }{e.Level, e.Target}
		}
		revSvc.LoadFromEntries(typed)
		obs.Ok("BROKER", "main", "revocations loaded", fmt.Sprintf("count=%d", len(revEntries)))
	}
	idSvc := identity.NewIdSvc(sqlStore, tknSvc, c.TrustDomain, auditLog, c.Audience)
	delegSvc := deleg.NewDelegSvc(tknSvc, sqlStore, auditLog, privKey)
	adminSvc := admin.NewAdminSvc(c.AdminSecretHash, tknSvc, sqlStore, auditLog, c.Audience)
	appSvc := app.NewAppSvc(sqlStore, tknSvc, auditLog, c.Audience, c.AppTokenTTL)

	// Seed tokens for development (AA_SEED_TOKENS=true)
	if c.SeedTokens {
		seedLaunch(adminSvc)
		seedAdmin(tknSvc)
	}

	// Middleware
	valMw := authz.NewValMw(tknSvc, revSvc, auditLog, c.Audience)

	// Handlers
	challengeHdl := handler.NewChallengeHdl(sqlStore)
	regHdl := handler.NewRegHdl(idSvc)
	valHdl := handler.NewValHdl(tknSvc, revSvc)
	renewHdl := handler.NewRenewHdl(tknSvc, auditLog)
	revokeHdl := handler.NewRevokeHdl(revSvc, auditLog)
	releaseHdl := handler.NewReleaseHdl(revSvc, auditLog)
	delegHdl := handler.NewDelegHdl(delegSvc)
	auditHdl := handler.NewAuditHdl(auditLog)
	healthHdl := handler.NewHealthHdl(version, auditLog, sqlStore)
	metricsHdl := handler.NewMetricsHdl()
	adminHdl := admin.NewAdminHdl(adminSvc, valMw, auditLog, revSvc, sqlStore)
	appHdl := app.NewAppHdl(appSvc, valMw)

	// Route table per Tech Spec Section 2
	mux := http.NewServeMux()

	// Public endpoints (no auth)
	mux.Handle("GET /v1/challenge", challengeHdl)
	mux.Handle("GET /v1/health", healthHdl)
	mux.Handle("GET /v1/metrics", metricsHdl)

	// Token validation (no auth required per spec)
	mux.Handle("POST /v1/token/validate", valHdl)

	// Agent registration (launch token auth, not Bearer)
	mux.Handle("POST /v1/register", regHdl)

	// Authenticated endpoints (Bearer token)
	mux.Handle("POST /v1/token/renew", valMw.Wrap(renewHdl))
	mux.Handle("POST /v1/delegate", valMw.Wrap(delegHdl))
	mux.Handle("POST /v1/token/release", valMw.Wrap(releaseHdl))

	// Admin endpoints (Bearer + admin scope)
	mux.Handle("POST /v1/revoke",
		valMw.Wrap(valMw.RequireScope("admin:revoke:*", revokeHdl)))
	mux.Handle("GET /v1/audit/events",
		valMw.Wrap(valMw.RequireScope("admin:audit:*", auditHdl)))

	// Admin auth and launch token routes (registered by AdminHdl)
	adminHdl.RegisterRoutes(mux)

	// App registration and auth routes (registered by AppHdl)
	appHdl.RegisterRoutes(mux)

	// Global Middleware
	var rootHandler http.Handler = mux
	rootHandler = handler.SecurityHeaders(c.TLSMode)(rootHandler)
	rootHandler = problemdetails.MaxBytesBody(rootHandler)
	rootHandler = handler.LoggingMiddleware(rootHandler)
	rootHandler = problemdetails.RequestIDMiddleware(rootHandler)

	// Background goroutines for token hygiene (60s intervals).
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if n := sqlStore.PruneExpiredJTIs(); n > 0 {
				obs.Ok("BROKER", "jti-pruner", "pruned expired JTIs", fmt.Sprintf("count=%d", n))
			}
			if n := sqlStore.ExpireAgents(); n > 0 {
				obs.Ok("BROKER", "agent-expiry", "expired agents", fmt.Sprintf("count=%d", n))
			}
		}
	}()

	addr := c.BindAddress + ":" + c.Port
	if c.BindAddress == "0.0.0.0" && c.TLSMode == "none" {
		obs.Warn("BROKER", "main", "binding to 0.0.0.0 without TLS — use AA_TLS_MODE=tls in production")
	}
	obs.Ok("BROKER", "main", "starting broker", "addr="+addr, "version="+version)
	fmt.Printf("AgentAuth broker v%s listening on %s\n", version, addr)

	if err := serve(c, addr, rootHandler, func() {
		if err := sqlStore.Close(); err != nil {
			obs.Warn("BROKER", "shutdown", "database close error", "error="+err.Error())
		}
		obs.Ok("BROKER", "shutdown", "database closed")
	}); err != nil {
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
	}, "seed", "")
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
