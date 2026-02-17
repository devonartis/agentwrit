package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// brokerClient wraps all HTTP calls from the sidecar to the AgentAuth broker.
// It handles admin authentication, sidecar activation lifecycle, token
// exchange, token renewal, and health checks.
type brokerClient struct {
	baseURL string
	http    *http.Client
}

// activateResp holds the parsed response from POST /v1/sidecar/activate.
type activateResp struct {
	accessToken string
	expiresIn   int
	sidecarID   string
}

// exchangeResp holds the parsed response from POST /v1/token/exchange.
type exchangeResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	SidecarID   string `json:"sidecar_id"`
}

// newBrokerClient creates a broker HTTP client pointing at the given base URL.
// All requests use a 10-second timeout.
func newBrokerClient(baseURL string) *brokerClient {
	return &brokerClient{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// healthCheck performs GET /v1/health and returns nil when the broker
// responds with HTTP 200.
func (c *brokerClient) healthCheck() error {
	_, err := c.doJSON("GET", "/v1/health", nil, "")
	return err
}

// adminAuth authenticates with the broker using the admin shared secret
// and returns the admin JWT. Calls POST /v1/admin/auth.
func (c *brokerClient) adminAuth(secret string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"client_id":     "sidecar",
		"client_secret": secret,
	})
	if err != nil {
		return "", fmt.Errorf("marshal admin auth request: %w", err)
	}

	resp, err := c.doJSON("POST", "/v1/admin/auth", body, "")
	if err != nil {
		return "", fmt.Errorf("admin auth: %w", err)
	}

	token, ok := resp["access_token"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("admin auth: missing access_token in response")
	}
	return token, nil
}

// createSidecarActivation requests a sidecar activation token from the
// broker. Requires an admin Bearer token. Calls POST /v1/admin/sidecar-activations.
func (c *brokerClient) createSidecarActivation(adminToken string, scopes []string, ttl int) (string, error) {
	body, err := json.Marshal(map[string]any{
		"allowed_scopes": scopes,
		"ttl":            ttl,
	})
	if err != nil {
		return "", fmt.Errorf("marshal sidecar activation request: %w", err)
	}

	resp, err := c.doJSON("POST", "/v1/admin/sidecar-activations", body, adminToken)
	if err != nil {
		return "", fmt.Errorf("create sidecar activation: %w", err)
	}

	actToken, ok := resp["activation_token"].(string)
	if !ok || actToken == "" {
		return "", fmt.Errorf("create sidecar activation: missing activation_token in response")
	}
	return actToken, nil
}

// activateSidecar exchanges a single-use activation token for a sidecar
// bearer token. Calls POST /v1/sidecar/activate.
func (c *brokerClient) activateSidecar(activationToken string) (*activateResp, error) {
	body, err := json.Marshal(map[string]string{
		"sidecar_activation_token": activationToken,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal activate sidecar request: %w", err)
	}

	resp, err := c.doJSON("POST", "/v1/sidecar/activate", body, "")
	if err != nil {
		return nil, fmt.Errorf("activate sidecar: %w", err)
	}

	token, _ := resp["access_token"].(string)
	sidecarID, _ := resp["sidecar_id"].(string)

	// JSON numbers decode as float64 in map[string]any
	var expiresIn int
	if v, ok := resp["expires_in"].(float64); ok {
		expiresIn = int(v)
	}

	if token == "" {
		return nil, fmt.Errorf("activate sidecar: missing access_token in response")
	}

	return &activateResp{
		accessToken: token,
		expiresIn:   expiresIn,
		sidecarID:   sidecarID,
	}, nil
}

// tokenExchange requests a scoped agent token via the sidecar's bearer
// token. Calls POST /v1/token/exchange with Bearer auth.
func (c *brokerClient) tokenExchange(sidecarToken, agentID string, scope []string, ttl int) (*exchangeResp, error) {
	body, err := json.Marshal(map[string]any{
		"agent_id": agentID,
		"scope":    scope,
		"ttl":      ttl,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal token exchange request: %w", err)
	}

	resp, err := c.doJSON("POST", "/v1/token/exchange", body, sidecarToken)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	token, _ := resp["access_token"].(string)
	sidecarID, _ := resp["sidecar_id"].(string)

	var expiresIn int
	if v, ok := resp["expires_in"].(float64); ok {
		expiresIn = int(v)
	}

	if token == "" {
		return nil, fmt.Errorf("token exchange: missing access_token in response")
	}

	return &exchangeResp{
		AccessToken: token,
		ExpiresIn:   expiresIn,
		SidecarID:   sidecarID,
	}, nil
}

// renewResp holds the parsed response from POST /v1/token/renew.
type renewResp struct {
	AccessToken  string
	ExpiresIn    int
	ScopeCeiling []string // nil if not present in response
}

// tokenRenew renews the sidecar's own bearer token before it expires.
// Calls POST /v1/token/renew with Bearer auth. If the broker includes a
// scope_ceiling field in the response, it is returned so the sidecar can
// update its ceiling cache.
func (c *brokerClient) tokenRenew(token string) (*renewResp, error) {
	resp, err := c.doJSON("POST", "/v1/token/renew", nil, token)
	if err != nil {
		return nil, fmt.Errorf("token renew: %w", err)
	}

	newToken, _ := resp["access_token"].(string)

	var expiresIn int
	if v, ok := resp["expires_in"].(float64); ok {
		expiresIn = int(v)
	}

	if newToken == "" {
		return nil, fmt.Errorf("token renew: missing access_token in response")
	}

	var scopeCeiling []string
	if raw, ok := resp["scope_ceiling"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				scopeCeiling = append(scopeCeiling, s)
			}
		}
	}

	return &renewResp{
		AccessToken:  newToken,
		ExpiresIn:    expiresIn,
		ScopeCeiling: scopeCeiling,
	}, nil
}

