// Package admin provides administrator authentication and launch token
// lifecycle management for the AgentAuth broker.
//
// Administrators authenticate via POST /v1/admin/auth with a shared
// client_secret. On success they receive a short-lived JWT with admin
// scopes that authorizes further operations such as creating launch
// tokens and revoking agent tokens.
//
// Launch tokens are opaque 64-character hex strings with an associated
// policy (allowed scope, max TTL, single-use flag). They are created via
// POST /v1/admin/launch-tokens and consumed during agent registration.
package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

const (
	mod = "ADMIN"
	cmp = "AdminSvc"

	adminSub        = "admin"
	adminTTL        = 300
	defaultMaxTTL   = 300
	defaultTokenTTL = 30

	launchTokenBytes = 32
)

// adminScope is the fixed set of scopes granted to an admin JWT.
var adminScope = []string{
	"admin:launch-tokens:*",
	"admin:revoke:*",
	"admin:audit:*",
}

// Sentinel errors returned by admin operations.
var (
	ErrInvalidSecret  = errors.New("invalid client secret")
	ErrAgentNameEmpty = errors.New("agent_name is required")
	ErrScopeEmpty     = errors.New("allowed_scope must not be empty")
)

// CreateLaunchTokenReq is the JSON request body for
// POST /v1/admin/launch-tokens. AgentName and AllowedScope are required;
// all other fields have sensible defaults.
type CreateLaunchTokenReq struct {
	AgentName    string   `json:"agent_name"`
	AllowedScope []string `json:"allowed_scope"`
	MaxTTL       int      `json:"max_ttl"`
	SingleUse    *bool    `json:"single_use"`
	TTL          int      `json:"ttl"`
}

// CreateLaunchTokenResp is the JSON response returned on successful
// launch token creation.
type CreateLaunchTokenResp struct {
	LaunchToken string            `json:"launch_token"`
	ExpiresAt   string            `json:"expires_at"`
	Policy      LaunchTokenPolicy `json:"policy"`
}

// LaunchTokenPolicy describes the scope ceiling and TTL cap bound to a
// launch token. Agents registering with this token cannot exceed these
// constraints.
type LaunchTokenPolicy struct {
	AllowedScope []string `json:"allowed_scope"`
	MaxTTL       int      `json:"max_ttl"`
}

// AdminSvc handles administrator authentication (shared secret) and
// launch token lifecycle (create, validate, consume). All launch token
// storage is delegated to [store.SqlStore] so that tokens are visible
// to the identity service during registration.
type AdminSvc struct {
	adminSecret []byte
	tknSvc      *token.TknSvc
	store       *store.SqlStore
	auditLog    *audit.AuditLog
}

// NewAdminSvc creates a new admin service. The adminSecret is the shared
// secret that administrators must present to authenticate. The auditLog
// parameter may be nil to disable audit recording.
func NewAdminSvc(adminSecret string, tknSvc *token.TknSvc, st *store.SqlStore, al *audit.AuditLog) *AdminSvc {
	return &AdminSvc{
		adminSecret: []byte(adminSecret),
		tknSvc:      tknSvc,
		store:       st,
		auditLog:    al,
	}
}

// Authenticate validates the client secret using constant-time comparison
// (preventing timing attacks) and issues a short-lived admin JWT with
// the full admin scope set. It returns [ErrInvalidSecret] on mismatch.
func (s *AdminSvc) Authenticate(clientID, clientSecret string) (*token.IssueResp, error) {
	secretBytes := []byte(clientSecret)

	// Constant-time comparison to prevent timing attacks (Security Invariant #8).
	if subtle.ConstantTimeCompare(secretBytes, s.adminSecret) != 1 {
		obs.AdminAuthTotal.WithLabelValues("failure").Inc()
		obs.Warn(mod, cmp, "authentication failed", "client_id="+clientID)
		if s.auditLog != nil {
			s.auditLog.Record(audit.EventAdminAuthFailed, "", "", "",
				fmt.Sprintf("failed authentication attempt for client_id=%s", clientID))
		}
		return nil, ErrInvalidSecret
	}

	resp, err := s.tknSvc.Issue(token.IssueReq{
		Sub:   adminSub,
		Scope: adminScope,
		TTL:   adminTTL,
	})
	if err != nil {
		obs.Fail(mod, cmp, "failed to issue admin token", "err="+err.Error())
		return nil, fmt.Errorf("issue admin token: %w", err)
	}

	obs.AdminAuthTotal.WithLabelValues("success").Inc()
	obs.Ok(mod, cmp, "admin authenticated", "client_id="+clientID)
	if s.auditLog != nil {
		s.auditLog.Record(audit.EventAdminAuth, "", "", "",
			fmt.Sprintf("admin authenticated as %s", adminSub))
	}

	return resp, nil
}

