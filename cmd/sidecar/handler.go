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

	"github.com/divineartis/agentauth/internal/obs"
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
	cb           *circuitBreaker
}

// newTokenHandler creates a tokenHandler wired to the given broker client,
// sidecar state, scope ceiling, agent registry, admin secret, and circuit breaker.
func newTokenHandler(bc *brokerClient, state *sidecarState, ceiling []string, reg *agentRegistry, adminSecret string, cb *circuitBreaker) *tokenHandler {
	return &tokenHandler{
		broker:       bc,
		state:        state,
		scopeCeiling: ceiling,
		registry:     reg,
		adminSecret:  adminSecret,
		cb:           cb,
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
		RecordScopeDenial()
		obs.Warn("SIDECAR", "TOKEN", "scope ceiling exceeded",
			"event_type=scope_ceiling_exceeded",
			"agent_name="+req.AgentName,
			"task_id="+req.TaskID,
			"requested="+strings.Join(req.Scope, ","),
			"ceiling="+strings.Join(h.scopeCeiling, ","))
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

	// Check circuit breaker before calling broker.
	if h.cb != nil && !h.cb.Allow() {
		// Circuit is open — try to serve cached token.
		if token, remaining, ok := h.registry.cachedToken(agentKey, req.Scope); ok {
			SidecarCachedTokensServedTotal.Inc()
			obs.Warn("SIDECAR", "TOKEN", "serving cached token (circuit open)", "agent="+agentKey)
			w.Header().Set("X-AgentAuth-Cached", "true")
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token": token,
				"expires_in":   remaining,
				"scope":        req.Scope,
				"agent_id":     agentID,
			})
			return
		}
		writeError(w, http.StatusServiceUnavailable, "broker unavailable and no cached token")
		return
	}

	// Delegate to broker token exchange using the agent's SPIFFE ID.
	exResp, err := h.broker.tokenExchange(h.state.getToken(), agentID, req.Scope, ttl)
	if err != nil {
		RecordExchange("failure")
		if h.cb != nil {
			h.cb.RecordFailure()
		}
		writeError(w, http.StatusBadGateway, "broker token exchange failed: "+err.Error())
		return
	}

	RecordExchange("success")
	if h.cb != nil {
		h.cb.RecordSuccess()
	}

	// Cache the token for failsafe fallback.
	h.registry.cacheToken(agentKey, exResp.AccessToken, req.Scope, exResp.ExpiresIn)

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
	SidecarAgentsRegistered.Inc()
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

	obs.Ok("SIDECAR", "REGISTRY", "agent registered", "agent="+agentName, "agent_id="+agentID)
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
	registry     *agentRegistry
}

// newHealthHandler creates a healthHandler with the given state, ceiling, and registry.
func newHealthHandler(state *sidecarState, ceiling []string, registry *agentRegistry) *healthHandler {
	return &healthHandler{
		state:        state,
		scopeCeiling: ceiling,
		registry:     registry,
	}
}

func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET is allowed")
		return
	}

	if h.state == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":  "bootstrapping",
			"healthy": false,
		})
		return
	}

	healthy := h.state.isHealthy()
	connected := h.state.getToken() != ""

	status := "ok"
	httpStatus := http.StatusOK
	if !healthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	resp := map[string]any{
		"status":           status,
		"broker_connected": connected,
		"healthy":          healthy,
		"scope_ceiling":    h.scopeCeiling,
	}

	if h.registry != nil {
		resp["agents_registered"] = h.registry.count()
	}

	if h.state != nil {
		if lr := h.state.getLastRenewal(); !lr.IsZero() {
			resp["last_renewal"] = lr.Format(time.RFC3339)
		}
		if st := h.state.getStartTime(); !st.IsZero() {
			resp["uptime_seconds"] = time.Since(st).Seconds()
		}
	}

	writeJSON(w, httpStatus, resp)
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
