package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/obs"
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
		h.valMw.Wrap(authz.WithRequiredScope("admin:launch-tokens:*",
			http.HandlerFunc(h.handleCreateLaunchToken))))
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
		writeProblem(w, http.StatusBadRequest, "invalid_request",
			"Invalid Request", "malformed JSON body", r.URL.Path)
		return
	}

	if req.ClientID == "" || req.ClientSecret == "" {
		writeProblem(w, http.StatusBadRequest, "invalid_request",
			"Invalid Request", "client_id and client_secret are required", r.URL.Path)
		return
	}

	issueResp, err := h.adminSvc.Authenticate(req.ClientID, req.ClientSecret)
	if err != nil {
		obs.Warn(mod, hdlCmp, "auth failed", "client_id="+req.ClientID)
		writeProblem(w, http.StatusUnauthorized, "unauthorized",
			"Unauthorized", "invalid credentials", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(authResp{
		AccessToken: issueResp.AccessToken,
		ExpiresIn:   issueResp.ExpiresIn,
		TokenType:   "Bearer",
	})
}

func (h *AdminHdl) handleCreateLaunchToken(w http.ResponseWriter, r *http.Request) {
	claims := authz.ClaimsFromContext(r.Context())
	if claims == nil {
		writeProblem(w, http.StatusUnauthorized, "unauthorized",
			"Unauthorized", "missing authentication", r.URL.Path)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req CreateLaunchTokenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request",
			"Invalid Request", "malformed JSON body", r.URL.Path)
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
			writeProblem(w, http.StatusBadRequest, "invalid_request",
				"Invalid Request", "agent_name is required", r.URL.Path)
		case errors.Is(err, ErrScopeEmpty):
			writeProblem(w, http.StatusBadRequest, "invalid_request",
				"Invalid Request", "allowed_scope must not be empty", r.URL.Path)
		default:
			obs.Fail(mod, hdlCmp, "launch token creation failed", "err="+err.Error())
			writeProblem(w, http.StatusInternalServerError, "internal_error",
				"Internal Error", "failed to create launch token", r.URL.Path)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// writeProblem writes an RFC 7807 application/problem+json response.
func writeProblem(w http.ResponseWriter, status int, errType, title, detail, instance string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"type":     "urn:agentauth:error:" + errType,
		"title":    title,
		"status":   status,
		"detail":   detail,
		"instance": instance,
	})
}
