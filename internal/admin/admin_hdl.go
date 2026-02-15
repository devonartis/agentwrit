package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/problemdetails"
)

const hdlCmp = "AdminHdl"

// AdminHdl is the HTTP handler for admin endpoints. It registers
// POST /v1/admin/auth (rate-limited, unauthenticated) and
// POST /v1/admin/launch-tokens (requires admin:launch-tokens:* scope
// via [authz.ValMw]).
type AdminHdl struct {
	adminSvc    *AdminSvc
	valMw       *authz.ValMw
	auditLog    *audit.AuditLog
	rateLimiter *authz.RateLimiter
}

// NewAdminHdl creates a new admin handler. The valMw is used to protect
// the launch-token creation endpoint with Bearer token authentication.
// A rate limiter is applied to the admin auth endpoint to prevent brute
// force attacks. The auditLog parameter may be nil to disable audit
// recording.
func NewAdminHdl(adminSvc *AdminSvc, valMw *authz.ValMw, auditLog *audit.AuditLog) *AdminHdl {
	return &AdminHdl{
		adminSvc:    adminSvc,
		valMw:       valMw,
		auditLog:    auditLog,
		rateLimiter: authz.NewRateLimiter(5, 10), // 5 req/s, burst 10
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
	mux.Handle("POST /v1/admin/sidecar-activations",
		h.valMw.Wrap(h.valMw.RequireScope("admin:launch-tokens:*",
			http.HandlerFunc(h.handleCreateSidecarActivation))))
	mux.Handle("POST /v1/sidecar/activate",
		h.rateLimiter.Wrap(http.HandlerFunc(h.handleActivateSidecar)))
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

	resp, err := h.adminSvc.CreateLaunchToken(req, claims.Sub)
	if err != nil {
		if h.auditLog != nil {
			h.auditLog.Record(audit.EventLaunchTokenDenied, claims.Sub, "", "",
				fmt.Sprintf("launch token denied for agent=%s by=%s reason=%s",
					req.AgentName, claims.Sub, err.Error()))
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
				fmt.Sprintf("sidecar activation token creation denied: %v", err))
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
				fmt.Sprintf("sidecar activation failed: %v", err))
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
