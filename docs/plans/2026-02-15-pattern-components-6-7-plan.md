# Pattern Components 6+7 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire Mutual Auth (Component 6) and Delegation Chain (Component 7) through broker HTTP endpoints and sidecar proxy endpoints, making all 7 Ephemeral Agent Credentialing pattern components exercisable end-to-end.

**Architecture:** Refactor the existing `MutAuthHdl` service to accept pre-signed nonces (never transmit private keys). Create 3 broker HTTP handlers wrapping the service. Add 4 sidecar proxy endpoints (3 handshake + 1 delegation) with auto-signing for managed agents and pass-through for BYOK. Extend Docker E2E to prove all 7 pattern components.

**Tech Stack:** Go 1.24, stdlib `net/http`, Ed25519, existing `internal/` packages

**Source of truth:** `plans/archive/Security-Pattern-That-Is-Why-We-Built-AgentAuth.md` — Components 6 (lines 429-448) and 7 (lines 450-553)

---

### Task 1: Refactor RespondToHandshake — accept pre-signed nonce

The current `RespondToHandshake` takes `ed25519.PrivateKey` and signs internally. Private keys must never cross HTTP. Refactor to accept `signedNonce []byte` and add defense-in-depth signature verification.

**Files:**
- Modify: `internal/mutauth/mut_auth_hdl.go:100-161`
- Modify: `internal/mutauth/mut_auth_hdl_test.go`

**Step 1: Change the method signature**

In `internal/mutauth/mut_auth_hdl.go`, change `RespondToHandshake`:

```go
// Before:
func (h *MutAuthHdl) RespondToHandshake(req *HandshakeReq, responderToken string, responderKey ed25519.PrivateKey) (*HandshakeResp, error) {

// After:
func (h *MutAuthHdl) RespondToHandshake(req *HandshakeReq, responderToken string, signedNonce []byte) (*HandshakeResp, error) {
```

Replace the internal signing line:
```go
// Before:
signed := ed25519.Sign(responderKey, []byte(req.Nonce))

// After — use the pre-signed nonce AND verify it against the registered public key:
rec, err := h.store.GetAgent(respClaims.Sub)
if err != nil {
    obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "responder not registered for key lookup", "agent_id="+respClaims.Sub)
    return nil, ErrHandshakeUnknownAgent
}
if !ed25519.Verify(ed25519.PublicKey(rec.PublicKey), []byte(req.Nonce), signedNonce) {
    obs.Fail("MUTAUTH", "MutAuthHdl.Respond", "nonce signature invalid", "responder="+respClaims.Sub)
    return nil, ErrHandshakeNonceMismatch
}
```

Note: the existing `GetAgent(respClaims.Sub)` call on line 125 already fetches the agent for registration check. Remove that earlier call and consolidate — fetch once, use for both registration check AND key lookup.

Update the return to use the pre-signed nonce:
```go
return &HandshakeResp{
    ResponderToken: responderToken,
    ResponderID:    respClaims.Sub,
    SignedNonce:    signedNonce,  // was: signed
    Nonce:          counterNonce,
}, nil
```

**Step 2: Update all tests to sign before calling**

In `internal/mutauth/mut_auth_hdl_test.go`, every test that calls `RespondToHandshake` currently passes `privB` (or `privC`, `wrongKey`). Change each to sign the nonce first, then pass `signedNonce []byte`.

Pattern for each test:
```go
// Before:
resp, err := hdl.RespondToHandshake(req, tokB, privB)

// After:
signedNonce := ed25519.Sign(privB, []byte(req.Nonce))
resp, err := hdl.RespondToHandshake(req, tokB, signedNonce)
```

For `TestHandshakeWrongSigningKey`, sign with the wrong key:
```go
_, wrongKey, _ := ed25519.GenerateKey(nil)
badSig := ed25519.Sign(wrongKey, []byte(req.Nonce))
resp, err := hdl.RespondToHandshake(req, tokB, badSig)
// Now this should fail at step 2 (ErrHandshakeNonceMismatch) instead of step 3
if !errors.Is(err, ErrHandshakeNonceMismatch) {
    t.Fatalf("expected ErrHandshakeNonceMismatch at respond step, got %v", err)
}
```

