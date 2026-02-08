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

	pass("config", "broker_url="+baseURL)

	// Step 1: Health check.
	healthBody := httpGet(baseURL + "/v1/health")
	if !strings.Contains(healthBody, `"status":"healthy"`) {
		fail("health check: unexpected body: " + healthBody)
	}
	pass("health check")

	// Step 2: Get challenge nonce.
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

	// Step 3: Register agent with Ed25519 proof-of-possession.
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

	// Step 4: Validate token.
	valReq, _ := json.Marshal(map[string]any{
		"token":          accessToken,
		"required_scope": "read:Customers:12345",
	})
	valStatus, _ := httpPost(baseURL+"/v1/token/validate", valReq)
	if valStatus != 200 {
		fail(fmt.Sprintf("validate: expected 200, got %d", valStatus))
	}
	pass("token validated with matching scope")

	// Step 5: Access protected resource.
	protStatus, protBody := httpGetAuth(baseURL+"/v1/protected/customers/12345", accessToken)
	if protStatus != 200 {
		fail(fmt.Sprintf("protected: expected 200, got %d", protStatus))
	}
	if !strings.Contains(protBody, `"customer_id"`) {
		fail("protected: missing customer_id in body")
	}
	pass("protected resource accessed with valid token")

	// Step 6: No-auth access denied.
	denyStatus, _ := httpGetNoAuth(baseURL + "/v1/protected/customers/12345")
	if denyStatus != 401 {
		fail(fmt.Sprintf("protected no-auth: expected 401, got %d", denyStatus))
	}
	pass("protected resource denied without token")

	// Step 7: Renew token.
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

	// Step 8: Revoke the renewed token.
	// Extract JTI by decoding the payload.
	jti := extractJTI(renewedToken)
	revokeReq, _ := json.Marshal(map[string]string{
		"level":     "token",
		"target_id": jti,
		"reason":    "smoke test revocation",
	})
	revokeStatus, _ := httpPost(baseURL+"/v1/revoke", revokeReq)
	if revokeStatus != 200 {
		fail(fmt.Sprintf("revoke: expected 200, got %d", revokeStatus))
	}
	pass("token revoked", "jti="+jti)

	// Step 9: Verify revoked token is denied.
	revokedStatus, _ := httpGetAuth(baseURL+"/v1/protected/customers/12345", renewedToken)
	if revokedStatus != 401 {
		fail(fmt.Sprintf("revoked access: expected 401, got %d", revokedStatus))
	}
	pass("revoked token denied on protected resource")

	// Step 10: Reused launch token rejected.
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

	fmt.Println("[AA:SMOKE:PASS] all smoke tests passed")
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
	resp, err := http.Get(url)
	if err != nil {
		fail("GET " + url + ": " + err.Error())
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
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
	resp, err := http.Get(url)
	if err != nil {
		fail("GET " + url + ": " + err.Error())
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func httpPost(url string, body []byte) (int, string) {
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fail("POST " + url + ": " + err.Error())
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
