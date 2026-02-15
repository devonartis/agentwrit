// Command smoketest runs a live E2E smoke test against a running AgentAuth broker.
//
// It exercises the full sidecar bootstrap lifecycle:
//
//  1. GET  /v1/health         – readiness check
//  2. GET  /v1/metrics        – Prometheus endpoint
//  3. POST /v1/admin/auth     – obtain admin token
//  4. POST /v1/admin/launch-tokens        – create launch token
//  5. GET  /v1/challenge      – obtain nonce
//  6. POST /v1/register       – register agent (Ed25519 challenge-response)
//  7. POST /v1/admin/sidecar-activations  – create sidecar activation token
//  8. POST /v1/sidecar/activate           – activate sidecar (single-use)
//  9. POST /v1/sidecar/activate           – replay denied
//  10. POST /v1/token/exchange            – exchange for agent token (happy path)
//  11. POST /v1/token/exchange            – scope escalation denied
//  12. POST /v1/token/validate            – validate exchanged token
//
// Usage:
//
//	go run ./cmd/smoketest [base-url] [admin-secret]
//
// Defaults: http://127.0.0.1:8080, "live-test-secret-32bytes-long!!"
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	baseURL     = "http://127.0.0.1:8080"
	adminSecret = "live-test-secret-32bytes-long!!"
	pass, fail  int
)

func main() {
	if len(os.Args) > 1 {
		baseURL = os.Args[1]
	}
	if len(os.Args) > 2 {
		adminSecret = os.Args[2]
	}

	client := &http.Client{Timeout: 10 * time.Second}

	fmt.Println("=== AgentAuth Sidecar Smoke Test ===")
	fmt.Printf("  broker: %s\n\n", baseURL)

	// Step 1: Health
	checkGET(client, "Step 1: GET /v1/health", "/v1/health", 200)

	// Step 2: Metrics
	checkGET(client, "Step 2: GET /v1/metrics", "/v1/metrics", 200)

	// Step 3: Admin auth
	adminToken := ""
	{
		body := jsonEnc(map[string]string{
			"client_id":     "admin",
			"client_secret": adminSecret,
		})
		resp := doJSON(client, "Step 3: POST /v1/admin/auth", "POST", "/v1/admin/auth", body, nil, 200)
		if t, ok := resp["access_token"].(string); ok && t != "" {
			adminToken = t
		} else {
			reportFail("Step 3: no access_token in response")
		}
	}
	if adminToken == "" {
		summarize()
		return
	}

	// Step 4: Create launch token
	launchToken := ""
	{
		body := jsonEnc(map[string]any{
			"agent_name":    "smoke-agent",
			"allowed_scope": []string{"read:data:*"},
			"max_ttl":       600,
			"ttl":           600,
		})
		resp := doJSON(client, "Step 4: POST /v1/admin/launch-tokens", "POST", "/v1/admin/launch-tokens", body, &adminToken, 201)
		if t, ok := resp["launch_token"].(string); ok && t != "" {
			launchToken = t
		} else {
			reportFail("Step 4: no launch_token in response")
		}
	}
	if launchToken == "" {
		summarize()
		return
	}

	// Step 5: Get challenge nonce
	nonce := ""
	{
		resp := doJSON(client, "Step 5: GET /v1/challenge", "GET", "/v1/challenge", nil, nil, 200)
		if n, ok := resp["nonce"].(string); ok && n != "" {
			nonce = n
		} else {
			reportFail("Step 5: no nonce in response")
		}
	}
	if nonce == "" {
		summarize()
		return
	}

	// Step 6: Register agent with Ed25519 signature
	agentToken := ""
	agentID := ""
	{
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			reportFail(fmt.Sprintf("Step 6: keygen: %v", err))
			summarize()
			return
		}
		nonceBytes, err := hex.DecodeString(nonce)
		if err != nil {
			reportFail(fmt.Sprintf("Step 6: nonce decode: %v", err))
			summarize()
			return
		}
		sig := ed25519.Sign(priv, nonceBytes)

		body := jsonEnc(map[string]any{
			"launch_token":    launchToken,
			"nonce":           nonce,
			"public_key":      base64.StdEncoding.EncodeToString(pub),
			"signature":       base64.StdEncoding.EncodeToString(sig),
			"orch_id":         "smoke-orch",
			"task_id":         "smoke-task",
			"requested_scope": []string{"read:data:*"},
		})
		resp := doJSON(client, "Step 6: POST /v1/register", "POST", "/v1/register", body, nil, 200)
		if t, ok := resp["access_token"].(string); ok && t != "" {
			agentToken = t
		}
		if id, ok := resp["agent_id"].(string); ok && id != "" {
			agentID = id
		}
		if agentToken == "" || agentID == "" {
			reportFail("Step 6: missing access_token or agent_id")
		}
		_ = agentToken // Used for future steps if needed
	}
	if agentID == "" {
		summarize()
		return
	}

	// Step 7: Create sidecar activation token
	activationToken := ""
	{
		body := jsonEnc(map[string]any{
			"allowed_scopes": []string{"read:data:*"},
			"ttl":            120,
		})
		resp := doJSON(client, "Step 7: POST /v1/admin/sidecar-activations", "POST", "/v1/admin/sidecar-activations", body, &adminToken, 201)
		if t, ok := resp["activation_token"].(string); ok && t != "" {
			activationToken = t
		} else {
			reportFail("Step 7: no activation_token in response")
		}
	}
	if activationToken == "" {
		summarize()
		return
	}

	// Step 8: Activate sidecar (single-use)
	sidecarToken := ""
	{
		body := jsonEnc(map[string]string{
			"sidecar_activation_token": activationToken,
		})
		resp := doJSON(client, "Step 8: POST /v1/sidecar/activate", "POST", "/v1/sidecar/activate", body, nil, 200)
		if t, ok := resp["access_token"].(string); ok && t != "" {
			sidecarToken = t
		} else {
			reportFail("Step 8: no access_token in response")
		}
	}
	if sidecarToken == "" {
		summarize()
		return
	}

	// Step 9: Replay denial
	{
		body := jsonEnc(map[string]string{
			"sidecar_activation_token": activationToken,
		})
		resp := doJSON(client, "Step 9: POST /v1/sidecar/activate (replay)", "POST", "/v1/sidecar/activate", body, nil, 401)
		if ec, ok := resp["error_code"].(string); ok && ec == "activation_token_replayed" {
			// good, error code verified
		} else {
			reportFail(fmt.Sprintf("Step 9: expected error_code=activation_token_replayed, got %v", resp["error_code"]))
		}
	}

	// Step 10: Token exchange (happy path)
	exchangedToken := ""
	{
		body := jsonEnc(map[string]any{
			"agent_id": agentID,
			"scope":    []string{"read:data:*"},
			"ttl":      300,
		})
		resp := doJSON(client, "Step 10: POST /v1/token/exchange", "POST", "/v1/token/exchange", body, &sidecarToken, 200)
		if t, ok := resp["access_token"].(string); ok && t != "" {
			exchangedToken = t
		} else {
			reportFail("Step 10: no access_token in response")
		}
		// Verify sidecar_id is present
		if sid, ok := resp["sidecar_id"].(string); !ok || sid == "" {
			reportFail("Step 10: missing sidecar_id in exchange response")
		}
	}

	// Step 11: Scope escalation denied
	{
		body := jsonEnc(map[string]any{
			"agent_id": agentID,
			"scope":    []string{"write:data:*"},
			"ttl":      300,
		})
		resp := doJSON(client, "Step 11: POST /v1/token/exchange (escalation)", "POST", "/v1/token/exchange", body, &sidecarToken, 403)
		if ec, ok := resp["error_code"].(string); ok && ec == "scope_escalation_denied" {
			// good
		} else {
			reportFail(fmt.Sprintf("Step 11: expected error_code=scope_escalation_denied, got %v", resp["error_code"]))
		}
	}

	// Step 12: Validate exchanged token
	if exchangedToken != "" {
		body := jsonEnc(map[string]string{
			"token": exchangedToken,
		})
		resp := doJSON(client, "Step 12: POST /v1/token/validate", "POST", "/v1/token/validate", body, nil, 200)
		if valid, ok := resp["valid"].(bool); ok && valid {
			// good
		} else {
			reportFail(fmt.Sprintf("Step 12: expected valid=true, got %v", resp["valid"]))
		}
	}

	summarize()
}

