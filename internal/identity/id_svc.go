// Package identity implements the agent registration flow including
// challenge-response verification, SPIFFE ID generation, Ed25519 key
// management, and scope enforcement.
//
// The core workflow is:
//  1. Agent obtains a nonce via GET /v1/challenge.
//  2. Agent signs the nonce with its Ed25519 private key.
//  3. Agent calls POST /v1/register with the signed nonce, its public key,
//     a pre-authorized launch token, and the requested scope.
//  4. [IdSvc.Register] validates everything, generates a SPIFFE ID, issues
//     a JWT token, and saves the agent record.
//
// Scope enforcement is strict: requested scopes must be a subset of the
// launch token's allowed scopes, checked before the launch token is consumed.
package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

// Sentinel errors returned by [IdSvc.Register].
var (
	ErrScopeViolation   = errors.New("requested scope exceeds allowed scope")
	ErrInvalidSignature = errors.New("nonce signature verification failed")
	ErrInvalidPublicKey = errors.New("invalid Ed25519 public key")
	ErrMissingField     = errors.New("missing required field")
)

// RegisterReq contains the fields submitted by an agent in the
// POST /v1/register request body. All fields are required.
type RegisterReq struct {
	LaunchToken    string   `json:"launch_token"`
	Nonce          string   `json:"nonce"`
	PublicKey      string   `json:"public_key"`       // base64-encoded Ed25519 public key
	Signature      string   `json:"signature"`        // base64-encoded Ed25519 signature of nonce
	// OrchID identifies the orchestrator that launched this agent.
	OrchID string `json:"orch_id"`
	// TaskID identifies the specific task this agent was created for.
	TaskID string `json:"task_id"`
	// RequestedScope is the permissions the agent requests, which must be
	// a subset of the launch token's AllowedScope.
	RequestedScope []string `json:"requested_scope"`
}

