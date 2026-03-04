// Package app provides the AppSvc service for app registration and
// authentication. Apps are first-class entities with their own scoped
// credentials (client_id + client_secret) that authenticate directly
// with the broker without requiring the admin master key.
package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/store"
	"github.com/divineartis/agentauth/internal/token"
)

const appTokenTTL = 300 // 5 minutes, matches admin token TTL

// Sentinel errors returned by AppSvc. Callers use errors.Is to match.
var (
	// ErrInvalidCredentials is returned by AuthenticateApp for any auth
	// failure — unknown client_id, wrong secret, or inactive app.
	// A single sentinel prevents credential enumeration.
	ErrInvalidCredentials = errors.New("invalid client credentials")

	// ErrInvalidAppName is returned when the app name fails format validation.
	ErrInvalidAppName = errors.New("invalid app name")

	// ErrInvalidScopeFormat is returned when a scope string is malformed.
	ErrInvalidScopeFormat = errors.New("invalid scope format")
)

// appNameRe matches valid app names: lowercase letters/digits/hyphens,
// must start with a letter, no consecutive hyphens, max 64 chars.
var appNameRe = regexp.MustCompile(`^[a-z][a-z0-9](?:-?[a-z0-9]+)*$`)

// AppSvc handles app registration and authentication business logic.
type AppSvc struct {
	store    *store.SqlStore
	tknSvc   *token.TknSvc
	auditLog *audit.AuditLog
	audience string
}