// CreateLaunchToken generates a cryptographically random opaque launch
// token and binds it to the given policy (scope ceiling, max TTL,
// single-use flag). The createdBy parameter is the subject of the admin
// who issued the request (for audit purposes).
func (s *AdminSvc) CreateLaunchToken(req CreateLaunchTokenReq, createdBy string) (*CreateLaunchTokenResp, error) {
	if req.AgentName == "" {
		return nil, ErrAgentNameEmpty
	}
	if len(req.AllowedScope) == 0 {
		return nil, ErrScopeEmpty
	}

	maxTTL := req.MaxTTL
	if maxTTL <= 0 {
		maxTTL = defaultMaxTTL
	}
	ttl := req.TTL
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}
	singleUse := true
	if req.SingleUse != nil {
		singleUse = *req.SingleUse
	}

	tokenBytes := make([]byte, launchTokenBytes)
	if _, err := rand.Read(tokenBytes); err != nil {
		obs.Fail(mod, cmp, "failed to generate launch token", "err="+err.Error())
		return nil, fmt.Errorf("generate launch token: %w", err)
	}
	tokenStr := hex.EncodeToString(tokenBytes)

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(ttl) * time.Second)

	rec := &store.LaunchTokenRecord{
		Token:        tokenStr,
		AgentName:    req.AgentName,
		AllowedScope: req.AllowedScope,
		MaxTTL:       maxTTL,
		SingleUse:    singleUse,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		CreatedBy:    createdBy,
	}

	if err := s.store.SaveLaunchToken(*rec); err != nil {
		obs.Fail(mod, cmp, "failed to save launch token", "err="+err.Error())
		return nil, fmt.Errorf("save launch token: %w", err)
	}

	obs.LaunchTokensCreatedTotal.Inc()
	obs.Ok(mod, cmp, "launch token created",
		"agent_name="+req.AgentName,
		"scope="+fmt.Sprintf("%v", req.AllowedScope),
	)
	if s.auditLog != nil {
		s.auditLog.Record(audit.EventLaunchTokenIssued, "", "", "",
			fmt.Sprintf("launch token issued for agent=%s scope=%v max_ttl=%d created_by=%s",
				req.AgentName, req.AllowedScope, maxTTL, createdBy))
	}

	return &CreateLaunchTokenResp{
		LaunchToken: tokenStr,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
		Policy: LaunchTokenPolicy{
			AllowedScope: req.AllowedScope,
			MaxTTL:       maxTTL,
		},
	}, nil
}

// ValidateLaunchToken looks up a launch token and checks that it has not
// expired or been consumed. It does NOT consume the token; call
// [AdminSvc.ConsumeLaunchToken] after successful registration.
func (s *AdminSvc) ValidateLaunchToken(tokenStr string) (*store.LaunchTokenRecord, error) {
	return s.store.GetLaunchToken(tokenStr)
}

// ConsumeLaunchToken marks a single-use launch token as consumed by
// setting its ConsumedAt timestamp. Multi-use tokens are left unchanged.
// Returns [store.ErrTokenNotFound] if the token does not exist.
func (s *AdminSvc) ConsumeLaunchToken(tokenStr string) error {
	rec, err := s.store.GetLaunchToken(tokenStr)
	if err != nil {
		return err
	}
	if !rec.SingleUse {
		return nil
	}
	return s.store.ConsumeLaunchToken(tokenStr)
}
