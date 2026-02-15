package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
)

// ---------------------------------------------------------------------------
// challengeProxyHandler — GET /v1/challenge
// ---------------------------------------------------------------------------

// challengeProxyHandler proxies the broker's challenge endpoint for BYOK
// developers who need a nonce to sign with their own Ed25519 key.
type challengeProxyHandler struct {
	broker *brokerClient
}

// newChallengeProxyHandler creates a challengeProxyHandler wired to the
// given broker client.
func newChallengeProxyHandler(bc *brokerClient) *challengeProxyHandler {
	return &challengeProxyHandler{broker: bc}
}

func (h *challengeProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET is allowed")
		return
	}
	nonce, err := h.broker.getChallenge()
	if err != nil {
		writeError(w, http.StatusBadGateway, "broker challenge unavailable: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"nonce":      nonce,
		"expires_in": 30,
	})
}

// ---------------------------------------------------------------------------
// registerHandler — POST /v1/register (BYOK)
// ---------------------------------------------------------------------------

// registerReq is the JSON body for POST /v1/register.
type registerReq struct {
	AgentName string `json:"agent_name"`
	TaskID    string `json:"task_id"`
	PublicKey string `json:"public_key"` // base64-encoded Ed25519 public key
	Signature string `json:"signature"`  // base64-encoded Ed25519 signature of nonce
	Nonce     string `json:"nonce"`      // hex nonce from GET /v1/challenge
}

// registerHandler serves POST /v1/register — the BYOK registration endpoint.
// Developers who manage their own Ed25519 keys use this to register with the
// broker, passing through their public key and a signed nonce. The handler
// validates key size, obtains a launch token via admin auth, and delegates
// the actual challenge-response to the broker. On success, it caches the
// result in the agent registry with nil privKey (marking the agent as BYOK).
type registerHandler struct {
	broker       *brokerClient
	registry     *agentRegistry
	adminSecret  string
	scopeCeiling []string
}

// newRegisterHandler creates a registerHandler wired to the given broker
// client, agent registry, admin secret, and scope ceiling.
func newRegisterHandler(bc *brokerClient, reg *agentRegistry, adminSecret string, ceiling []string) *registerHandler {
	return &registerHandler{
		broker:       bc,
		registry:     reg,
		adminSecret:  adminSecret,
		scopeCeiling: ceiling,
	}
}

func (h *registerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST is allowed")
		return
	}

	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	if req.AgentName == "" {
		writeError(w, http.StatusBadRequest, "agent_name is required")
		return
	}
	if req.PublicKey == "" || req.Signature == "" || req.Nonce == "" {
		writeError(w, http.StatusBadRequest, "public_key, signature, and nonce are required")
		return
	}

	// Validate public key is valid base64 and correct size.
	pubKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		writeError(w, http.StatusBadRequest, "invalid public key: must be 32-byte Ed25519 key, base64-encoded")
		return
	}

	agentKey := req.AgentName
	if req.TaskID != "" {
		agentKey = req.AgentName + ":" + req.TaskID
	}

	// Check if already registered.
	if entry, ok := h.registry.lookup(agentKey); ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"agent_id": entry.spiffeID,
			"cached":   true,
		})
		return
	}

	// Admin auth to get launch token.
	adminToken, err := h.broker.adminAuth(h.adminSecret)
	if err != nil {
		writeError(w, http.StatusBadGateway, "admin auth failed: "+err.Error())
		return
	}

	launchToken, err := h.broker.createLaunchToken(adminToken, req.AgentName, h.scopeCeiling, 600)
	if err != nil {
		writeError(w, http.StatusBadGateway, "create launch token failed: "+err.Error())
		return
	}

	// Register at broker — pass through developer's nonce, pubkey, signature.
	orchID := req.AgentName
	taskID := req.TaskID
	if taskID == "" {
		taskID = "default"
	}

	agentID, err := h.broker.registerAgent(launchToken, req.Nonce, req.PublicKey, req.Signature, orchID, taskID, h.scopeCeiling)
	if err != nil {
		writeError(w, http.StatusBadGateway, "broker registration failed: "+err.Error())
		return
	}

	// Cache in registry — BYOK so privKey is nil.
	h.registry.store(agentKey, &agentEntry{
		spiffeID:     agentID,
		pubKey:       ed25519.PublicKey(pubKeyBytes),
		privKey:      nil,
		registeredAt: time.Now(),
	})

	obs.Ok("SIDECAR", "REGISTRY", "BYOK agent registered", "agent="+req.AgentName, "agent_id="+agentID)
	SidecarAgentsRegistered.Inc()

	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id": agentID,
	})
}