Note: `TestHandshakeWrongSigningKey` changes behavior — the bad signature is now caught in step 2 (respond) instead of step 3 (complete). This is correct — defense in depth. Remove the `CompleteHandshake` call from this test since it never reaches step 3.

Tests affected (6 callers of `RespondToHandshake`):
1. `TestHandshakeSuccess` (line 89) — sign with `privB`
2. `TestHandshakeWrongSigningKey` (line 160) — sign with `wrongKey`, expect fail at respond
3. `TestHandshakeInvalidResponderToken` (line 180) — sign with `privB` (still fails on bad token first)
4. `TestHandshakeInitiatorIDTampering` (line 203) — sign with `privB` (still fails on ID mismatch first)
5. `TestHandshakePeerMismatch` (line 239) — sign with `privC` (still fails on peer mismatch first)
6. `TestHandshakeDiscoveryNotBoundPassthrough` (line 278) — sign with `privB`

**Step 3: Run tests**

Run: `go test ./internal/mutauth/... -v -count=1`
Expected: All tests PASS (19 tests including discovery tests)

**Step 4: Commit**

```bash
git add internal/mutauth/mut_auth_hdl.go internal/mutauth/mut_auth_hdl_test.go
git commit -m "refactor(mutauth): accept pre-signed nonce in RespondToHandshake

Private keys must never cross HTTP boundaries. Agents sign locally;
broker verifies against registered public key (defense in depth).
Signature now checked at step 2, not deferred to step 3.

Maps to Security Pattern Component 6: Agent-to-Agent Mutual Auth."
```

---

### Task 2: Broker HTTP handlers for handshake (3 handlers)

Create HTTP handlers that wrap the `MutAuthHdl` service methods. Follow the same pattern as `DelegHdl` (see `internal/handler/deleg_hdl.go`): extract claims from context, decode JSON, call service, map errors to RFC 7807 responses.

**Files:**
- Create: `internal/handler/handshake_hdl.go`
- Create: `internal/handler/handshake_hdl_test.go`

**Step 1: Write failing tests**

In `internal/handler/handshake_hdl_test.go`, write tests using the existing `handler_test.go` test infrastructure pattern (httptest + real services):

```go
package handler_test

import (
    "bytes"
    "crypto/ed25519"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/divineartis/agentauth/internal/authz"
    "github.com/divineartis/agentauth/internal/cfg"
    "github.com/divineartis/agentauth/internal/handler"
    "github.com/divineartis/agentauth/internal/mutauth"
    "github.com/divineartis/agentauth/internal/store"
    "github.com/divineartis/agentauth/internal/token"
)

func setupHandshakeTest(t *testing.T) (*http.ServeMux, string, string, ed25519.PrivateKey, ed25519.PrivateKey, string, string) {
    t.Helper()
    pubBroker, privBroker, _ := ed25519.GenerateKey(nil)
    tknSvc := token.NewTknSvc(privBroker, pubBroker, cfg.Cfg{TrustDomain: "agentauth.local", DefaultTTL: 300})
    st := store.NewSqlStore()
    revSvc := revoke.NewRevSvc()

    pubA, privA, _ := ed25519.GenerateKey(nil)
    pubB, privB, _ := ed25519.GenerateKey(nil)
    agentAID := "spiffe://agentauth.local/agent/orch-1/task-1/inst-a"
    agentBID := "spiffe://agentauth.local/agent/orch-1/task-2/inst-b"
    st.SaveAgent(store.AgentRecord{AgentID: agentAID, OrchID: "orch-1", TaskID: "task-1", Scope: []string{"read:Data:*"}, RegisteredAt: time.Now().UTC(), PublicKey: pubA})
    st.SaveAgent(store.AgentRecord{AgentID: agentBID, OrchID: "orch-1", TaskID: "task-2", Scope: []string{"write:Data:*"}, RegisteredAt: time.Now().UTC(), PublicKey: pubB})

    tokA, _ := tknSvc.Issue(token.IssueReq{Sub: agentAID, OrchId: "orch-1", TaskId: "task-1", Scope: []string{"read:Data:*"}})
    tokB, _ := tknSvc.Issue(token.IssueReq{Sub: agentBID, OrchId: "orch-1", TaskId: "task-2", Scope: []string{"write:Data:*"}})

    mutAuthHdl := mutauth.NewMutAuthHdl(tknSvc, st, nil)
    valMw := authz.NewValMw(tknSvc, revSvc, nil)

    mux := http.NewServeMux()
    mux.Handle("POST /v1/handshake/initiate", valMw.Wrap(handler.NewInitiateHdl(mutAuthHdl)))
    mux.Handle("POST /v1/handshake/respond", valMw.Wrap(handler.NewRespondHdl(mutAuthHdl)))
    mux.Handle("POST /v1/handshake/complete", valMw.Wrap(handler.NewCompleteHdl(mutAuthHdl)))

    return mux, tokA.AccessToken, tokB.AccessToken, privA, privB, agentAID, agentBID
}

func TestHandshakeHTTP_FullFlow(t *testing.T) {
    // Test the full 3-step handshake over HTTP
    // 1. POST /v1/handshake/initiate with Agent A's token
    // 2. POST /v1/handshake/respond with Agent B's token + signed nonce
    // 3. POST /v1/handshake/complete with Agent A's token + response
    // Assert: 200 at each step, verified=true at the end
}

func TestHandshakeHTTP_InitiateNoAuth(t *testing.T) {
    // POST /v1/handshake/initiate without Bearer token
    // Assert: 401
}

func TestHandshakeHTTP_InitiateUnknownTarget(t *testing.T) {
    // POST /v1/handshake/initiate with valid token but unknown target
    // Assert: 404 (target not registered)
}

func TestHandshakeHTTP_RespondWrongKey(t *testing.T) {
    // Step 2 with signature from wrong key
    // Assert: error at respond step
}
```

