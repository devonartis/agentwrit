package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	baseURL := os.Getenv("AA_BROKER_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}
	launchToken := os.Getenv("SEED_LAUNCH_TOKEN")
	if launchToken == "" {
		fail("SEED_LAUNCH_TOKEN not set")
	}
	adminToken := os.Getenv("SEED_ADMIN_TOKEN")
	if adminToken == "" {
		fail("SEED_ADMIN_TOKEN not set")
	}

	pass("config", "broker_url="+baseURL)

	// Step 1: Health check.
	healthStatus, healthBody := httpGetNoAuth(baseURL + "/v1/health")
	if healthStatus != 200 {
		fail(fmt.Sprintf("health check: expected 200, got %d body=%s", healthStatus, healthBody))
	}
	if !strings.Contains(healthBody, `"status":"healthy"`) {
		fail("health check: unexpected body: " + healthBody)
	}
	pass("health check")

	// Step 2: Metrics endpoint is live.
	metricsStatus, metricsBody := httpGetNoAuth(baseURL + "/v1/metrics")
	if metricsStatus != 200 {
		fail(fmt.Sprintf("metrics: expected 200, got %d", metricsStatus))
	}
	if !strings.Contains(metricsBody, "aa_token_issuance_duration_ms") {
		fail("metrics: expected aa_token_issuance_duration_ms")
	}
	pass("metrics endpoint exposed")

	// Step 3: Get challenge nonce.
	challengeBody := httpGet(baseURL + "/v1/challenge")
	var ch map[string]string
	mustUnmarshal(challengeBody, &ch)
	nonce := ch["nonce"]
	if nonce == "" {
		fail("challenge: missing nonce")
	}
	if ch["expires_at"] == "" {
		fail("challenge: missing expires_at")
	}
	pass("challenge nonce received", "nonce_len="+itoa(len(nonce)))

	// Step 4: Register agent with Ed25519 proof-of-possession.
	agentPub, agentPriv, _ := ed25519.GenerateKey(nil)
	sig := ed25519.Sign(agentPriv, []byte(nonce))
	pubJWK, _ := json.Marshal(map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(agentPub),
	})

	regReq, _ := json.Marshal(map[string]any{
		"launch_token":     launchToken,
		"nonce":            nonce,
		"agent_public_key": json.RawMessage(pubJWK),
		"signature":        base64.RawURLEncoding.EncodeToString(sig),
		"orchestration_id": "seed-orch",
		"task_id":          "seed-task",
		"requested_scope":  []string{"read:Customers:12345"},
	})

	regStatus, regBody := httpPost(baseURL+"/v1/register", regReq)
	if regStatus != 201 {
		fail(fmt.Sprintf("register: expected 201, got %d body=%s", regStatus, regBody))
	}
	var reg map[string]any
	mustUnmarshal(regBody, &reg)
	agentID, _ := reg["agent_instance_id"].(string)
	accessToken, _ := reg["access_token"].(string)
	if agentID == "" || accessToken == "" {
		fail("register: missing agent_instance_id or access_token")
	}
	if !strings.HasPrefix(agentID, "spiffe://") {
		fail("register: agent_instance_id not a SPIFFE ID: " + agentID)
	}
	pass("agent registered", "agent_id="+agentID)

	// Step 5: Validate token.
	valReq, _ := json.Marshal(map[string]any{
		"token":          accessToken,
		"required_scope": "read:Customers:12345",
	})
	valStatus, _ := httpPost(baseURL+"/v1/token/validate", valReq)
	if valStatus != 200 {
		fail(fmt.Sprintf("validate: expected 200, got %d", valStatus))
	}
	pass("token validated with matching scope")

	// Step 6: Access protected resource.
	protStatus, protBody := httpGetAuth(baseURL+"/v1/protected/customers/12345", accessToken)
	if protStatus != 200 {
		fail(fmt.Sprintf("protected: expected 200, got %d", protStatus))
	}
	if !strings.Contains(protBody, `"customer_id"`) {
		fail("protected: missing customer_id in body")
	}
	pass("protected resource accessed with valid token")

	// Step 7: No-auth access denied.
	denyStatus, _ := httpGetNoAuth(baseURL + "/v1/protected/customers/12345")
	if denyStatus != 401 {
		fail(fmt.Sprintf("protected no-auth: expected 401, got %d", denyStatus))
	}
	pass("protected resource denied without token")

	// Step 8: Renew token.
	renewReq, _ := json.Marshal(map[string]any{
		"token": accessToken,
	})
	renewStatus, renewBody := httpPost(baseURL+"/v1/token/renew", renewReq)
	if renewStatus != 200 {
		fail(fmt.Sprintf("renew: expected 200, got %d", renewStatus))
	}
	var ren map[string]any
	mustUnmarshal(renewBody, &ren)
	renewedToken, _ := ren["access_token"].(string)
	if renewedToken == "" || renewedToken == accessToken {
		fail("renew: should return a different token")
	}
	pass("token renewed")

	// Step 9: Revoke without admin token should fail.
	jti := extractJTI(renewedToken)
	revokeReq, _ := json.Marshal(map[string]string{
		"level":     "token",
		"target_id": jti,
		"reason":    "smoke test revocation",
	})
	revokeNoAuthStatus, _ := httpPost(baseURL+"/v1/revoke", revokeReq)
	if revokeNoAuthStatus != 401 {
		fail(fmt.Sprintf("revoke without admin auth: expected 401, got %d", revokeNoAuthStatus))
	}
	pass("revoke denied without admin token")

	// Step 10: Revoke the renewed token with admin auth.
	// Extract JTI by decoding the payload.
	revokeStatus, _ := httpPostAuth(baseURL+"/v1/revoke", adminToken, revokeReq)
	if revokeStatus != 200 {
		fail(fmt.Sprintf("revoke: expected 200, got %d", revokeStatus))
	}
	pass("token revoked", "jti="+jti)

	// Step 11: Verify revoked token is denied.
	revokedStatus, _ := httpGetAuth(baseURL+"/v1/protected/customers/12345", renewedToken)
	if revokedStatus != 401 {
		fail(fmt.Sprintf("revoked access: expected 401, got %d", revokedStatus))
	}
	pass("revoked token denied on protected resource")

	// Step 12: Reused launch token rejected.
	regReq2, _ := json.Marshal(map[string]any{
		"launch_token":     launchToken,
		"nonce":            nonce,
		"agent_public_key": json.RawMessage(pubJWK),
		"signature":        base64.RawURLEncoding.EncodeToString(sig),
		"orchestration_id": "seed-orch",
		"task_id":          "seed-task",
		"requested_scope":  []string{"read:Customers:12345"},
	})
	reuse2Status, _ := httpPost(baseURL+"/v1/register", regReq2)
	if reuse2Status != 401 {
		fail(fmt.Sprintf("reused launch token: expected 401, got %d", reuse2Status))
	}
	pass("reused launch token rejected")

	// ── M07 Delegation Chain Tests ──────────────────────────────────

	// Step 13: Delegate scope from Agent A to Agent B.
	// Use the original accessToken (pre-revocation, still valid).
	delegReq, _ := json.Marshal(map[string]any{
		"delegator_token": accessToken,
		"target_agent_id": "spiffe://agentauth.local/agent/seed-orch/seed-task/agentB",
		"delegated_scope": []string{"read:Customers:12345"},
		"max_ttl":         60,
	})
	delegStatus, delegBody := httpPost(baseURL+"/v1/delegate", delegReq)
	if delegStatus != 201 {
		fail(fmt.Sprintf("delegate: expected 201, got %d body=%s", delegStatus, delegBody))
	}
	var delegResp map[string]any
	mustUnmarshal(delegBody, &delegResp)
	delegToken, _ := delegResp["delegation_token"].(string)
	chainHash, _ := delegResp["chain_hash"].(string)
	delegDepth, _ := delegResp["delegation_depth"].(float64)
	if delegToken == "" {
		fail("delegate: missing delegation_token")
	}
	if chainHash == "" {
		fail("delegate: missing chain_hash")
	}
	if int(delegDepth) != 1 {
		fail(fmt.Sprintf("delegate: expected depth=1, got %d", int(delegDepth)))
	}
	pass("delegation created", "depth=1", "chain_hash="+chainHash[:16]+"...")

	// Step 14: Scope escalation blocked.
	// Agent A (scope read:Customers:12345) tries to delegate read:Customers:* (broader).
	escalateReq, _ := json.Marshal(map[string]any{
		"delegator_token": accessToken,
		"target_agent_id": "spiffe://agentauth.local/agent/seed-orch/seed-task/agentC",
		"delegated_scope": []string{"read:Customers:*"},
		"max_ttl":         60,
	})
	escalateStatus, escalateBody := httpPost(baseURL+"/v1/delegate", escalateReq)
	if escalateStatus != 403 {
		fail(fmt.Sprintf("scope escalation: expected 403, got %d body=%s", escalateStatus, escalateBody))
	}
	var escalateResp map[string]any
	mustUnmarshal(escalateBody, &escalateResp)
	if escalateResp["type"] != "urn:agentauth:error:scope-escalation" {
		fail(fmt.Sprintf("scope escalation: expected scope-escalation error type, got %v", escalateResp["type"]))
	}
	pass("scope escalation blocked (403)")

	// Step 15: Validate delegation token is a valid JWT.
	valDelegReq, _ := json.Marshal(map[string]any{
		"token":          delegToken,
		"required_scope": "read:Customers:12345",
	})
	valDelegStatus, _ := httpPost(baseURL+"/v1/token/validate", valDelegReq)
	if valDelegStatus != 200 {
		fail(fmt.Sprintf("validate delegation token: expected 200, got %d", valDelegStatus))
	}
	pass("delegation token validated with correct scope")

	// ── M05 Audit Trail Test ─────────────────────────────────────

	// Step 16: Query audit trail (requires admin token).
	stepAuditTrail(baseURL, adminToken)

	fmt.Println("[AA:SMOKE:PASS] all smoke tests passed (core + delegation + audit)")
}

