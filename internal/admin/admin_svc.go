// Package admin handles the operator side of the broker — authenticating
// with the admin secret and managing launch tokens. The operator is the
// human or automation that bootstraps the system: they register apps,
// create launch tokens for testing/dev, and have oversight over revocation
// and audit.
//
// Launch tokens are how agents get their first credential. An app (or
// admin, for dev/testing) creates a launch token with a scope ceiling and
// max TTL. The agent presents this token during registration to receive
// its short-lived JWT. The launch token is the trust anchor — it defines
// the upper bound of what the agent can do.
package admin

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/devonartis/agentauth/internal/audit"
	"github.com/devonartis/agentauth/internal/obs"
	"github.com/devonartis/agentauth/internal/store"
	"github.com/devonartis/agentauth/internal/token"
	"golang.org/x/crypto/bcrypt"
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

// AdminSvc handles admin auth (shared secret → short-lived JWT) and launch
// token lifecycle (create, validate, consume). Launch tokens are stored in
// SqlStore so the identity service can look them up during agent registration
// without depending on this package.
type AdminSvc struct {
	adminSecretHash []byte
	tknSvc          *token.TknSvc
	store           *store.SqlStore
	auditLog        *audit.AuditLog
	audience        string
}

// NewAdminSvc creates a new admin service. The adminSecretHash is the
// bcrypt hash of the admin secret, derived at config load time. The
// auditLog parameter may be nil to disable audit recording. The audience
// is populated into all issued tokens.
func NewAdminSvc(adminSecretHash string, tknSvc *token.TknSvc, st *store.SqlStore, al *audit.AuditLog, audience string) *AdminSvc {
	return &AdminSvc{
		adminSecretHash: []byte(adminSecretHash),
		tknSvc:          tknSvc,
		store:           st,
		auditLog:        al,
		audience:        audience,
	}
}

// audienceSlice returns the audience as a single-element slice, or nil
// when no audience is configured.
func (s *AdminSvc) audienceSlice() []string {
	if s.audience == "" {
		return nil
	}
	return []string{s.audience}
}

// Authenticate validates the admin secret against the stored bcrypt hash
// and issues a short-lived admin JWT with the full admin scope set.
// It returns [ErrInvalidSecret] on mismatch.
func (s *AdminSvc) Authenticate(secret string) (*token.IssueResp, error) {
	if err := bcrypt.CompareHashAndPassword(s.adminSecretHash, []byte(secret)); err != nil {
		obs.AdminAuthTotal.WithLabelValues("failure").Inc()
		obs.Warn(mod, cmp, "authentication failed")
		if s.auditLog != nil {
			s.auditLog.Record(audit.EventAdminAuthFailed, "", "", "",
				"failed admin authentication attempt",
				audit.WithOutcome("denied"))
		}
		return nil, ErrInvalidSecret
	}

	resp, err := s.tknSvc.Issue(token.IssueReq{
		Sub:   adminSub,
		Aud:   s.audienceSlice(),
		Scope: adminScope,
		TTL:   adminTTL,
	})
	if err != nil {
		obs.Fail(mod, cmp, "failed to issue admin token", "err="+err.Error())
		return nil, fmt.Errorf("issue admin token: %w", err)
	}

	obs.AdminAuthTotal.WithLabelValues("success").Inc()
	obs.Ok(mod, cmp, "admin authenticated")
	if s.auditLog != nil {
		s.auditLog.Record(audit.EventAdminAuth, "", "", "",
			fmt.Sprintf("admin authenticated as %s", adminSub),
			audit.WithOutcome("success"))
	}

	return resp, nil
}

// CreateLaunchToken generates a launch token bound to a policy (scope
// ceiling, max TTL, single-use). Both admin and apps call this — admin
// directly for dev/testing, apps in production for their agents. When
// appID is non-empty, the token is associated with the creating app so
// the full chain (App → Launch Token → Agent) is traceable in audit.
// Scope ceiling enforcement for app callers happens in the handler layer
// (AdminHdl.handleCreateLaunchToken), not here — this function trusts
// its callers to have already validated scopes.
func (s *AdminSvc) CreateLaunchToken(req CreateLaunchTokenReq, createdBy, appID string) (*CreateLaunchTokenResp, error) {
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
		AppID:        appID,
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
		detail := fmt.Sprintf("launch token issued for agent=%s scope=%v max_ttl=%d created_by=%s",
			req.AgentName, req.AllowedScope, maxTTL, createdBy)
		if appID != "" {
			detail += fmt.Sprintf(" app_id=%s", appID)
		}
		s.auditLog.Record(audit.EventLaunchTokenIssued, "", "", "",
			detail,
			audit.WithOutcome("success"))
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
