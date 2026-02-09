//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/handler"
	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

func TestAuditTrailIntegration(t *testing.T) {
	sqlStore := store.NewSqlStore()
	brokerPub, brokerPriv, _ := identity.GenerateSigningKeyPair()
	c := cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300}
	idSvc := identity.NewIdSvc(sqlStore, brokerPriv, c.TrustDomain)
	tknSvc := token.NewTknSvc(brokerPriv, brokerPub, c)
	auditLog := audit.NewAuditLog()
	revSvc := revoke.NewRevSvc()
	valMw := authz.NewValMw(tknSvc, revSvc, auditLog)
	auditHdl := handler.NewAuditHdl(auditLog)

	mux := http.NewServeMux()
	mux.Handle("/v1/challenge", handler.NewChallengeHdl(sqlStore))
	mux.Handle("/v1/register", handler.NewRegHdl(idSvc, tknSvc, c, auditLog))
	mux.Handle("/v1/token/validate", handler.NewValHdl(tknSvc))
	mux.Handle("/v1/revoke", authz.WithRequiredScope("admin:Broker:*", valMw.Wrap(handler.NewRevokeHdl(revSvc, auditLog))))
	mux.Handle("/v1/audit/events", authz.WithRequiredScope("admin:Broker:*", valMw.Wrap(auditHdl)))
	mux.Handle("/v1/protected/customers/12345", authz.WithRequiredScope("read:Customers:12345", valMw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"customer_id":"12345"}`))
	}))))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Step 1: Register an agent (triggers credential_issued audit event).
	_, accessToken, _ := registerAgent(t, srv, sqlStore, "orch-audit", "task-audit", []string{"read:Customers:12345"})

	// Step 2: Access a protected route (triggers access_granted audit event).
	protReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/protected/customers/12345", nil)
	protReq.Header.Set("Authorization", "Bearer "+accessToken)
	protRes, err := http.DefaultClient.Do(protReq)
	if err != nil {
		t.Fatalf("protected route request failed: %v", err)
	}
	defer protRes.Body.Close()
	if protRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on protected route, got %d", protRes.StatusCode)
	}

	// Step 3: Query audit events with admin token.
	adminResp, err := tknSvc.Issue(token.IssueReq{
		AgentID:   "spiffe://agentauth.local/agent/orch-audit/task-audit/admin",
		OrchID:    "orch-audit",
		TaskID:    "task-audit",
		Scope:     []string{"admin:Broker:*"},
		TTLSecond: 300,
	})
	if err != nil {
		t.Fatalf("issue admin token: %v", err)
	}

	auditReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/audit/events", nil)
	auditReq.Header.Set("Authorization", "Bearer "+adminResp.AccessToken)
	auditRes, err := http.DefaultClient.Do(auditReq)
	if err != nil {
		t.Fatalf("audit query failed: %v", err)
	}
	defer auditRes.Body.Close()
	if auditRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from audit, got %d", auditRes.StatusCode)
	}

	var result struct {
		Events     []audit.AuditEvt `json:"events"`
		Total      int              `json:"total"`
		NextOffset int              `json:"next_offset"`
	}
	if err := json.NewDecoder(auditRes.Body).Decode(&result); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}

	// We expect at least: credential_issued + access_granted (protected) + access_granted (audit query itself).
	if result.Total < 2 {
		t.Fatalf("expected at least 2 audit events, got %d", result.Total)
	}

	// Verify credential_issued event is present.
	foundCredIssued := false
	foundAccessGranted := false
	for _, evt := range result.Events {
		if evt.EventType == audit.EvtCredentialIssued {
			foundCredIssued = true
		}
		if evt.EventType == audit.EvtAccessGranted {
			foundAccessGranted = true
		}
	}
	if !foundCredIssued {
		t.Fatal("expected credential_issued audit event")
	}
	if !foundAccessGranted {
		t.Fatal("expected access_granted audit event")
	}

	// Verify chain integrity.
	ok, idx := audit.VerifyChain(result.Events)
	if !ok {
		t.Fatalf("audit chain invalid at index %d", idx)
	}
}