**Step 2: Run tests, verify they fail**

Run: `go test ./internal/handler/... -run TestHandshakeHTTP -v`
Expected: FAIL (handlers don't exist yet)

**Step 3: Implement the 3 handlers**

In `internal/handler/handshake_hdl.go`:

```go
package handler

import (
    "encoding/base64"
    "encoding/json"
    "net/http"

    "github.com/divineartis/agentauth/internal/authz"
    "github.com/divineartis/agentauth/internal/mutauth"
    "github.com/divineartis/agentauth/internal/obs"
    "github.com/divineartis/agentauth/internal/problemdetails"
)

// --- InitiateHdl ---

type InitiateHdl struct {
    mutAuth *mutauth.MutAuthHdl
}

func NewInitiateHdl(m *mutauth.MutAuthHdl) *InitiateHdl {
    return &InitiateHdl{mutAuth: m}
}

func (h *InitiateHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Extract bearer token from Authorization header (raw token string for service call)
    rawToken := r.Header.Get("Authorization")
    // strip "Bearer " prefix
    // ...

    var req struct {
        TargetAgentID string `json:"target_agent_id"`
    }
    // decode body, validate, call h.mutAuth.InitiateHandshake(rawToken, req.TargetAgentID)
    // map errors to RFC 7807
    // encode response as JSON
}

// --- RespondHdl ---

type RespondHdl struct {
    mutAuth *mutauth.MutAuthHdl
}

func NewRespondHdl(m *mutauth.MutAuthHdl) *RespondHdl {
    return &RespondHdl{mutAuth: m}
}

func (h *RespondHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Decode JSON body containing HandshakeReq fields + signed_nonce (base64)
    // Build mutauth.HandshakeReq from body
    // base64-decode signed_nonce
    // Extract responder's raw bearer token
    // Call h.mutAuth.RespondToHandshake(handshakeReq, rawToken, signedNonce)
    // Map errors: ErrHandshakeInvalidToken->401, ErrPeerMismatch->403, ErrHandshakeNonceMismatch->401
    // Encode response: responder_id, responder_token, signed_nonce (base64), counter_nonce
}

// --- CompleteHdl ---

type CompleteHdl struct {
    mutAuth *mutauth.MutAuthHdl
}

func NewCompleteHdl(m *mutauth.MutAuthHdl) *CompleteHdl {
    return &CompleteHdl{mutAuth: m}
}

func (h *CompleteHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Decode JSON body containing HandshakeResp fields + original_nonce
    // Build mutauth.HandshakeResp from body
    // base64-decode signed_nonce
    // Call h.mutAuth.CompleteHandshake(handshakeResp, originalNonce)
    // Map errors
    // Return { "verified": true/false }
}
```

Error mapping (consistent with existing handlers):
- `ErrHandshakeInvalidToken` → 401 `"unauthorized"`
- `ErrHandshakeUnknownAgent` → 404 `"not_found"`
- `ErrPeerMismatch` → 403 `"forbidden"`
- `ErrInitiatorMismatch` → 403 `"forbidden"`
- `ErrResponderMismatch` → 403 `"forbidden"`
- `ErrHandshakeNonceMismatch` → 401 `"unauthorized"`

**Step 4: Run tests, verify they pass**

Run: `go test ./internal/handler/... -v -count=1`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/handler/handshake_hdl.go internal/handler/handshake_hdl_test.go
git commit -m "feat(handler): add HTTP handlers for 3-step mutual auth handshake

POST /v1/handshake/initiate — initiator presents token, gets nonce
POST /v1/handshake/respond — responder signs nonce, presents credentials
POST /v1/handshake/complete — initiator verifies responder signature

Maps to Security Pattern Component 6: Agent-to-Agent Mutual Auth."
```

---

### Task 3: Wire broker handshake routes in main.go

**Files:**
- Modify: `cmd/broker/main.go:44,90-95,111-115`

**Step 1: Add MutAuthHdl instantiation and route wiring**

In `cmd/broker/main.go`, after the existing handler declarations (around line 95):

```go
// Add import for mutauth package
import "github.com/divineartis/agentauth/internal/mutauth"

// After delegHdl declaration (line 90):
mutAuthHdl := mutauth.NewMutAuthHdl(tknSvc, sqlStore, nil)

// After existing handlers (line 95):
initiateHdl := handler.NewInitiateHdl(mutAuthHdl)
respondHdl := handler.NewRespondHdl(mutAuthHdl)
completeHdl := handler.NewCompleteHdl(mutAuthHdl)
```

Wire routes after the existing Bearer-auth routes (around line 115):
```go
// Mutual authentication handshake (Bearer auth)
mux.Handle("POST /v1/handshake/initiate", problemdetails.MaxBytesBody(valMw.Wrap(initiateHdl)))
mux.Handle("POST /v1/handshake/respond", problemdetails.MaxBytesBody(valMw.Wrap(respondHdl)))
mux.Handle("POST /v1/handshake/complete", problemdetails.MaxBytesBody(valMw.Wrap(completeHdl)))
```

Update the route table comment at the top to include the 3 new routes.

**Step 2: Build and run existing tests**

Run: `go build ./... && go test ./... -short -count=1`
Expected: All compile and pass

**Step 3: Commit**

```bash
git add cmd/broker/main.go
git commit -m "feat(broker): wire mutual auth handshake routes

POST /v1/handshake/{initiate,respond,complete} behind ValMw.
MutAuthHdl instantiated with nil DiscoveryRegistry (not a pattern component).

Maps to Security Pattern Component 6."
```

---

### Task 4: Sidecar broker client — handshake + delegation methods

Add 4 new methods to `brokerClient` for calling the broker's handshake and delegation endpoints.

**Files:**
- Modify: `cmd/sidecar/broker_client.go`
- Create: `cmd/sidecar/broker_client_handshake_test.go`

**Step 1: Write failing tests**

In `cmd/sidecar/broker_client_handshake_test.go`, test each method against a mock HTTP server (same pattern as existing `broker_client_test.go`):

```go
func TestBrokerClient_HandshakeInitiate(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" || r.URL.Path != "/v1/handshake/initiate" {
            t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
        }
        // Verify Authorization header
        // Verify JSON body has target_agent_id
        json.NewEncoder(w).Encode(map[string]any{
            "nonce": "abc123", "initiator_id": "spiffe://test/a", "target_agent_id": "spiffe://test/b",
        })
    }))
    defer srv.Close()
    bc := newBrokerClient(srv.URL)
    resp, err := bc.handshakeInitiate("bearer-token", "spiffe://test/b")
    // assert resp fields, err==nil
}

