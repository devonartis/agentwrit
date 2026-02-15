package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// tokenHandler — POST /v1/token
// ---------------------------------------------------------------------------

// tokenReq is the JSON body for POST /v1/token.
type tokenReq struct {
	AgentName string   `json:"agent_name"`
	TaskID    string   `json:"task_id"`
	Scope     []string `json:"scope"`
	TTL       int      `json:"ttl"`
}

// tokenHandler serves POST /v1/token — the developer-facing endpoint for
// requesting a scoped agent token. It validates the request, checks scope
// against the sidecar's configured ceiling, and delegates to the broker's
// token exchange endpoint.
type tokenHandler struct {
	broker       *brokerClient
	state        *sidecarState
	scopeCeiling []string
	registry     *agentRegistry
	adminSecret  string
}

// newTokenHandler creates a tokenHandler wired to the given broker client,
// sidecar state, scope ceiling, agent registry, and admin secret.
func newTokenHandler(bc *brokerClient, state *sidecarState, ceiling []string, reg *agentRegistry, adminSecret string) *tokenHandler {
	return &tokenHandler{
		broker:       bc,
		state:        state,
		scopeCeiling: ceiling,
		registry:     reg,
		adminSecret:  adminSecret,
	}
}

func (h *tokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST is allowed")
		return
	}

	var req tokenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	// Validate required fields.
	if len(req.Scope) == 0 {
		writeError(w, http.StatusBadRequest, "scope is required")
		return
	}
	if req.AgentName == "" {
		writeError(w, http.StatusBadRequest, "agent_name is required")
		return
	}

	// Check requested scope against ceiling.
	if !scopeIsSubset(req.Scope, h.scopeCeiling) {
		writeError(w, http.StatusForbidden, "requested scope exceeds sidecar ceiling")
		return
	}

	// Default TTL if not provided.
	ttl := req.TTL
	if ttl <= 0 {
		ttl = 300
	}

	// Resolve agent identity: check registry or lazy-register.
	agentKey := req.AgentName
	if req.TaskID != "" {
		agentKey = req.AgentName + ":" + req.TaskID
	}

	agentID, err := h.resolveAgent(agentKey, req.AgentName, req.TaskID, req.Scope)
	if err != nil {
		writeError(w, http.StatusBadGateway, "agent registration failed: "+err.Error())
		return
	}

	// Delegate to broker token exchange using the agent's SPIFFE ID.
	exResp, err := h.broker.tokenExchange(h.state.getToken(), agentID, req.Scope, ttl)
	if err != nil {
		writeError(w, http.StatusBadGateway, "broker token exchange failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": exResp.AccessToken,
		"expires_in":   exResp.ExpiresIn,
		"scope":        req.Scope,
		"agent_id":     agentID,
	})
}

// resolveAgent checks the registry for a cached agent entry. If found, it
// returns the SPIFFE ID immediately. Otherwise it acquires a per-agent lock
// and runs the full challenge-response registration flow against the broker.
func (h *tokenHandler) resolveAgent(key, agentName, taskID string, scope []string) (string, error) {
	entry, unlock := h.registry.getOrLock(key)
	if entry != nil {
		return entry.spiffeID, nil
	}
	defer unlock()

	spiffeID, pubKey, privKey, err := h.lazyRegister(agentName, taskID, scope)
	if err != nil {
		return "", err
	}

	h.registry.store(key, &agentEntry{
		spiffeID:     spiffeID,
		pubKey:       pubKey,
		privKey:      privKey,
		registeredAt: time.Now(),
	})
	return spiffeID, nil
}

// lazyRegister performs the full agent registration sequence: admin auth,
// launch token creation, challenge-response, and agent registration.
func (h *tokenHandler) lazyRegister(agentName, taskID string, scope []string) (string, ed25519.PublicKey, ed25519.PrivateKey, error) {
	// Step 1: Admin auth.
	adminToken, err := h.broker.adminAuth(h.adminSecret)
	if err != nil {
		return "", nil, nil, fmt.Errorf("admin auth: %w", err)
	}

	// Step 2: Create launch token.
	launchToken, err := h.broker.createLaunchToken(adminToken, agentName, scope, 600)
	if err != nil {
		return "", nil, nil, fmt.Errorf("create launch token: %w", err)
	}

	// Step 3: Get challenge nonce.
	nonce, err := h.broker.getChallenge()
	if err != nil {
		return "", nil, nil, fmt.Errorf("get challenge: %w", err)
	}

	// Step 4: Generate Ed25519 keypair and sign the nonce.
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, nil, fmt.Errorf("generate keypair: %w", err)
	}

	nonceBytes, err := hex.DecodeString(nonce)
	if err != nil {
		return "", nil, nil, fmt.Errorf("decode nonce: %w", err)
	}
	sig := ed25519.Sign(privKey, nonceBytes)

	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	// Step 5: Register at broker.
	orchID := agentName
	tid := taskID
	if tid == "" {
		tid = "default"
	}

	agentID, err := h.broker.registerAgent(launchToken, nonce, pubKeyB64, sigB64, orchID, tid, scope)
	if err != nil {
		return "", nil, nil, fmt.Errorf("register: %w", err)
	}

	fmt.Printf("[sidecar] lazy-registered agent %s → %s\n", agentName, agentID)
	return agentID, pubKey, privKey, nil
}

