// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/devonartis/agentwrit/internal/audit"
	"github.com/devonartis/agentwrit/internal/authz"
	"github.com/devonartis/agentwrit/internal/obs"
	"github.com/devonartis/agentwrit/internal/problemdetails"
	"github.com/devonartis/agentwrit/internal/revoke"
	"github.com/devonartis/agentwrit/internal/store"
)

const hdlCmp = "AdminHdl"

// AdminHdl serves admin auth and launch token creation. Two separate routes
// hit the same handleCreateLaunchToken handler because both admin and apps
// can create launch tokens — but with different trust levels. Admin callers
// have no scope ceiling (they're the operator). App callers are constrained
// by the app's scope ceiling set at registration, enforced in the handler
// via authz.ScopeIsSubset. This is the key security boundary for the
// app→agent delegation chain (see TD-013 for the admin bypass design question).
type AdminHdl struct {
	adminSvc    *AdminSvc
	valMw       *authz.ValMw
	revSvc      *revoke.RevSvc
	auditLog    *audit.AuditLog
	rateLimiter *authz.RateLimiter
	store       *store.SqlStore // for app ceiling lookups
}

// NewAdminHdl creates a new admin handler. The valMw is used to protect
// the launch-token creation endpoint with Bearer token authentication.
// A rate limiter is applied to the admin auth endpoint to prevent brute
// force attacks. The auditLog parameter may be nil to disable audit
// recording. The revSvc parameter may be nil if revocation is not needed.
// The st parameter provides app record access for scope ceiling
// enforcement when app-authenticated callers create launch tokens.
func NewAdminHdl(adminSvc *AdminSvc, valMw *authz.ValMw, auditLog *audit.AuditLog, revSvc *revoke.RevSvc, st *store.SqlStore) *AdminHdl {
	return &AdminHdl{
		adminSvc:    adminSvc,
		valMw:       valMw,
		revSvc:      revSvc,
		auditLog:    auditLog,
		rateLimiter: authz.NewRateLimiter(5, 10), // 5 req/s, burst 10
		store:       st,
	}
}

// RegisterRoutes registers the admin HTTP routes on the given [http.ServeMux].
// This method should be called once during broker startup.
func (h *AdminHdl) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /v1/admin/auth",
		h.rateLimiter.Wrap(http.HandlerFunc(h.handleAuth)))
	mux.Handle("POST /v1/admin/launch-tokens",
		h.valMw.Wrap(h.valMw.RequireScope("admin:launch-tokens:*",
			http.HandlerFunc(h.handleCreateLaunchToken))))
	mux.Handle("POST /v1/app/launch-tokens",
		h.valMw.Wrap(h.valMw.RequireScope("app:launch-tokens:*",
			http.HandlerFunc(h.handleCreateLaunchToken))))
}

// authReq is the JSON body for POST /v1/admin/auth.
type authReq struct {
	Secret string `json:"secret"`
}

// legacyAuthReq detects the old {"client_id","client_secret"} shape so we
// can return a helpful migration error.
type legacyAuthReq struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// authResp is the JSON body for a successful POST /v1/admin/auth response.
type authResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// maxBodyBytes is the maximum allowed request body size (1 MB).
const maxBodyBytes int64 = 1 << 20

// handleAuth — POST /v1/admin/auth. The operator authenticates with the
// shared admin secret to get a short-lived JWT. Supports legacy detection
// for the old client_id/client_secret shape so callers get a helpful error
// instead of a cryptic auth failure.
func (h *AdminHdl) handleAuth(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	// Read body once so we can try both shapes.
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}

	// Detect legacy {"client_id","client_secret"} shape.
	var legacy legacyAuthReq
	if json.Unmarshal(raw, &legacy) == nil && (legacy.ClientID != "" || legacy.ClientSecret != "") {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request",
			`Admin auth format changed. Use {"secret": "..."} instead of client_id/client_secret`, r.URL.Path)
		return
	}

	var req authReq
	if err := json.Unmarshal(raw, &req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}

	if req.Secret == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "secret is required", r.URL.Path)
		return
	}

	issueResp, err := h.adminSvc.Authenticate(req.Secret)
	if err != nil {
		obs.Warn(mod, hdlCmp, "auth failed")
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "invalid credentials", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(authResp{
		AccessToken: issueResp.AccessToken,
		ExpiresIn:   issueResp.ExpiresIn,
		TokenType:   "Bearer",
	}); err != nil {
		obs.Warn(mod, hdlCmp, "failed to encode auth response", "err="+err.Error())
	}
}

// handleCreateLaunchToken — POST /v1/admin/launch-tokens (admin) and
// POST /v1/app/launch-tokens (app). Same handler, different trust: admin
// callers pass through unconstrained, app callers have their requested
// scopes checked against the app's scope ceiling. If an app tries to
// create a launch token exceeding its ceiling, we deny and audit it.
func (h *AdminHdl) handleCreateLaunchToken(w http.ResponseWriter, r *http.Request) {
	claims := authz.ClaimsFromContext(r.Context())
	if claims == nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "missing authentication", r.URL.Path)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req CreateLaunchTokenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}

	var appID string

	// If caller is an app, enforce scope ceiling.
	if strings.HasPrefix(claims.Sub, "app:") {
		appID = strings.TrimPrefix(claims.Sub, "app:")
		appRec, err := h.store.GetAppByID(appID)
		if err != nil {
			problemdetails.WriteProblem(r.Context(), w, http.StatusForbidden, "forbidden", "app not found", r.URL.Path)
			return
		}
		if !authz.ScopeIsSubset(req.AllowedScope, appRec.ScopeCeiling) {
			if h.auditLog != nil {
				h.auditLog.Record(audit.EventScopeCeilingExceeded, claims.Sub, "", "",
					fmt.Sprintf("app=%s requested=%v ceiling=%v", appID, req.AllowedScope, appRec.ScopeCeiling),
					audit.WithOutcome("denied"))
			}
			problemdetails.WriteProblem(r.Context(), w, http.StatusForbidden, "forbidden",
				fmt.Sprintf("requested scopes exceed app ceiling; allowed: %v", appRec.ScopeCeiling),
				r.URL.Path)
			return
		}
	}

	resp, err := h.adminSvc.CreateLaunchToken(req, claims.Sub, appID)
	if err != nil {
		if h.auditLog != nil {
			h.auditLog.Record(audit.EventLaunchTokenDenied, claims.Sub, "", "",
				fmt.Sprintf("launch token denied for agent=%s by=%s reason=%s",
					req.AgentName, claims.Sub, err.Error()),
				audit.WithOutcome("denied"))
		}
		switch {
		case errors.Is(err, ErrAgentNameEmpty):
			problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "agent_name is required", r.URL.Path)
		case errors.Is(err, ErrScopeEmpty):
			problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "allowed_scope must not be empty", r.URL.Path)
		default:
			obs.Fail(mod, hdlCmp, "launch token creation failed", "err="+err.Error())
			problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to create launch token", r.URL.Path)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		obs.Warn(mod, hdlCmp, "failed to encode launch token response", "err="+err.Error())
	}
}