func TestBrokerClient_HandshakeRespond(t *testing.T) { /* similar */ }
func TestBrokerClient_HandshakeComplete(t *testing.T) { /* similar */ }
func TestBrokerClient_Delegate(t *testing.T) { /* similar */ }
```

**Step 2: Run tests, verify they fail**

Run: `go test ./cmd/sidecar/... -run TestBrokerClient_Handshake -v`
Expected: FAIL (methods don't exist)

**Step 3: Implement 4 methods**

In `cmd/sidecar/broker_client.go`, add:

```go
// handshakeInitiateResp holds the parsed response from POST /v1/handshake/initiate.
type handshakeInitiateResp struct {
    Nonce         string `json:"nonce"`
    InitiatorID   string `json:"initiator_id"`
    TargetAgentID string `json:"target_agent_id"`
}

func (c *brokerClient) handshakeInitiate(bearerToken, targetAgentID string) (*handshakeInitiateResp, error) {
    body, _ := json.Marshal(map[string]string{"target_agent_id": targetAgentID})
    resp, err := c.doJSON("POST", "/v1/handshake/initiate", body, bearerToken)
    // parse into handshakeInitiateResp
}

type handshakeRespondResp struct {
    ResponderID    string `json:"responder_id"`
    ResponderToken string `json:"responder_token"`
    SignedNonce    string `json:"signed_nonce"`    // base64
    CounterNonce  string `json:"counter_nonce"`
}

