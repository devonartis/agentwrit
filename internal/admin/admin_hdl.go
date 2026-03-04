package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/store"
)

const hdlCmp = "AdminHdl"

// AdminHdl is the HTTP handler for admin endpoints. It registers
// POST /v1/admin/auth (rate-limited, unauthenticated) and
// POST /v1/admin/launch-tokens (requires admin:launch-tokens:* scope
// via [authz.ValMw]).
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
		h.valMw.Wrap(h.valMw.RequireAnyScope(
			[]string{"admin:launch-tokens:*", "app:launch-tokens:*"},
			http.HandlerFunc(h.handleCreateLaunchToken))))
	mux.Handle("POST /v1/admin/sidecar-activations",
		h.valMw.Wrap(h.valMw.RequireScope("admin:launch-tokens:*",
			http.HandlerFunc(h.handleCreateSidecarActivation))))
	mux.Handle("POST /v1/sidecar/activate",
		h.rateLimiter.Wrap(http.HandlerFunc(h.handleActivateSidecar)))
	mux.Handle("GET /v1/admin/sidecars/{id}/ceiling",
		h.valMw.Wrap(h.valMw.RequireScope("admin:launch-tokens:*",
			http.HandlerFunc(h.handleGetCeiling))))
	mux.Handle("PUT /v1/admin/sidecars/{id}/ceiling",
		h.valMw.Wrap(h.valMw.RequireScope("admin:launch-tokens:*",
			http.HandlerFunc(h.handleUpdateCeiling))))
	mux.Handle("GET /v1/admin/sidecars",
		h.valMw.Wrap(h.valMw.RequireScope("admin:launch-tokens:*",
			http.HandlerFunc(h.handleListSidecars))))
}

// authReq is the JSON body for POST /v1/admin/auth.
type authReq struct {
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

func (h *AdminHdl) handleAuth(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}

	if req.ClientID == "" || req.ClientSecret == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "client_id and client_secret are required", r.URL.Path)
		return
	}

	issueResp, err := h.adminSvc.Authenticate(req.ClientID, req.ClientSecret)
	if err != nil {
		obs.Warn(mod, hdlCmp, "auth failed", "client_id="+req.ClientID)
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

func (h *AdminHdl) handleCreateSidecarActivation(w http.ResponseWriter, r *http.Request) {
	claims := authz.ClaimsFromContext(r.Context())
	if claims == nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "missing authentication", r.URL.Path)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req CreateSidecarActivationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}

	resp, err := h.adminSvc.CreateSidecarActivationToken(req, claims.Sub)
	if err != nil {
		if h.auditLog != nil {
			h.auditLog.Record(audit.EventSidecarActivationFailed, claims.Sub, "", "",
				fmt.Sprintf("sidecar activation token creation denied: %v", err),
				audit.WithOutcome("denied"))
		}
		switch {
		case errors.Is(err, ErrActivationScopeEmpty):
			problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "allowed_scopes is required", r.URL.Path)
		default:
			problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to create sidecar activation token", r.URL.Path)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		obs.Warn(mod, hdlCmp, "failed to encode sidecar activation response", "err="+err.Error())
	}
}

func (h *AdminHdl) handleActivateSidecar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req ActivateSidecarReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}

	resp, err := h.adminSvc.ActivateSidecar(req)
	if err != nil {
		if !errors.Is(err, ErrActivationTokenReplayed) && h.auditLog != nil {
			h.auditLog.Record(audit.EventSidecarActivationFailed, "", "", "",
				fmt.Sprintf("sidecar activation failed: %v", err),
				audit.WithOutcome("denied"))
		}
		switch {
		case errors.Is(err, ErrActivationTokenReplayed):
			problemdetails.WriteProblemExtended(r.Context(), w, http.StatusUnauthorized, "unauthorized", "activation token replay detected", r.URL.Path, "activation_token_replayed", "")
		case errors.Is(err, ErrActivationTokenInvalid):
			problemdetails.WriteProblemExtended(r.Context(), w, http.StatusUnauthorized, "unauthorized", "invalid activation token", r.URL.Path, "invalid_activation_token", "")
		default:
			problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "sidecar activation failed", r.URL.Path)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		obs.Warn(mod, hdlCmp, "failed to encode activate sidecar response", "err="+err.Error())
	}
}

// ceilingResp is the JSON response for GET /v1/admin/sidecars/{id}/ceiling.
type ceilingResp struct {
	SidecarID    string   `json:"sidecar_id"`
	ScopeCeiling []string `json:"scope_ceiling"`
}