func stepAuditTrail(baseURL, adminToken string) {
	status, body := httpGetAuth(baseURL+"/v1/audit/events", adminToken)
	if status != 200 {
		fail(fmt.Sprintf("audit trail: expected 200, got %d body=%s", status, body))
	}
	var result map[string]any
	mustUnmarshal(body, &result)
	eventsRaw, ok := result["events"].([]any)
	if !ok || len(eventsRaw) == 0 {
		fail("audit trail: expected non-empty events array")
	}
	pass("audit trail queried", fmt.Sprintf("event_count=%d", len(eventsRaw)))

	// Verify at least one credential_issued event exists.
	found := false
	for _, e := range eventsRaw {
		evt, _ := e.(map[string]any)
		if evt["event_type"] == "credential_issued" {
			found = true
			break
		}
	}
	if !found {
		fail("audit trail: no credential_issued event found")
	}
	pass("audit trail contains credential_issued event")
}

func pass(msg string, ctx ...string) {
	line := "[AA:SMOKE:PASS] " + msg
	if len(ctx) > 0 {
		line += " | " + strings.Join(ctx, " ")
	}
	fmt.Println(line)
}

func fail(msg string) {
	fmt.Println("[AA:SMOKE:FAIL] " + msg)
	os.Exit(1)
}