func (c *brokerClient) handshakeRespond(bearerToken string, initiatorToken, initiatorID, targetAgentID, nonce, signedNonceB64 string) (*handshakeRespondResp, error) {
    body, _ := json.Marshal(map[string]string{
        "initiator_token": initiatorToken,
        "initiator_id":    initiatorID,
        "target_agent_id": targetAgentID,
        "nonce":           nonce,
        "signed_nonce":    signedNonceB64,
    })
    resp, err := c.doJSON("POST", "/v1/handshake/respond", body, bearerToken)
    // parse
}

func (c *brokerClient) handshakeComplete(bearerToken string, responderToken, responderID, signedNonceB64, counterNonce, originalNonce string) (bool, error) {
    body, _ := json.Marshal(map[string]string{
        "responder_token": responderToken,
        "responder_id":    responderID,
        "signed_nonce":    signedNonceB64,
        "counter_nonce":   counterNonce,
        "original_nonce":  originalNonce,
    })
    resp, err := c.doJSON("POST", "/v1/handshake/complete", body, bearerToken)
    // parse verified field
}

type delegateResp struct {
    AccessToken     string `json:"access_token"`
    ExpiresIn       int    `json:"expires_in"`
    DelegationChain []any  `json:"delegation_chain"`
}

func (c *brokerClient) delegate(bearerToken, delegateTo string, scope []string, ttl int) (*delegateResp, error) {
    body, _ := json.Marshal(map[string]any{
        "delegate_to": delegateTo,
        "scope":       scope,
        "ttl":         ttl,
    })
    resp, err := c.doJSON("POST", "/v1/delegate", body, bearerToken)
    // parse
}
```

**Step 4: Run tests, verify pass**

Run: `go test ./cmd/sidecar/... -run TestBrokerClient -v -count=1`
Expected: All PASS

**Step 5: Commit**

```bash
git add cmd/sidecar/broker_client.go cmd/sidecar/broker_client_handshake_test.go
git commit -m "feat(sidecar): add broker client methods for handshake + delegation

4 new methods: handshakeInitiate, handshakeRespond, handshakeComplete, delegate.
Follow existing doJSON pattern.