// listSidecarsResp is the JSON response for GET /v1/admin/sidecars.
type listSidecarsResp struct {
	Sidecars []sidecarEntry `json:"sidecars"`
	Total    int            `json:"total"`
}

type sidecarEntry struct {
	SidecarID    string   `json:"sidecar_id"`
	ScopeCeiling []string `json:"scope_ceiling"`
	Status       string   `json:"status"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

func (h *AdminHdl) handleGetCeiling(w http.ResponseWriter, r *http.Request) {
	sidecarID := r.PathValue("id")
	if sidecarID == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "sidecar id is required", r.URL.Path)
		return
	}

	ceiling, err := h.adminSvc.GetSidecarCeiling(sidecarID)
	if err != nil {
		if errors.Is(err, store.ErrCeilingNotFound) {
			problemdetails.WriteProblem(r.Context(), w, http.StatusNotFound, "not_found", "sidecar ceiling not found", r.URL.Path)
			return
		}
		problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to get sidecar ceiling", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ceilingResp{
		SidecarID:    sidecarID,
		ScopeCeiling: ceiling,
	}); err != nil {
		obs.Warn(mod, hdlCmp, "failed to encode ceiling response", "err="+err.Error())
	}
}

func (h *AdminHdl) handleListSidecars(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	sidecars, err := h.adminSvc.ListSidecars()
	if err != nil {
		obs.Fail(mod, hdlCmp, "list sidecars failed", "err="+err.Error())
		problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to list sidecars", r.URL.Path)
		return
	}

	entries := make([]sidecarEntry, len(sidecars))
	for i, sc := range sidecars {
		entries[i] = sidecarEntry{
			SidecarID:    sc.ID,
			ScopeCeiling: sc.Ceiling,
			Status:       sc.Status,
			CreatedAt:    sc.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    sc.UpdatedAt.Format(time.RFC3339),
		}
	}

	obs.SidecarListDuration.Observe(time.Since(start).Seconds())
	obs.Ok(mod, hdlCmp, "listed sidecars", fmt.Sprintf("count=%d", len(entries)))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(listSidecarsResp{
		Sidecars: entries,
		Total:    len(entries),
	}); err != nil {
		obs.Warn(mod, hdlCmp, "failed to encode list sidecars response", "err="+err.Error())
	}
}

// updateCeilingReq is the JSON body for PUT /v1/admin/sidecars/{id}/ceiling.
type updateCeilingReq struct {
	ScopeCeiling []string `json:"scope_ceiling"`
}

func (h *AdminHdl) handleUpdateCeiling(w http.ResponseWriter, r *http.Request) {
	claims := authz.ClaimsFromContext(r.Context())
	if claims == nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "missing authentication", r.URL.Path)
		return
	}

	sidecarID := r.PathValue("id")
	if sidecarID == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "sidecar id is required", r.URL.Path)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req updateCeilingReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}

	if len(req.ScopeCeiling) == 0 {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "scope_ceiling must not be empty", r.URL.Path)
		return
	}

	result, err := h.adminSvc.UpdateSidecarCeiling(sidecarID, req.ScopeCeiling, claims.Sub)
	if err != nil {
		if errors.Is(err, ErrInvalidScopeFormat) {
			problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request",
				"invalid scope format: "+strings.TrimPrefix(err.Error(), ErrInvalidScopeFormat.Error()+": "), r.URL.Path)
			return
		}
		obs.Fail(mod, hdlCmp, "ceiling update failed", "err="+err.Error())
		problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to update sidecar ceiling", r.URL.Path)
		return
	}

	// When the ceiling narrows, revoke the sidecar's token so agents
	// re-authenticate under the tighter ceiling.
	if result.Narrowed && h.revSvc != nil {
		target := "sidecar:" + sidecarID
		count, revErr := h.revSvc.Revoke("agent", target)
		if revErr != nil {
			obs.Fail(mod, hdlCmp, "revocation after ceiling narrowing failed",
				"sidecar_id="+sidecarID, "err="+revErr.Error())
		} else {
			result.Revoked = true
			result.RevokedCount = count
			obs.Ok(mod, hdlCmp, "revoked sidecar token after ceiling narrowing",
				"sidecar_id="+sidecarID, fmt.Sprintf("revoked_count=%d", count))
			if h.auditLog != nil {
				h.auditLog.Record(
					audit.EventTokenRevoked,
					target,
					"",
					"",
					fmt.Sprintf("auto-revoked on ceiling narrowing sidecar=%s by=%s", sidecarID, claims.Sub),
					audit.WithOutcome("success"),
				)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		obs.Warn(mod, hdlCmp, "failed to encode ceiling update response", "err="+err.Error())
	}
}
