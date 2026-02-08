package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/handler"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/mutauth"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

func main() {
	c := cfg.Load()
	obs.Configure(c.LogLevel)

	mux := http.NewServeMux()
	sqlStore := store.NewSqlStore()
	_, signingKey, err := identity.GenerateSigningKeyPair()
	if err != nil {
		obs.Fail("IDENTITY", "Broker.Main", "failed to generate signing key", "error="+err.Error())
		os.Exit(1)
	}
	idSvc := identity.NewIdSvc(sqlStore, signingKey, c.TrustDomain)
	tknSvc := token.NewTknSvc(signingKey, signingKey.Public().(ed25519.PublicKey), c)
	challengeHdl := handler.NewChallengeHdl(sqlStore)
	regHdl := handler.NewRegHdl(idSvc, tknSvc, c)
	valHdl := handler.NewValHdl(tknSvc)
	renewHdl := handler.NewRenewHdl(tknSvc)
	revSvc := revoke.NewRevSvc()
	revokeHdl := handler.NewRevokeHdl(revSvc)
	valMw := authz.NewValMw(tknSvc, revSvc)

	// M06: Mutual authentication components.
	discoveryReg := mutauth.NewDiscoveryRegistry()
	_ = mutauth.NewMutAuthHdl(tknSvc, sqlStore, discoveryReg)
	heartbeatMgr := mutauth.NewHeartbeatMgr(revSvc)

	mux.Handle("/v1/challenge", challengeHdl)
	mux.Handle("/v1/register", regHdl)
	mux.Handle("/v1/token/validate", valHdl)
	mux.Handle("/v1/token/renew", renewHdl)
	mux.Handle("/v1/revoke", revokeHdl)
	mux.Handle("/v1/protected/customers/12345", authz.WithRequiredScope("read:Customers:12345", valMw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"customer_id": "12345",
			"message":     "protected customer data",
		})
	}))))
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		obs.Ok("OBS", "HealthHdl.ServeHTTP", "health check", "status=healthy")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	// ADR-001: Seed tokens for dev/test bootstrap.
	// When AA_SEED_TOKENS=true, print a launch token and admin token to stdout
	// before starting the server so external test clients can use them.
	if c.SeedTokens {
		launchToken, ltErr := identity.CreateLaunchToken(
			sqlStore, "seed-orch", "seed-task",
			[]string{"read:Customers:*", "write:Customers:*"},
			5*time.Minute,
		)
		if ltErr != nil {
			obs.Fail("SEED", "Broker.Main", "failed to create seed launch token", "error="+ltErr.Error())
			os.Exit(1)
		}
		adminResp, adminErr := tknSvc.Issue(token.IssueReq{
			AgentID: "spiffe://" + c.TrustDomain + "/agent/seed-orch/seed-task/admin",
			OrchID:  "seed-orch",
			TaskID:  "seed-task",
			Scope:   []string{"admin:Broker:*"},
			TTLSecond: 600,
		})
		if adminErr != nil {
			obs.Fail("SEED", "Broker.Main", "failed to create seed admin token", "error="+adminErr.Error())
			os.Exit(1)
		}
		fmt.Println("SEED_LAUNCH_TOKEN=" + launchToken)
		fmt.Println("SEED_ADMIN_TOKEN=" + adminResp.AccessToken)
		obs.Ok("SEED", "Broker.Main", "seed tokens created")
	}

	// Start heartbeat monitor with graceful shutdown context.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	heartbeatMgr.StartMonitor(ctx, 30*time.Second)

	s := &http.Server{
		Addr:              ":" + c.Port,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	obs.Ok("OBS", "Broker.Main", "starting broker", "port="+c.Port, "log_level="+c.LogLevel)
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = s.Shutdown(shutdownCtx)
	}()

	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		obs.Fail("OBS", "Broker.Main", "server exited", "error="+err.Error())
		os.Exit(1)
	}
}