Maps to Security Pattern Components 6 and 7."
```

---

### Task 5: Sidecar handshake proxy handlers (3 handlers + auto-sign)

The sidecar's value for Component 6: managed agents never touch keys. The sidecar auto-signs the nonce for step 2.

**Files:**
- Create: `cmd/sidecar/handshake_handler.go`
- Create: `cmd/sidecar/handshake_handler_test.go`

**Step 1: Write failing tests**

Tests should cover:
1. Full handshake flow — managed agent (auto-sign)
2. Full handshake flow — BYOK agent (pass-through signed_nonce)
3. Initiate with unknown agent_name → 400
4. Respond with managed agent that has no private key (should not happen but defensive)

Test pattern: mock broker HTTP server returns expected responses. The handler resolves agent from registry, auto-signs or passes through, proxies to mock broker.

```go
func TestSidecarHandshake_ManagedAgent_FullFlow(t *testing.T) {
    // Setup: register agent in registry with privKey (managed)
    // Mock broker: /v1/handshake/initiate returns nonce,
    //              /v1/handshake/respond returns counter-nonce,
    //              /v1/handshake/complete returns verified=true
    // Call sidecar initiate → respond (no signed_nonce in body, auto-sign) → complete
    // Assert: all 200s, verified=true
}

func TestSidecarHandshake_BYOKAgent_Respond(t *testing.T) {
    // Setup: register agent in registry with nil privKey (BYOK)
    // Call respond with signed_nonce in body
    // Assert: passes through to broker, 200
}

func TestSidecarHandshake_BYOKAgent_Respond_MissingSig(t *testing.T) {
    // Setup: BYOK agent (nil privKey)
    // Call respond WITHOUT signed_nonce
    // Assert: 400 "BYOK agent must provide signed_nonce"
}
```

**Step 2: Implement handlers**

In `cmd/sidecar/handshake_handler.go`:

```go
package main

// --- handshakeInitiateHandler ---
// POST /v1/handshake/initiate
// Body: { "agent_name": "...", "target_agent_id": "spiffe://..." }
// Resolves agent from registry, gets token via broker exchange, proxies initiate.

type handshakeInitiateHandler struct {
    broker   *brokerClient
    state    *sidecarState
    registry *agentRegistry
}

func (h *handshakeInitiateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Decode body: agent_name, target_agent_id
    // 2. Lookup agent in registry (must already be registered via POST /v1/token)
    // 3. Get agent's token: need the token from the last token exchange
    //    OR: the sidecar state token (sidecar acts on behalf of agent)
    //    Decision: Use the agent's last cached token from registry
    // 4. Proxy to broker: bc.handshakeInitiate(agentToken, targetAgentID)
    // 5. Return broker response
}

// --- handshakeRespondHandler ---
// POST /v1/handshake/respond
// Body: { "agent_name": "...", "handshake_req": {...}, "signed_nonce": "..." }
// For managed agents: auto-signs nonce with registry private key
// For BYOK agents: passes through provided signed_nonce

type handshakeRespondHandler struct {
    broker   *brokerClient
    state    *sidecarState
    registry *agentRegistry
}

func (h *handshakeRespondHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Decode body
    // 2. Lookup agent in registry
    // 3. Get agent's token from last exchange cache
    // 4. Determine signing mode:
    //    - If signed_nonce provided in body → BYOK pass-through
    //    - If not provided AND agent.privKey != nil → auto-sign
    //    - If not provided AND agent.privKey == nil → 400 error
    // 5. For auto-sign: ed25519.Sign(agent.privKey, []byte(handshakeReq.Nonce))
    // 6. Base64-encode the signed nonce
    // 7. Proxy to broker: bc.handshakeRespond(...)
    // 8. Return broker response
}

// --- handshakeCompleteHandler ---
// POST /v1/handshake/complete
// Body: { "agent_name": "...", "handshake_resp": {...}, "original_nonce": "..." }

type handshakeCompleteHandler struct {
    broker   *brokerClient
    state    *sidecarState
    registry *agentRegistry
}

func (h *handshakeCompleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Decode body
    // 2. Lookup agent in registry, get cached token
    // 3. Proxy to broker: bc.handshakeComplete(...)
    // 4. Return broker response
}
```

**Token for handshake calls:** The agent needs a valid bearer token for each handshake step. The sidecar caches the last exchanged token in `agentEntry.lastToken`. Use that. If no cached token exists, the agent hasn't called `POST /v1/token` yet — return 400.

**Step 3: Run tests**

Run: `go test ./cmd/sidecar/... -run TestSidecarHandshake -v -count=1`
Expected: All PASS

**Step 4: Commit**

```bash
git add cmd/sidecar/handshake_handler.go cmd/sidecar/handshake_handler_test.go
git commit -m "feat(sidecar): add handshake proxy handlers with auto-sign

