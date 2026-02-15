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
func (c *brokerClient) createSidecarActivation(adminToken, scopePrefix string, ttl int) (string, error) {
	body, err := json.Marshal(map[string]any{
		"allowed_scope_prefix": scopePrefix,
		"ttl":                  ttl,
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

// tokenRenew renews the sidecar's own bearer token before it expires.
// Calls POST /v1/token/renew with Bearer auth.
func (c *brokerClient) tokenRenew(token string) (string, int, error) {
	resp, err := c.doJSON("POST", "/v1/token/renew", nil, token)
	if err != nil {
		return "", 0, fmt.Errorf("token renew: %w", err)
	}

	newToken, _ := resp["access_token"].(string)

	var expiresIn int
	if v, ok := resp["expires_in"].(float64); ok {
		expiresIn = int(v)
	}

	if newToken == "" {
		return "", 0, fmt.Errorf("token renew: missing access_token in response")
	}

	return newToken, expiresIn, nil
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