// getChallenge fetches a nonce from the broker. Calls GET /v1/challenge.
func (c *brokerClient) getChallenge() (string, error) {
	resp, err := c.doJSON("GET", "/v1/challenge", nil, "")
	if err != nil {
		return "", fmt.Errorf("get challenge: %w", err)
	}
	nonce, ok := resp["nonce"].(string)
	if !ok || nonce == "" {
		return "", fmt.Errorf("get challenge: missing nonce in response")
	}
	return nonce, nil
}

// createLaunchToken creates a launch token via the admin API.
// Calls POST /v1/admin/launch-tokens with admin Bearer auth.
func (c *brokerClient) createLaunchToken(adminToken, agentName string, scope []string, ttl int) (string, error) {
	body, err := json.Marshal(map[string]any{
		"agent_name":    agentName,
		"allowed_scope": scope,
		"max_ttl":       ttl,
		"ttl":           ttl,
	})
	if err != nil {
		return "", fmt.Errorf("marshal launch token request: %w", err)
	}
	resp, err := c.doJSON("POST", "/v1/admin/launch-tokens", body, adminToken)
	if err != nil {
		return "", fmt.Errorf("create launch token: %w", err)
	}
	lt, ok := resp["launch_token"].(string)
	if !ok || lt == "" {
		return "", fmt.Errorf("create launch token: missing launch_token in response")
	}
	return lt, nil
}

// registerAgent registers an agent with the broker via challenge-response.
// Calls POST /v1/register.
func (c *brokerClient) registerAgent(launchToken, nonce, pubKeyB64, sigB64, orchID, taskID string, scope []string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"launch_token":    launchToken,
		"nonce":           nonce,
		"public_key":      pubKeyB64,
		"signature":       sigB64,
		"orch_id":         orchID,
		"task_id":         taskID,
		"requested_scope": scope,
	})
	if err != nil {
		return "", fmt.Errorf("marshal register request: %w", err)
	}
	resp, err := c.doJSON("POST", "/v1/register", body, "")
	if err != nil {
		return "", fmt.Errorf("register agent: %w", err)
	}
	agentID, ok := resp["agent_id"].(string)
	if !ok || agentID == "" {
		return "", fmt.Errorf("register agent: missing agent_id in response")
	}
	return agentID, nil
}

// doJSON is the shared HTTP helper for all broker calls. It builds the
// request, sets Content-Type and Authorization headers as needed, executes
// the call, and parses the JSON response body into a generic map.
//
// Any HTTP status >= 400 is treated as an error; the response body is
// included in the error message for diagnostic purposes.
func (c *brokerClient) doJSON(method, path string, body []byte, bearerToken string) (map[string]any, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("decode response JSON: %w", err)
		}
	}

	return result, nil
}
