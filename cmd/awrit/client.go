package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// defaultHTTPClient returns the shared HTTP client for all awrit requests.
func defaultHTTPClient() *http.Client {
	return &http.Client{}
}

// client holds the broker base URL, admin secret, cached auth token, and the
// underlying HTTP client used for all requests.
type client struct {
	baseURL string
	secret  string
	token   string
	http    *http.Client
}

// newClient constructs a client from the AACTL_BROKER_URL and AACTL_ADMIN_SECRET
// environment variables. Returns an error if either variable is unset.
func newClient() (*client, error) {
	url := os.Getenv("AACTL_BROKER_URL")
	if url == "" {
		return nil, fmt.Errorf("AACTL_BROKER_URL is not set")
	}
	secret := os.Getenv("AACTL_ADMIN_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("AACTL_ADMIN_SECRET is not set")
	}
	return &client{
		baseURL: url,
		secret:  secret,
		http:    &http.Client{},
	}, nil
}

// authenticate exchanges the admin secret for a short-lived JWT and caches it
// for the session. It is a no-op if a token is already cached.
func (c *client) authenticate() error {
	if c.token != "" {
		return nil
	}
	body, err := json.Marshal(map[string]string{
		"secret": c.secret,
	})
	if err != nil {
		return fmt.Errorf("marshal auth request: %w", err)
	}
	resp, err := c.http.Post(c.baseURL+"/v1/admin/auth", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("auth failed (HTTP %d): <unreadable body: %w>", resp.StatusCode, readErr)
		}
		return fmt.Errorf("auth failed (HTTP %d): %s", resp.StatusCode, string(b))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode auth response: %w", err)
	}
	c.token = result.AccessToken
	return nil
}

// doGet performs an authenticated GET request to the given path and returns the
// response body. Returns an error for HTTP 4xx/5xx responses.
func (c *client) doGet(path string) ([]byte, error) {
	if err := c.authenticate(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return b, nil
}

// doPost performs an authenticated POST request to the given path with the
// JSON-encoded payload and returns the response body. Returns an error for
// HTTP 4xx/5xx responses.
func (c *client) doPost(path string, payload any) ([]byte, error) {
	if err := c.authenticate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return b, nil
}

// doPostWithToken performs a POST request using a caller-supplied Bearer token
// instead of the admin auth flow. It returns the HTTP status code and response
// body. This is used for agent-facing endpoints like /v1/token/release where
// the caller's own token is the credential being acted on.
func (c *client) doPostWithToken(path, bearerToken string) (int, []byte, error) {
	req, err := http.NewRequest("POST", c.baseURL+path, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response body: %w", err)
	}
	return resp.StatusCode, b, nil
}

// doDelete performs an authenticated DELETE request to the given path and
// returns the response body. Returns an error for HTTP 4xx/5xx responses.
func (c *client) doDelete(path string) ([]byte, error) {
	if err := c.authenticate(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("DELETE", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return b, nil
}

// doPut performs an authenticated PUT request to the given path with the
// JSON-encoded payload and returns the response body. Returns an error for
// HTTP 4xx/5xx responses.
func (c *client) doPut(path string, payload any) ([]byte, error) {
	if err := c.authenticate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequest("PUT", c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return b, nil
}