func httpGet(url string) string {
	status, body := httpRequest(http.MethodGet, url, "", nil, "")
	if status != http.StatusOK {
		fail(fmt.Sprintf("GET %s: expected 200, got %d body=%s", url, status, body))
	}
	return body
}

func httpGetAuth(url, token string) (int, string) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("GET " + url + ": " + err.Error())
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func httpGetNoAuth(url string) (int, string) {
	return httpRequest(http.MethodGet, url, "", nil, "")
}

func httpPost(url string, body []byte) (int, string) {
	return httpRequest(http.MethodPost, url, "", bytes.NewReader(body), "application/json")
}

func httpPostAuth(url, token string, body []byte) (int, string) {
	return httpRequest(http.MethodPost, url, token, bytes.NewReader(body), "application/json")
}

func httpRequest(method, url, token string, body io.Reader, contentType string) (int, string) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		fail(method + " " + url + ": " + err.Error())
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail(method + " " + url + ": " + err.Error())
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func mustUnmarshal(data string, v any) {
	if err := json.Unmarshal([]byte(data), v); err != nil {
		fail("json unmarshal: " + err.Error() + " body=" + data)
	}
}

func extractJTI(tokenStr string) string {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) < 2 {
		fail("extractJTI: malformed token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		fail("extractJTI: base64 decode: " + err.Error())
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		fail("extractJTI: json unmarshal: " + err.Error())
	}
	jti, _ := claims["jti"].(string)
	if jti == "" {
		fail("extractJTI: missing jti claim")
	}
	return jti
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func init() {
	http.DefaultClient.Timeout = 10 * time.Second
}