// RegisterAppResp is returned by RegisterApp. The ClientSecret is the
// plaintext secret — it is only returned here and never stored unencrypted.
type RegisterAppResp struct {
	AppID        string   `json:"app_id"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	ScopeCeiling []string `json:"scopes"`
}

// NewAppSvc returns an AppSvc wired with the given dependencies.
// auditLog may be nil to disable audit recording.
func NewAppSvc(st *store.SqlStore, tknSvc *token.TknSvc, al *audit.AuditLog, audience string) *AppSvc {
	return &AppSvc{
		store:    st,
		tknSvc:   tknSvc,
		auditLog: al,
		audience: audience,
	}
}

// RegisterApp creates a new app with generated credentials. The plaintext
// client_secret is returned exactly once in RegisterAppResp and never stored.
func (s *AppSvc) RegisterApp(name string, scopes []string, createdBy string) (*RegisterAppResp, error) {
	if err := validateAppName(name); err != nil {
		return nil, err
	}
	if err := validateScopes(scopes); err != nil {
		return nil, err
	}

	appID := "app-" + name + "-" + randomHex(3)
	abbrev := appAbbrev(name)
	clientID := abbrev + "-" + randomHex(6)
	secret := randomHex(32) // 64-char hex string

	hash, err := bcrypt.GenerateFromPassword([]byte(secret), 12)
	if err != nil {
		return nil, fmt.Errorf("hash client secret: %w", err)
	}

	now := time.Now().UTC()
	rec := store.AppRecord{
		AppID:            appID,
		Name:             name,
		ClientID:         clientID,
		ClientSecretHash: string(hash),
		ScopeCeiling:     scopes,
		Status:           "active",
		CreatedAt:        now,
		UpdatedAt:        now,
		CreatedBy:        createdBy,
	}

	if err := s.store.SaveApp(rec); err != nil {
		return nil, fmt.Errorf("save app: %w", err)
	}

	s.record(audit.EventAppRegistered, "",
		fmt.Sprintf("app=%s client_id=%s scopes=%v", name, clientID, scopes),
		audit.WithOutcome("success"))

	return &RegisterAppResp{
		AppID:        appID,
		ClientID:     clientID,
		ClientSecret: secret,
		ScopeCeiling: scopes,
	}, nil
}

// AuthenticateApp validates client_id + client_secret and issues a scoped JWT.
// Returns ErrInvalidCredentials for any auth failure (unknown id, wrong
// secret, or inactive app) to prevent credential enumeration.
func (s *AppSvc) AuthenticateApp(clientID, clientSecret string) (*token.IssueResp, error) {
	rec, err := s.store.GetAppByClientID(clientID)
	if errors.Is(err, store.ErrAppNotFound) {
		s.record(audit.EventAppAuthFailed, "",
			fmt.Sprintf("client_id=%s reason=unknown_client_id", clientID),
			audit.WithOutcome("denied"))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("lookup app: %w", err)
	}

	if rec.Status != "active" {
		s.record(audit.EventAppAuthFailed, "",
			fmt.Sprintf("client_id=%s reason=app_inactive", clientID),
			audit.WithOutcome("denied"))
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(rec.ClientSecretHash), []byte(clientSecret)); err != nil {
		s.record(audit.EventAppAuthFailed, "",
			fmt.Sprintf("client_id=%s reason=wrong_secret", clientID),
			audit.WithOutcome("denied"))
		return nil, ErrInvalidCredentials
	}

	aud := []string{}
	if s.audience != "" {
		aud = []string{s.audience}
	}

	resp, err := s.tknSvc.Issue(token.IssueReq{
		Sub:   "app:" + rec.AppID,
		Aud:   aud,
		Scope: []string{"app:launch-tokens:*", "app:agents:*", "app:audit:read"},
		TTL:   appTokenTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}

	s.record(audit.EventAppAuthenticated, "",
		fmt.Sprintf("client_id=%s app_id=%s", clientID, rec.AppID),
		audit.WithOutcome("success"))

	return resp, nil
}

// ListApps returns all registered apps.
func (s *AppSvc) ListApps() ([]store.AppRecord, error) {
	return s.store.ListApps()
}

// GetApp returns a single app by ID. Returns store.ErrAppNotFound if not found.
func (s *AppSvc) GetApp(appID string) (*store.AppRecord, error) {
	return s.store.GetAppByID(appID)
}

// UpdateApp replaces the scope ceiling for an existing app.
func (s *AppSvc) UpdateApp(appID string, newScopes []string, updatedBy string) error {
	if err := validateScopes(newScopes); err != nil {
		return err
	}
	if err := s.store.UpdateAppCeiling(appID, newScopes); err != nil {
		return err
	}
	s.record(audit.EventAppUpdated, "",
		fmt.Sprintf("app_id=%s scopes=%v updated_by=%s", appID, newScopes, updatedBy),
		audit.WithOutcome("success"))
	return nil
}

// DeregisterApp marks an app as inactive. Its credentials stop working
// immediately. The record is retained (soft delete).
func (s *AppSvc) DeregisterApp(appID string, deregisteredBy string) error {
	rec, err := s.store.GetAppByID(appID)
	if err != nil {
		return err
	}
	if err := s.store.UpdateAppStatus(appID, "inactive"); err != nil {
		return err
	}
	s.record(audit.EventAppDeregistered, "",
		fmt.Sprintf("app_id=%s name=%s deregistered_by=%s", appID, rec.Name, deregisteredBy),
		audit.WithOutcome("success"))
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func (s *AppSvc) record(eventType, agentID, detail string, opts ...audit.RecordOption) {
	if s.auditLog == nil {
		return
	}
	s.auditLog.Record(eventType, agentID, "", "", detail, opts...)
}

func validateAppName(name string) error {
	if name == "" || len(name) > 64 || !appNameRe.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidAppName, name)
	}
	return nil
}

func validateScopes(scopes []string) error {
	for _, sc := range scopes {
		if _, _, _, err := authz.ParseScope(sc); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidScopeFormat, sc)
		}
	}
	return nil
}

// randomHex returns n random bytes encoded as a hex string (2n chars).
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// appAbbrev returns a 2-3 character abbreviation for use in client_id.
func appAbbrev(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) == 1 {
		if len(name) >= 2 {
			return name[:2]
		}
		return name
	}
	abbrev := ""
	for _, p := range parts {
		if len(p) > 0 {
			abbrev += string(p[0])
		}
		if len(abbrev) >= 3 {
			break
		}
	}
	return abbrev
}
