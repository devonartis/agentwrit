package main

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/handler"
	"github.com/divineartis/agentauth/internal/identity"
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

	s := &http.Server{
		Addr:              ":" + c.Port,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	obs.Ok("OBS", "Broker.Main", "starting broker", "port="+c.Port, "log_level="+c.LogLevel)
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		obs.Fail("OBS", "Broker.Main", "server exited", "error="+err.Error())
		os.Exit(1)
	}
}