// RegisterResp is returned on successful registration. It contains the
// agent's SPIFFE ID, the issued Bearer token, and its TTL in seconds.
type RegisterResp struct {
	// AgentID is the SPIFFE URI assigned to the registered agent
	// (format: spiffe://{trustDomain}/agent/{orchID}/{taskID}/{instanceID}).
	AgentID string `json:"agent_id"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// AuditRecorder is the interface for recording audit events during
// registration. It is satisfied by [audit.AuditLog]. A nil value
// disables audit recording.
type AuditRecorder interface {
	Record(eventType, agentID, taskID, orchID, detail string, opts ...audit.RecordOption)
}

// IdSvc is the identity service responsible for the full agent registration
// flow: launch token validation, scope enforcement, nonce verification,
// Ed25519 key validation, SPIFFE ID generation, token issuance, and agent
// record persistence.
type IdSvc struct {
	store       *store.SqlStore
	tknSvc      *token.TknSvc
	trustDomain string
	auditLog    AuditRecorder
	audience    string
}

// NewIdSvc creates a new identity service. The auditLog parameter may
// be nil to disable audit recording. The audience parameter is populated
// into issued tokens; empty means no audience claim.
func NewIdSvc(sqlStore *store.SqlStore, tknSvc *token.TknSvc, trustDomain string, auditLog AuditRecorder, audience string) *IdSvc {
	return &IdSvc{
		store:       sqlStore,
		tknSvc:      tknSvc,
		trustDomain: trustDomain,
		auditLog:    auditLog,
		audience:    audience,
	}
}

// audienceSlice returns the audience as a single-element slice, or nil
// when no audience is configured.
func (s *IdSvc) audienceSlice() []string {
	if s.audience == "" {
		return nil
	}
	return []string{s.audience}
}

// Register performs the complete agent registration flow:
//
//  1. Validate that all required fields are present.
//  2. Look up and validate the launch token.
//  3. Enforce scope attenuation (requested must be subset of allowed).
//  4. Consume the nonce (one-time use).
//  5. Decode and validate the Ed25519 public key.
//  6. Verify the nonce signature against the public key.
//  7. Consume the launch token (if single-use).
//  8. Generate a SPIFFE ID for the agent.
//  9. Issue a JWT token with the granted scope.
//  10. Persist the agent record.
//
// SECURITY: Scope enforcement (step 3) occurs before launch token
// consumption so that a scope violation does not waste a single-use token.
func (s *IdSvc) Register(req RegisterReq) (*RegisterResp, error) {
	// Validate required fields
	if req.LaunchToken == "" || req.Nonce == "" || req.PublicKey == "" ||
		req.Signature == "" || req.OrchID == "" || req.TaskID == "" {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, ErrMissingField
	}
	if len(req.RequestedScope) == 0 {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, fmt.Errorf("%w: requested_scope", ErrMissingField)
	}

	// Validate launch token
	ltRec, err := s.store.GetLaunchToken(req.LaunchToken)
	if err != nil {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, fmt.Errorf("launch token: %w", err)
	}

	// CRITICAL: Check scope BEFORE consuming the launch token
	if !authz.ScopeIsSubset(req.RequestedScope, ltRec.AllowedScope) {
		if s.auditLog != nil {
			s.auditLog.Record("registration_policy_violation", "", req.TaskID, req.OrchID,
				fmt.Sprintf("scope violation: requested %v exceeds allowed %v", req.RequestedScope, ltRec.AllowedScope),
			audit.WithOutcome("denied"))
		}
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		obs.Warn("IDENTITY", "Register", "scope violation",
			fmt.Sprintf("requested=%v", req.RequestedScope),
			fmt.Sprintf("allowed=%v", ltRec.AllowedScope))
		return nil, ErrScopeViolation
	}

	// Consume nonce
	if err := s.store.ConsumeNonce(req.Nonce); err != nil {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, fmt.Errorf("nonce: %w", err)
	}

	// Decode and verify Ed25519 public key
	pubKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, fmt.Errorf("%w: base64 decode failed", ErrInvalidPublicKey)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, fmt.Errorf("%w: wrong key size", ErrInvalidPublicKey)
	}
	pubKey := ed25519.PublicKey(pubKeyBytes)

	// Verify nonce signature
	sigBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, fmt.Errorf("%w: base64 decode failed", ErrInvalidSignature)
	}
	nonceBytes, err := hex.DecodeString(req.Nonce)
	if err != nil {
		// If nonce is not hex, verify against raw string
		nonceBytes = []byte(req.Nonce)
	}
	if !ed25519.Verify(pubKey, nonceBytes, sigBytes) {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, ErrInvalidSignature
	}

	// Consume launch token (after all checks pass)
	if ltRec.SingleUse {
		if err := s.store.ConsumeLaunchToken(req.LaunchToken); err != nil {
			obs.RegistrationsTotal.WithLabelValues("failure").Inc()
			return nil, fmt.Errorf("consume launch token: %w", err)
		}
	}

	// Generate SPIFFE ID
	instanceID := randomInstanceID()
	agentID, err := NewSpiffeId(s.trustDomain, req.OrchID, req.TaskID, instanceID)
	if err != nil {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, fmt.Errorf("generate SPIFFE ID: %w", err)
	}

	// Determine TTL (use launch token's MaxTTL as ceiling)
	ttl := ltRec.MaxTTL
	if ttl <= 0 {
		ttl = 300
	}

	// Issue token
	issResp, err := s.tknSvc.Issue(token.IssueReq{
		Sub:    agentID,
		Aud:    s.audienceSlice(),
		Scope:  req.RequestedScope,
		TaskId: req.TaskID,
		OrchId: req.OrchID,
		TTL:    ttl,
	})
	if err != nil {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, fmt.Errorf("issue token: %w", err)
	}

	// Save agent record
	now := time.Now()
	if err := s.store.SaveAgent(store.AgentRecord{
		AgentID:      agentID,
		PublicKey:    pubKeyBytes,
		OrchID:       req.OrchID,
		TaskID:       req.TaskID,
		Scope:        req.RequestedScope,
		RegisteredAt: now,
		LastSeen:     now,
		AppID:        ltRec.AppID, // inherit from launch token
	}); err != nil {
		obs.RegistrationsTotal.WithLabelValues("failure").Inc()
		return nil, fmt.Errorf("save agent: %w", err)
	}

	// Audit events
	if s.auditLog != nil {
		regDetail := fmt.Sprintf("Agent registered with scope %v", req.RequestedScope)
		tknDetail := fmt.Sprintf("Token issued, jti=%s, ttl=%d", issResp.Claims.Jti, ttl)
		if ltRec.AppID != "" {
			regDetail += fmt.Sprintf(" app_id=%s", ltRec.AppID)
			tknDetail += fmt.Sprintf(" app_id=%s", ltRec.AppID)
		}
		s.auditLog.Record("agent_registered", agentID, req.TaskID, req.OrchID,
			regDetail,
			audit.WithOutcome("success"))
		s.auditLog.Record("token_issued", agentID, req.TaskID, req.OrchID,
			tknDetail,
			audit.WithOutcome("success"))
	}

	// Metrics
	obs.ActiveAgents.Inc()
	obs.RegistrationsTotal.WithLabelValues("success").Inc()

	obs.Ok("IDENTITY", "Register", "agent registered",
		"agent_id="+agentID, fmt.Sprintf("scope=%v", req.RequestedScope))

	return &RegisterResp{
		AgentID:     agentID,
		AccessToken: issResp.AccessToken,
		ExpiresIn:   issResp.ExpiresIn,
	}, nil
}

// GenerateSigningKeyPair generates a new Ed25519 key pair suitable for
// token signing and verification. It uses [crypto/rand.Reader] as the
// entropy source.
func GenerateSigningKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate Ed25519 key pair: %w", err)
	}
	return pub, priv, nil
}

func randomInstanceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