3 proxy endpoints for mutual auth handshake.
Managed agents: sidecar auto-signs nonce with registry private key.
BYOK agents: developer provides signed_nonce, sidecar passes through.

Maps to Security Pattern Component 6."
```

---

### Task 6: Sidecar delegation proxy handler

Simple pass-through: proxy to broker's `POST /v1/delegate` with the agent's cached token.

**Files:**
- Create: `cmd/sidecar/delegate_handler.go`
- Create: `cmd/sidecar/delegate_handler_test.go`

**Step 1: Write failing test**

```go
func TestSidecarDelegate_ProxiesToBroker(t *testing.T) {
    // Mock broker: /v1/delegate returns access_token + delegation_chain
    // Register agent in registry with cached token
    // POST /v1/delegate to sidecar with agent_name, delegate_to, scope
    // Assert: 200, response contains delegated token
}

func TestSidecarDelegate_UnknownAgent(t *testing.T) {
    // POST /v1/delegate with unregistered agent_name
    // Assert: 400
}
```

**Step 2: Implement**

In `cmd/sidecar/delegate_handler.go`:

```go
package main

type delegateHandler struct {
    broker   *brokerClient
    registry *agentRegistry
}

type sidecarDelegateReq struct {
    AgentName  string   `json:"agent_name"`
    DelegateTo string   `json:"delegate_to"`
    Scope      []string `json:"scope"`
    TTL        int      `json:"ttl"`
}

func (h *delegateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Decode body
    // 2. Lookup agent, get cached token
    // 3. Proxy: bc.delegate(agentToken, req.DelegateTo, req.Scope, req.TTL)
    // 4. Return broker response
}
```

**Step 3: Run tests, verify pass**

Run: `go test ./cmd/sidecar/... -run TestSidecarDelegate -v -count=1`

**Step 4: Commit**

```bash
git add cmd/sidecar/delegate_handler.go cmd/sidecar/delegate_handler_test.go
git commit -m "feat(sidecar): add delegation proxy handler

Proxies to broker POST /v1/delegate with agent's cached token.
Scope attenuation enforced by broker.

Maps to Security Pattern Component 7: Delegation Chain Verification."
```

---

### Task 7: Wire sidecar routes in main.go

**Files:**
- Modify: `cmd/sidecar/main.go:109-114`

**Step 1: Wire 4 new routes**

After the existing post-bootstrap route wiring (line 114):

```go
// Mutual auth handshake proxy
mux.Handle("/v1/handshake/initiate", &handshakeInitiateHandler{broker: bc, state: state, registry: registry})
mux.Handle("/v1/handshake/respond", &handshakeRespondHandler{broker: bc, state: state, registry: registry})
mux.Handle("/v1/handshake/complete", &handshakeCompleteHandler{broker: bc, state: state, registry: registry})

// Delegation proxy
mux.Handle("/v1/delegate", &delegateHandler{broker: bc, registry: registry})
```

**Step 2: Build and run all tests**

Run: `go build ./... && go test ./... -short -count=1`
Expected: All compile and pass

**Step 3: Commit**

```bash
git add cmd/sidecar/main.go
git commit -m "feat(sidecar): wire handshake + delegation proxy routes

4 new sidecar routes: 3 handshake proxy + 1 delegation proxy.
All wired post-bootstrap alongside existing token/renew routes.

Maps to Security Pattern Components 6 and 7."
```

---

### Task 8: Extend Docker E2E for all 7 pattern components

The canonical proof that the Ephemeral Agent Credentialing pattern works.

**Files:**
- Modify: `scripts/live_test_docker.sh`
- Modify: `scripts/live_test_sidecar.sh`

**Step 1: Extend broker live test**

Add steps to `live_test_docker.sh` after the existing 9 steps:

```bash
# Step 10: Mutual Auth — Initiate handshake (A → B)
# POST /v1/handshake/initiate with Agent A's token, targeting Agent B
# Assert: 200, response contains nonce

# Step 11: Mutual Auth — Respond (B signs nonce)
# Sign the nonce with Agent B's private key
# POST /v1/handshake/respond with Agent B's token + signed nonce
# Assert: 200, response contains counter_nonce