func checkGET(client *http.Client, name, path string, expected int) {
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		reportFail(fmt.Sprintf("%s: %v", name, err))
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		reportFail(fmt.Sprintf("%s: %v", name, err))
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == expected {
		reportPass(name)
	} else {
		reportFail(fmt.Sprintf("%s: expected %d, got %d", name, expected, resp.StatusCode))
	}
}

func doJSON(client *http.Client, name, method, path string, body []byte, bearerToken *string, expected int) map[string]any {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		reportFail(fmt.Sprintf("%s: %v", name, err))
		return nil
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearerToken != nil {
		req.Header.Set("Authorization", "Bearer "+*bearerToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		reportFail(fmt.Sprintf("%s: %v", name, err))
		return nil
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != expected {
		reportFail(fmt.Sprintf("%s: expected %d, got %d: %s", name, expected, resp.StatusCode, truncate(string(respBody), 200)))
		return nil
	}
	reportPass(name)

	var result map[string]any
	ct := resp.Header.Get("Content-Type")
	if len(respBody) > 0 && (strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "application/problem+json")) {
		_ = json.Unmarshal(respBody, &result)
	}
	return result
}

func jsonEnc(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func reportPass(name string) {
	pass++
	fmt.Printf("  PASS: %s\n", name)
}

func reportFail(msg string) {
	fail++
	fmt.Printf("  FAIL: %s\n", msg)
}

func summarize() {
	fmt.Println()
	fmt.Println("=== Smoke Test Summary ===")
	fmt.Printf("  PASS: %d\n", pass)
	fmt.Printf("  FAIL: %d\n", fail)
	if fail > 0 {
		fmt.Println("RESULT: FAILED")
		os.Exit(1)
	}
	fmt.Println("RESULT: PASSED")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