// ---------------------------------------------------------------------------
// renewHandler — POST /v1/token/renew
// ---------------------------------------------------------------------------

// renewHandler serves POST /v1/token/renew — the developer-facing endpoint
// for renewing an existing token. It reads the Bearer token from the
// Authorization header and delegates to the broker's renew endpoint.
type renewHandler struct {
	broker *brokerClient
}

// newRenewHandler creates a renewHandler wired to the given broker client.
func newRenewHandler(bc *brokerClient) *renewHandler {
	return &renewHandler{broker: bc}
}

func (h *renewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST is allowed")
		return
	}

	// Extract bearer token from Authorization header.
	token := extractBearer(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing or invalid Authorization bearer token")
		return
	}

	newToken, expiresIn, err := h.broker.tokenRenew(token)
	if err != nil {
		writeError(w, http.StatusBadGateway, "broker token renew failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": newToken,
		"expires_in":   expiresIn,
	})
}

// ---------------------------------------------------------------------------
// healthHandler — GET /v1/health
// ---------------------------------------------------------------------------

// healthHandler serves GET /v1/health — the developer-facing readiness
// endpoint. It reports sidecar status, broker connectivity, and the
// configured scope ceiling.
type healthHandler struct {
	state        *sidecarState
	scopeCeiling []string
}

// newHealthHandler creates a healthHandler with the given state and ceiling.
func newHealthHandler(state *sidecarState, ceiling []string) *healthHandler {
	return &healthHandler{
		state:        state,
		scopeCeiling: ceiling,
	}
}

func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET is allowed")
		return
	}

	// The sidecar is considered connected to the broker when it holds a
	// valid sidecar token (bootstrap succeeded).
	connected := h.state != nil && h.state.getToken() != ""

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"broker_connected": connected,
		"scope_ceiling":    h.scopeCeiling,
	})
}

// ---------------------------------------------------------------------------
// scopeIsSubset — scope ceiling enforcement
// ---------------------------------------------------------------------------

// scopeIsSubset returns true when every scope in requested is covered by at
// least one scope in allowed. Scope format is "action:resource:identifier".
// A wildcard "*" in the identifier position of an allowed scope covers any
// specific identifier in a requested scope.
func scopeIsSubset(requested, allowed []string) bool {
	for _, req := range requested {
		if !scopeCovers(req, allowed) {
			return false
		}
	}
	return true
}

// scopeCovers returns true when at least one scope in allowed covers the
// given scope string.
func scopeCovers(scope string, allowed []string) bool {
	for _, a := range allowed {
		if scopeMatches(scope, a) {
			return true
		}
	}
	return false
}

// scopeMatches returns true when the ceiling scope covers the requested
// scope. Both are in "action:resource:identifier" format. The ceiling may
// use "*" as a wildcard identifier.
func scopeMatches(requested, ceiling string) bool {
	rParts := strings.SplitN(requested, ":", 3)
	cParts := strings.SplitN(ceiling, ":", 3)

	if len(rParts) != 3 || len(cParts) != 3 {
		// Malformed scopes do not match.
		return false
	}

	// Action and resource must match exactly.
	if rParts[0] != cParts[0] || rParts[1] != cParts[1] {
		return false
	}

	// Identifier: wildcard in ceiling covers any requested identifier.
	if cParts[2] == "*" {
		return true
	}

	// Otherwise exact match required.
	return rParts[2] == cParts[2]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeError writes a JSON error response with the given HTTP status code.
func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]any{
		"error":  http.StatusText(status),
		"detail": detail,
	})
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// extractBearer parses a "Bearer <token>" value from the Authorization
// header. Returns the token string or empty string if not present.
func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimPrefix(auth, prefix)
}