# Step 12: Mutual Auth — Complete (A verifies B)
# POST /v1/handshake/complete with Agent A's token + response data
# Assert: 200, verified=true

# Step 13: Delegation — A delegates narrowed scope to B
# POST /v1/delegate with Agent A's token, delegate_to=B, scope=["read:Data:specific"]
# Assert: 200, response contains delegated token + delegation_chain
```

**Step 2: Extend sidecar live test**

Add steps to `live_test_sidecar.sh`:

```bash
# Step 10: Mutual Auth via Sidecar — full 3-step handshake
# POST /v1/handshake/initiate to sidecar (agent_name=agent-a, target=agent-b)
# POST /v1/handshake/respond to sidecar (agent_name=agent-b, auto-sign)
# POST /v1/handshake/complete to sidecar
# Assert: verified=true

# Step 11: Delegation via Sidecar
# POST /v1/delegate to sidecar (agent_name=agent-a, delegate_to=agent-b spiffe id)
# Assert: 200, delegated token returned

# Step 12: Verify audit trail includes handshake + delegation events
# GET /v1/audit/events
# Assert: events for handshake initiate/respond/complete + delegation
```

**Step 3: Run Docker E2E**

Run: `./scripts/gates.sh module`
Expected: broker all pass, sidecar all pass

**Step 4: Commit**

```bash
git add scripts/live_test_docker.sh scripts/live_test_sidecar.sh
git commit -m "test(e2e): extend Docker E2E for all 7 pattern components

Broker: 4 new steps (mutual auth 3-step + delegation)
Sidecar: 3 new steps (handshake via proxy + delegation + audit verification)

Proves Security Pattern Components 1-7 work end-to-end."
```

---

### Task 9: Documentation

Update all required docs per project policy.

**Files:**
- Modify: `docs/API_REFERENCE.md`
- Modify: `docs/DEVELOPER_GUIDE.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/api/openapi.yaml`

**Step 1: API Reference**

Add 3 new broker endpoints and 4 new sidecar endpoints to `docs/API_REFERENCE.md`:
- `POST /v1/handshake/initiate` (broker + sidecar)
- `POST /v1/handshake/respond` (broker + sidecar)
- `POST /v1/handshake/complete` (broker + sidecar)
- `POST /v1/delegate` (sidecar — broker already documented)

Include request/response schemas, auth requirements, error codes.

**Step 2: Developer Guide**

Add section on mutual auth architecture:
- 3-step handshake protocol
- Service layer → HTTP handler → sidecar proxy data flow
- BYOK vs managed agent auto-sign
- How delegation proxy works

**Step 3: OpenAPI**

Add endpoint definitions to `docs/api/openapi.yaml`.

**Step 4: Changelog**

Add entries under `[Unreleased]`:
- `feat(broker): add mutual auth handshake endpoints (POST /v1/handshake/{initiate,respond,complete})`
- `feat(sidecar): add handshake proxy with auto-sign for managed agents`
- `feat(sidecar): add delegation proxy endpoint`
- `refactor(mutauth): accept pre-signed nonces (private keys never cross HTTP)`
- `test(e2e): prove all 7 Security Pattern components end-to-end`

**Step 5: Commit**

```bash
git add docs/API_REFERENCE.md docs/DEVELOPER_GUIDE.md CHANGELOG.md docs/api/openapi.yaml
git commit -m "docs: document mutual auth + delegation endpoints

API Reference: 7 new endpoint specs (3 broker + 4 sidecar)
Developer Guide: handshake architecture, BYOK vs managed flow
OpenAPI: endpoint definitions
Changelog: all changes under [Unreleased]"
```

---

## Task Dependency Summary

```
Task 1 (service refactor)
  └→ Task 2 (broker HTTP handlers) + Task 4 (sidecar broker client) [parallel-safe]
       └→ Task 3 (wire broker routes)
       └→ Task 5 (sidecar handshake handlers) + Task 6 (sidecar delegation handler) [parallel-safe]
            └→ Task 7 (wire sidecar routes)
                 └→ Task 8 (Docker E2E)
                      └→ Task 9 (documentation)
```

Tasks 2+4 can run in parallel. Tasks 5+6 can run in parallel. All others are sequential.
