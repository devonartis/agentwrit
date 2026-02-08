//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/handler"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

func metricValue(metricsBody, metricPrefix string) (float64, bool) {
	lines := strings.Split(metricsBody, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metricPrefix+" ") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				return 0, false
			}
			v, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return 0, false
			}
			return v, true
		}
	}
	return 0, false
}

func TestObservabilityMetricsAndHealthIntegration(t *testing.T) {
	sqlStore := store.NewSqlStore()
	brokerPub, brokerPriv, _ := identity.GenerateSigningKeyPair()
	c := cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300}
	idSvc := identity.NewIdSvc(sqlStore, brokerPriv, c.TrustDomain)
	tknSvc := token.NewTknSvc(brokerPriv, brokerPub, c)

	mux := http.NewServeMux()
	mux.Handle("/v1/challenge", handler.NewChallengeHdl(sqlStore))
	mux.Handle("/v1/register", handler.NewRegHdl(idSvc, tknSvc, c))
	mux.Handle("/v1/token/validate", handler.NewValHdl(tknSvc))
	mux.Handle("/v1/health", handler.NewHealthHdl(sqlStore, nil, "0.1.0"))
	mux.Handle("/v1/metrics", handler.NewMetricsHdl())

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// registerAgent issues a token (drives issuance metric).
	_, accessToken, _ := registerAgent(t, srv, sqlStore, "orch-obs", "task-obs", []string{"read:Customers:12345"})

	validateBody, _ := json.Marshal(map[string]any{
		"token":          accessToken,
		"required_scope": "read:Customers:12345",
	})
	validateRes, err := http.Post(srv.URL+"/v1/token/validate", "application/json", bytes.NewReader(validateBody))
	if err != nil {
		t.Fatalf("validate request failed: %v", err)
	}
	defer validateRes.Body.Close()
	if validateRes.StatusCode != http.StatusOK {
		t.Fatalf("expected validate 200, got %d", validateRes.StatusCode)
	}

	healthRes, err := http.Get(srv.URL + "/v1/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer healthRes.Body.Close()
	if healthRes.StatusCode != http.StatusOK {
		t.Fatalf("expected health 200, got %d", healthRes.StatusCode)
	}
	var health map[string]any
	if err := json.NewDecoder(healthRes.Body).Decode(&health); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if health["status"] != "healthy" {
		t.Fatalf("expected health status healthy, got %v", health["status"])
	}
	components, ok := health["components"].(map[string]any)
	if !ok {
		t.Fatalf("expected components object in health response")
	}
	if components["sqlite"] != "healthy" {
		t.Fatalf("expected sqlite healthy, got %v", components["sqlite"])
	}

	metricsRes, err := http.Get(srv.URL + "/v1/metrics")
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	defer metricsRes.Body.Close()
	if metricsRes.StatusCode != http.StatusOK {
		t.Fatalf("expected metrics 200, got %d", metricsRes.StatusCode)
	}
	rawMetrics, err := io.ReadAll(metricsRes.Body)
	if err != nil {
		t.Fatalf("read metrics body: %v", err)
	}
	metricsBody := string(rawMetrics)

	issuanceCount, ok := metricValue(metricsBody, "aa_token_issuance_duration_ms_count")
	if !ok || issuanceCount <= 0 {
		t.Fatalf("expected issuance metric count > 0, got %v", issuanceCount)
	}
	allowCount, ok := metricValue(metricsBody, `aa_validation_decision_total{decision="allow"}`)
	if !ok || allowCount <= 0 {
		t.Fatalf("expected allow validation metric > 0, got %v", allowCount)
	}
}
